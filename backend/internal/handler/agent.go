package handler

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/llm"
	"github.com/songhuang/flowpartner/backend/internal/sanitize"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/internal/tools"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentHandler struct {
	proto.UnimplementedFlowPartnerServiceServer
	manager         *bridge.Manager
	llmClient       *llm.LLMClient
	approvalManager *tools.ApprovalManager
}

// NewAgentHandler 创建 AgentHandler。approvalManager 用于越权审批流程。
func NewAgentHandler(m *bridge.Manager, am *tools.ApprovalManager) *AgentHandler {
	return &AgentHandler{
		manager:         m,
		llmClient:       llm.NewClient(),
		approvalManager: am,
	}
}

func (h *AgentHandler) SyncChannel(stream proto.FlowPartnerService_SyncChannelServer) error {
	log.Println("Agent gRPC bidirectional stream established")

	go func() {
		for {
			select {
			case cmd := <-h.manager.CmdChan:
				sendDone := make(chan error, 1)
				go func() {
					sendDone <- stream.Send(cmd)
				}()
				select {
				case err := <-sendDone:
					if err != nil {
						log.Printf("Failed to send command to Python: %s", sanitize.Error(err))
						return
					}
				case <-stream.Context().Done():
					return
				case <-time.After(30 * time.Second):
					log.Println("Sending command to Python timed out, exiting goroutine")
					return
				}
			case <-stream.Context().Done():
				return
			}
		}
	}()

	for {
		event, err := stream.Recv()
		if err == io.EOF {
			log.Println("Agent disconnected bidirectional stream")
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive event: %s", sanitize.Error(err))
		}

		h.manager.SendToSession(event.SessionId, event)
	}
}

// CallLLM 服务端流式 RPC：解析 Python 请求 → 合并配置 → 调用 LLM → 逐 chunk 返回
func (h *AgentHandler) CallLLM(req *proto.LLMRequest, stream proto.FlowPartnerService_CallLLMServer) error {
	log.Printf("[CallLLM] Session: %s, Payload length: %d", req.SessionId, len(req.JsonPayload))

	messageID := uuid.NewString()

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(req.JsonPayload), &payload); err != nil {
		return h.sendError(stream, messageID, llm.InvalidJSONError())
	}

	messages, ok := payload["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return h.sendError(stream, messageID, llm.MessagesEmptyError())
	}

	settings := LoadSettings()
	cfg := settings.activeConfig()
	if cfg == nil {
		return h.sendError(stream, messageID, &llm.LLMError{
			Code:    4002,
			Message: "没有激活的模型配置",
			Guess:   "请先在设置中添加并激活模型配置",
		})
	}

	ks := keystore.Instance()
	apiKey, ok := ks.GetKey()
	if !ok {
		return h.sendError(stream, messageID, &llm.LLMError{
			Code:    4001,
			Message: "模型配置已锁定",
			Guess:   "请先在设置中解锁模型配置",
		})
	}
	keyCopy := make([]byte, len(apiKey))
	copy(keyCopy, apiKey)

	targetURL, err := llm.NormalizeChatCompletionsURL(cfg.BaseURL)
	if err != nil {
		for i := range keyCopy {
			keyCopy[i] = 0
		}
		return h.sendError(stream, messageID, &llm.LLMError{
			Code:    400,
			Message: "接口地址格式无效",
			Guess:   "请检查模型配置中的接口地址（Base URL）",
		})
	}

	var tools, toolChoice []byte
	if t, ok := payload["tools"].([]interface{}); ok {
		tools, _ = json.Marshal(t)
	}
	if tc, ok := payload["tool_choice"]; ok {
		toolChoice, _ = json.Marshal(tc)
	}

	messagesJSON, _ := json.Marshal(payload["messages"])

	streamReq := llm.StreamRequest{
		RawPayload:     []byte(req.JsonPayload),
		Messages:       messagesJSON,
		Tools:          tools,
		ToolChoice:     toolChoice,
		Model:          cfg.ModelName,
		Temperature:    cfg.Temperature,
		ResponseFormat: cfg.ResponseFormat,
		APIKey:         keyCopy,
		TargetURL:      targetURL,
		Timeout:        time.Duration(cfg.TimeoutSecs) * time.Second,
	}

	chunkChan, err := h.llmClient.Stream(stream.Context(), streamReq)
	if err != nil {
		for i := range keyCopy {
			keyCopy[i] = 0
		}
		return h.sendError(stream, messageID, llm.NetworkError(err))
	}

	for chunk := range chunkChan {
		if chunk.Done && chunk.Data == "" {
			continue
		}
		resp := &proto.LLMResponse{
			IsError:      chunk.Done && chunk.Data != "",
			JsonResponse: chunk.Data,
			MessageId:    messageID,
		}
		if err := stream.Send(resp); err != nil {
			log.Printf("[CallLLM] Send failed: %s", sanitize.Error(err))
			return nil
		}
	}

	return nil
}

func (h *AgentHandler) ListAgents(ctx context.Context, _ *proto.Empty) (*proto.AgentDefList, error) {
	defs, err := storage.LoadAgents()
	if err != nil {
		log.Printf("[ListAgents] 读取智能体定义失败: %s", sanitize.Error(err))
		return nil, status.Errorf(codes.Internal, "读取智能体定义失败")
	}

	main := BuiltinMainAgent()
	result := []*proto.AgentDef{agentDefToProto(main)}
	for _, d := range defs {
		result = append(result, agentDefToProto(d))
	}
	return &proto.AgentDefList{Agents: result}, nil
}

func (h *AgentHandler) GetAgent(ctx context.Context, req *proto.AgentId) (*proto.AgentDef, error) {
	if req == nil || req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "智能体 ID 不能为空")
	}
	if req.Id == mainAgentID {
		return agentDefToProto(BuiltinMainAgent()), nil
	}

	defs, err := storage.LoadAgents()
	if err != nil {
		log.Printf("[GetAgent] 读取智能体定义失败: %s", sanitize.Error(err))
		return nil, status.Errorf(codes.Internal, "读取智能体定义失败")
	}
	for _, d := range defs {
		if d.ID == req.Id {
			return agentDefToProto(d), nil
		}
	}
	return nil, status.Errorf(codes.NotFound, "智能体不存在: %s", req.Id)
}

// agentDefToProto 将存储层智能体定义转换为 proto 消息。
func agentDefToProto(def storage.AgentDef) *proto.AgentDef {
	return &proto.AgentDef{
		Id:           def.ID,
		Name:         def.Name,
		Description:  def.Description,
		SystemPrompt: def.SystemPrompt,
		CreatedAt:    def.CreatedAt,
		UpdatedAt:    def.UpdatedAt,
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (h *AgentHandler) sendError(stream proto.FlowPartnerService_CallLLMServer, messageID string, llmErr *llm.LLMError) error {
	data, _ := json.Marshal(llmErr)
	return stream.Send(&proto.LLMResponse{
		IsError:      true,
		JsonResponse: string(data),
		MessageId:    messageID,
	})
}

func (h *AgentHandler) ExecuteTool(ctx context.Context, req *proto.ToolRequest) (*proto.ToolResponse, error) {
	log.Printf("[ExecuteTool] Session: %s, Tool: %s, Args length: %d", req.SessionId, req.ToolName, len(req.Arguments))

	settings := LoadSettings()
	workingDir := settings.WorkingDirectory

	// 工作目录为空时回退到用户主目录
	if workingDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Printf("[ExecuteTool] 无法获取用户主目录: %v", err)
			return &proto.ToolResponse{
				Success:   false,
				Result:    "无法获取工作目录：未设置工作目录且无法解析用户主目录",
				ErrorCode: tools.ErrToolError,
			}, nil
		}
		workingDir = homeDir
		log.Printf("[ExecuteTool] 未设置工作目录，已回退到用户主目录: %s", workingDir)
	}

	executor, err := tools.NewToolExecutor(workingDir, tools.WithTrashDir(settings.TrashDir))
	if err != nil {
		log.Printf("[ExecuteTool] 创建工具执行器失败: %v", err)
		return &proto.ToolResponse{
			Success:   false,
			Result:    "无法创建工具执行器: " + err.Error(),
			ErrorCode: tools.ErrToolError,
		}, nil
	}

	// 审批流程：检查路径是否越权
	needsPermission, rawPath, resolvedPath, checkErr := executor.CheckPath(req.ToolName, req.Arguments)
	if checkErr != nil {
		log.Printf("[ExecuteTool] 路径检查异常: %v", checkErr)
		return &proto.ToolResponse{
			Success:   false,
			Result:    checkErr.Error(),
			ErrorCode: tools.ErrToolError,
		}, nil
	}

	if needsPermission {
		if req.ApprovalId == "" {
			if req.ToolName != "purge" && h.approvalManager.IsTrusted(req.SessionId, req.ToolName, resolvedPath) {
				log.Printf("[ExecuteTool] Session trust hit, skipping approval: tool=%s path=%s", req.ToolName, resolvedPath)
			} else {
				requestID := h.approvalManager.Create(req.SessionId, req.ToolName, rawPath, resolvedPath)
				log.Printf("[ExecuteTool] 路径越权，已创建审批请求: request_id=%s path=%s", requestID, rawPath)
				return &proto.ToolResponse{
					NeedsPermission: true,
					RequestId:       requestID,
				}, nil
			}
		} else {
			if !h.approvalManager.Consume(req.SessionId, req.ApprovalId, req.ToolName, resolvedPath) {
				return &proto.ToolResponse{
					Success:   false,
					Result:    "审批无效：审批记录不存在、已过期、参数不匹配或已被消费",
					ErrorCode: tools.ErrPathOutside,
				}, nil
			}
			log.Printf("[ExecuteTool] 审批已通过，执行工具: tool=%s path=%s", req.ToolName, resolvedPath)
		}
	}

	// 正常执行（审批通过或路径在工作目录内）
	// 审批通过时携带 WithApproval 标记，跳过工具内的二次路径校验
	execCtx := ctx
	if needsPermission {
		execCtx = tools.WithApproval(ctx)
		execCtx = tools.WithApprovalID(execCtx, req.ApprovalId)
	}
	result := executor.Execute(execCtx, req.SessionId, req.ToolName, req.Arguments)

	return &proto.ToolResponse{
		Success:   result.Success,
		Result:    result.Result,
		ErrorCode: result.ErrorCode,
	}, nil
}

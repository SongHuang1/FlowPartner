package handler

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/sanitize"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentHandler struct {
	proto.UnimplementedFlowPartnerServiceServer
	manager *bridge.Manager
}

// NewAgentHandler 注入 Manager
func NewAgentHandler(m *bridge.Manager) *AgentHandler {
	return &AgentHandler{manager: m}
}

// SyncChannel 处理双向流核心逻辑
func (h *AgentHandler) SyncChannel(stream proto.FlowPartnerService_SyncChannelServer) error {
	log.Println("🔗 Agent gRPC 双向流已建立")

	// 协程 1：不断从 Manager.CmdChan 读取前端发来的指令，发给 Python
	// 通过 stream.Context().Done() 退出，避免 Python 断开后 goroutine 永久泄漏
	// 使用带超时的 send 避免 Python Agent 卡死不消费时 goroutine 永久阻塞
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
						log.Printf("发送指令给 Python 失败: %s", sanitize.Error(err))
						return
					}
				case <-stream.Context().Done():
					return
				case <-time.After(30 * time.Second):
					log.Println("发送指令给 Python 超时，退出 goroutine")
					return
				}
			case <-stream.Context().Done():
				return
			}
		}
	}()

	// 协程 2 (主循环)：不断接收 Python 发来的实时事件，转发给前端 WebSocket
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			log.Println("🔌 Agent 断开了双向流")
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "接收事件失败: %s", sanitize.Error(err))
		}

		// 将事件路由给对应的前端 WebSocket 连接
		h.manager.SendToSession(event.SessionId, event)
	}
}

// CallLLM 保持之前的逻辑 (后续接入真实 API 时修改这里)
func (h *AgentHandler) CallLLM(ctx context.Context, req *proto.LLMRequest) (*proto.LLMResponse, error) {
	log.Printf("[CallLLM] Session: %s, Payload 长度: %d", req.SessionId, len(req.JsonPayload))

	// 解析 Python 发来的 Payload，检查是否包含 messages
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(req.JsonPayload), &payload); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "解析 JsonPayload 失败: %s", sanitize.Error(err))
	}

	messages, ok := payload["messages"].([]interface{})
	if !ok {
		return nil, status.Errorf(codes.InvalidArgument, "JsonPayload 缺少 messages 字段或类型错误")
	}

	// 模拟智能行为：
	// - 如果消息列表中只有 user 消息（第一轮），返回 tool_call 让 Python 去读文件
	// - 如果消息列表中已经有 tool 结果（第二轮），返回最终回答
	hasToolResult := false
	for _, msg := range messages {
		if m, ok := msg.(map[string]interface{}); ok {
			if role, _ := m["role"].(string); role == "tool" {
				hasToolResult = true
				break
			}
		}
	}

	var mockResponse string
	if !hasToolResult {
		// 第一轮：要求调用 read_file 工具
		mockResponse = `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": null,
					"tool_calls": [{
						"id": "call_mock_01",
						"type": "function",
						"function": {
							"name": "read_file",
							"arguments": "{\"path\": \"` + strings.ReplaceAll(req.SessionId, `"`, `\"`) + `.txt\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}]
		}`
		log.Println("[CallLLM] 返回: tool_call (read_file)")
	} else {
		// 第二轮：给出最终回答
		mockResponse = `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "我已经读取了文件内容，这是最终回答。"
				},
				"finish_reason": "stop"
			}]
		}`
		log.Println("[CallLLM] 返回: final_answer")
	}

	return &proto.LLMResponse{
		Success:      true,
		JsonResponse: mockResponse,
	}, nil
}

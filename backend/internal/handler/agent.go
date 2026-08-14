package handler

import (
	"encoding/json"
	"io"
	"log"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/llm"
	"github.com/songhuang/flowpartner/backend/internal/sanitize"
	"github.com/songhuang/flowpartner/backend/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentHandler struct {
	proto.UnimplementedFlowPartnerServiceServer
	manager  *bridge.Manager
	llmClient *llm.LLMClient
}

func NewAgentHandler(m *bridge.Manager) *AgentHandler {
	return &AgentHandler{
		manager:   m,
		llmClient: llm.NewClient(),
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
			Message: "No active model configuration",
			Guess:   "Please add and activate a model configuration",
		})
	}

	ks := keystore.Instance()
	apiKey, ok := ks.GetKey()
	if !ok {
		return h.sendError(stream, messageID, &llm.LLMError{
			Code:    4001,
			Message: "Model configuration is locked",
			Guess:   "Please unlock the model configuration in Settings first",
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
			Message: "Invalid BaseURL format",
			Guess:   "Please check the BaseURL in the model configuration",
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
			IsError:     chunk.Done && chunk.Data != "",
			JsonResponse: chunk.Data,
			MessageId:   messageID,
		}
		if err := stream.Send(resp); err != nil {
			log.Printf("[CallLLM] Send failed: %s", sanitize.Error(err))
			return nil
		}
	}

	return nil
}

func (h *AgentHandler) sendError(stream proto.FlowPartnerService_CallLLMServer, messageID string, llmErr *llm.LLMError) error {
	data, _ := json.Marshal(llmErr)
	return stream.Send(&proto.LLMResponse{
		IsError:      true,
		JsonResponse: string(data),
		MessageId:   messageID,
	})
}

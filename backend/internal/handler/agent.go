package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AgentHandler struct {
	proto.UnimplementedFlowPartnerServiceServer
}

// SyncChannel 处理双向流核心逻辑
func (h *AgentHandler) SyncChannel(stream proto.FlowPartnerService_SyncChannelServer) error {
	log.Println("新的 Agent 双向流已建立")

	// 开启一个协程，模拟 Go 主动向 Python 下发“开始对话”的指令
	go func() {
		time.Sleep(3 * time.Second) // 等待连接稳定

		// 生成高随机、不可预测的 Session ID
		// 格式: sess_纳秒级时间戳_32位随机十六进制字符串
		b := make([]byte, 16) // 16字节 = 32个十六进制字符
		rand.Read(b)
		realSessionID := fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), hex.EncodeToString(b))

		cmd := &proto.ServerCommand{
			SessionId:   realSessionID,
			CommandType: "start_chat",
			Payload:     `{"user_message": "帮我看看今天的天气"}`,
		}
		log.Printf("[Go 下发指令] start_chat | Session: %s", realSessionID)
		if err := stream.Send(cmd); err != nil {
			log.Printf("发送指令失败: %v", err)
		}
	}()

	// 主循环：不断接收 Python 发来的实时事件
	for {
		event, err := stream.Recv()
		if err == io.EOF {
			log.Println("Agent 断开了双向流")
			return nil
		}
		if err != nil {
			return status.Errorf(codes.Internal, "接收事件失败: %v", err)
		}

		// 打印 Python 发来的实时过程 (未来这里 Go 会把 event 转发给前端的 WebSocket)
		log.Printf("[收到 Agent 实时事件] Type: %s | Session: %s | Payload: %s",
			event.EventType, event.SessionId, event.Payload)
	}
}

func (h *AgentHandler) CallLLM(ctx context.Context, req *proto.LLMRequest) (*proto.LLMResponse, error) {
	log.Printf("[CallLLM] Session: %s, Payload 长度: %d", req.SessionId, len(req.JsonPayload))

	// 解析 Python 发来的 Payload，检查是否包含 messages
	var payload map[string]interface{}
	json.Unmarshal([]byte(req.JsonPayload), &payload)

	messages, _ := payload["messages"].([]interface{})

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

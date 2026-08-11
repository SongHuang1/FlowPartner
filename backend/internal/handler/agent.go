package handler

import (
	"context"
	"io"
	"log"
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
		mockSessionID := "sess_from_go_12345"
		cmd := &proto.ServerCommand{
			SessionId:   mockSessionID,
			CommandType: "start_chat",
			Payload:     `{"user_message": "帮我看看今天的天气"}`,
		}
		log.Printf("[Go 下发指令] start_chat | Session: %s", mockSessionID)
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

// CallLLM 保持之前的 Mock 实现
func (h *AgentHandler) CallLLM(ctx context.Context, req *proto.LLMRequest) (*proto.LLMResponse, error) {
	// ... (保持之前的 mock 代码不变，或者留空)
	return &proto.LLMResponse{Success: true, JsonResponse: `{"mock": true}`}, nil
}

package handler

import (
	"context"
	"log"
	"time"

	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AgentHandler 实现 gRPC 服务端接口
type AgentHandler struct {
	// 嵌入未实现的 Server，这是 gRPC 的推荐做法，保证向前兼容
	proto.UnimplementedFlowPartnerServiceServer
}

// ReceiveTasks 处理 Agent 的连接，并通过流主动下发任务
func (h *AgentHandler) ReceiveTasks(req *proto.RegisterRequest, stream proto.FlowPartnerService_ReceiveTasksServer) error {
	log.Printf(" Agent 已连接! 版本: %s, 工作目录: %s", req.AgentVersion, req.WorkspacePath)

	// 模拟业务延迟：连接建立 2 秒后，主动向 Python 下发一个测试任务
	time.Sleep(2 * time.Second)

	task := &proto.TaskCommand{
		TaskId:   "task-test-001",
		TaskType: "test_echo",
		Payload:  `{"message": "Hello Python, this is Go Server!"}`,
	}

	log.Printf("向 Agent 下发任务: %s", task.TaskId)
	if err := stream.Send(task); err != nil {
		return status.Errorf(codes.Internal, "发送任务失败: %v", err)
	}

	// 保持连接打开，直到 Python 断开或发生错误
	// 在实际业务中，这里未来会是一个 for 循环，监听任务队列 channel 并不断 stream.Send
	<-stream.Context().Done()
	log.Println(" Agent 断开连接")
	return nil
}

// SubmitResult 接收 Agent 执行完毕后的结果汇报
func (h *AgentHandler) SubmitResult(ctx context.Context, req *proto.TaskResult) (*proto.SubmitResponse, error) {
	log.Printf("收到 Agent 提交的结果! TaskID: %s, 成功: %v, 消息: %s", req.TaskId, req.Success, req.Message)
	return &proto.SubmitResponse{Received: true}, nil
}

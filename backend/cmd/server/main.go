package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/songhuang/flowpartner/backend/internal/handler"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc"
)

func main() {
	// 1. 监听 TCP 端口 (50051 是 Python 端配置的端口)
	// TODO: 动态获取端口功能
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen on port 50051: %v", err)
	}

	// 2. 创建 gRPC Server
	grpcServer := grpc.NewServer()

	// 3. 注册我们的 Handler
	agentHandler := &handler.AgentHandler{}
	proto.RegisterFlowPartnerServiceServer(grpcServer, agentHandler)

	// 4. 通知 Electron (或终端) 后端已就绪
	fmt.Fprintln(os.Stderr, "__FP_BACKEND_READY__")
	log.Println(" gRPC server starting on :50051")

	// 5. 启动 Server (非阻塞，放入协程)
	errChan := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			errChan <- err
		}
	}()

	// 6. 优雅退出逻辑 (监听 Ctrl+C 或系统 kill 信号)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Fatalf("gRPC server error: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v, gracefully shutting down gRPC server...", sig)
		grpcServer.GracefulStop() // 优雅关闭，处理完当前连接再退出
	}

	log.Println("Server exited")
}

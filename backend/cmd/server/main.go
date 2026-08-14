package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/config"
	"github.com/songhuang/flowpartner/backend/internal/handler"
	"github.com/songhuang/flowpartner/backend/internal/server"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc"
)

func main() {
	// 1. 读取配置
	cfg := config.Load()

	// 2. 创建 bridge.Manager 和 WebSocketHandler（共享桥接层）
	mgr := bridge.NewManager()
	wsHandler := handler.NewWebSocketHandler(mgr)

	// 3. 端口探索
	httpListener, httpPort, err := server.FindAvailablePort(cfg.HTTPPort, nil)
	if err != nil {
		log.Fatalf("HTTP port discovery failed: %v", err)
	}
	defer httpListener.Close()

	// gRPC 端口探索，排除 HTTP 已占用的端口
	exclude := map[string]bool{fmt.Sprintf("127.0.0.1:%d", httpPort): true}
	grpcListener, grpcPort, err := server.FindAvailablePort(":50051", exclude)
	if err != nil {
		log.Fatalf("gRPC port discovery failed: %v", err)
	}
	defer grpcListener.Close()

	// 4. 注册 HTTP 路由
	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler)

	httpServer := &http.Server{Handler: mux}

	// 5. 创建 gRPC Server
	grpcServer := grpc.NewServer()
	proto.RegisterFlowPartnerServiceServer(grpcServer, handler.NewAgentHandler(mgr))

	// 6. 启动 HTTP Server (goroutine)
	httpErrChan := make(chan error, 1)
	readyChan := make(chan struct{}, 2)
	go func() {
		log.Printf("HTTP server starting on :%d", httpPort)
		readyChan <- struct{}{}
		if err := httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			httpErrChan <- err
		}
	}()

	// 7. 启动 gRPC Server (goroutine)
	grpcErrChan := make(chan error, 1)
	go func() {
		log.Printf("gRPC server starting on :%d", grpcPort)
		readyChan <- struct{}{}
		if err := grpcServer.Serve(grpcListener); err != nil {
			grpcErrChan <- err
		}
	}()

	// 8. 等待两个服务真正开始 Accept 连接后，输出就绪信号
	<-readyChan
	<-readyChan
	fmt.Fprintln(os.Stderr, readySignal(httpPort, grpcPort))

	// 9. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrChan:
		log.Fatalf("HTTP server error: %v", err)
	case err := <-grpcErrChan:
		log.Fatalf("gRPC server error: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v, gracefully shutting down...", sig)
		shutdown(grpcServer, httpServer, mgr, wsHandler)
	}

	log.Println("Server exited")
}

func registerRoutes(mux *http.ServeMux, wsHandler *handler.WebSocketHandler) {
	settingsHandler := &handler.SettingsHandler{}
	conversationHandler := &handler.ConversationHandler{}
	unlockHandler := &handler.UnlockHandler{}
	modelConfigHandler := &handler.ModelConfigHandler{}

	mux.HandleFunc("/api/settings", settingsHandler.Handle)
	mux.HandleFunc("/api/settings/clear_api_key", settingsHandler.HandleClearAPIKey)
	mux.HandleFunc("/api/conversation", conversationHandler.Handle)
	mux.HandleFunc("/api/unlock", unlockHandler.Handle)
	mux.HandleFunc("/api/lock", unlockHandler.Handle)
	mux.HandleFunc("/api/lock_status", unlockHandler.Handle)
	mux.HandleFunc("/api/model_configs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/model_configs" {
			modelConfigHandler.Handle(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/activate") {
			modelConfigHandler.HandleActivate(w, r)
			return
		}
		modelConfigHandler.HandleByID(w, r)
	})
	mux.HandleFunc("/api/model_configs/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/activate") {
			modelConfigHandler.HandleActivate(w, r)
			return
		}
		modelConfigHandler.HandleByID(w, r)
	})
	mux.HandleFunc("/ws", wsHandler.HandleWS)
}

// readySignal 生成 Electron 识别的后端就绪信号（A7）
func readySignal(httpPort, grpcPort int) string {
	return fmt.Sprintf("__FP_BACKEND_READY__ HTTP=:%d gRPC=:%d", httpPort, grpcPort)
}

func shutdown(grpcServer *grpc.Server, httpServer *http.Server, mgr *bridge.Manager, wsHandler *handler.WebSocketHandler) {
	// 1. 先关闭 gRPC Server（等待当前 RPC 完成，超时 2 秒强制停止）
	gracefulDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(gracefulDone)
	}()
	select {
	case <-gracefulDone:
	case <-time.After(2 * time.Second):
		log.Println("gRPC graceful stop timed out, forcing stop")
		grpcServer.Stop()
	}

	// 2. 断开所有 WebSocket 连接
	mgr.CloseAllSessions()

	// 3. 关闭 WebSocketHandler 的 done channel，让 HandleWS 循环退出
	wsHandler.Close()

	// 4. 关闭 HTTP Server（2 秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server did not shut down within timeout: %v", err)
	}
}

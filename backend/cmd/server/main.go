package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/config"
	"github.com/songhuang/flowpartner/backend/internal/handler"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/server"
	"github.com/songhuang/flowpartner/backend/internal/snapshot"
	"github.com/songhuang/flowpartner/backend/internal/static"
	"github.com/songhuang/flowpartner/backend/internal/thread"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc"
)

const gracefulShutdownTimeout = 2 * time.Second

func main() {
	cfg := config.Load()
	initializeKeystore()

	threadMgr := thread.NewManager()
	agentEventCh := make(chan *proto.AgentEvent, 100)
	globalEventCh := make(chan handler.GlobalEvent, 100)

	snapshotMgr := snapshot.NewManager(
		func(status snapshot.Status) {
			select {
			case globalEventCh <- handler.GlobalEvent{EventType: "snapshot_status", Payload: mustJSON(status)}:
			default:
			}
		},
		func(msg snapshot.Message) {
			select {
			case globalEventCh <- handler.GlobalEvent{EventType: "snapshot_message", Payload: mustJSON(msg)}:
			default:
			}
		},
	)

	wsHandler := handler.NewWebSocketHandler(threadMgr, snapshotMgr, globalEventCh)
	go wsHandler.StartBroadcastLoop(globalEventCh)
	applySnapshotConfig(snapshotMgr)

	httpListener, httpPort, err := server.FindAvailablePort(cfg.HTTPPort, nil)
	if err != nil {
		log.Fatalf("HTTP port discovery failed: %v", err)
	}
	defer httpListener.Close()

	exclude := map[string]bool{fmt.Sprintf("127.0.0.1:%d", httpPort): true}
	grpcListener, grpcPort, err := server.FindAvailablePort(":50051", exclude)
	if err != nil {
		log.Fatalf("gRPC port discovery failed: %v", err)
	}
	defer grpcListener.Close()

	grpcServer := grpc.NewServer()
	agentHandler := handler.NewAgentHandler(threadMgr, agentEventCh)
	proto.RegisterFlowPartnerServiceServer(grpcServer, agentHandler)

	go agentHandler.StartEventPump(agentEventCh)

	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, snapshotMgr, threadMgr, agentHandler)
	staticHandler := static.NewHandler(cfg.FrontendDir)
	staticHandler.Handle(mux)

	httpServer := &http.Server{Handler: mux}

	httpErrChan := make(chan error, 1)
	readyChan := make(chan struct{}, 2)
	go func() {
		log.Printf("HTTP server starting on :%d", httpPort)
		readyChan <- struct{}{}
		if err := httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			httpErrChan <- err
		}
	}()

	grpcErrChan := make(chan error, 1)
	go func() {
		log.Printf("gRPC server starting on :%d", grpcPort)
		readyChan <- struct{}{}
		if err := grpcServer.Serve(grpcListener); err != nil {
			grpcErrChan <- err
		}
	}()

	<-readyChan
	<-readyChan
	fmt.Fprintln(os.Stderr, readySignal(httpPort, grpcPort))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-httpErrChan:
		log.Fatalf("HTTP server error: %v", err)
	case err := <-grpcErrChan:
		log.Fatalf("gRPC server error: %v", err)
	case sig := <-quit:
		log.Printf("Received signal %v, gracefully shutting down...", sig)
		shutdown(grpcServer, httpServer, wsHandler, snapshotMgr, threadMgr)
	}

	log.Println("Server exited")
}

func registerRoutes(mux *http.ServeMux, wsHandler *handler.WebSocketHandler, snapshotMgr *snapshot.Manager, threadMgr *thread.Manager, agentHandler *handler.AgentHandler) {
	settingsHandler := handler.NewSettingsHandler(snapshotMgr)
	historyHandler := &handler.HistoryHandler{}
	unlockHandler := &handler.UnlockHandler{}
	modelConfigHandler := &handler.ModelConfigHandler{}
	snapshotHandler := handler.NewSnapshotHandler(snapshotMgr)
	agentDefHandler := handler.NewAgentDefHandler(threadMgr, agentHandler.SendCommand, wsHandler.BroadcastEvent)

	mux.HandleFunc("/api/settings", settingsHandler.Handle)
	mux.HandleFunc("/api/settings/clear_api_key", settingsHandler.HandleClearAPIKey)
	mux.HandleFunc("/api/history", historyHandler.Handle)
	mux.HandleFunc("/api/history/", historyHandler.Handle)
	mux.HandleFunc("/api/unlock", unlockHandler.Handle)
	mux.HandleFunc("/api/lock", unlockHandler.Handle)
	mux.HandleFunc("/api/lock_status", unlockHandler.Handle)
	mux.HandleFunc("/api/snapshots", snapshotHandler.Handle)
	mux.HandleFunc("/api/snapshots/", snapshotHandler.Handle)
	mux.HandleFunc("/api/agents", agentDefHandler.Handle)
	mux.HandleFunc("/api/agents/", agentDefHandler.HandleByID)
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

func applySnapshotConfig(snapshotMgr *snapshot.Manager) {
	settings := handler.LoadSettings()
	workingDir := handler.ResolveWorkingDir(settings)
	if workingDir == "" {
		log.Println("[snapshot] 无法解析工作目录，快照未启用")
		return
	}
	if err := snapshotMgr.Configure(
		workingDir,
		settings.SnapshotDir,
		settings.SnapshotEnabled,
		settings.SnapshotIncludeSecrets,
		settings.SnapshotDebounceSecs,
		settings.SnapshotTickerMins,
		settings.SnapshotRetentionDays,
		settings.SnapshotMaxStorageMB,
	); err != nil {
		log.Printf("[snapshot] 启动配置失败: %v", err)
	}
}

func mustJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func initializeKeystore() {
	settings := handler.LoadSettings()
	ks := keystore.Instance()
	if settings.EncryptedAPIKey != "" {
		ks.SetAPIKeyConfigured(true)
		return
	}
	for _, cfg := range settings.ModelConfigs {
		if cfg.EncryptedAPIKey != "" {
			ks.SetAPIKeyConfigured(true)
			return
		}
	}
}

func readySignal(httpPort, grpcPort int) string {
	return fmt.Sprintf("__FP_BACKEND_READY__ HTTP=:%d gRPC=:%d", httpPort, grpcPort)
}

func shutdown(grpcServer *grpc.Server, httpServer *http.Server, wsHandler *handler.WebSocketHandler, snapshotMgr *snapshot.Manager, threadMgr *thread.Manager) {
	if snapshotMgr != nil {
		snapshotMgr.Close()
	}

	gracefulDone := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(gracefulDone)
	}()
	select {
	case <-gracefulDone:
	case <-time.After(gracefulShutdownTimeout):
		log.Println("gRPC graceful stop timed out, forcing stop")
		grpcServer.Stop()
	}

	wsHandler.Close()
	threadMgr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server did not shut down within timeout: %v", err)
	}
}



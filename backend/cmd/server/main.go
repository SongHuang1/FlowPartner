package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/SongHuang1/FlowPartner/backend/internal/config"
	"github.com/SongHuang1/FlowPartner/backend/internal/handler"
	"github.com/SongHuang1/FlowPartner/backend/internal/keystore"
	"github.com/SongHuang1/FlowPartner/backend/internal/server"
	"github.com/SongHuang1/FlowPartner/backend/internal/snapshot"
	"github.com/SongHuang1/FlowPartner/backend/internal/static"
	"github.com/SongHuang1/FlowPartner/backend/internal/thread"
	"github.com/SongHuang1/FlowPartner/backend/proto"
	"google.golang.org/grpc"
)

func main() {
	const (
		agentEventBufferSize  = 100
		globalEventBufferSize = 100
	)
	cfg := config.Load()
	settings := handler.LoadSettings()
	initializeKeystore(settings)

	threadMgr := thread.NewManager()
	agentEventCh := make(chan *proto.AgentEvent, agentEventBufferSize)
	globalEventCh := make(chan handler.GlobalEvent, globalEventBufferSize)

	snapshotMgr := snapshot.NewManager(
		func(status snapshot.Status) {
			select {
			case globalEventCh <- handler.GlobalEvent{EventType: "snapshot_status", Payload: jsonPayload(status)}:
			default:
				log.Printf("[snapshot] dropping status update, global event channel full")
			}
		},
		func(msg snapshot.Message) {
			select {
			case globalEventCh <- handler.GlobalEvent{EventType: "snapshot_message", Payload: jsonPayload(msg)}:
			default:
				log.Printf("[snapshot] dropping message, global event channel full")
			}
		},
	)

	applySnapshotConfig(snapshotMgr, settings)

	httpListener, httpPort, err := server.FindAvailablePort(cfg.HTTPPort, nil)
	if err != nil {
		log.Fatalf("HTTP port discovery failed: %v", err)
	}
	defer httpListener.Close()

	exclude := map[string]bool{net.JoinHostPort("127.0.0.1", strconv.Itoa(httpPort)): true}
	grpcListener, grpcPort, err := server.FindAvailablePort("50051", exclude)
	if err != nil {
		log.Fatalf("gRPC port discovery failed: %v", err)
	}
	defer grpcListener.Close()

	grpcServer := grpc.NewServer()
	agentHandler := handler.NewAgentHandler(threadMgr, agentEventCh)
	proto.RegisterFlowPartnerServiceServer(grpcServer, agentHandler)

	wsHandler := handler.NewWebSocketHandler(threadMgr, snapshotMgr, globalEventCh, agentHandler)
	go wsHandler.StartBroadcastLoop(globalEventCh)

	go agentHandler.StartEventPump(agentEventCh)

	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, snapshotMgr, threadMgr, agentHandler)
	staticHandler := static.NewHandler(cfg.FrontendDir)
	staticHandler.Handle(mux)

	httpServer := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	httpErrChan := make(chan error, 1)
	grpcErrChan := make(chan error, 1)
	go func() {
		log.Printf("HTTP server starting on :%d", httpPort)
		if err := httpServer.Serve(httpListener); err != nil && err != http.ErrServerClosed {
			httpErrChan <- err
		}
	}()

	go func() {
		log.Printf("gRPC server starting on :%d", grpcPort)
		if err := grpcServer.Serve(grpcListener); err != nil {
			grpcErrChan <- err
		}
	}()

	if err := waitForServersReady(httpPort, grpcPort, httpErrChan, grpcErrChan); err != nil {
		log.Fatalf("backend startup failed: %v", err)
	}
	fmt.Fprintln(os.Stdout, readySignal(httpPort, grpcPort))

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
	historyHandler := handler.NewHistoryHandler()
	unlockHandler := handler.NewUnlockHandler()
	modelConfigHandler := handler.NewModelConfigHandler()
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

	mux.HandleFunc("/api/model_configs", modelConfigHandler.Handle)
	mux.HandleFunc("/api/model_configs/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/activate") {
			modelConfigHandler.HandleActivate(w, r)
			return
		}
		modelConfigHandler.HandleByID(w, r)
	})
	mux.HandleFunc("/ws", wsHandler.HandleWS)
}

func applySnapshotConfig(snapshotMgr *snapshot.Manager, settings handler.Settings) {
	workingDir := handler.ResolveWorkingDir(settings)
	if workingDir == "" {
		log.Println("[snapshot] Unable to parse the working directory. Snapshot is not enabled.")
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
		log.Printf("[snapshot] fail to start: %v", err)
	}
}

func jsonPayload(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		log.Printf("[events] marshal payload failed, falling back to {}: %v", err)
		return "{}"
	}
	return string(data)
}

// TODO: We will uniformly activate a password across the entire system and expand its functions.

func initializeKeystore(settings handler.Settings) {
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

	const gracefulShutdownTimeout = 2 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server did not shut down within timeout: %v", err)
		if err := httpServer.Close(); err != nil {
			log.Printf("HTTP server force close failed: %v", err)
		}
	}

	if wsHandler != nil {
		wsHandler.Close()
	}
	if threadMgr != nil {
		threadMgr.Close()
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

	if snapshotMgr != nil {
		snapshotMgr.Close()
	}
}

func waitForServersReady(httpPort, grpcPort int, httpErrChan, grpcErrChan <-chan error) error {
	deadline := time.Now().Add(5 * time.Second)
	for {

		select {
		case err := <-httpErrChan:
			return fmt.Errorf("http serve failed: %w", err)
		case err := <-grpcErrChan:
			return fmt.Errorf("grpc serve failed: %w", err)
		default:
		}

		httpOK := dialOK(httpPort)
		grpcOK := dialOK(grpcPort)

		if httpOK && grpcOK {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("ports not accepting connections within 5s (http=%v grpc=%v)", httpOK, grpcOK)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

func dialOK(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Package main wires together all backend subsystems: HTTP, gRPC, WebSocket,
// snapshot manager, and keystore. It owns the process lifecycle from startup
// through graceful shutdown.
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

// main is the composition root. It binds both listeners before anything can
// connect, blocks on the first of {signal, HTTP error, gRPC error}, and
// publishes the ready signal only after both ports accept connections so the
// Electron parent never races the server.
func main() {
	const (
		// Both channels are fed by goroutines that must not block on a slow
		// frontend, so the buffers absorb bursts and the sends drop rather
		// than stall the snapshot manager's watcher loop.
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

	// a listener held open from here on is what keeps another backend instance from claiming the same port.
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

	// Both pumps run until the process exits; neither has a stop channel because
	// the channels they drain are process-scoped and shutdown only needs to stop
	// new work from arriving, not to join these goroutines.
	go wsHandler.StartBroadcastLoop(globalEventCh)
	go agentHandler.StartEventPump(agentEventCh)

	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, snapshotMgr, threadMgr, agentHandler)
	staticHandler := static.NewHandler(cfg.FrontendDir)
	// Registered after the API routes so an API path can never be swallowed by
	// the SPA fallback.
	staticHandler.Handle(mux)

	httpServer := &http.Server{
		Handler: mux,
		// Guards against a client that opens a socket and sends nothing.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	// Buffered by one so the Serve goroutines can report a failure and exit
	// instead of blocking forever if startup gets this far before the select.
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
	// Printed only after both listeners accept connections: the Electron parent
	// connects the moment it sees this line.
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

// registerRoutes mounts the whole HTTP surface. Handlers are constructed here
// rather than injected so this package owns the wiring graph outright.
//
// Every subtree appears twice: an exact path for the collection and a trailing
// slash for its children. Both delegate to the same handler, which is what
// splits method and ID on its own.
func registerRoutes(mux *http.ServeMux, wsHandler *handler.WebSocketHandler, snapshotMgr *snapshot.Manager, threadMgr *thread.Manager, agentHandler *handler.AgentHandler) {
	settingsHandler := handler.NewSettingsHandler(snapshotMgr)
	historyHandler := handler.NewHistoryHandler()
	unlockHandler := handler.NewUnlockHandler()
	modelConfigHandler := handler.NewModelConfigHandler()
	snapshotHandler := handler.NewSnapshotHandler(snapshotMgr)
	agentDefHandler := handler.NewAgentDefHandler(threadMgr, agentHandler.SendCommand, wsHandler.BroadcastEvent)

	// unlock/lock/lock_status share one handler: it dispatches on method.
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
		dispatchModelConfig(w, r, modelConfigHandler.HandleActivate, modelConfigHandler.HandleByID)
	})
	mux.HandleFunc("/ws", wsHandler.HandleWS)
}

// dispatchModelConfig routes the /api/model_configs/ subtree. ServeMux cannot
// express "same prefix, two different suffixes", so the split happens here.
// The activate check comes first: an activate request must not be mistaken for
// an ID lookup.
func dispatchModelConfig(w http.ResponseWriter, r *http.Request, activate, byID http.HandlerFunc) {
	if strings.HasSuffix(r.URL.Path, "/activate") {
		activate(w, r)
		return
	}
	byID(w, r)
}

// applySnapshotConfig pushes persisted settings into the snapshot manager so a
// restart resumes watching instead of waiting for the frontend to nudge it.
// A bad config is logged and ignored rather than fatal: snapshots are a
// convenience, and the backend still has to serve chat without them.
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

// snapshotPayload is the closed set of values the snapshot manager reports
// through the event channel. Constraining the parameter to this set means a new
// event payload has to be declared here on purpose, rather than silently
// starting to send a shape the frontend does not parse.
type snapshotPayload interface {
	snapshot.Status | snapshot.Message
}

// jsonPayload marshals a snapshot event payload. Both members of
// snapshotPayload are plain structs of scalars, so Marshal cannot fail today;
// the fallback is kept because a dropped event is a silent UI stall whereas a
// "{}" body surfaces as a parse error in the frontend.
func jsonPayload[T snapshotPayload](v T) string {
	data, err := marshalPayload(v)
	if err != nil {
		log.Printf("[events] marshal payload failed, falling back to {}: %v", err)
		return "{}"
	}
	return data
}

func marshalPayload(v any) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	return string(data), nil
}

// TODO: We will uniformly activate a password across the entire system and expand its functions.

// initializeKeystore primes the "API key configured" flag so that
// /api/lock_status answers correctly before the user ever opens the unlock
// dialog. Both the legacy top-level key and per-config keys count.
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

// readySignal is the line the Electron parent greps for to learn the ports
// that port discovery settled on. Changing the format breaks startup, so the
// format is pinned by test.
func readySignal(httpPort, grpcPort int) string {
	return fmt.Sprintf("__FP_BACKEND_READY__ HTTP=:%d gRPC=:%d", httpPort, grpcPort)
}

// shutdown drains in-flight requests before closing listeners. Order matters:
// HTTP stops accepting first so no new WebSocket upgrades arrive, then gRPC
// gets GracefulStop with a force fallback because an agent mid-CallLLM will
// otherwise hold the process open past the timeout. Components are nil-checked
// because a partially constructed process still has to be able to exit.
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

// waitForServersReady blocks until both ports accept a connection, so the ready
// signal is only printed once the parent can actually connect. Serve errors
// take priority over dialing on every iteration: a listener that closed between
// discovery and Serve would otherwise be reported as a mere timeout.
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

// dialOK reports whether a TCP connection to 127.0.0.1:port succeeds. It
// closes the connection immediately: these probes run in a polling loop, and
// an unclosed socket would accumulate until the process exits.
func dialOK(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SongHuang1/FlowPartner/backend/internal/handler"
	"github.com/SongHuang1/FlowPartner/backend/internal/keystore"
	"github.com/SongHuang1/FlowPartner/backend/internal/snapshot"
	"github.com/SongHuang1/FlowPartner/backend/internal/storage"
	"github.com/SongHuang1/FlowPartner/backend/internal/thread"
	"github.com/SongHuang1/FlowPartner/backend/proto"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestMain(m *testing.M) {
	tmpDir, err := os.MkdirTemp("", "flowpartner-cmd-test-*")
	if err != nil {
		panic(err)
	}
	storage.SetDataDirForTest(tmpDir)
	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

// testWiring holds the object graph registerRoutes builds, so tests can reach
// into a specific handler instead of going through the mux.
type testWiring struct {
	mux           *http.ServeMux
	threadMgr     *thread.Manager
	snapshotMgr   *snapshot.Manager
	wsHandler     *handler.WebSocketHandler
	agentHandler  *handler.AgentHandler
	globalEventCh chan handler.GlobalEvent
	agentEventCh  chan *proto.AgentEvent
}

// makeTestWiring builds the same graph main() builds and registers it on a mux
// that never listens. Tests drive it through ServeHTTP.
func makeTestWiring(t *testing.T) *testWiring {

	t.Helper()
	keystore.Reset()

	// The keystore singleton and settings.json are reset first because both are
	// process-global and would otherwise leak between tests: the unlock flow
	// depends on the persisted key, and a rate-limited keystore rejects the next
	// test's attempts outright.

	dataDir, err := storage.DataDir()
	if err != nil {
		t.Fatalf("resolve data dir: %v", err)
	}
	if err := os.Remove(filepath.Join(dataDir, "settings.json")); err != nil && !os.IsNotExist(err) {
		t.Fatalf("reset settings.json: %v", err)
	}
	storage.ResetDataDirCache()

	w := &testWiring{
		threadMgr:     thread.NewManager(),
		snapshotMgr:   snapshot.NewManager(nil, nil),
		globalEventCh: make(chan handler.GlobalEvent, 10),
		agentEventCh:  make(chan *proto.AgentEvent, 100),
	}
	w.agentHandler = handler.NewAgentHandler(w.threadMgr, w.agentEventCh)
	w.wsHandler = handler.NewWebSocketHandler(w.threadMgr, w.snapshotMgr, w.globalEventCh, w.agentHandler)
	w.mux = http.NewServeMux()
	registerRoutes(w.mux, w.wsHandler, w.snapshotMgr, w.threadMgr, w.agentHandler)

	t.Cleanup(func() {
		w.snapshotMgr.Close()
		w.threadMgr.Close()
	})

	return w
}

// TestReadySignal pins the wire format the Electron parent parses.
func TestReadySignal(t *testing.T) {
	got := readySignal(8080, 50051)
	want := "__FP_BACKEND_READY__ HTTP=:8080 gRPC=:50051"
	if got != want {
		t.Errorf("readySignal = %q, want %q", got, want)
	}
}

// TestJSONPayload_Status checks the status event shape the frontend decodes.
func TestJSONPayload_Status(t *testing.T) {
	got := jsonPayload(snapshot.Status{Phase: "idle", Count: 3, SizeBytes: 1024})

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload is not valid JSON: %v (payload: %s)", err, got)
	}
	if decoded["phase"] != "idle" {
		t.Errorf("phase = %v, want idle", decoded["phase"])
	}
	if decoded["count"] != float64(3) {
		t.Errorf("count = %v, want 3", decoded["count"])
	}
	// last_at is omitempty, so a zero time must not reach the frontend as a
	// bogus timestamp.
	if _, ok := decoded["last_at"]; ok {
		t.Errorf("last_at present for a zero time, want omitted (payload: %s)", got)
	}
}

// TestJSONPayload_Message pins the user-facing snapshot message shape.
func TestJSONPayload_Message(t *testing.T) {
	got := jsonPayload(snapshot.Message{Type: "warning", Text: "snapshot skipped"})

	var decoded snapshot.Message
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("payload does not decode into snapshot.Message: %v (payload: %s)", err, got)
	}
	if decoded.Type != "warning" || decoded.Text != "snapshot skipped" {
		t.Errorf("decoded = %+v, want {warning snapshot skipped}", decoded)
	}
}

// TestMarshalPayload_Failure exercises the error branch. It goes through
// marshalPayload rather than jsonPayload because the generic constraint now
// admits only types that marshal cleanly, making jsonPayload's fallback
// unreachable from a test.
func TestMarshalPayload_Failure(t *testing.T) {
	got, err := marshalPayload(make(chan int))
	if err == nil {
		t.Fatal("marshalPayload succeeded on a channel, want error")
	}
	if got != "" {
		t.Errorf("marshalPayload returned %q alongside an error, want empty", got)
	}
}

// TestInitializeKeystore covers every code path that sets or clears the
// "API key configured" flag on the keystore singleton.
func TestInitializeKeystore(t *testing.T) {
	tests := []struct {
		name     string
		settings handler.Settings
		wantKey  bool
	}{
		{
			name:     "legacy top-level encrypted key",
			settings: handler.Settings{EncryptedAPIKey: "enc:legacy"},
			wantKey:  true,
		},
		{
			name: "first model config with encrypted key",
			settings: handler.Settings{
				ModelConfigs: []handler.ModelConfig{
					{ID: "cfg_a"},
					{ID: "cfg_b", EncryptedAPIKey: "enc:b"},
				},
			},
			wantKey: true,
		},
		{
			name: "model configs without encrypted keys",
			settings: handler.Settings{
				ModelConfigs: []handler.ModelConfig{{ID: "cfg_a"}},
			},
			wantKey: false,
		},
		{
			name:     "no key anywhere",
			settings: handler.Settings{},
			wantKey:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keystore.Reset()
			initializeKeystore(tt.settings)
			got := keystore.Instance().GetLockStatus().HasAPIKey
			if got != tt.wantKey {
				t.Errorf("HasAPIKey = %v, want %v", got, tt.wantKey)
			}
		})
	}
}

// TestApplySnapshotConfig checks that main refuses to leave the snapshot
// manager half-configured. Every rejection case asserts the manager stayed
// disabled: the bug this guards against is a watcher running on a workspace
// that does not exist.
func TestApplySnapshotConfig(t *testing.T) {
	t.Run("enabled configures manager", func(t *testing.T) {
		workspace := t.TempDir()
		snapshotDir := filepath.Join(t.TempDir(), "snapshots")
		mgr := snapshot.NewManager(nil, nil)
		t.Cleanup(mgr.Close)

		applySnapshotConfig(mgr, handler.Settings{
			WorkingDirectory:     workspace,
			SnapshotDir:          snapshotDir,
			SnapshotEnabled:      true,
			SnapshotDebounceSecs: 5,
			SnapshotTickerMins:   30,
		})

		if !mgr.Enabled() {
			t.Error("snapshot manager should be enabled after valid configuration")
		}
		if got, want := mgr.SnapshotDir(), snapshotDir; got != want {
			t.Errorf("SnapshotDir = %q, want %q", got, want)
		}
	})

	t.Run("disabled leaves manager off", func(t *testing.T) {
		mgr := snapshot.NewManager(nil, nil)
		t.Cleanup(mgr.Close)

		applySnapshotConfig(mgr, handler.Settings{
			WorkingDirectory: t.TempDir(),
			SnapshotDir:      filepath.Join(t.TempDir(), "snapshots"),
			SnapshotEnabled:  false,
		})

		if mgr.Enabled() {
			t.Error("snapshot manager should stay disabled when SnapshotEnabled is false")
		}
	})

	t.Run("missing workspace does not panic and stays off", func(t *testing.T) {
		mgr := snapshot.NewManager(nil, nil)
		t.Cleanup(mgr.Close)

		applySnapshotConfig(mgr, handler.Settings{
			WorkingDirectory: filepath.Join(t.TempDir(), "does-not-exist"),
			SnapshotDir:      filepath.Join(t.TempDir(), "snapshots"),
			SnapshotEnabled:  true,
		})

		if mgr.Enabled() {
			t.Error("snapshot manager must stay disabled when workspace root is invalid")
		}
	})

	t.Run("nested snapshot dir rejected without panic", func(t *testing.T) {
		workspace := t.TempDir()
		mgr := snapshot.NewManager(nil, nil)
		t.Cleanup(mgr.Close)

		applySnapshotConfig(mgr, handler.Settings{
			WorkingDirectory: workspace,
			SnapshotDir:      filepath.Join(workspace, "inner", "snapshots"),
			SnapshotEnabled:  true,
		})

		if mgr.Enabled() {
			t.Error("snapshot manager must stay disabled when snapshot dir nests inside workspace")
		}
	})
}

// TestRegisterRoutes_Patterns asserts path-to-pattern resolution only. What
// each handler does with the request belongs to internal/handler's own tests;
// duplicating those assertions here would report a handler bug against this
// package and fail for the wrong reason.
//
// mux.Handler returns the matched pattern without invoking it, which is exactly
// the question this package owns: which rule claims which path.
func TestRegisterRoutes_Patterns(t *testing.T) {
	w := makeTestWiring(t)

	tests := []struct {
		name        string
		path        string
		wantPattern string
	}{
		{"settings collection", "/api/settings", "/api/settings"},
		{"clear api key", "/api/settings/clear_api_key", "/api/settings/clear_api_key"},
		{"history collection", "/api/history", "/api/history"},
		{"history session", "/api/history/sess_test_1", "/api/history/"},
		{"unlock", "/api/unlock", "/api/unlock"},
		{"lock", "/api/lock", "/api/lock"},
		{"lock status", "/api/lock_status", "/api/lock_status"},
		{"snapshots collection", "/api/snapshots", "/api/snapshots"},
		{"snapshot item", "/api/snapshots/snap_1", "/api/snapshots/"},
		{"agents collection", "/api/agents", "/api/agents"},
		{"agent item", "/api/agents/code-reviewer", "/api/agents/"},
		{"model configs collection", "/api/model_configs", "/api/model_configs"},
		{"model config item", "/api/model_configs/cfg_a", "/api/model_configs/"},
		{"model config activate", "/api/model_configs/cfg_a/activate", "/api/model_configs/"},
		{"websocket", "/ws", "/ws"},
		{"unregistered path", "/api/unknown", ""},
		{"api prefix typo is not a subtree", "/api/setting", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			_, pattern := w.mux.Handler(req)
			if pattern != tt.wantPattern {
				t.Errorf("%s resolved to pattern %q, want %q", tt.path, pattern, tt.wantPattern)
			}
		})
	}
}

// TestDispatchModelConfig verifies the activate branch wins over the ID branch.
func TestDispatchModelConfig(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		wantCall string
	}{
		{"activate suffix goes to activate", "/api/model_configs/cfg_a/activate", "activate"},
		{"plain id goes to by id", "/api/model_configs/cfg_a", "byID"},
		{"subtree root goes to by id", "/api/model_configs/", "byID"},
		{"id merely containing activate does not match", "/api/model_configs/activate-now", "byID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, tt.path, nil)
			dispatchModelConfig(rec, req,
				func(http.ResponseWriter, *http.Request) { got = "activate" },
				func(http.ResponseWriter, *http.Request) { got = "byID" },
			)
			if got != tt.wantCall {
				t.Errorf("dispatchModelConfig(%q) called %q, want %q", tt.path, got, tt.wantCall)
			}
		})
	}
}

// TestRegisterRoutes_Wiring persists a setting and reads it back through the
// mux, proving the mounted handler talks to the shared storage layer. The fields
// asserted are the ones the flat format still owns.
func TestRegisterRoutes_Wiring(t *testing.T) {
	w := makeTestWiring(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"model":"gpt-4","context_window":4096,"language":"zh-CN"}`))
	rec := httptest.NewRecorder()
	w.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	listRec := httptest.NewRecorder()
	w.mux.ServeHTTP(listRec, listReq)

	var resp struct {
		Data handler.Settings `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse settings response: %v (body: %s)", err, listRec.Body.String())
	}
	if resp.Data.ContextWindow != 4096 {
		t.Errorf("round-tripped context_window = %d, want 4096", resp.Data.ContextWindow)
	}
	if resp.Data.Language != "zh-CN" {
		t.Errorf("round-tripped language = %q, want zh-CN", resp.Data.Language)
	}
}

// TestRegisterRoutes_UnlockFlow walks lock → unlock → status through the mux.
// Covered in internal/handler at the handler level;
// the reason to repeat it is that the three paths must all resolve to the one UnlockHandler instance.
// Divergent state (locked but reported unlocked) would mean two keystores.
func TestRegisterRoutes_UnlockFlow(t *testing.T) {
	w := makeTestWiring(t)

	putReq := httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key-abc","password":"TestPass123"}`))
	putRec := httptest.NewRecorder()
	w.mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d, want 200: %s", putRec.Code, putRec.Body.String())
	}

	lockReq := httptest.NewRequest(http.MethodPost, "/api/lock", nil)
	lockRec := httptest.NewRecorder()
	w.mux.ServeHTTP(lockRec, lockReq)
	if lockRec.Code != http.StatusOK {
		t.Fatalf("POST lock = %d, want 200", lockRec.Code)
	}

	unlockReq := httptest.NewRequest(http.MethodPost, "/api/unlock",
		strings.NewReader(`{"password":"TestPass123"}`))
	unlockRec := httptest.NewRecorder()
	w.mux.ServeHTTP(unlockRec, unlockReq)
	if unlockRec.Code != http.StatusOK {
		t.Fatalf("POST unlock = %d, want 200: %s", unlockRec.Code, unlockRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	statusRec := httptest.NewRecorder()
	w.mux.ServeHTTP(statusRec, statusReq)
	if statusRec.Code != http.StatusOK {
		t.Fatalf("GET lock_status = %d, want 200", statusRec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse lock_status: %v", err)
	}
	data, _ := resp["data"].(map[string]interface{})
	if data["locked"] != false {
		t.Errorf("expected unlocked after correct password, got %v", data["locked"])
	}
}

// TestDialOK checks both outcomes of the readiness probe.
//
// The closed-port half is inherently racy: the kernel can hand the freed port to
// an unrelated process before the probe runs. A failure there would be a false
// alarm, so that case asserts only the permissive direction.
func TestDialOK(t *testing.T) {
	listener := listenT(t)
	port := portOf(listener)
	if !dialOK(port) {
		t.Error("dialOK = false for a listening port, want true")
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	if dialOK(port) {
		t.Error("dialOK = true for a port whose listener is closed, want false")
	}
}

// TestDialOK_ClosesConnection confirms the probe does not leak sockets.
func TestDialOK_ClosesConnection(t *testing.T) {
	listener := listenT(t)
	port := portOf(listener)

	accepted := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			accepted <- err
			return
		}
		defer conn.Close()
		// Read returns EOF only when dialOK closed its half.
		if _, err := conn.Read(make([]byte, 1)); err == nil {
			accepted <- fmt.Errorf("read returned data, want EOF")
			return
		}
		accepted <- nil
	}()

	if !dialOK(port) {
		t.Fatal("dialOK = false for a listening port, want true")
	}

	select {
	case err := <-accepted:
		if err != nil {
			t.Fatalf("probe connection was not closed by the client: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("accepted connection never saw EOF; dialOK leaks its socket")
	}
}

// TestWaitForServersReady covers the three outcomes main depends on: both ports
// up, HTTP failed, gRPC failed. The error cases use a real listening port so the
// only reason to return early is the channel, not a failed dial.
//
// The timeout path is intentionally untested.
func TestWaitForServersReady(t *testing.T) {
	t.Run("both ports listening", func(t *testing.T) {
		httpLis := listenT(t)
		grpcLis := listenT(t)

		if err := waitForServersReady(
			portOf(httpLis), portOf(grpcLis),
			make(chan error, 1), make(chan error, 1),
		); err != nil {
			t.Errorf("waitForServersReady = %v, want nil", err)
		}
	})

	t.Run("http serve error propagates", func(t *testing.T) {
		httpErrChan := make(chan error, 1)
		httpErrChan <- context.Canceled

		err := waitForServersReady(portOf(listenT(t)), portOf(listenT(t)),
			httpErrChan, make(chan error, 1))
		if err == nil {
			t.Fatal("waitForServersReady = nil, want error")
		}
		if !strings.Contains(err.Error(), "http serve failed") {
			t.Errorf("error = %v, want it to mention http serve failed", err)
		}
	})

	t.Run("grpc serve error propagates", func(t *testing.T) {
		grpcErrChan := make(chan error, 1)
		grpcErrChan <- context.Canceled

		err := waitForServersReady(portOf(listenT(t)), portOf(listenT(t)),
			make(chan error, 1), grpcErrChan)
		if err == nil {
			t.Fatal("waitForServersReady = nil, want error")
		}
		if !strings.Contains(err.Error(), "grpc serve failed") {
			t.Errorf("error = %v, want it to mention grpc serve failed", err)
		}
	})
}

func listenT(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })
	return listener
}

func portOf(listener net.Listener) int {
	return listener.Addr().(*net.TCPAddr).Port
}

// TestShutdown_ClosesAllServers checks the clean path: nothing is mid-request,
// so shutdown must return inside the timeout and leave nothing reachable. An
// open WebSocket is the interesting part, since HTTP Shutdown alone does not
// close hijacked connections.
func TestShutdown_ClosesAllServers(t *testing.T) {
	w := makeTestWiring(t)

	httpServer := &http.Server{Handler: w.mux}
	httpLis := listenT(t)
	go httpServer.Serve(httpLis)

	grpcLis := listenT(t)
	grpcServer := grpc.NewServer()
	proto.RegisterFlowPartnerServiceServer(grpcServer, w.agentHandler)
	go grpcServer.Serve(grpcLis)

	// The listeners are bound but Serve may not be scheduled yet, so dial with
	// retry rather than assuming the first connect wins.
	wsURL := "ws://" + httpLis.Addr().String() + "/ws"
	var conn *websocket.Conn
	var err error
	deadline := time.Now().Add(2 * time.Second)
	for {
		conn, _, err = websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("dial ws: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer conn.Close()

	start := time.Now()
	shutdown(grpcServer, httpServer, w.wsHandler, w.snapshotMgr, w.threadMgr)

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("shutdown took %v, expected < 2s", elapsed)
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	if _, err := client.Get("http://" + httpLis.Addr().String() + "/api/settings"); err == nil {
		t.Error("expected HTTP request to fail after shutdown")
	}

	if _, _, err := conn.ReadMessage(); err == nil {
		t.Error("expected websocket to be closed after shutdown")
	}
}

// TestShutdown_ForceStopsStuckGRPC is the reason shutdown force-stops rather
// than only asking politely. GracefulStop waits for in-flight RPCs forever, so
// a live SyncChannel stream would hang the process on quit. The 4s bound is the
// two 2s timeouts back to back.
func TestShutdown_ForceStopsStuckGRPC(t *testing.T) {
	w := makeTestWiring(t)

	httpServer := &http.Server{Handler: http.NewServeMux()}

	grpcLis := listenT(t)
	grpcServer := grpc.NewServer()
	proto.RegisterFlowPartnerServiceServer(grpcServer, w.agentHandler)
	go grpcServer.Serve(grpcLis)

	grpcConn, err := grpc.NewClient(grpcLis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("grpc dial: %v", err)
	}
	defer grpcConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client := proto.NewFlowPartnerServiceClient(grpcConn)
	stream, err := client.SyncChannel(ctx)
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer stream.CloseSend()

	start := time.Now()
	shutdown(grpcServer, httpServer, w.wsHandler, w.snapshotMgr, w.threadMgr)

	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("shutdown took %v, expected force stop within ~2s", elapsed)
	}

	recvDone := make(chan struct{})
	go func() {
		defer close(recvDone)
		stream.Recv()
	}()
	select {
	case <-recvDone:
	case <-time.After(2 * time.Second):
		t.Fatal("gRPC stream still open after shutdown")
	}
}

// TestShutdown_ToleratesNilOptionalComponents pins the nil guards. A nil
// deref here would turn a partial startup failure into a panic that masks the
// original error, since this is the last code to run before the process exits.
func TestShutdown_ToleratesNilOptionalComponents(t *testing.T) {
	grpcServer := grpc.NewServer()
	httpServer := &http.Server{Handler: http.NewServeMux()}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("shutdown panicked with nil components: %v", r)
		}
	}()
	shutdown(grpcServer, httpServer, nil, nil, nil)
}

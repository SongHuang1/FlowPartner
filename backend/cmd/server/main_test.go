package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
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

type testWiring struct {
	mux           *http.ServeMux
	threadMgr     *thread.Manager
	snapshotMgr   *snapshot.Manager
	wsHandler     *handler.WebSocketHandler
	agentHandler  *handler.AgentHandler
	globalEventCh chan handler.GlobalEvent
	agentEventCh  chan *proto.AgentEvent
}

func newTestWiring(t *testing.T) *testWiring {
	t.Helper()
	keystore.Reset()
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

func TestReadySignal(t *testing.T) {
	got := readySignal(8080, 50051)
	want := "__FP_BACKEND_READY__ HTTP=:8080 gRPC=:50051"
	if got != want {
		t.Errorf("readySignal = %q, want %q", got, want)
	}
}

func TestReadySignal_ParsedByElectron(t *testing.T) {
	signal := readySignal(8080, 50051)

	httpMatch := regexp.MustCompile(`HTTP=:(\d+)`).FindStringSubmatch(signal)
	if httpMatch == nil {
		t.Fatalf("HTTP port not parseable from ready signal %q", signal)
	}
	if httpMatch[1] != "8080" {
		t.Errorf("HTTP port = %s, want 8080", httpMatch[1])
	}

	grpcMatch := regexp.MustCompile(`gRPC=:(\d+)`).FindStringSubmatch(signal)
	if grpcMatch == nil {
		t.Fatalf("gRPC port not parseable from ready signal %q", signal)
	}
	if grpcMatch[1] != "50051" {
		t.Errorf("gRPC port = %s, want 50051", grpcMatch[1])
	}
}

func TestJSONPayload_Success(t *testing.T) {
	got := jsonPayload(map[string]int{"a": 1})
	if got != `{"a":1}` {
		t.Errorf("jsonPayload = %q, want {\"a\":1}", got)
	}
}

func TestJSONPayload_MarshalFailureFallsBackToEmptyObject(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("jsonPayload panicked on unmarshalable value: %v", r)
		}
	}()
	if got := jsonPayload(make(chan int)); got != "{}" {
		t.Errorf("jsonPayload = %q, want {}", got)
	}
}

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

func TestRegisterRoutes(t *testing.T) {
	w := newTestWiring(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"settings GET", http.MethodGet, "/api/settings", "", http.StatusOK},
		{"settings PUT", http.MethodPut, "/api/settings",
			`{"model":"gpt-4","context_window":4096,"language":"zh-CN"}`, http.StatusOK},
		{"clear_api_key POST", http.MethodPost, "/api/settings/clear_api_key", "", http.StatusOK},
		{"history GET", http.MethodGet, "/api/history", "", http.StatusOK},
		{"history session GET", http.MethodGet, "/api/history/sess_test_1", "", http.StatusNotFound},
		{"unlock POST", http.MethodPost, "/api/unlock", `{"password":"WrongPass123"}`, http.StatusBadRequest},
		{"lock POST", http.MethodPost, "/api/lock", "", http.StatusOK},
		{"lock_status GET", http.MethodGet, "/api/lock_status", "", http.StatusOK},
		{"settings POST 405", http.MethodPost, "/api/settings", "", http.StatusMethodNotAllowed},
		{"history POST 405", http.MethodPost, "/api/history", "", http.StatusMethodNotAllowed},
		{"lock GET 405", http.MethodGet, "/api/lock", "", http.StatusMethodNotAllowed},
		{"unknown 404", http.MethodGet, "/api/unknown", "", http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			w.mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s = %d, want %d (body: %s)", tt.method, tt.path, rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestRegisterRoutes_ModelConfigsDispatch(t *testing.T) {
	w := newTestWiring(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{"collection GET", http.MethodGet, "/api/model_configs", "", http.StatusOK},
		{"collection POST invalid body", http.MethodPost, "/api/model_configs", "not-json", http.StatusBadRequest},
		{"collection POST missing model name", http.MethodPost, "/api/model_configs",
			`{"name":"cfg-one","base_url":"https://api.example.com"}`, http.StatusBadRequest},
		{"collection PUT 405", http.MethodPut, "/api/model_configs", "", http.StatusMethodNotAllowed},
		{"subtree root DELETE 400", http.MethodDelete, "/api/model_configs/", "", http.StatusBadRequest},
		{"item GET 405", http.MethodGet, "/api/model_configs/cfg_missing", "", http.StatusMethodNotAllowed},
		{"item DELETE missing 404", http.MethodDelete, "/api/model_configs/cfg_missing", "", http.StatusNotFound},
		{"activate GET 405", http.MethodGet, "/api/model_configs/cfg_missing/activate", "", http.StatusMethodNotAllowed},
		{"activate unknown id 404", http.MethodPost, "/api/model_configs/cfg_missing/activate",
			`{"password":"TestPass123"}`, http.StatusNotFound},
		{"activate invalid body 400", http.MethodPost, "/api/model_configs/cfg_missing/activate",
			"not-json", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			w.mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s = %d, want %d (body: %s)", tt.method, tt.path, rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestRegisterRoutes_CollectionListReturnsConfigs(t *testing.T) {
	w := newTestWiring(t)

	req := httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"model":"gpt-4","context_window":4096,"language":"zh-CN"}`))
	rec := httptest.NewRecorder()
	w.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/model_configs",
		strings.NewReader(`{"name":"cfg-one","base_url":"https://api.example.com","model_name":"gpt-4"}`))
	createRec := httptest.NewRecorder()
	w.mux.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("POST model_configs = %d, want 201: %s", createRec.Code, createRec.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/model_configs", nil)
	listRec := httptest.NewRecorder()
	w.mux.ServeHTTP(listRec, listReq)

	var resp struct {
		Data []handler.ModelConfig `json:"data"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse list response: %v (body: %s)", err, listRec.Body.String())
	}
	if len(resp.Data) != 1 {
		t.Fatalf("list returned %d configs, want 1", len(resp.Data))
	}
	if resp.Data[0].Name != "cfg-one" {
		t.Errorf("listed config name = %q, want cfg-one", resp.Data[0].Name)
	}
}

func TestRegisterRoutes_UnlockFlow(t *testing.T) {
	w := newTestWiring(t)

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

func TestDialOK(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	openPort := listener.Addr().(*net.TCPAddr).Port
	if !dialOK(openPort) {
		t.Error("dialOK = false for a listening port, want true")
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	if dialOK(openPort) {
		t.Error("dialOK = true for a closed port, want false")
	}
}

func TestWaitForServersReady(t *testing.T) {
	t.Run("both ports listening", func(t *testing.T) {
		httpLis := mustListen(t)
		grpcLis := mustListen(t)

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

		err := waitForServersReady(portOf(mustListen(t)), portOf(mustListen(t)),
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

		err := waitForServersReady(portOf(mustListen(t)), portOf(mustListen(t)),
			make(chan error, 1), grpcErrChan)
		if err == nil {
			t.Fatal("waitForServersReady = nil, want error")
		}
		if !strings.Contains(err.Error(), "grpc serve failed") {
			t.Errorf("error = %v, want it to mention grpc serve failed", err)
		}
	})
}

func mustListen(t *testing.T) net.Listener {
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

func TestShutdown_ClosesAllServers(t *testing.T) {
	w := newTestWiring(t)

	httpServer := &http.Server{Handler: w.mux}
	httpLis := mustListen(t)
	go httpServer.Serve(httpLis)

	grpcLis := mustListen(t)
	grpcServer := grpc.NewServer()
	proto.RegisterFlowPartnerServiceServer(grpcServer, w.agentHandler)
	go grpcServer.Serve(grpcLis)

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

func TestShutdown_ForceStopsStuckGRPC(t *testing.T) {
	w := newTestWiring(t)

	httpServer := &http.Server{Handler: http.NewServeMux()}

	grpcLis := mustListen(t)
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

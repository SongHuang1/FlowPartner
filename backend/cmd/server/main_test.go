package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/handler"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/snapshot"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/internal/thread"
	"github.com/songhuang/flowpartner/backend/proto"
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

func TestReadySignal(t *testing.T) {
	got := readySignal(8080, 50051)
	want := "__FP_BACKEND_READY__ HTTP=:8080 gRPC=:50051"
	if got != want {
		t.Errorf("readySignal = %q, want %q", got, want)
	}
}

func TestHTTPRoutes(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	threadMgr := thread.NewManager()
	snapshotMgr := snapshot.NewManager(nil, nil)
	globalEventCh := make(chan handler.GlobalEvent, 10)
	wsHandler := handler.NewWebSocketHandler(threadMgr, snapshotMgr, globalEventCh)
	agentEventCh := make(chan *proto.AgentEvent, 100)
	agentHandler := handler.NewAgentHandler(threadMgr, agentEventCh)
	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, snapshotMgr, threadMgr, agentHandler)

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
			body := strings.NewReader(tt.body)
			req := httptest.NewRequest(tt.method, tt.path, body)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s = %d, want %d (body: %s)", tt.method, tt.path, rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestHTTPRoutes_UnlockFlow(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	threadMgr := thread.NewManager()
	snapshotMgr := snapshot.NewManager(nil, nil)
	globalEventCh := make(chan handler.GlobalEvent, 10)
	wsHandler := handler.NewWebSocketHandler(threadMgr, snapshotMgr, globalEventCh)
	agentEventCh := make(chan *proto.AgentEvent, 100)
	agentHandler := handler.NewAgentHandler(threadMgr, agentEventCh)
	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, snapshotMgr, threadMgr, agentHandler)

	putReq := httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key-abc","password":"TestPass123"}`))
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d, want 200: %s", putRec.Code, putRec.Body.String())
	}

	lockReq := httptest.NewRequest(http.MethodPost, "/api/lock", nil)
	lockRec := httptest.NewRecorder()
	mux.ServeHTTP(lockRec, lockReq)
	if lockRec.Code != http.StatusOK {
		t.Fatalf("POST lock = %d, want 200", lockRec.Code)
	}

	unlockReq := httptest.NewRequest(http.MethodPost, "/api/unlock",
		strings.NewReader(`{"password":"TestPass123"}`))
	unlockRec := httptest.NewRecorder()
	mux.ServeHTTP(unlockRec, unlockReq)
	if unlockRec.Code != http.StatusOK {
		t.Fatalf("POST unlock = %d, want 200: %s", unlockRec.Code, unlockRec.Body.String())
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/lock_status", nil)
	statusRec := httptest.NewRecorder()
	mux.ServeHTTP(statusRec, statusReq)
	var resp map[string]interface{}
	if err := json.Unmarshal(statusRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse lock_status: %v", err)
	}
	data, _ := resp["data"].(map[string]interface{})
	if data["locked"] != false {
		t.Errorf("expected unlocked after correct password, got %v", data["locked"])
	}
}

func TestShutdown_ClosesAllServers(t *testing.T) {
	threadMgr := thread.NewManager()
	snapshotMgr := snapshot.NewManager(nil, nil)
	globalEventCh := make(chan handler.GlobalEvent, 10)
	wsHandler := handler.NewWebSocketHandler(threadMgr, snapshotMgr, globalEventCh)
	agentEventCh := make(chan *proto.AgentEvent, 100)
	agentHandler := handler.NewAgentHandler(threadMgr, agentEventCh)

	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, snapshotMgr, threadMgr, agentHandler)
	httpServer := &http.Server{Handler: mux}

	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen http: %v", err)
	}
	defer httpLis.Close()
	go httpServer.Serve(httpLis)

	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc: %v", err)
	}
	defer grpcLis.Close()
	grpcServer := grpc.NewServer()
	proto.RegisterFlowPartnerServiceServer(grpcServer, agentHandler)
	go grpcServer.Serve(grpcLis)

	wsURL := "ws://" + httpLis.Addr().String() + "/ws"
	var conn *websocket.Conn
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
	shutdown(grpcServer, httpServer, wsHandler, snapshotMgr, threadMgr)

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("shutdown took %v, expected < 2s", elapsed)
	}

	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err = client.Get("http://" + httpLis.Addr().String() + "/api/settings")
	if err == nil {
		t.Error("expected HTTP request to fail after shutdown")
	}

	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected websocket to be closed after shutdown")
	}
}

func TestShutdown_ForceStopsStuckGRPC(t *testing.T) {
	threadMgr := thread.NewManager()
	snapshotMgr := snapshot.NewManager(nil, nil)
	globalEventCh := make(chan handler.GlobalEvent, 10)
	wsHandler := handler.NewWebSocketHandler(threadMgr, snapshotMgr, globalEventCh)
	agentEventCh := make(chan *proto.AgentEvent, 100)
	agentHandler := handler.NewAgentHandler(threadMgr, agentEventCh)

	httpServer := &http.Server{Handler: http.NewServeMux()}
	httpLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen http: %v", err)
	}
	defer httpLis.Close()

	grpcLis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen grpc: %v", err)
	}
	defer grpcLis.Close()
	grpcServer := grpc.NewServer()
	proto.RegisterFlowPartnerServiceServer(grpcServer, agentHandler)
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
	shutdown(grpcServer, httpServer, wsHandler, snapshotMgr, threadMgr)

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

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
	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/handler"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/internal/tools"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestMain 将数据目录隔离到临时目录，避免与其他测试二进制并行写入 ~/.flowpartner 冲突
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

// TestReadySignal 验证就绪信号格式
func TestReadySignal(t *testing.T) {
	got := readySignal(8080, 50051)
	want := "__FP_BACKEND_READY__ HTTP=:8080 gRPC=:50051"
	if got != want {
		t.Errorf("readySignal = %q, want %q", got, want)
	}
}

// TestHTTPRoutes 验证 registerRoutes 注册的全部 REST 端点
func TestHTTPRoutes(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	mgr := bridge.NewManager()
	wsHandler := handler.NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)
	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, nil)

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
			var body *strings.Reader
			if tt.body == "" {
				body = strings.NewReader("")
			} else {
				body = strings.NewReader(tt.body)
			}
			req := httptest.NewRequest(tt.method, tt.path, body)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != tt.wantStatus {
				t.Errorf("%s %s = %d, want %d (body: %s)", tt.method, tt.path, rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

// TestHTTPRoutes_UnlockFlow 验证完整解锁流程：设置 API Key → 解锁 → 状态变化
func TestHTTPRoutes_UnlockFlow(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	mgr := bridge.NewManager()
	wsHandler := handler.NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)
	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, nil)

	// 设置 API Key
	putReq := httptest.NewRequest(http.MethodPut, "/api/settings",
		strings.NewReader(`{"model":"gpt-4","context_window":4096,"language":"zh-CN","api_key":"sk-test-key-abc","password":"TestPass123"}`))
	putRec := httptest.NewRecorder()
	mux.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("PUT settings = %d, want 200: %s", putRec.Code, putRec.Body.String())
	}

	// 锁定
	lockReq := httptest.NewRequest(http.MethodPost, "/api/lock", nil)
	lockRec := httptest.NewRecorder()
	mux.ServeHTTP(lockRec, lockReq)
	if lockRec.Code != http.StatusOK {
		t.Fatalf("POST lock = %d, want 200", lockRec.Code)
	}

	// 正确密码解锁
	unlockReq := httptest.NewRequest(http.MethodPost, "/api/unlock",
		strings.NewReader(`{"password":"TestPass123"}`))
	unlockRec := httptest.NewRecorder()
	mux.ServeHTTP(unlockRec, unlockReq)
	if unlockRec.Code != http.StatusOK {
		t.Fatalf("POST unlock = %d, want 200: %s", unlockRec.Code, unlockRec.Body.String())
	}

	// 状态应为已解锁
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

// TestShutdown_ClosesAllServers 验证 shutdown 按顺序关闭 gRPC → WebSocket → HTTP
func TestShutdown_ClosesAllServers(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := handler.NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	mux := http.NewServeMux()
	registerRoutes(mux, wsHandler, nil)
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
	proto.RegisterFlowPartnerServiceServer(grpcServer, handler.NewAgentHandler(mgr, tools.NewApprovalManager()))
	go grpcServer.Serve(grpcLis)

	// 建立 WebSocket 连接并注册 session（带重试等待 server 就绪）
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
	mgr.RegisterSession("sess_shutdown_test", conn)

	start := time.Now()
	shutdown(grpcServer, httpServer, mgr, wsHandler, nil)

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("shutdown took %v, expected < 2s", elapsed)
	}

	// HTTP server 应已关闭：请求应失败
	client := &http.Client{Timeout: 500 * time.Millisecond}
	_, err = client.Get("http://" + httpLis.Addr().String() + "/api/settings")
	if err == nil {
		t.Error("expected HTTP request to fail after shutdown")
	}

	// WebSocket 连接应已关闭
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Error("expected websocket to be closed after shutdown")
	}
}

// TestShutdown_ForceStopsStuckGRPC 验证 GracefulStop 超时 2 秒后强制停止（§5.5）
func TestShutdown_ForceStopsStuckGRPC(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := handler.NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

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
	proto.RegisterFlowPartnerServiceServer(grpcServer, handler.NewAgentHandler(mgr, tools.NewApprovalManager()))
	go grpcServer.Serve(grpcLis)

	// 建立 gRPC 双向流并保持打开，模拟 Python Agent 卡死
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
	shutdown(grpcServer, httpServer, mgr, wsHandler, nil)

	if elapsed := time.Since(start); elapsed > 4*time.Second {
		t.Fatalf("shutdown took %v, expected force stop within ~2s", elapsed)
	}

	// 强制停止后流应被终止：Recv 应在超时内返回错误
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

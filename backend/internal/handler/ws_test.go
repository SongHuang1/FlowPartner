package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/tools"
	"github.com/songhuang/flowpartner/backend/proto"
)

func TestWebSocketHandler_HandleWS_StartChat(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	// Connect WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// Send start_chat message
	sendMsg := map[string]string{
		"action":  "start_chat",
		"content": "Hello, AI!",
	}
	if err := conn.WriteJSON(sendMsg); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	// Wait for CmdChan to receive the message
	select {
	case cmd := <-mgr.CmdChan:
		if cmd.SessionId == "" {
			t.Error("SessionId should not be empty")
		}
		if cmd.CommandType != "start_chat" {
			t.Errorf("CommandType should be 'start_chat', got %q", cmd.CommandType)
		}
		if !strings.Contains(cmd.Payload, "Hello, AI!") {
			t.Errorf("Payload should contain user message, got %q", cmd.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for CmdChan message")
	}
}

func TestWebSocketHandler_HandleWS_SpecialChars(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// Send message with special characters that would break naive string concatenation
	sendMsg := map[string]string{
		"action":  "start_chat",
		"content": `He said "hello" and left \ end`,
	}
	if err := conn.WriteJSON(sendMsg); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	// Wait for CmdChan
	select {
	case cmd := <-mgr.CmdChan:
		// Verify the payload is valid JSON
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
			t.Errorf("Payload should be valid JSON, got %q", cmd.Payload)
		}
		if payload["user_message"] != `He said "hello" and left \ end` {
			t.Errorf("Payload message mismatch: got %q", payload["user_message"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for CmdChan message")
	}
}

// TestWebSocketHandler_HandleWS_SessionAndHistory 验证前端传入的 session_id 与 history 被透传到 Python
func TestWebSocketHandler_HandleWS_SessionAndHistory(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	sendMsg := map[string]interface{}{
		"action":     "start_chat",
		"content":    "第二个问题",
		"session_id": "sess_test_123",
		"history": []map[string]string{
			{"role": "user", "content": "第一个问题"},
			{"role": "assistant", "content": "第一个回答"},
		},
	}
	if err := conn.WriteJSON(sendMsg); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	select {
	case cmd := <-mgr.CmdChan:
		if cmd.SessionId != "sess_test_123" {
			t.Errorf("SessionId should be reused from frontend, got %q", cmd.SessionId)
		}
		var payload map[string]interface{}
		if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
			t.Fatalf("Payload should be valid JSON, got %q", cmd.Payload)
		}
		history, ok := payload["history"].([]interface{})
		if !ok || len(history) != 2 {
			t.Fatalf("history should contain 2 entries, got %#v", payload["history"])
		}
		first := history[0].(map[string]interface{})
		if first["role"] != "user" || first["content"] != "第一个问题" {
			t.Errorf("history[0] mismatch: %#v", first)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for CmdChan message")
	}
}

// TestWebSocketHandler_HandleWS_InvalidSessionID 验证非法 session_id 被拒绝（不进入 CmdChan）
func TestWebSocketHandler_HandleWS_InvalidSessionID(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	sendMsg := map[string]interface{}{
		"action":     "start_chat",
		"content":    "hello",
		"session_id": "../../etc/passwd",
	}
	if err := conn.WriteJSON(sendMsg); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	select {
	case cmd := <-mgr.CmdChan:
		t.Fatalf("invalid session_id should be rejected, got command: %+v", cmd)
	case <-time.After(500 * time.Millisecond):
		// 期望：消息被丢弃，CmdChan 无消息
	}
}

// TestWebSocketHandler_Close 验证 Close 后 HandleWS 循环确实退出
func TestWebSocketHandler_Close(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	handlerReturned := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsHandler.HandleWS(w, r)
		close(handlerReturned)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}

	// Register a session
	mgr.RegisterSession("test-session", conn)

	// Close the handler
	wsHandler.Close()

	select {
	case <-handlerReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleWS did not return after Close")
	}
}

// TestWebSocketHandler_Close_MessageInFlight 验证消息在途时 Close 不泄漏 goroutine
// 场景：读取 goroutine 已读到消息但尚未被主循环消费时触发 Close
func TestWebSocketHandler_Close_MessageInFlight(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	handlerReturned := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wsHandler.HandleWS(w, r)
		close(handlerReturned)
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}

	// 发送消息并消费，确保服务端进入稳定处理状态
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "in flight"}); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for CmdChan message")
	}

	// 立即发送第二条消息并马上 Close（消息在途时触发关闭）
	conn.WriteJSON(map[string]string{"action": "start_chat", "content": "second"})
	wsHandler.Close()

	select {
	case <-handlerReturned:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleWS did not return after Close")
	}

	// 等待读取 goroutine 退出（连接关闭 + done 信号双重保障）
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mgr.SessionCount() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("read goroutine leaked: session not unregistered after Close")
}

// TestWebSocketHandler_CmdChanFull_ErrorEvent 验证 CmdChan 满时向前端回发 error 事件
func TestWebSocketHandler_CmdChanFull_ErrorEvent(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	// 填满 CmdChan（无消费者）
	for i := 0; i < cap(mgr.CmdChan); i++ {
		mgr.CmdChan <- &proto.ServerCommand{SessionId: "dummy", CommandType: "start_chat"}
	}

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 发送 start_chat，CmdChan 已满 → 应回发 error 事件
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "hello"}); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("expected error event from server, got: %v", err)
	}

	var event map[string]interface{}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("error event is not valid JSON: %v", err)
	}
	if event["event_type"] != "error" {
		t.Errorf("expected event_type 'error', got %v", event["event_type"])
	}
	payload, ok := event["payload"].(string)
	if !ok || !strings.Contains(payload, "重试") {
		t.Errorf("expected error payload with retry hint, got %v", event["payload"])
	}
}

// TestWebSocketHandler_UnregistersSessionOnDisconnect 验证连接断开后 session 被注销
func TestWebSocketHandler_UnregistersSessionOnDisconnect(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}

	// 发送 start_chat 触发 session 注册
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "hi"}); err != nil {
		t.Fatalf("failed to write message: %v", err)
	}

	// 等待注册完成（CmdChan 收到消息时 RegisterSession 已执行）
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for CmdChan message")
	}
	if mgr.SessionCount() != 1 {
		t.Fatalf("expected 1 session registered, got %d", mgr.SessionCount())
	}

	// 断开连接
	conn.Close()

	// 等待注销完成
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if mgr.SessionCount() == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("session not unregistered after disconnect, %d remain", mgr.SessionCount())
}

// --- permission_response 测试 ---

func TestWebSocketHandler_PermissionResponse_Allow(t *testing.T) {
	am := tools.NewApprovalManager()
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, am, nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 先注册 session
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "init"}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// 创建一个审批请求
	requestID := am.Create("test-session-allow", "read", "/tmp/secret.txt", "/tmp/secret.txt")

	// 发送 permission_response (allow)
	if err := conn.WriteJSON(map[string]string{
		"action":     "permission_response",
		"session_id": "test-session-allow",
		"request_id": requestID,
		"decision":   "allow",
	}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// CmdChan 应收到 permission_response 命令
	select {
	case cmd := <-mgr.CmdChan:
		if cmd.CommandType != "permission_response" {
			t.Errorf("expected CommandType 'permission_response', got %q", cmd.CommandType)
		}
		if cmd.SessionId != "test-session-allow" {
			t.Errorf("expected SessionId 'test-session-allow', got %q", cmd.SessionId)
		}
		// 验证 payload 中包含 request_id 和 decision
		var payload map[string]string
		if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
			t.Fatalf("invalid payload: %v", err)
		}
		if payload["request_id"] != requestID {
			t.Errorf("expected request_id %q, got %q", requestID, payload["request_id"])
		}
		if payload["decision"] != "allow" {
			t.Errorf("expected decision 'allow', got %q", payload["decision"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for permission_response command")
	}

	// 验证审批已生效
	if !am.Resolve("test-session-allow", requestID, "allow") {
		t.Error("expected Resolve to succeed")
	}
}

func TestWebSocketHandler_PermissionResponse_Deny(t *testing.T) {
	am := tools.NewApprovalManager()
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, am, nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 注册 session
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "init"}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// 创建审批请求
	requestID := am.Create("test-session-deny", "write", "/tmp/secret.txt", "/tmp/secret.txt")

	// 发送 permission_response (deny)
	if err := conn.WriteJSON(map[string]string{
		"action":     "permission_response",
		"session_id": "test-session-deny",
		"request_id": requestID,
		"decision":   "deny",
	}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// CmdChan 应收到 permission_response 命令
	select {
	case cmd := <-mgr.CmdChan:
		if cmd.CommandType != "permission_response" {
			t.Errorf("expected CommandType 'permission_response', got %q", cmd.CommandType)
		}
		var payload map[string]string
		if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
			t.Fatalf("invalid payload: %v", err)
		}
		if payload["decision"] != "deny" {
			t.Errorf("expected decision 'deny', got %q", payload["decision"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for permission_response command")
	}

	// 验证审批已被拒绝
	if am.Consume("test-session-deny", requestID, "write", "/tmp/secret.txt") {
		t.Error("expected Consume to fail after deny")
	}
}

func TestWebSocketHandler_PermissionResponse_InvalidDecision(t *testing.T) {
	am := tools.NewApprovalManager()
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, am, nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 注册 session
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "init"}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// 发送无效 decision
	if err := conn.WriteJSON(map[string]string{
		"action":     "permission_response",
		"session_id": "test-session-invalid",
		"request_id": "some-request-id",
		"decision":   "maybe",
	}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// CmdChan 不应收到消息（无效 decision 被静默拒绝）
	select {
	case cmd := <-mgr.CmdChan:
		t.Fatalf("invalid decision should be rejected, got command: %+v", cmd)
	case <-time.After(500 * time.Millisecond):
		// 期望：无消息
	}
}

func TestWebSocketHandler_PermissionResponse_EmptyRequestID(t *testing.T) {
	am := tools.NewApprovalManager()
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, am, nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 注册 session
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "init"}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// 发送空 request_id
	if err := conn.WriteJSON(map[string]string{
		"action":     "permission_response",
		"session_id": "test-session-empty-rid",
		"request_id": "",
		"decision":   "allow",
	}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// 应被拒绝
	select {
	case cmd := <-mgr.CmdChan:
		t.Fatalf("empty request_id should be rejected, got command: %+v", cmd)
	case <-time.After(500 * time.Millisecond):
		// 期望：无消息
	}
}

// --- cancel_task 测试 ---

func TestWebSocketHandler_CancelTask_ForwardsToPython(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 注册 session
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "init"}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// 发送 cancel_task
	if err := conn.WriteJSON(map[string]string{
		"action":     "cancel_task",
		"session_id": "test-cancel-session",
	}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// CmdChan 应收到 cancel_task 命令
	select {
	case cmd := <-mgr.CmdChan:
		if cmd.CommandType != "cancel_task" {
			t.Errorf("expected CommandType 'cancel_task', got %q", cmd.CommandType)
		}
		if cmd.SessionId != "test-cancel-session" {
			t.Errorf("expected SessionId 'test-cancel-session', got %q", cmd.SessionId)
		}
		if cmd.Payload != "{}" {
			t.Errorf("expected Payload '{}', got %q", cmd.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for cancel_task command")
	}
}

func TestWebSocketHandler_CancelTask_InvalidSessionID(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr, tools.NewApprovalManager(), nil)

	server := httptest.NewServer(http.HandlerFunc(wsHandler.HandleWS))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close()

	// 注册 session
	if err := conn.WriteJSON(map[string]string{"action": "start_chat", "content": "init"}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	select {
	case <-mgr.CmdChan:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}

	// 发送非法 session_id 的 cancel_task
	if err := conn.WriteJSON(map[string]string{
		"action":     "cancel_task",
		"session_id": "../../etc/passwd",
	}); err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	// 应被拒绝，CmdChan 无 cancel_task 消息
	select {
	case cmd := <-mgr.CmdChan:
		if cmd.CommandType == "cancel_task" {
			t.Fatalf("invalid session_id cancel_task should be rejected, got: %+v", cmd)
		}
	case <-time.After(500 * time.Millisecond):
		// 期望：无消息
	}
}

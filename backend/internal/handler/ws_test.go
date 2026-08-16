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
	"github.com/songhuang/flowpartner/backend/proto"
)

func TestWebSocketHandler_HandleWS_StartChat(t *testing.T) {
	mgr := bridge.NewManager()
	wsHandler := NewWebSocketHandler(mgr)

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
	wsHandler := NewWebSocketHandler(mgr)

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
	wsHandler := NewWebSocketHandler(mgr)

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
	wsHandler := NewWebSocketHandler(mgr)

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
	wsHandler := NewWebSocketHandler(mgr)

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
	wsHandler := NewWebSocketHandler(mgr)

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
	wsHandler := NewWebSocketHandler(mgr)

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
	wsHandler := NewWebSocketHandler(mgr)

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

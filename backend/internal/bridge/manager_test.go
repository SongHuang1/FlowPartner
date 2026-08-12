package bridge

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/proto"
)

// newTestWSConn 建立到 echo 服务器的 WebSocket 连接
func newTestWSConn(t *testing.T, server *httptest.Server) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	return conn
}

func newEchoServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// newConnServer 建立 WebSocket 服务器，并通过通道暴露服务端连接（与生产环境的 RegisterSession 用法一致）
func newConnServer(t *testing.T) (*httptest.Server, chan *websocket.Conn) {
	t.Helper()
	connCh := make(chan *websocket.Conn, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		connCh <- conn
		defer conn.Close()
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}))
	t.Cleanup(server.Close)
	return server, connCh
}

// TestManager_SendToSession_EventFormat 验证事件序列化格式 {event_type, payload}
func TestManager_SendToSession_EventFormat(t *testing.T) {
	mgr := NewManager()
	server, serverConnCh := newConnServer(t)
	clientConn := newTestWSConn(t, server)
	defer clientConn.Close()
	serverConn := <-serverConnCh

	mgr.RegisterSession("sess_1", serverConn)

	event := &proto.AgentEvent{EventType: "llm_chunk", Payload: `{"text":"hello"}`}
	mgr.SendToSession("sess_1", event)

	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := clientConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}

	var msg struct {
		EventType string `json:"event_type"`
		Payload   string `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("event is not valid JSON: %v", err)
	}
	if msg.EventType != "llm_chunk" {
		t.Errorf("expected event_type 'llm_chunk', got %q", msg.EventType)
	}
	if msg.Payload != `{"text":"hello"}` {
		t.Errorf("payload mismatch, got %q", msg.Payload)
	}
}

// TestManager_SendToSession_Concurrent 验证并发写入同一连接不产生交错/竞态
func TestManager_SendToSession_Concurrent(t *testing.T) {
	mgr := NewManager()
	server, serverConnCh := newConnServer(t)
	clientConn := newTestWSConn(t, server)
	defer clientConn.Close()
	serverConn := <-serverConnCh

	mgr.RegisterSession("sess_1", serverConn)

	const writers = 4
	const perWriter = 50
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				payload := fmt.Sprintf(`{"writer":%d,"n":%d}`, id, j)
				mgr.SendToSession("sess_1", &proto.AgentEvent{EventType: "token", Payload: payload})
			}
		}(i)
	}

	// 并发读取全部消息，验证每条都是完整合法 JSON
	clientConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	total := writers * perWriter
	for i := 0; i < total; i++ {
		_, data, err := clientConn.ReadMessage()
		if err != nil {
			t.Fatalf("read %d/%d failed: %v", i, total, err)
		}
		var msg struct {
			EventType string `json:"event_type"`
			Payload   string `json:"payload"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("message %d interleaved/corrupted: %v (raw: %s)", i, err, string(data))
		}
		if msg.EventType != "token" {
			t.Errorf("message %d: unexpected event_type %q", i, msg.EventType)
		}
	}
	wg.Wait()

	if mgr.SessionCount() != 1 {
		t.Errorf("expected 1 session, got %d", mgr.SessionCount())
	}
}

func TestManager_CloseAllSessions(t *testing.T) {
	mgr := NewManager()

	server := newEchoServer(t)
	conn1 := newTestWSConn(t, server)
	conn2 := newTestWSConn(t, server)

	mgr.RegisterSession("sess_1", conn1)
	mgr.RegisterSession("sess_2", conn2)

	// Close all sessions
	mgr.CloseAllSessions()

	// Verify sessions are cleared
	mgr.mu.RLock()
	count := len(mgr.sessions)
	mgr.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 sessions after CloseAllSessions, got %d", count)
	}

	// Verify connections are closed (ReadMessage should error)
	_, _, err := conn1.ReadMessage()
	if err == nil {
		t.Error("expected error reading from closed conn1")
	}
	_, _, err = conn2.ReadMessage()
	if err == nil {
		t.Error("expected error reading from closed conn2")
	}
}

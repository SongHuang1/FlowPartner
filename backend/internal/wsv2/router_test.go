package wsv2

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func setupTestServer(t *testing.T, handlers map[string]HandlerFunc) (*httptest.Server, func()) {
	t.Helper()
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		router := NewRouter(conn)
		for method, fn := range handlers {
			router.RegisterMethod(method, fn)
		}
		router.Serve()
	}))

	return srv, func() { srv.Close() }
}

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func connectWS(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL(srv), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	return conn
}

func TestRouter_Handshake(t *testing.T) {
	srv, cleanup := setupTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, srv)
	defer conn.Close()

	// Send initialize
	req := Envelope{
		Id:     ptr(RequestIdFromString("a1")),
		Method: "initialize",
		Params: json.RawMessage(`{"clientInfo":{"name":"FlowPartner","title":"FlowPartner","version":"0.3.0"}}`),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatal(err)
	}

	var resp Envelope
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}

	if resp.Id == nil || resp.Id.Str != "a1" {
		t.Errorf("expected id=a1, got %+v", resp.Id)
	}
	if resp.Error != nil {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatal(err)
	}
	if result["protocolVersion"] != "2" {
		t.Errorf("protocolVersion = %s", result["protocolVersion"])
	}
	if result["userAgent"] != "flowpartner-backend" {
		t.Errorf("userAgent = %s", result["userAgent"])
	}

	// Send initialized notification
	notif := Envelope{
		Method: "initialized",
	}
	if err := conn.WriteJSON(notif); err != nil {
		t.Fatal(err)
	}

	// Should not receive any response for notification
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var noResp Envelope
	if err := conn.ReadJSON(&noResp); err == nil {
		t.Errorf("expected no response for notification, got %+v", noResp)
	}
}

func TestRouter_NotInitialized(t *testing.T) {
	srv, cleanup := setupTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, srv)
	defer conn.Close()

	// Send a request before initialize
	req := Envelope{
		Id:     ptr(RequestIdFromString("x1")),
		Method: "thread/start",
		Params: json.RawMessage(`{}`),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatal(err)
	}

	var resp Envelope
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error == nil {
		t.Fatal("expected error")
	}
	if resp.Error.Code != ErrNotInitialized {
		t.Errorf("expected code %d, got %d", ErrNotInitialized, resp.Error.Code)
	}
}

func TestRouter_DuplicateInitialize(t *testing.T) {
	srv, cleanup := setupTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, srv)
	defer conn.Close()

	// First initialize
	req := Envelope{
		Id:     ptr(RequestIdFromString("a1")),
		Method: "initialize",
		Params: json.RawMessage(`{"clientInfo":{"name":"FP","title":"FP","version":"0.3.0"}}`),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatal(err)
	}

	var resp Envelope
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("first initialize failed: %+v", resp.Error)
	}

	// Second initialize
	req2 := Envelope{
		Id:     ptr(RequestIdFromString("a2")),
		Method: "initialize",
		Params: json.RawMessage(`{"clientInfo":{"name":"FP","title":"FP","version":"0.3.0"}}`),
	}
	if err := conn.WriteJSON(req2); err != nil {
		t.Fatal(err)
	}

	var resp2 Envelope
	if err := conn.ReadJSON(&resp2); err != nil {
		t.Fatal(err)
	}
	if resp2.Error == nil {
		t.Fatal("expected error for duplicate initialize")
	}
	if resp2.Error.Code != ErrInvalidParams {
		t.Errorf("expected code %d, got %d", ErrInvalidParams, resp2.Error.Code)
	}
}

func TestRouter_UnknownMethod(t *testing.T) {
	srv, cleanup := setupTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, srv)
	defer conn.Close()

	// Complete handshake
	req := Envelope{
		Id:     ptr(RequestIdFromString("a1")),
		Method: "initialize",
		Params: json.RawMessage(`{"clientInfo":{"name":"FP","title":"FP","version":"0.3.0"}}`),
	}
	if err := conn.WriteJSON(req); err != nil {
		t.Fatal(err)
	}
	var resp Envelope
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}

	notif := Envelope{Method: "initialized"}
	if err := conn.WriteJSON(notif); err != nil {
		t.Fatal(err)
	}

	// Unknown method
	req2 := Envelope{
		Id:     ptr(RequestIdFromString("b1")),
		Method: "unknown/method",
		Params: json.RawMessage(`{}`),
	}
	if err := conn.WriteJSON(req2); err != nil {
		t.Fatal(err)
	}

	var resp2 Envelope
	if err := conn.ReadJSON(&resp2); err != nil {
		t.Fatal(err)
	}
	if resp2.Error == nil {
		t.Fatal("expected error")
	}
	if resp2.Error.Code != ErrMethodNotFound {
		t.Errorf("expected code %d, got %d", ErrMethodNotFound, resp2.Error.Code)
	}
}

func TestRouter_NotificationDroppedBeforeHandshake(t *testing.T) {
	srv, cleanup := setupTestServer(t, nil)
	defer cleanup()

	conn := connectWS(t, srv)
	defer conn.Close()

	// Send unknown notification before handshake - should be dropped silently
	notif := Envelope{
		Method: "unknown/event",
		Params: json.RawMessage(`{}`),
	}
	if err := conn.WriteJSON(notif); err != nil {
		t.Fatal(err)
	}

	// Should not receive any response
	conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var resp Envelope
	if err := conn.ReadJSON(&resp); err == nil {
		t.Errorf("expected no response, got %+v", resp)
	}
}

func TestRouter_CustomHandler(t *testing.T) {
	called := false
	handlers := map[string]HandlerFunc{
		"test/ping": func(conn *Conn, params json.RawMessage) (interface{}, *ErrorPayload) {
			called = true
			return map[string]string{"pong": "ok"}, nil
		},
	}

	srv, cleanup := setupTestServer(t, handlers)
	defer cleanup()

	conn := connectWS(t, srv)
	defer conn.Close()

	// Handshake
	if err := conn.WriteJSON(Envelope{
		Id:     ptr(RequestIdFromString("a1")),
		Method: "initialize",
		Params: json.RawMessage(`{"clientInfo":{"name":"FP","title":"FP","version":"0.3.0"}}`),
	}); err != nil {
		t.Fatal(err)
	}
	var resp Envelope
	if err := conn.ReadJSON(&resp); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(Envelope{Method: "initialized"}); err != nil {
		t.Fatal(err)
	}

	// Call custom handler
	if err := conn.WriteJSON(Envelope{
		Id:     ptr(RequestIdFromString("c1")),
		Method: "test/ping",
		Params: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatal(err)
	}

	var resp2 Envelope
	if err := conn.ReadJSON(&resp2); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("handler was not called")
	}
	if resp2.Error != nil {
		t.Errorf("unexpected error: %+v", resp2.Error)
	}
}

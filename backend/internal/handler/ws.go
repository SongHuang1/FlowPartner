package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/snapshot"
	"github.com/songhuang/flowpartner/backend/internal/thread"
	"github.com/songhuang/flowpartner/backend/internal/wsv2"
)

// GlobalEvent is a system-level event to broadcast to all frontends.
type GlobalEvent struct {
	EventType string
	Payload   string
}

// WebSocketHandler is the v2-only WebSocket handler.
type WebSocketHandler struct {
	threadMgr     *thread.Manager
	snapshotMgr   *snapshot.Manager
	globalEventCh chan<- GlobalEvent

	mu    sync.Mutex
	conns map[*wsConn]struct{}
}

type wsConn struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewWebSocketHandler creates a new v2 WebSocket handler.
func NewWebSocketHandler(threadMgr *thread.Manager, snapshotMgr *snapshot.Manager, globalEventCh chan<- GlobalEvent) *WebSocketHandler {
	return &WebSocketHandler{
		threadMgr:     threadMgr,
		snapshotMgr:   snapshotMgr,
		globalEventCh: globalEventCh,
		conns:         make(map[*wsConn]struct{}),
	}
}

// Close closes all connections.
func (h *WebSocketHandler) Close() {
	h.mu.Lock()
	conns := make([]*wsConn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.conns = make(map[*wsConn]struct{})
	h.mu.Unlock()
	for _, c := range conns {
		c.conn.Close()
	}
}

// BroadcastEvent sends a global event to all connected frontends.
func (h *WebSocketHandler) BroadcastEvent(eventType, payload string) {
	if h.globalEventCh != nil {
		select {
		case h.globalEventCh <- GlobalEvent{EventType: eventType, Payload: payload}:
		default:
		}
	}
}

// SnapshotStatusSink forwards snapshot status to all frontends.
func (h *WebSocketHandler) SnapshotStatusSink(status snapshot.Status) {
	data, err := json.Marshal(status)
	if err != nil {
		return
	}
	h.BroadcastEvent("snapshot_status", string(data))
}

// SnapshotMessageSink forwards snapshot messages to all frontends.
func (h *WebSocketHandler) SnapshotMessageSink(msg snapshot.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.BroadcastEvent("snapshot_message", string(data))
}

// StartBroadcastLoop starts a goroutine that broadcasts global events to all connections.
func (h *WebSocketHandler) StartBroadcastLoop(evtCh <-chan GlobalEvent) {
	for evt := range evtCh {
		h.mu.Lock()
		conns := make([]*wsConn, 0, len(h.conns))
		for c := range h.conns {
			conns = append(conns, c)
		}
		h.mu.Unlock()

		msg := map[string]string{"event_type": evt.EventType, "payload": evt.Payload}
		for _, c := range conns {
			c.mu.Lock()
			err := c.conn.WriteJSON(msg)
			c.mu.Unlock()
			if err != nil {
				log.Printf("WebSocket broadcast failed: %v", err)
			}
		}
	}
}

// HandleWS handles a WebSocket connection using the v2 envelope protocol.
func (h *WebSocketHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	conn.SetReadLimit(wsv2.MaxPayloadSize)

	ws := &wsConn{conn: conn}
	h.mu.Lock()
	h.conns[ws] = struct{}{}
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.conns, ws)
		h.mu.Unlock()
	}()

	log.Println("Frontend WebSocket connected (v2)")

	router := wsv2.NewRouter(conn)
	threadHandler := thread.NewHandler(h.threadMgr, thread.NewScheduler())

	for _, method := range threadHandler.MethodNames() {
		m := method
		router.RegisterMethod(m, func(conn *wsv2.Conn, params json.RawMessage) (interface{}, *wsv2.ErrorPayload) {
			result, herr := threadHandler.Dispatch(m, params)
			if herr != nil {
				return nil, &wsv2.ErrorPayload{Code: herr.Code, Message: herr.Message, Data: herr.Data}
			}
			return result, nil
		})
	}

	router.RegisterMethod("snapshot/trigger", h.handleSnapshotTrigger)
	router.RegisterMethod("snapshot/restore", h.handleSnapshotRestore)
	router.RegisterMethod("system/lock", h.handleSystemLock)

	router.SetThreadManager(h.threadMgr)
	router.Serve()
}

func (h *WebSocketHandler) handleSnapshotTrigger(conn *wsv2.Conn, params json.RawMessage) (interface{}, *wsv2.ErrorPayload) {
	if h.snapshotMgr != nil {
		h.snapshotMgr.TriggerManual()
	}
	return map[string]string{"status": "triggered"}, nil
}

func (h *WebSocketHandler) handleSnapshotRestore(conn *wsv2.Conn, params json.RawMessage) (interface{}, *wsv2.ErrorPayload) {
	var p struct {
		SnapshotID   string `json:"snapshotId"`
		DeleteExtras bool   `json:"deleteExtras"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &wsv2.ErrorPayload{Code: wsv2.ErrInvalidParams, Message: "参数格式错误"}
	}
	if h.snapshotMgr != nil && p.SnapshotID != "" {
		h.snapshotMgr.RestoreAsync(p.SnapshotID, p.DeleteExtras)
	}
	return map[string]string{"status": "restoring", "snapshotId": p.SnapshotID}, nil
}

func (h *WebSocketHandler) handleSystemLock(conn *wsv2.Conn, params json.RawMessage) (interface{}, *wsv2.ErrorPayload) {
	if h.snapshotMgr != nil {
		h.snapshotMgr.TriggerLock()
	}
	return map[string]string{"status": "locked"}, nil
}

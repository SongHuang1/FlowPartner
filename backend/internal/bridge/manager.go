package bridge

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/proto"
)

type Manager struct {
	// 发往 Python 的指令通道
	CmdChan  chan *proto.ServerCommand
	sessions map[string]*websocket.Conn

	connMu map[*websocket.Conn]*sync.Mutex
	mu     sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		CmdChan:  make(chan *proto.ServerCommand, 100),
		sessions: make(map[string]*websocket.Conn),
		connMu:   make(map[*websocket.Conn]*sync.Mutex),
	}
}

func (m *Manager) RegisterSession(sessionId string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connMu[conn] == nil {
		m.connMu[conn] = &sync.Mutex{}
	}
	m.sessions[sessionId] = conn
}

func (m *Manager) UnregisterSession(sessionId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn, existed := m.sessions[sessionId]
	delete(m.sessions, sessionId)
	if existed && !m.connStillInUse(conn) {
		delete(m.connMu, conn)
	}
}

func (m *Manager) connStillInUse(conn *websocket.Conn) bool {
	for _, c := range m.sessions {
		if c == conn {
			return true
		}
	}
	return false
}

func extractEventTypeAndPayload(event *proto.AgentEvent) (string, string) {
	switch p := event.Payload.(type) {
	case *proto.AgentEvent_TurnStarted:
		return "turn_started", mustMarshal(p.TurnStarted)
	case *proto.AgentEvent_TurnCompleted:
		return "turn_completed", mustMarshal(p.TurnCompleted)
	case *proto.AgentEvent_TurnAborted:
		return "turn_aborted", mustMarshal(p.TurnAborted)
	case *proto.AgentEvent_ItemStarted:
		return "item_started", mustMarshal(p.ItemStarted)
	case *proto.AgentEvent_ItemCompleted:
		return "item_completed", mustMarshal(p.ItemCompleted)
	case *proto.AgentEvent_ItemDelta:
		return "item_delta", mustMarshal(p.ItemDelta)
	case *proto.AgentEvent_UsageUpdate:
		return "usage_update", mustMarshal(p.UsageUpdate)
	case *proto.AgentEvent_PermissionRequest:
		return "permission_request", mustMarshal(p.PermissionRequest)
	case *proto.AgentEvent_Subagent:
		return p.Subagent.EventType, p.Subagent.Payload
	case *proto.AgentEvent_Error:
		return "error", mustMarshal(p.Error)
	default:
		return "unknown", "{}"
	}
}

func mustMarshal(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (m *Manager) SendToSession(sessionId string, event *proto.AgentEvent) {
	m.mu.RLock()
	conn, ok := m.sessions[sessionId]
	var writeMu *sync.Mutex
	if ok {
		writeMu = m.connMu[conn]
	}
	m.mu.RUnlock()

	if !ok || writeMu == nil {
		eventType, _ := extractEventTypeAndPayload(event)
		log.Printf("[Bridge] Session %s not found, dropping event: %s", sessionId, eventType)
		return
	}

	writeMu.Lock()
	defer writeMu.Unlock()

	eventType, payload := extractEventTypeAndPayload(event)
	wirePayload := map[string]interface{}{
		"event_type": eventType,
		"payload":    payload,
		"thread_id":  event.ThreadId,
		"turn_id":    event.TurnId,
	}
	if err := conn.WriteJSON(wirePayload); err != nil {
		log.Printf("Failed to send WebSocket message to frontend: %v", err)
	}
}

// CloseAllSessions 关闭所有活跃的 WebSocket 连接并清空 sessions map
func (m *Manager) CloseAllSessions() {
	m.mu.Lock()
	conns := make([]*websocket.Conn, 0, len(m.sessions))
	for id, conn := range m.sessions {
		conns = append(conns, conn)
		delete(m.sessions, id)
	}
	m.connMu = make(map[*websocket.Conn]*sync.Mutex)
	m.mu.Unlock()

	// 锁外关闭连接，避免网络操作阻塞其他 goroutine
	for _, conn := range conns {
		conn.Close()
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// SessionCount 返回当前活跃的 session 数量
func (m *Manager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

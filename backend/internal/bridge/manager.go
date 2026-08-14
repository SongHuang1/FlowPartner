package bridge

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/proto"
)

// Manager 负责桥接前端 WebSocket 和 Python gRPC
type Manager struct {
	// 发往 Python 的指令通道
	CmdChan chan *proto.ServerCommand

	// 记录 SessionID -> WebSocket 连接的映射
	sessions map[string]*websocket.Conn

	// 每个连接的写互斥锁（gorilla/websocket 不支持并发写）
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

// RegisterSession 前端建立连接时注册
func (m *Manager) RegisterSession(sessionId string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.connMu[conn] == nil {
		m.connMu[conn] = &sync.Mutex{}
	}
	m.sessions[sessionId] = conn
}

// UnregisterSession 前端断开连接时注销
func (m *Manager) UnregisterSession(sessionId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	conn, existed := m.sessions[sessionId]
	delete(m.sessions, sessionId)
	if existed && !m.connStillInUse(conn) {
		delete(m.connMu, conn)
	}
}

// connStillInUse 检查是否有其他 session 仍在使用该连接（调用方必须持有 m.mu）
func (m *Manager) connStillInUse(conn *websocket.Conn) bool {
	for _, c := range m.sessions {
		if c == conn {
			return true
		}
	}
	return false
}

// SendToSession 将 Python 的事件转发给指定的前端 WebSocket
func (m *Manager) SendToSession(sessionId string, event *proto.AgentEvent) {
	m.mu.RLock()
	conn, ok := m.sessions[sessionId]
	var writeMu *sync.Mutex
	if ok {
		writeMu = m.connMu[conn]
	}
	m.mu.RUnlock()

	if !ok || writeMu == nil {
		return
	}

	// 同一连接可能被多个 session 共享，多个 gRPC 事件循环可能并发写同一连接
	// gorilla/websocket 不支持并发写，必须串行化
	writeMu.Lock()
	defer writeMu.Unlock()

	// 将 protobuf 消息组装成前端易懂的 JSON 格式
	payload := map[string]interface{}{
		"event_type": event.EventType,
		"payload":    event.Payload, // 这里已经是 JSON 字符串了
	}
	if err := conn.WriteJSON(payload); err != nil {
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

// SessionCount 返回当前活跃的 session 数量
func (m *Manager) SessionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

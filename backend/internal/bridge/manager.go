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
	mu       sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		CmdChan:  make(chan *proto.ServerCommand, 100),
		sessions: make(map[string]*websocket.Conn),
	}
}

// RegisterSession 前端建立连接时注册
func (m *Manager) RegisterSession(sessionId string, conn *websocket.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[sessionId] = conn
}

// UnregisterSession 前端断开连接时注销
func (m *Manager) UnregisterSession(sessionId string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionId)
}

// SendToSession 将 Python 的事件转发给指定的前端 WebSocket
func (m *Manager) SendToSession(sessionId string, event *proto.AgentEvent) {
	m.mu.RLock()
	conn, ok := m.sessions[sessionId]
	m.mu.RUnlock()

	if ok {
		// 将 protobuf 消息组装成前端易懂的 JSON 格式
		payload := map[string]interface{}{
			"event_type": event.EventType,
			"payload":    event.Payload, // 这里已经是 JSON 字符串了
		}
		if err := conn.WriteJSON(payload); err != nil {
			log.Printf("向前端发送 WebSocket 消息失败: %v", err)
		}
	}
}

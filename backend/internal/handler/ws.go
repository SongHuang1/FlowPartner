package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/proto"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WebSocketHandler struct {
	manager *bridge.Manager
	done    chan struct{}
}

func NewWebSocketHandler(m *bridge.Manager) *WebSocketHandler {
	return &WebSocketHandler{
		manager: m,
		done:    make(chan struct{}),
	}
}

// Close 通知所有 HandleWS 循环退出
func (h *WebSocketHandler) Close() {
	close(h.done)
}

func (h *WebSocketHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	log.Println("Frontend WebSocket connected")

	type wsMessage struct {
		Action  string `json:"action"`
		Content string `json:"content"`
	}

	// 记录本连接注册的 session，退出时逐一注销，防止 sessions map 无限增长
	var sessionIDs []string
	defer func() {
		for _, id := range sessionIDs {
			h.manager.UnregisterSession(id)
		}
	}()

	readChan := make(chan wsMessage, 1)
	errChan := make(chan error, 1)

	// 读取 goroutine：将 ReadJSON 结果发送到 channel
	// 发送端 select 监听 h.done，确保消息在途时 Close() 也能退出，避免 goroutine 泄漏
	go func() {
		for {
			var msg wsMessage
			if err := conn.ReadJSON(&msg); err != nil {
				errChan <- err
				return
			}
			select {
			case readChan <- msg:
			case <-h.done:
				return
			}
		}
	}()

	for {
		select {
		case <-h.done:
			log.Println("WebSocket handler received shutdown signal, closing connection")
			return
		case err := <-errChan:
			if err != nil {
				log.Println("WebSocket read failed or frontend disconnected:", err)
			}
			return
		case msg := <-readChan:
			if msg.Action == "start_chat" {
				b := make([]byte, 16)
				if _, err := rand.Read(b); err != nil {
					log.Printf("Failed to generate session ID: %v", err)
					continue
				}
				sessionId := fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), hex.EncodeToString(b))
				sessionIDs = append(sessionIDs, sessionId)

				h.manager.RegisterSession(sessionId, conn)

				// 使用 json.Marshal 对用户输入进行正确的 JSON 转义，防止注入
				payloadBytes, err := json.Marshal(map[string]string{"user_message": msg.Content})
				if err != nil {
					log.Printf("JSON encoding failed: %v", err)
					continue
				}

				cmd := &proto.ServerCommand{
					SessionId:   sessionId,
					CommandType: "start_chat",
					Payload:     string(payloadBytes),
				}
				// 非阻塞发送：Python Agent 未连接（无消费者）时不阻塞 WebSocket handler
				select {
				case h.manager.CmdChan <- cmd:
					// 日志不记录聊天明文内容，仅记录长度（用户可能在消息中粘贴敏感信息）
					log.Printf("[Chat started] Session: %s | Content length: %d", sessionId, utf8.RuneCountInString(msg.Content))
				default:
					log.Printf("CmdChan full, dropping command: session=%s", sessionId)
					// 丢弃时必须通知前端，避免用户误以为消息已发送
					h.manager.SendToSession(sessionId, &proto.AgentEvent{
						EventType: "error",
						Payload:   `{"message": "Message sending failed: system is busy, please retry later"}`,
					})
				}
			}
		}
	}
}

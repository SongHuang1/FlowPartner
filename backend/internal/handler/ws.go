package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/proto"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// maxHistoryMessages 单次请求携带的历史消息上限，防止超大 payload 拖垮后端
const maxHistoryMessages = 100

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

	// 历史上下文可能较大，放宽默认 4096 字节的读限制
	conn.SetReadLimit(4 << 20) // 4MB

	log.Println("Frontend WebSocket connected")

	type wsHistoryMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	type wsMessage struct {
		Action    string             `json:"action"`
		Content   string             `json:"content"`
		SessionID string             `json:"session_id"`
		History   []wsHistoryMessage `json:"history"`
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
				// 会话 ID：前端在同一会话内复用；缺失时由后端生成（兼容旧客户端）
				sessionId := msg.SessionID
				if sessionId == "" {
					b := make([]byte, 16)
					if _, err := rand.Read(b); err != nil {
						log.Printf("Failed to generate session ID: %v", err)
						continue
					}
					sessionId = fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), hex.EncodeToString(b))
				} else if !storage.ValidSessionID(sessionId) {
					// 防御：非法 session_id 可能被 Python 用作文件名（路径遍历风险），直接丢弃
					log.Printf("Rejecting start_chat with invalid session ID: %q", sessionId)
					continue
				}
				sessionIDs = append(sessionIDs, sessionId)

				h.manager.RegisterSession(sessionId, conn)

				// 校验并过滤历史消息：仅保留合法 role 与非空内容，限制条数
				history := make([]wsHistoryMessage, 0, len(msg.History))
				for _, hm := range msg.History {
					if len(history) >= maxHistoryMessages {
						break
					}
					if hm.Role != "user" && hm.Role != "assistant" {
						continue
					}
					if strings.TrimSpace(hm.Content) == "" {
						continue
					}
					if utf8.RuneCountInString(hm.Content) > 10000 {
						continue
					}
					history = append(history, hm)
				}

				// 使用 json.Marshal 对用户输入进行正确的 JSON 转义，防止注入
				payloadBytes, err := json.Marshal(map[string]interface{}{
					"user_message": msg.Content,
					"history":      history,
				})
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
						Payload:   `{"message": "消息发送失败：系统繁忙，请稍后重试"}`,
					})
				}
			}
		}
	}
}

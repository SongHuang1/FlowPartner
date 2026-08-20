package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/snapshot"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/internal/tools"
	"github.com/songhuang/flowpartner/backend/proto"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const maxHistoryMessages = 100

type WebSocketHandler struct {
	manager         *bridge.Manager
	approvalManager *tools.ApprovalManager
	snapshotMgr     *snapshot.Manager
	done            chan struct{}

	// 所有已连接前端的连接集合（用于全局事件广播，如 snapshot_status）。
	connsMu   sync.Mutex
	conns     map[*websocket.Conn]struct{}
	connLocks map[*websocket.Conn]*sync.Mutex
}

func NewWebSocketHandler(m *bridge.Manager, am *tools.ApprovalManager, snapshotMgr *snapshot.Manager) *WebSocketHandler {
	return &WebSocketHandler{
		manager:         m,
		approvalManager: am,
		snapshotMgr:     snapshotMgr,
		done:            make(chan struct{}),
		conns:           make(map[*websocket.Conn]struct{}),
		connLocks:       make(map[*websocket.Conn]*sync.Mutex),
	}
}

// registerConn 注册新连接用于广播。
func (h *WebSocketHandler) registerConn(conn *websocket.Conn) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	h.conns[conn] = struct{}{}
	h.connLocks[conn] = &sync.Mutex{}
}

// unregisterConn 注销连接。
func (h *WebSocketHandler) unregisterConn(conn *websocket.Conn) {
	h.connsMu.Lock()
	defer h.connsMu.Unlock()
	delete(h.conns, conn)
	delete(h.connLocks, conn)
}

// Close 关闭 handler：停止广播并断开所有连接。
func (h *WebSocketHandler) Close() {
	close(h.done)
	h.connsMu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.conns = make(map[*websocket.Conn]struct{})
	h.connLocks = make(map[*websocket.Conn]*sync.Mutex)
	h.connsMu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}

// BroadcastEvent 向所有已连接前端广播一条 {event_type, payload} 事件。
// 每个连接使用互斥写串行化，避免并发写同一连接。
func (h *WebSocketHandler) BroadcastEvent(eventType, payload string) {
	h.connsMu.Lock()
	conns := make([]*websocket.Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.connsMu.Unlock()

	msg := map[string]string{"event_type": eventType, "payload": payload}
	for _, c := range conns {
		h.connsMu.Lock()
		mu := h.connLocks[c]
		h.connsMu.Unlock()
		if mu == nil {
			continue
		}
		mu.Lock()
		err := c.WriteJSON(msg)
		mu.Unlock()
		if err != nil {
			log.Printf("WebSocket broadcast failed: %v", err)
		}
	}
}

// snapshotStatusSink 将快照状态事件转发给所有前端。
func (h *WebSocketHandler) snapshotStatusSink(status snapshot.Status) {
	data, err := json.Marshal(status)
	if err != nil {
		log.Printf("序列化 snapshot_status 失败: %v", err)
		return
	}
	h.BroadcastEvent("snapshot_status", string(data))
}

// snapshotMessageSink 将快照消息（还原结果等）转发给所有前端。
func (h *WebSocketHandler) snapshotMessageSink(msg snapshot.Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("序列化 snapshot_message 失败: %v", err)
		return
	}
	h.BroadcastEvent("snapshot_message", string(data))
}

func (h *WebSocketHandler) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	conn.SetReadLimit(4 << 20) // 4MB
	h.registerConn(conn)
	defer h.unregisterConn(conn)

	log.Println("Frontend WebSocket connected")

	type wsHistoryToolCallFunction struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}

	type wsHistoryToolCall struct {
		ID       string                    `json:"id"`
		Type     string                    `json:"type"`
		Function wsHistoryToolCallFunction `json:"function"`
	}

	type wsHistoryMessage struct {
		Role       string              `json:"role"`
		Content    string              `json:"content"`
		ToolCalls  []wsHistoryToolCall `json:"tool_calls,omitempty"`
		ToolCallID string              `json:"tool_call_id,omitempty"`
		Name       string              `json:"name,omitempty"`
	}

	type wsMessage struct {
		Action       string             `json:"action"`
		Content      string             `json:"content"`
		SessionID    string             `json:"session_id"`
		History      []wsHistoryMessage `json:"history"`
		RequestID    string             `json:"request_id"`
		Decision     string             `json:"decision"`
		Scope        string             `json:"scope"`
		SnapshotID   string             `json:"snapshot_id"`
		DeleteExtras bool               `json:"delete_extras"`
	}

	var sessionIDs []string
	defer func() {
		for _, id := range sessionIDs {
			h.approvalManager.CancelSession(id)
			h.manager.UnregisterSession(id)
		}
	}()

	readChan := make(chan wsMessage, 1)
	errChan := make(chan error, 1)

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
			switch msg.Action {
			case "manual_snapshot":
				if h.snapshotMgr != nil {
					log.Println("[Snapshot] 手动快照触发")
					h.snapshotMgr.TriggerManual()
				}

			case "system_lock":
				if h.snapshotMgr != nil {
					log.Println("[Snapshot] 系统锁屏，触发 flush 快照")
					h.snapshotMgr.TriggerLock()
				}

			case "restore":
				if h.snapshotMgr != nil {
					log.Printf("[Snapshot] 还原指令: snapshot_id=%q delete_extras=%v", msg.SnapshotID, msg.DeleteExtras)
					if msg.SnapshotID != "" && storage.ValidSnapshotID(msg.SnapshotID) {
						h.snapshotMgr.RestoreAsync(msg.SnapshotID, msg.DeleteExtras)
					} else {
						h.BroadcastEvent("snapshot_message", `{"type":"error","text":"还原失败：快照编号无效"}`)
					}
				}

			case "start_chat":
				sessionId := msg.SessionID
				if sessionId == "" {
					b := make([]byte, 16)
					if _, err := rand.Read(b); err != nil {
						log.Printf("Failed to generate session ID: %v", err)
						continue
					}
					sessionId = fmt.Sprintf("sess_%d_%s", time.Now().UnixNano(), hex.EncodeToString(b))
				} else if !storage.ValidSessionID(sessionId) {
					log.Printf("Rejecting start_chat with invalid session ID: %q", sessionId)
					continue
				}
				sessionIDs = append(sessionIDs, sessionId)

				h.manager.RegisterSession(sessionId, conn)

				history := make([]wsHistoryMessage, 0, len(msg.History))
				for _, hm := range msg.History {
					if len(history) >= maxHistoryMessages {
						break
					}
					if hm.Role != "user" && hm.Role != "assistant" && hm.Role != "tool" {
						continue
					}
					if hm.Role != "tool" && strings.TrimSpace(hm.Content) == "" {
						continue
					}
					if hm.Role != "tool" && utf8.RuneCountInString(hm.Content) > 10000 {
						continue
					}
					history = append(history, hm)
				}

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
				select {
				case h.manager.CmdChan <- cmd:
					log.Printf("[Chat started] Session: %s | Content length: %d", sessionId, utf8.RuneCountInString(msg.Content))
				default:
					log.Printf("CmdChan full, dropping command: session=%s", sessionId)
					h.manager.SendToSession(sessionId, &proto.AgentEvent{
						EventType: "error",
						Payload:   `{"message": "消息发送失败：系统繁忙，请稍后重试"}`,
					})
				}

			case "permission_response":
				sessionId := msg.SessionID
				requestID := msg.RequestID
				decision := msg.Decision

				if !storage.ValidSessionID(sessionId) || requestID == "" {
					log.Printf("Rejecting permission_response with invalid params: session=%q request_id=%q", sessionId, requestID)
					continue
				}
				if decision != "allow" && decision != "deny" {
					log.Printf("Rejecting permission_response with invalid decision: %q", decision)
					continue
				}

				if !h.approvalManager.Resolve(sessionId, requestID, decision) {
					log.Printf("permission_response resolve failed: session=%s request_id=%s", sessionId, requestID)
					continue
				}

				if decision == "allow" && msg.Scope == "session" {
					if toolName, resolvedPath, ok := h.approvalManager.GetApproval(sessionId, requestID); ok {
						h.approvalManager.AddTrust(sessionId, toolName, resolvedPath)
					} else {
						log.Printf("permission_response: cannot get approval details for trust, request_id=%s", requestID)
					}
				}

				payloadBytes, _ := json.Marshal(map[string]string{
					"request_id": requestID,
					"decision":   decision,
				})
				cmd := &proto.ServerCommand{
					SessionId:   sessionId,
					CommandType: "permission_response",
					Payload:     string(payloadBytes),
				}
				select {
				case h.manager.CmdChan <- cmd:
					log.Printf("[Permission] %s request_id=%s session=%s scope=%s", decision, requestID, sessionId, msg.Scope)
				default:
					log.Printf("CmdChan full, dropping permission_response: session=%s request_id=%s", sessionId, requestID)
				}

			case "cancel_task":
				sessionId := msg.SessionID
				if !storage.ValidSessionID(sessionId) {
					log.Printf("Rejecting cancel_task with invalid session ID: %q", sessionId)
					continue
				}

				cmd := &proto.ServerCommand{
					SessionId:   sessionId,
					CommandType: "cancel_task",
					Payload:     "{}",
				}
				select {
				case h.manager.CmdChan <- cmd:
					log.Printf("[Cancel] Task cancelled: session=%s", sessionId)
				default:
					log.Printf("CmdChan full, dropping cancel_task: session=%s", sessionId)
				}

			default:
				log.Printf("Unknown action: %q, dropping", msg.Action)
			}
		}
	}
}

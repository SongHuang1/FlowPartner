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
	done            chan struct{}
}

func NewWebSocketHandler(m *bridge.Manager, am *tools.ApprovalManager) *WebSocketHandler {
	return &WebSocketHandler{
		manager:         m,
		approvalManager: am,
		done:            make(chan struct{}),
	}
}

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

	conn.SetReadLimit(4 << 20) // 4MB

	log.Println("Frontend WebSocket connected")

	type wsHistoryToolCallFunction struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	}

	type wsHistoryToolCall struct {
		ID       string                   `json:"id"`
		Type     string                   `json:"type"`
		Function wsHistoryToolCallFunction `json:"function"`
	}

	type wsHistoryMessage struct {
		Role       string              `json:"role"`
		Content    string              `json:"content"`
		ToolCalls  []wsHistoryToolCall `json:"tool_calls,omitempty"`
		ToolCallID string             `json:"tool_call_id,omitempty"`
		Name       string             `json:"name,omitempty"`
	}

	type wsMessage struct {
		Action    string             `json:"action"`
		Content   string             `json:"content"`
		SessionID string             `json:"session_id"`
		History   []wsHistoryMessage `json:"history"`
		RequestID string             `json:"request_id"`
		Decision  string             `json:"decision"`
		Scope     string             `json:"scope"`
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

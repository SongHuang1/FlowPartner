package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// Message 单条消息
type Message struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
}

// Conversation 对话结构
type Conversation struct {
	Messages  []Message `json:"messages"`
	UpdatedAt int64     `json:"updated_at"`
}

// SaveConversationRequest POST 请求体
type SaveConversationRequest struct {
	Messages  []Message `json:"messages"`
	UpdatedAt int64     `json:"updated_at"`
}

// EmptyConversation 返回空对话
func EmptyConversation() Conversation {
	return Conversation{
		Messages:  []Message{},
		UpdatedAt: 0,
	}
}

// ConversationHandler 处理对话相关请求
type ConversationHandler struct{}

// Get 返回当前对话（文件不存在则返回空对话）
func (h *ConversationHandler) Get(w http.ResponseWriter, r *http.Request) {
	var conv Conversation
	err := storage.ReadJSON("conversations.json", &conv)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			response.WriteJSON(w, http.StatusOK, response.Success(EmptyConversation()))
			return
		}
		log.Printf("Warning: conversations.json parse error, using empty: %v", err)
		response.WriteJSON(w, http.StatusOK, response.Success(EmptyConversation()))
		return
	}
	if conv.Messages == nil {
		conv.Messages = []Message{}
	}
	response.WriteJSON(w, http.StatusOK, response.Success(conv))
}

// Post 接收完整消息列表，覆盖写入 conversations.json
func (h *ConversationHandler) Post(w http.ResponseWriter, r *http.Request) {
	var req SaveConversationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}
	for i, msg := range req.Messages {
		if strings.TrimSpace(msg.Content) == "" {
			response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, fmt.Sprintf("message[%d] content cannot be empty", i)))
			return
		}
		if utf8.RuneCountInString(msg.Content) > 10000 {
			response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, fmt.Sprintf("message[%d] content exceeds 10000 chars", i)))
			return
		}
		if msg.Role != "user" && msg.Role != "assistant" {
			response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, fmt.Sprintf("message[%d] invalid role: %s", i, msg.Role)))
			return
		}
	}
	conv := Conversation{
		Messages:  req.Messages,
		UpdatedAt: req.UpdatedAt,
	}
	if err := storage.WriteJSON("conversations.json", conv); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError, response.Error(response.CodeInternalError, "Failed to save conversation"))
		return
	}
	response.WriteJSON(w, http.StatusOK, response.Success(conv))
}

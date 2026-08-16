package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// HistoryHandler 处理历史记录相关请求
type HistoryHandler struct{}

// Handle 根据 HTTP 方法分发
func (h *HistoryHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.Get(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// Get 返回历史会话列表（/api/history）或单个会话（/api/history/{session_id}）
func (h *HistoryHandler) Get(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/history"), "/")
	if path == "" {
		entries, err := storage.ListHistory()
		if err != nil {
			response.WriteJSON(w, http.StatusInternalServerError, response.Error(response.CodeInternalError, "Failed to list history"))
			return
		}
		response.WriteJSON(w, http.StatusOK, response.Success(entries))
		return
	}

	sessionID := path
	if !storage.ValidSessionID(sessionID) {
		response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, "Invalid session id"))
		return
	}
	msgs, err := storage.ReadHistory(sessionID)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			response.WriteJSON(w, http.StatusNotFound, response.Error(response.CodeInvalidParam, "Session not found"))
			return
		}
		response.WriteJSON(w, http.StatusInternalServerError, response.Error(response.CodeInternalError, "Failed to read history"))
		return
	}
	response.WriteJSON(w, http.StatusOK, response.Success(map[string]interface{}{
		"session_id": sessionID,
		"messages":   msgs,
	}))
}
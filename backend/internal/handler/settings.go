package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// Settings 用户设置
type Settings struct {
	Model            string `json:"model"`
	AgentID          string `json:"agent_id"`
	ContextWindow    int    `json:"context_window"`
	WorkingDirectory string `json:"working_directory"`
	Language         string `json:"language"`
}

// DefaultSettings 返回默认设置
func DefaultSettings() Settings {
	return Settings{
		Model:            "gpt-4",
		AgentID:          "default",
		ContextWindow:    8192,
		WorkingDirectory: "",
		Language:         "zh-CN",
	}
}

// SettingsHandler 处理设置相关请求
type SettingsHandler struct{}

// Get 返回当前设置（文件不存在则返回默认值）
func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	var settings Settings
	err := storage.ReadJSON("settings.json", &settings)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			response.WriteJSON(w, http.StatusOK, response.Success(DefaultSettings()))
			return
		}
		log.Printf("Warning: settings.json parse error, using defaults: %v", err)
		response.WriteJSON(w, http.StatusOK, response.Success(DefaultSettings()))
		return
	}
	response.WriteJSON(w, http.StatusOK, response.Success(settings))
}

// Put 接收完整 Settings JSON，覆盖写入 settings.json
func (h *SettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var settings Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}
	if strings.TrimSpace(settings.Model) == "" {
		response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, "model cannot be empty"))
		return
	}
	if settings.ContextWindow <= 0 {
		response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, "context_window must be positive"))
		return
	}
	if strings.TrimSpace(settings.Language) == "" {
		response.WriteJSON(w, http.StatusBadRequest, response.Error(response.CodeInvalidParam, "language cannot be empty"))
		return
	}
	if err := storage.WriteJSON("settings.json", settings); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError, response.Error(response.CodeInternalError, "Failed to save settings"))
		return
	}
	response.WriteJSON(w, http.StatusOK, response.Success(settings))
}

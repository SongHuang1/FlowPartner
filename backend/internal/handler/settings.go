package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

type Settings struct {
	Model            string  `json:"model"`
	AgentID          string  `json:"agent_id"`
	ContextWindow    int     `json:"context_window"`
	WorkingDirectory string  `json:"working_directory"`
	Language         string  `json:"language"`

	BaseURL         string `json:"base_url"`
	EncryptedAPIKey string `json:"encrypted_api_key"`
	ModelName       string `json:"model_name"`

	SystemPrompt string  `json:"system_prompt"`
	Temperature  float64 `json:"temperature"`

	CloseBehavior   string `json:"close_behavior"`
	CloseRemembered bool   `json:"close_remembered"`

	WindowX        int    `json:"window_x"`
	WindowY        int    `json:"window_y"`
	WindowWidth    int    `json:"window_width"`
	WindowHeight   int    `json:"window_height"`
	SidebarVisible bool   `json:"sidebar_visible"`
	SidebarView    string `json:"sidebar_view"`
}

func DefaultSettings() Settings {
	return Settings{
		Model:            "gpt-4",
		AgentID:          "default",
		ContextWindow:    8192,
		WorkingDirectory: "",
		Language:         "zh-CN",
		BaseURL:          "https://api.openai.com/v1",
		ModelName:        "gpt-4",
		SystemPrompt:     "你是一个有帮助的 AI 助手。",
		Temperature:      0.7,
		CloseBehavior:    "ask",
		CloseRemembered:  false,
		WindowX:          100,
		WindowY:          100,
		WindowWidth:      1200,
		WindowHeight:     800,
		SidebarVisible:   true,
		SidebarView:      "conversation",
	}
}

// LoadSettings 读取设置，对缺失字段使用默认值填充
func LoadSettings() Settings {
	var settings Settings
	err := storage.ReadJSON("settings.json", &settings)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return DefaultSettings()
		}
		log.Printf("Warning: settings.json parse error: %v", err)
		return DefaultSettings()
	}

	defaults := DefaultSettings()
	isOldConfig := settings.WindowWidth == 0

	if settings.BaseURL == "" {
		settings.BaseURL = defaults.BaseURL
	}
	if settings.ModelName == "" {
		settings.ModelName = defaults.ModelName
	}
	if settings.SystemPrompt == "" {
		settings.SystemPrompt = defaults.SystemPrompt
	}
	if settings.Temperature == 0 {
		settings.Temperature = defaults.Temperature
	}
	if settings.CloseBehavior == "" {
		settings.CloseBehavior = defaults.CloseBehavior
	}
	if isOldConfig {
		settings.WindowX = defaults.WindowX
		settings.WindowY = defaults.WindowY
		settings.WindowWidth = defaults.WindowWidth
		settings.WindowHeight = defaults.WindowHeight
		settings.SidebarVisible = defaults.SidebarVisible
		settings.SidebarView = defaults.SidebarView
	}

	return settings
}

type SettingsHandler struct{}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings := LoadSettings()
	response.WriteJSON(w, http.StatusOK, response.Success(settings))
}

func (h *SettingsHandler) Put(w http.ResponseWriter, r *http.Request) {
	var rawReq map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&rawReq); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}

	settingsJSON, _ := json.Marshal(rawReq)
	var settings Settings
	if err := json.Unmarshal(settingsJSON, &settings); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid settings format"))
		return
	}

	// 保留已有的 encrypted_api_key（当 api_key 为空或未提供时）
	// 使用类型安全的值检查：api_key: null 和 api_key: "" 都视为"未提供"
	apiKeyVal, _ := rawReq["api_key"].(string)
	if apiKeyVal == "" {
		existing := LoadSettings()
		settings.EncryptedAPIKey = existing.EncryptedAPIKey
	}

	if settings.BaseURL != "" {
		if !strings.HasPrefix(settings.BaseURL, "http://") && !strings.HasPrefix(settings.BaseURL, "https://") {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "base_url must start with http:// or https://"))
			return
		}
		if isInternalURL(settings.BaseURL) {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "base_url must not point to internal/private network"))
			return
		}
	}
	if settings.Temperature < 0 || settings.Temperature > 2.0 {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "temperature must be between 0.0 and 2.0"))
		return
	}
	if settings.CloseBehavior != "" {
		validBehaviors := []string{"minimize", "quit", "ask"}
		if !containsString(validBehaviors, settings.CloseBehavior) {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "close_behavior must be minimize, quit, or ask"))
			return
		}
	}
	if strings.TrimSpace(settings.Model) == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "model cannot be empty"))
		return
	}
	if settings.ContextWindow <= 0 {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "context_window must be positive"))
		return
	}
	if strings.TrimSpace(settings.Language) == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "language cannot be empty"))
		return
	}

	apiKey, hasAPIKey := rawReq["api_key"].(string)
	password, hasPassword := rawReq["password"].(string)

	if hasAPIKey && apiKey != "" {
		if !hasPassword || password == "" {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "password is required when setting API Key"))
			return
		}
		if !isStrongPassword(password) {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "password must be at least 8 characters with uppercase, lowercase, and digit"))
			return
		}
		encrypted, err := flowcrypto.Encrypt(apiKey, []byte(password))
		if err != nil {
			response.WriteJSON(w, http.StatusInternalServerError,
				response.Error(response.CodeInternalError, "Failed to encrypt API Key"))
			return
		}
		settings.EncryptedAPIKey = encrypted

		ks := keystore.Instance()
		ks.SetAPIKeyConfigured(true)
		ks.Unlock([]byte(apiKey))

		// 零化密码字节（仅零化 []byte 转换副本）
		// 注意：Go string 不可变，原始 password 字符串仍驻留内存直到 GC 回收
		// 这是语言层面的限制，无法在代码层面完全避免
		defer flowcrypto.ZeroBytes([]byte(password))
	}

	if err := storage.WriteJSON("settings.json", settings); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "Failed to save settings"))
		return
	}

	response.WriteJSON(w, http.StatusOK, response.Success(settings))
}

// ClearAPIKey 清除 API Key（用户主动清除，需先解锁）
func (h *SettingsHandler) ClearAPIKey(w http.ResponseWriter, r *http.Request) {
	ks := keystore.Instance()
	settings := LoadSettings()

	// 清除加密的 API Key
	settings.EncryptedAPIKey = ""

	// 锁定 KeyStore
	ks.Lock()
	ks.SetAPIKeyConfigured(false)

	if err := storage.WriteJSON("settings.json", settings); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "Failed to clear API Key"))
		return
	}

	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "API Key 已清除",
	}))
}

func isInternalURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	hostname := parsed.Hostname()

	// 无主机名的 URL（如 "not-a-url"）视为不安全
	if hostname == "" {
		return true
	}

	if ip := net.ParseIP(hostname); ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return true
		}
	}

	internalHosts := []string{"localhost", "metadata.google.internal", "169.254.169.254"}
	for _, h := range internalHosts {
		if strings.EqualFold(hostname, h) {
			return true
		}
	}

	return false
}

func isStrongPassword(password string) bool {
	if len(password) < 8 {
		return false
	}
	hasUpper := false
	hasLower := false
	hasDigit := false
	for _, c := range password {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	return hasUpper && hasLower && hasDigit
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

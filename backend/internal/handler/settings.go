package handler

import (
	"encoding/json"
	"errors"
	"fmt"
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

type ModelConfig struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	BaseURL         string  `json:"base_url"`
	ModelName       string  `json:"model_name"`
	EncryptedAPIKey string  `json:"encrypted_api_key"`
	Temperature     float64 `json:"temperature"`
	ResponseFormat  string  `json:"response_format"`
	TimeoutSecs     int     `json:"timeout_secs"`
}

type Settings struct {
	Model            string  `json:"model"`
	AgentID          string  `json:"agent_id"`
	ContextWindow    int     `json:"context_window"`
	WorkingDirectory string  `json:"working_directory"`
	Language         string  `json:"language"`

	BaseURL         string `json:"base_url"`
	EncryptedAPIKey string `json:"encrypted_api_key"`
	ModelName       string `json:"model_name"`

	ModelConfigs   []ModelConfig `json:"model_configs"`
	ActiveConfigID string        `json:"active_config_id"`

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
		ModelConfigs:     []ModelConfig{},
		ActiveConfigID:   "",
		SystemPrompt:     "You are a helpful AI assistant.",
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

// LoadSettings 读取设置，对缺失字段使用默认值填充，自动迁移旧格式
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
	isOldConfig := settings.AgentID == "" && settings.WindowWidth == 0

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

	settings.migrateOldConfig()
	settings.deriveFlatFields()

	return settings
}

// migrateOldConfig 将旧格式扁平字段迁移为 ModelConfigs 数组
func (s *Settings) migrateOldConfig() {
	if len(s.ModelConfigs) > 0 {
		return
	}
	if s.BaseURL == "" && s.ModelName == "" {
		return
	}

	migrated := ModelConfig{
		ID:              "default",
		Name:            "Default configuration",
		BaseURL:         s.BaseURL,
		ModelName:       s.ModelName,
		EncryptedAPIKey: s.EncryptedAPIKey,
		Temperature:     s.Temperature,
		ResponseFormat:  "text",
		TimeoutSecs:     30,
	}
	s.ModelConfigs = []ModelConfig{migrated}
	s.ActiveConfigID = migrated.ID
}

// deriveFlatFields 从 ModelConfigs[active] 派生旧扁平字段，确保一致性
func (s *Settings) deriveFlatFields() {
	cfg := s.activeConfig()
	if cfg == nil {
		s.BaseURL = ""
		s.ModelName = ""
		s.EncryptedAPIKey = ""
		return
	}
	s.BaseURL = cfg.BaseURL
	s.ModelName = cfg.ModelName
	s.EncryptedAPIKey = cfg.EncryptedAPIKey
}

// activeConfig 返回当前激活的配置
func (s *Settings) activeConfig() *ModelConfig {
	if s.ActiveConfigID == "" {
		return nil
	}
	for i := range s.ModelConfigs {
		if s.ModelConfigs[i].ID == s.ActiveConfigID {
			return &s.ModelConfigs[i]
		}
	}
	return nil
}

// GetModelConfigByID 根据 ID 查找配置
func (s *Settings) GetModelConfigByID(id string) *ModelConfig {
	for i := range s.ModelConfigs {
		if s.ModelConfigs[i].ID == id {
			return &s.ModelConfigs[i]
		}
	}
	return nil
}

// ValidateBaseURL 校验 BaseURL 格式并防止 SSRF
func ValidateBaseURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return fmt.Errorf("base_url must start with http:// or https://")
	}
	if isInternalURL(rawURL) {
		return fmt.Errorf("base_url must not point to internal/private network")
	}
	return nil
}

type SettingsHandler struct{}

// Handle 根据 HTTP 方法分发到 Get/Put
func (h *SettingsHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.Get(w, r)
	case http.MethodPut:
		h.Put(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleClearAPIKey 校验 POST 方法后清除 API Key
func (h *SettingsHandler) HandleClearAPIKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.ClearAPIKey(w, r)
}

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

	password, _ := rawReq["password"].(string)
	passwordCopy := []byte(password)
	defer flowcrypto.ZeroBytes(passwordCopy)

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
		if err := ValidateBaseURL(settings.BaseURL); err != nil {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, err.Error()))
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
	}

	if len(settings.ModelConfigs) > 0 {
		existing := LoadSettings()
		settings.ModelConfigs = mergeModelConfigs(existing.ModelConfigs, settings.ModelConfigs)
		settings.deriveFlatFields()
	}

	if err := storage.WriteJSON("settings.json", settings); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "Failed to save settings"))
		return
	}

	response.WriteJSON(w, http.StatusOK, response.Success(settings))
}

// mergeModelConfigs 按 ID 合并配置：已存在的更新，不存在的新增，未传的保留
// 安全注意：若 incoming 配置的 EncryptedAPIKey 为空，保留 existing 的加密密钥
func mergeModelConfigs(existing, incoming []ModelConfig) []ModelConfig {
	configMap := make(map[string]ModelConfig, len(existing))
	for _, cfg := range existing {
		configMap[cfg.ID] = cfg
	}
	for _, cfg := range incoming {
		if cfg.EncryptedAPIKey == "" {
			if existingCfg, ok := configMap[cfg.ID]; ok {
				cfg.EncryptedAPIKey = existingCfg.EncryptedAPIKey
			}
		}
		configMap[cfg.ID] = cfg
	}

	result := make([]ModelConfig, 0, len(configMap))
	for _, cfg := range existing {
		if merged, ok := configMap[cfg.ID]; ok {
			result = append(result, merged)
			delete(configMap, cfg.ID)
		}
	}
	for _, cfg := range incoming {
		if _, ok := configMap[cfg.ID]; ok {
			result = append(result, cfg)
			delete(configMap, cfg.ID)
		}
	}
	return result
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
		"message": "API Key cleared",
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

	if parsed.User != nil {
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

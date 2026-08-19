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
		SystemPrompt:     "你是一个乐于助人的 AI 助手。",
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

	// 旧格式迁移必须在默认值填充之前执行：
	// 若先填充 base_url/model_name 默认值，删除全部配置或清除 API Key 后，
	// 下次加载会用默认值凭空重新迁移出一个 "default" 配置
	settings.migrateOldConfig()

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

	settings.deriveFlatFields()

	// 自动修复：若 ActiveConfigID 无效或指向无密钥的配置，自动激活第一个有密钥的配置
	if settings.ActiveConfigID == "" || settings.activeConfig() == nil {
		if cfg := settings.firstConfigWithKey(); cfg != nil {
			settings.ActiveConfigID = cfg.ID
			settings.deriveFlatFields()
		}
	}

	return settings
}

// migrateOldConfig 将旧格式扁平字段迁移为 ModelConfigs 数组
func (s *Settings) migrateOldConfig() {
	if len(s.ModelConfigs) > 0 {
		return
	}
	// 仅当确实存在旧格式数据时才迁移（扁平字段或扁平密钥任一非空），
	// 避免删除全部配置 / 清除 API Key 后凭空重新生成 "default" 配置
	if s.BaseURL == "" && s.ModelName == "" && s.EncryptedAPIKey == "" {
		return
	}

	migrated := ModelConfig{
		ID:              "default",
		Name:            "默认配置",
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
		s.Model = ""
		s.BaseURL = ""
		s.ModelName = ""
		s.EncryptedAPIKey = ""
		return
	}
	s.Model = cfg.ModelName
	s.BaseURL = cfg.BaseURL
	s.ModelName = cfg.ModelName
	s.EncryptedAPIKey = cfg.EncryptedAPIKey
}

// activeConfig 返回当前激活的配置（必须有加密密钥）
func (s *Settings) activeConfig() *ModelConfig {
	if s.ActiveConfigID != "" {
		for i := range s.ModelConfigs {
			if s.ModelConfigs[i].ID == s.ActiveConfigID && s.ModelConfigs[i].EncryptedAPIKey != "" {
				return &s.ModelConfigs[i]
			}
		}
	}
	return nil
}

// firstConfigWithKey 返回第一个有加密密钥的配置
func (s *Settings) firstConfigWithKey() *ModelConfig {
	for i := range s.ModelConfigs {
		if s.ModelConfigs[i].EncryptedAPIKey != "" {
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
		return fmt.Errorf("接口地址必须以 http:// 或 https:// 开头")
	}
	if isInternalURL(rawURL) {
		return fmt.Errorf("接口地址不能指向内部/私有网络")
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

	defaults := DefaultSettings()

	// 保留已有的 encrypted_api_key（当 api_key 为空或未提供时）
	// 使用类型安全的值检查：api_key: null 和 api_key: "" 都视为"未提供"
	apiKeyVal, _ := rawReq["api_key"].(string)
	existing := LoadSettings()
	if apiKeyVal == "" {
		settings.EncryptedAPIKey = existing.EncryptedAPIKey
	}
	// 合并 model_configs：保留前端未传回的 encrypted_api_key
	settings.ModelConfigs = mergeModelConfigs(existing.ModelConfigs, settings.ModelConfigs)

	if settings.BaseURL != "" {
		if err := ValidateBaseURL(settings.BaseURL); err != nil {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, err.Error()))
			return
		}
	}
	if settings.Temperature < 0 || settings.Temperature > 1.0 {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "温度必须在 0.0 到 1.0 之间"))
		return
	}
	if settings.CloseBehavior != "" {
		validBehaviors := []string{"minimize", "quit", "ask"}
		if !containsString(validBehaviors, settings.CloseBehavior) {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "close_behavior 必须是 minimize、quit 或 ask"))
			return
		}
	}
	if strings.TrimSpace(settings.Model) == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "模型不能为空"))
		return
	}
	if settings.ContextWindow <= 0 {
		existing := LoadSettings()
		settings.ContextWindow = existing.ContextWindow
		if settings.ContextWindow <= 0 {
			settings.ContextWindow = defaults.ContextWindow
		}
	}
	if strings.TrimSpace(settings.Language) == "" {
		existing := LoadSettings()
		settings.Language = existing.Language
		if strings.TrimSpace(settings.Language) == "" {
			settings.Language = defaults.Language
		}
	}

	apiKey, hasAPIKey := rawReq["api_key"].(string)
	password, hasPassword := rawReq["password"].(string)

	if hasAPIKey && apiKey != "" {
		if !hasPassword || password == "" {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "设置 API Key 时必须提供密码"))
			return
		}
		if !isStrongPassword(password) {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "密码至少 8 位，且需包含大写字母、小写字母和数字"))
			return
		}
		encrypted, err := flowcrypto.Encrypt(apiKey, []byte(password))
		if err != nil {
			response.WriteJSON(w, http.StatusInternalServerError,
				response.Error(response.CodeInternalError, "Failed to encrypt API Key"))
			return
		}
		settings.EncryptedAPIKey = encrypted

		if settings.ActiveConfigID != "" {
			for i := range settings.ModelConfigs {
				if settings.ModelConfigs[i].ID == settings.ActiveConfigID {
					settings.ModelConfigs[i].EncryptedAPIKey = encrypted
					break
				}
			}
		}

		ks := keystore.Instance()
		ks.SetAPIKeyConfigured(true)
		ks.Unlock([]byte(apiKey))
	}

	if len(settings.ModelConfigs) > 0 {
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

	// 清除加密的 API Key（扁平字段 + 所有模型配置中的密钥）
	settings.EncryptedAPIKey = ""
	for i := range settings.ModelConfigs {
		settings.ModelConfigs[i].EncryptedAPIKey = ""
	}

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

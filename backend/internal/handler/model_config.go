package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/google/uuid"
)

type ModelConfigHandler struct{}

func (h *ModelConfigHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.List(w, r)
	case http.MethodPost:
		h.Create(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *ModelConfigHandler) HandleByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPut:
		h.Update(w, r)
	case http.MethodDelete:
		h.Delete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (h *ModelConfigHandler) HandleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	h.Activate(w, r)
}

func (h *ModelConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	settings := LoadSettings()
	response.WriteJSON(w, http.StatusOK, response.Success(settings.ModelConfigs))
}

func (h *ModelConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name            string  `json:"name"`
		BaseURL         string  `json:"base_url"`
		ModelName       string  `json:"model_name"`
		APIKey          string  `json:"api_key"`
		Password        string  `json:"password"`
		Temperature     float64 `json:"temperature"`
		ResponseFormat  string  `json:"response_format"`
		TimeoutSecs     int     `json:"timeout_secs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}

	cfg := ModelConfig{
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		ModelName:      req.ModelName,
		Temperature:    req.Temperature,
		ResponseFormat: req.ResponseFormat,
		TimeoutSecs:    req.TimeoutSecs,
	}

	if err := validateModelConfig(&cfg); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, err.Error()))
		return
	}

	if req.APIKey != "" {
		if req.Password == "" {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, "password is required when providing API Key"))
			return
		}
		encrypted, err := flowcrypto.Encrypt(req.APIKey, []byte(req.Password))
		if err != nil {
			response.WriteJSON(w, http.StatusInternalServerError,
				response.Error(response.CodeInternalError, "Failed to encrypt API Key"))
			return
		}
		cfg.EncryptedAPIKey = encrypted
	}

	cfg.ID = uuid.NewString()

	settings := LoadSettings()
	uniqueName, err := ensureUniqueName(cfg.Name, settings.ModelConfigs, "")
	if err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, err.Error()))
		return
	}
	cfg.Name = uniqueName
	settings.ModelConfigs = append(settings.ModelConfigs, cfg)

	if err := storage.WriteJSON("settings.json", settings); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "Failed to save model config"))
		return
	}

	cfg.EncryptedAPIKey = "***"
	response.WriteJSON(w, http.StatusCreated, response.Success(cfg))
}

func (h *ModelConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID              string  `json:"id"`
		Name            string  `json:"name"`
		BaseURL         string  `json:"base_url"`
		ModelName       string  `json:"model_name"`
		APIKey          string  `json:"api_key"`
		Password        string  `json:"password"`
		Temperature     float64 `json:"temperature"`
		ResponseFormat  string  `json:"response_format"`
		TimeoutSecs     int     `json:"timeout_secs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}

	cfg := ModelConfig{
		ID:             req.ID,
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		ModelName:      req.ModelName,
		Temperature:    req.Temperature,
		ResponseFormat: req.ResponseFormat,
		TimeoutSecs:    req.TimeoutSecs,
	}

	if err := validateModelConfig(&cfg); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, err.Error()))
		return
	}

	settings := LoadSettings()
	found := false
	for i := range settings.ModelConfigs {
		if settings.ModelConfigs[i].ID == cfg.ID {
			if req.APIKey != "" {
				if req.Password == "" {
					response.WriteJSON(w, http.StatusBadRequest,
						response.Error(response.CodeInvalidParam, "password is required when providing API Key"))
					return
				}
				encrypted, err := flowcrypto.Encrypt(req.APIKey, []byte(req.Password))
				if err != nil {
					response.WriteJSON(w, http.StatusInternalServerError,
						response.Error(response.CodeInternalError, "Failed to encrypt API Key"))
					return
				}
				cfg.EncryptedAPIKey = encrypted
			} else {
				cfg.EncryptedAPIKey = settings.ModelConfigs[i].EncryptedAPIKey
			}
		uniqueName, err := ensureUniqueName(cfg.Name, settings.ModelConfigs, cfg.ID)
		if err != nil {
			response.WriteJSON(w, http.StatusInternalServerError,
				response.Error(response.CodeInternalError, err.Error()))
			return
		}
		cfg.Name = uniqueName
		settings.ModelConfigs[i] = cfg
			found = true
			break
		}
	}

	if !found {
		response.WriteJSON(w, http.StatusNotFound,
			response.Error(response.CodeInvalidParam, "Model config not found"))
		return
	}

	if err := storage.WriteJSON("settings.json", settings); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "Failed to save model config"))
		return
	}

	cfg.EncryptedAPIKey = "***"
	response.WriteJSON(w, http.StatusOK, response.Success(cfg))
}

func (h *ModelConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/model_configs/")
	if id == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Config ID is required"))
		return
	}
	if strings.ContainsAny(id, "/\\:%") {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid config ID format"))
		return
	}

	settings := LoadSettings()
	newConfigs := make([]ModelConfig, 0, len(settings.ModelConfigs))
	found := false
	for _, cfg := range settings.ModelConfigs {
		if cfg.ID != id {
			newConfigs = append(newConfigs, cfg)
		} else {
			found = true
		}
	}

	if !found {
		response.WriteJSON(w, http.StatusNotFound,
			response.Error(response.CodeInvalidParam, "Model config not found"))
		return
	}

	settings.ModelConfigs = newConfigs
	if settings.ActiveConfigID == id {
		settings.ActiveConfigID = ""
		keystore.Instance().Lock()
	}
	settings.deriveFlatFields()

	if err := storage.WriteJSON("settings.json", settings); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "Failed to delete model config"))
		return
	}

	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "配置已删除",
	}))
}

func (h *ModelConfigHandler) Activate(w http.ResponseWriter, r *http.Request) {
	if !strings.HasSuffix(r.URL.Path, "/activate") {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid activation endpoint"))
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}

	passwordCopy := []byte(req.Password)
	defer flowcrypto.ZeroBytes(passwordCopy)

	id := strings.TrimPrefix(r.URL.Path, "/api/model_configs/")
	id = strings.TrimSuffix(id, "/activate")

	settings := LoadSettings()
	cfg := settings.GetModelConfigByID(id)
	if cfg == nil {
		response.WriteJSON(w, http.StatusNotFound,
			response.Error(response.CodeInvalidParam, "Model config not found"))
		return
	}

	if cfg.EncryptedAPIKey == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeAPIKeyNotConfigured, "请先配置 API Key"))
		return
	}

	ks := keystore.Instance()
	_, err := ks.TryActivate(cfg.EncryptedAPIKey, passwordCopy)
	if err != nil {
		if err == keystore.ErrRateLimited {
			response.WriteJSON(w, http.StatusTooManyRequests,
				response.Error(response.CodeUnlockRateLimited, "Too many failed attempts, please wait"))
			return
		}
		response.WriteJSON(w, http.StatusUnauthorized,
			response.Error(response.CodeWrongPassword, "密码错误"))
		return
	}

	ks.SetAPIKeyConfigured(true)
	settings.ActiveConfigID = id
	settings.deriveFlatFields()

	if err := storage.WriteJSON("settings.json", settings); err != nil {
		log.Printf("Failed to save settings after activation: %v", err)
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "Failed to activate config"))
		return
	}

	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "配置已激活",
	}))
}

func validateModelConfig(cfg *ModelConfig) error {
	if strings.TrimSpace(cfg.Name) == "" {
		return fmt.Errorf("config name cannot be empty")
	}
	if cfg.BaseURL == "" {
		return fmt.Errorf("base_url cannot be empty")
	}
	if err := ValidateBaseURL(cfg.BaseURL); err != nil {
		return err
	}
	if cfg.ModelName == "" {
		return fmt.Errorf("model_name cannot be empty")
	}
	if cfg.Temperature < 0 || cfg.Temperature > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0")
	}
	if cfg.ResponseFormat != "" && cfg.ResponseFormat != "text" && cfg.ResponseFormat != "json_object" {
		return fmt.Errorf("response_format must be 'text' or 'json_object'")
	}
	return nil
}

func ensureUniqueName(name string, configs []ModelConfig, excludeID string) (string, error) {
	existingNames := make(map[string]bool, len(configs))
	for _, c := range configs {
		if c.ID != excludeID {
			existingNames[c.Name] = true
		}
	}
	if !existingNames[name] {
		return name, nil
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s (%d)", name, i)
		if !existingNames[candidate] {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to generate unique name for %q after 1000 attempts", name)
}

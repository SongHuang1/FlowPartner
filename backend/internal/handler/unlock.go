package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	flowcrypto "github.com/SongHuang1/FlowPartner/backend/internal/crypto"
	"github.com/SongHuang1/FlowPartner/backend/internal/keystore"
	"github.com/SongHuang1/FlowPartner/backend/internal/response"
	"github.com/SongHuang1/FlowPartner/backend/internal/storage"
)

type UnlockRequest struct {
	Password string `json:"password"`
}

type UnlockHandler struct{}

func NewUnlockHandler() *UnlockHandler {
	return &UnlockHandler{}
}

// Handle 根据路径和方法分发到 Post/Lock/Status
func (h *UnlockHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/unlock":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.Post(w, r)
	case "/api/lock":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.Lock(w, r)
	case "/api/lock_status":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.Status(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *UnlockHandler) Post(w http.ResponseWriter, r *http.Request) {
	var req UnlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}

	// 无论解密成功与否都零化密码的 []byte 副本（Go string 不可变，仅能零化副本）
	defer flowcrypto.ZeroBytes([]byte(req.Password))

	ks := keystore.Instance()
	settings := LoadSettings()
	encryptedKey := settings.EncryptedAPIKey
	activatedID := ""
	if encryptedKey == "" {
		for _, cfg := range settings.ModelConfigs {
			if cfg.EncryptedAPIKey != "" {
				encryptedKey = cfg.EncryptedAPIKey
				activatedID = cfg.ID
				break
			}
		}
	}
	if encryptedKey == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeAPIKeyNotConfigured, "请先配置 API Key"))
		return
	}

	_, err := ks.TryActivate(encryptedKey, []byte(req.Password))
	if err != nil {
		switch err {
		case keystore.ErrRateLimited:
			remaining := int(time.Until(ks.LockedUntil()).Seconds() + 0.5)
			if remaining < 0 {
				remaining = 0
			}
			response.WriteJSON(w, http.StatusTooManyRequests,
				response.Error(response.CodeUnlockRateLimited, fmt.Sprintf("失败次数过多，请在 %d 秒后重试", remaining)))
		case keystore.ErrWrongPassword:
			response.WriteJSON(w, http.StatusUnauthorized,
				response.Error(response.CodeWrongPassword, "密码错误"))
		default:
			response.WriteJSON(w, http.StatusInternalServerError,
				response.Error(response.CodeInternalError, "未知错误"))
		}
		return
	}

	if settings.ActiveConfigID == "" && activatedID != "" {
		settings.ActiveConfigID = activatedID
		settings.deriveFlatFields()
		if err := storage.WriteJSON("settings.json", settings); err != nil {
			response.WriteJSON(w, http.StatusInternalServerError,
				response.Error(response.CodeInternalError, "Failed to save active config"))
			return
		}
	}

	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "已解锁",
	}))
}

func (h *UnlockHandler) Lock(w http.ResponseWriter, r *http.Request) {
	ks := keystore.Instance()
	ks.Lock()
	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "已锁定",
	}))
}

func (h *UnlockHandler) Status(w http.ResponseWriter, r *http.Request) {
	ks := keystore.Instance()
	status := ks.GetLockStatus()
	response.WriteJSON(w, http.StatusOK, response.Success(status))
}

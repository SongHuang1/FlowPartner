package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
)

type UnlockRequest struct {
	Password string `json:"password"`
}

type UnlockHandler struct{}

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
	status := ks.GetLockStatus()
	if status.Locked && time.Now().Before(status.LockedUntil) {
		response.WriteJSON(w, http.StatusTooManyRequests,
			response.Error(response.CodeUnlockRateLimited, fmt.Sprintf("失败次数过多，请在 %v 后重试",
				time.Until(status.LockedUntil))))
		return
	}

	if !status.HasAPIKey {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeAPIKeyNotConfigured, "请先配置 API Key"))
		return
	}

	settings := LoadSettings()
	if settings.EncryptedAPIKey == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeAPIKeyNotConfigured, "请先配置 API Key"))
		return
	}

	apiKey, err := flowcrypto.Decrypt(settings.EncryptedAPIKey, []byte(req.Password))
	if err != nil {
		// 解密失败时直接增加计数器，无需再次解密（VerifyPassword 内部会重复 Argon2id 计算）
		ks.RecordFailedAttempt()
		response.WriteJSON(w, http.StatusUnauthorized,
			response.Error(response.CodeWrongPassword, "密码错误"))
		return
	}

	ks.Unlock([]byte(apiKey))

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

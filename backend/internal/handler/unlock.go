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

func (h *UnlockHandler) Post(w http.ResponseWriter, r *http.Request) {
	var req UnlockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}

	ks := keystore.Instance()
	status := ks.GetLockStatus()
	if status.Locked && time.Now().Before(status.LockedUntil) {
		response.WriteJSON(w, http.StatusTooManyRequests,
			response.Error(response.CodeUnlockRateLimited, fmt.Sprintf("Too many failed attempts, try again in %v",
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

	// 零化密码字节（仅零化 []byte 转换副本）
	// 注意：Go string 不可变，原始 password 字符串仍驻留内存直到 GC 回收
	// 这是语言层面的限制，无法在代码层面完全避免
	flowcrypto.ZeroBytes([]byte(req.Password))

	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "解锁成功",
	}))
}

func (h *UnlockHandler) Lock(w http.ResponseWriter, r *http.Request) {
	ks := keystore.Instance()
	ks.Lock()
	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "已上锁",
	}))
}

func (h *UnlockHandler) Status(w http.ResponseWriter, r *http.Request) {
	ks := keystore.Instance()
	status := ks.GetLockStatus()
	response.WriteJSON(w, http.StatusOK, response.Success(status))
}

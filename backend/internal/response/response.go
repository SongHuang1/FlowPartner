package response

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Response 所有 HTTP 响应的统一结构
type Response struct {
	Code      int         `json:"code"`       // 0=成功, 非0=错误码
	Message   string      `json:"message"`    // 人类可读消息
	Data      interface{} `json:"data"`       // 业务数据（成功时填充）
	Timestamp int64       `json:"timestamp"`  // Unix 秒
	RequestID string      `json:"request_id"` // UUID（暂未使用，保留字段）
}

// Error 错误响应快捷构造
func Error(code int, msg string) Response {
	return Response{
		Code:      code,
		Message:   msg,
		Data:      nil,
		Timestamp: time.Now().Unix(),
	}
}

// Success 成功响应快捷构造
func Success(data interface{}) Response {
	return Response{
		Code:      0,
		Message:   "success",
		Data:      data,
		Timestamp: time.Now().Unix(),
	}
}

// WriteJSON 将 Response 写入 HTTP 响应（统一 JSON 序列化 + 设置 header）
func WriteJSON(w http.ResponseWriter, statusCode int, resp Response) {
	if resp.RequestID == "" {
		resp.RequestID = uuid.NewString()
	}
	resp.Timestamp = time.Now().Unix()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

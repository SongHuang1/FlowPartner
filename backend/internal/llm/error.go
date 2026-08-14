package llm

import (
	"fmt"
)

// LLMError 分类后的 LLM 调用错误
type LLMError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Guess   string `json:"guess"`
}

func (e *LLMError) Error() string {
	return fmt.Sprintf("code=%d, message=%s, guess=%s", e.Code, e.Message, e.Guess)
}

// 错误码定义
const (
	CodeInvalidJSON     = 400
	CodeMessagesEmpty   = 400
	CodeNetworkError    = 1001
	CodeTimeout         = 1002
	CodeUnauthorized    = 401
	CodeForbidden       = 403
	CodeNotFound        = 404
	CodeTooManyRequests = 429
	CodeServerError     = 500
	CodeBadGateway      = 502
	CodeServiceUnavailable = 503
	CodeStreamInterrupted  = 1003
)

var errorMappings = []struct {
	statusCode int
	code       int
	message    string
	guess      string
}{
	{401, CodeUnauthorized, "Authentication failed", "Possible causes: invalid API Key, expired Key, or insufficient account balance"},
	{403, CodeForbidden, "Permission denied", "Possible causes: API Key lacks access to this model, or additional permissions are required"},
	{404, CodeNotFound, "Resource not found", "Possible causes: incorrect BaseURL path or invalid ModelName"},
	{429, CodeTooManyRequests, "Too many requests", "Possible causes: request rate exceeds provider limits, please retry later"},
	{500, CodeServerError, "Internal server error", "Possible causes: LLM provider server failure, please retry later"},
	{502, CodeBadGateway, "Bad gateway", "Possible causes: LLM provider gateway error, please retry later"},
	{503, CodeServiceUnavailable, "Service unavailable", "Possible causes: LLM provider temporarily unavailable, please retry later"},
}

// ClassifyHTTPError 根据 HTTP 状态码返回分类错误
func ClassifyHTTPError(statusCode int, retryAfter string) *LLMError {
	for _, m := range errorMappings {
		if m.statusCode == statusCode {
			guess := m.guess
			if retryAfter != "" && (statusCode == 502 || statusCode == 503) {
				guess += fmt.Sprintf(", retry suggested after %s seconds", retryAfter)
			}
			return &LLMError{Code: m.code, Message: m.message, Guess: guess}
		}
	}
	if statusCode >= 500 {
		return &LLMError{Code: CodeServerError, Message: "Server error", Guess: "Possible causes: LLM provider server failure, please retry later"}
	}
	return &LLMError{Code: statusCode, Message: fmt.Sprintf("HTTP %d", statusCode), Guess: "Possible causes: invalid request parameters or provider restrictions"}
}

// InvalidJSONError 请求体 JSON 格式错误
func InvalidJSONError() *LLMError {
	return &LLMError{
		Code:    CodeInvalidJSON,
		Message: "Invalid request body format",
		Guess:   "Possible causes: JSON syntax error or incorrect Content-Type",
	}
}

// MessagesEmptyError messages 为空错误
func MessagesEmptyError() *LLMError {
	return &LLMError{
		Code:    CodeMessagesEmpty,
		Message: "Message content cannot be empty",
		Guess:   "Possible causes: messages field missing, empty array, or malformed format",
	}
}

// NetworkError 网络连接失败
func NetworkError(err error) *LLMError {
	return &LLMError{
		Code:    CodeNetworkError,
		Message: "Network connection failed",
		Guess:   "Possible causes: device offline, DNS resolution failure, or firewall blocking",
	}
}

// TimeoutError 请求超时
func TimeoutError() *LLMError {
	return &LLMError{
		Code:    CodeTimeout,
		Message: "Request timeout",
		Guess:   "Possible causes: slow model response, high network latency, or timeout set too short",
	}
}

// StreamInterruptedError 流式中断（已收到部分 chunk）
func StreamInterruptedError() *LLMError {
	return &LLMError{
		Code:    CodeStreamInterrupted,
		Message: "Streaming response interrupted",
		Guess:   "The partially displayed answer may be incomplete",
	}
}

// IsRetryable 判断错误是否可重试（仅在首个 chunk 前）
func IsRetryable(err *LLMError) bool {
	return err.Code == CodeNetworkError || err.Code >= 500
}



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
	{401, CodeUnauthorized, "认证失败", "可能原因：API Key 无效、已过期或账户余额不足"},
	{403, CodeForbidden, "权限不足", "可能原因：API Key 没有该模型的访问权限，或需要额外授权"},
	{404, CodeNotFound, "资源不存在", "可能原因：接口地址（Base URL）路径错误或模型名称无效"},
	{429, CodeTooManyRequests, "请求过于频繁", "可能原因：请求频率超过服务商限制，请稍后重试"},
	{500, CodeServerError, "服务器内部错误", "可能原因：模型服务商服务器故障，请稍后重试"},
	{502, CodeBadGateway, "网关错误", "可能原因：模型服务商网关异常，请稍后重试"},
	{503, CodeServiceUnavailable, "服务不可用", "可能原因：模型服务商暂时不可用，请稍后重试"},
}

// ClassifyHTTPError 根据 HTTP 状态码返回分类错误
func ClassifyHTTPError(statusCode int, retryAfter string) *LLMError {
	for _, m := range errorMappings {
		if m.statusCode == statusCode {
			guess := m.guess
			if retryAfter != "" && (statusCode == 502 || statusCode == 503) {
				guess += fmt.Sprintf("，建议 %s 秒后重试", retryAfter)
			}
			return &LLMError{Code: m.code, Message: m.message, Guess: guess}
		}
	}
	if statusCode >= 500 {
		return &LLMError{Code: CodeServerError, Message: "服务器错误", Guess: "可能原因：模型服务商服务器故障，请稍后重试"}
	}
	return &LLMError{Code: statusCode, Message: fmt.Sprintf("HTTP %d", statusCode), Guess: "可能原因：请求参数无效或服务商限制"}
}

// InvalidJSONError 请求体 JSON 格式错误
func InvalidJSONError() *LLMError {
	return &LLMError{
		Code:    CodeInvalidJSON,
		Message: "请求体格式无效",
		Guess:   "可能原因：JSON 语法错误或 Content-Type 不正确",
	}
}

// MessagesEmptyError messages 为空错误
func MessagesEmptyError() *LLMError {
	return &LLMError{
		Code:    CodeMessagesEmpty,
		Message: "消息内容不能为空",
		Guess:   "可能原因：messages 字段缺失、为空数组或格式错误",
	}
}

// NetworkError 网络连接失败
func NetworkError(err error) *LLMError {
	return &LLMError{
		Code:    CodeNetworkError,
		Message: "网络连接失败",
		Guess:   "可能原因：设备离线、DNS 解析失败或被防火墙拦截",
	}
}

// TimeoutError 请求超时
func TimeoutError() *LLMError {
	return &LLMError{
		Code:    CodeTimeout,
		Message: "请求超时",
		Guess:   "可能原因：模型响应较慢、网络延迟高或超时时间设置过短",
	}
}

// StreamInterruptedError 流式中断（已收到部分 chunk）
func StreamInterruptedError() *LLMError {
	return &LLMError{
		Code:    CodeStreamInterrupted,
		Message: "流式响应中断",
		Guess:   "已显示的部分回答可能不完整",
	}
}

// IsRetryable 判断错误是否可重试（仅在首个 chunk 前）
func IsRetryable(err *LLMError) bool {
	return err.Code == CodeNetworkError || err.Code >= 500
}



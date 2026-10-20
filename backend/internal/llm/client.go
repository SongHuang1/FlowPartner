package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SongHuang1/FlowPartner/backend/proto"
)

// StreamRequest 流式请求参数
type StreamRequest struct {
	RawPayload     []byte
	Messages       []byte
	Tools          []byte
	ToolChoice     []byte
	Model          string
	Temperature    float64
	ResponseFormat string
	APIKey         []byte
	TargetURL      string
	Timeout        time.Duration
}

// StreamChunk 流式响应块
type StreamChunk struct {
	Data  string
	Done  bool
	Usage *proto.UsagePayload
}

// LLMClient LLM HTTP 流式客户端
type LLMClient struct {
	httpClient *http.Client
}

// loggingTransport 日志脱敏 RoundTripper
type loggingTransport struct {
	base http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	sanitizedURL := *req.URL
	if sanitizedURL.RawQuery != "" {
		sanitizedURL.RawQuery = "***"
	}
	log.Printf("[LLM] Request: %s %s", req.Method, sanitizedURL.String())

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		log.Printf("[LLM] Request failed: %v", err)
		return resp, err
	}

	log.Printf("[LLM] Response status: %d", resp.StatusCode)
	return resp, nil
}

// NewClient 创建 LLMClient
func NewClient() *LLMClient {
	transport := &http.Transport{
		MaxIdleConns:    20,
		IdleConnTimeout: 120 * time.Second,
		DialContext: (&net.Dialer{
			Timeout: 10 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &LLMClient{
		httpClient: &http.Client{
			Transport: &loggingTransport{base: transport},
		},
	}
}

// Stream 发起流式 HTTP 调用，返回 chunk 通道
func (c *LLMClient) Stream(ctx context.Context, req StreamRequest) (<-chan StreamChunk, error) {
	body, err := buildRequestBody(req)
	if err != nil {
		return nil, err
	}

	targetURL := req.TargetURL
	if targetURL == "" {
		return nil, fmt.Errorf("target URL is required")
	}

	// 64 是经验值：足够吸收网络抖动造成的突发，又不会在消费者变慢时堆积太多
	// 未处理数据；缓冲满会让发送阻塞，进而反压到 TCP，让服务商降速。
	const chunkBufferSize = 64

	chunkChan := make(chan StreamChunk, chunkBufferSize)

	go c.streamWithRetry(ctx, targetURL, body, req.APIKey, req.Timeout, chunkChan)

	return chunkChan, nil
}

func (c *LLMClient) streamWithRetry(ctx context.Context, targetURL string, body []byte, apiKey []byte, timeout time.Duration, chunkChan chan<- StreamChunk) {
	defer close(chunkChan)

	var lastErr error
	var dataSent bool
	for attempt := 0; attempt <= 1; attempt++ {
		if attempt > 0 {
			log.Printf("[LLM] Retrying (attempt %d)", attempt)
		}

		sent, err := c.doStream(ctx, targetURL, body, apiKey, timeout, chunkChan)
		if err == nil {
			return
		}

		lastErr = err
		dataSent = dataSent || sent

		if dataSent {
			chunkChan <- StreamChunk{Data: mustJSON(StreamInterruptedError()), Done: true}
			return
		}

		if attempt == 0 {
			if llmErr, ok := err.(*LLMError); ok && !IsRetryable(llmErr) {
				chunkChan <- StreamChunk{Data: mustJSON(llmErr), Done: true}
				return
			}
		}
	}

	if llmErr, ok := lastErr.(*LLMError); ok {
		chunkChan <- StreamChunk{Data: mustJSON(llmErr), Done: true}
	} else {
		chunkChan <- StreamChunk{Data: mustJSON(NetworkError(lastErr)), Done: true}
	}
}

func (c *LLMClient) doStream(ctx context.Context, targetURL string, body []byte, apiKey []byte, timeout time.Duration, chunkChan chan<- StreamChunk) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, strings.NewReader(string(body)))
	if err != nil {
		return false, fmt.Errorf("create request: %w", err)
	}

	keyCopy := make([]byte, len(apiKey))
	copy(keyCopy, apiKey)
	defer func() {
		for i := range keyCopy {
			keyCopy[i] = 0
		}
	}()

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+string(apiKey))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return false, TimeoutError()
		}
		return false, NetworkError(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		retryAfter := resp.Header.Get("Retry-After")
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[LLM] Error response body: %s", string(body))
		return false, ClassifyHTTPError(resp.StatusCode, retryAfter)
	}

	idleTimeout := timeout / 2
	if idleTimeout > 10*time.Second {
		idleTimeout = 10 * time.Second
	}
	if idleTimeout < time.Second {
		idleTimeout = time.Second
	}
	idleTimer := time.NewTimer(idleTimeout)
	defer idleTimer.Stop()

	scanner := NewSSEScanner(resp.Body)
	dataSent := false
	var usage *proto.UsagePayload
	for {
		event, done, err := scanner.Scan()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return dataSent, TimeoutError()
			}
			return dataSent, fmt.Errorf("sse scan: %w", err)
		}

		if done {
			chunkChan <- StreamChunk{Done: true, Usage: usage}
			return false, nil
		}

		if event.Data == "" {
			continue
		}

		if u, ok := parseUsage(event.Data); ok {
			usage = u
			continue
		}

		sent := false
		select {
		case chunkChan <- StreamChunk{Data: event.Data}:
			sent = true
		default:
		}
		if sent {
			dataSent = true
			if !idleTimer.Stop() {
				<-idleTimer.C
			}
			idleTimer.Reset(idleTimeout)
			continue
		}
		select {
		case chunkChan <- StreamChunk{Data: event.Data}:
			dataSent = true
			if !idleTimer.Stop() {
				<-idleTimer.C
			}
			idleTimer.Reset(idleTimeout)
		case <-ctx.Done():
			return dataSent, TimeoutError()
		case <-idleTimer.C:
			return dataSent, TimeoutError()
		}
	}
}

// parseUsage 尝试从 SSE chunk JSON 中提取 usage 尾帧。
//
// 这里是全项目唯一接触 OpenAI 字段名（prompt_tokens / completion_tokens）的地方：
// proto 与前端统一使用 input_tokens / output_tokens，翻译只发生在这一层。
func parseUsage(data string) (*proto.UsagePayload, bool) {
	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, false
	}
	usage := jsonObject(chunk, "usage")
	if usage == nil {
		return nil, false
	}

	// 缓存命中与推理 token 不在 usage 顶层，而在两个 *_details 子对象里。
	promptDetails := jsonObject(usage, "prompt_tokens_details")
	completionDetails := jsonObject(usage, "completion_tokens_details")

	return &proto.UsagePayload{
		InputTokens:           jsonInt32(usage, "prompt_tokens"),
		CachedInputTokens:     jsonInt32(promptDetails, "cached_tokens"),
		OutputTokens:          jsonInt32(usage, "completion_tokens"),
		ReasoningOutputTokens: jsonInt32(completionDetails, "reasoning_tokens"),
		TotalTokens:           jsonInt32(usage, "total_tokens"),
	}, true
}

// jsonObject 取出嵌套的 JSON 对象；缺失或类型不符时返回 nil。
func jsonObject(obj map[string]interface{}, key string) map[string]interface{} {
	nested, ok := obj[key].(map[string]interface{})
	if !ok {
		return nil
	}
	return nested
}

// jsonInt32 取出 JSON 数字并转成 int32；缺失或类型不符时返回 0。
// encoding/json 把数字统一解成 float64，所以这里必须按 float64 断言。
func jsonInt32(obj map[string]interface{}, key string) int32 {
	value, ok := obj[key].(float64)
	if !ok {
		return 0
	}
	return int32(value)
}

func buildRequestBody(req StreamRequest) ([]byte, error) {
	var payload map[string]interface{}
	if len(req.RawPayload) > 0 {
		if err := json.Unmarshal(req.RawPayload, &payload); err != nil {
			return nil, InvalidJSONError()
		}
	} else if len(req.Messages) > 0 {
		if err := json.Unmarshal(req.Messages, &payload); err != nil {
			return nil, InvalidJSONError()
		}
	} else {
		return nil, MessagesEmptyError()
	}

	messages, ok := payload["messages"].([]interface{})
	if !ok || len(messages) == 0 {
		return nil, MessagesEmptyError()
	}

	payload["model"] = req.Model
	payload["temperature"] = req.Temperature
	payload["stream"] = true
	payload["stream_options"] = map[string]bool{"include_usage": true}

	if req.ResponseFormat != "" && req.ResponseFormat != "text" {
		payload["response_format"] = map[string]string{"type": req.ResponseFormat}
	}

	if len(req.Tools) > 0 {
		var tools interface{}
		if err := json.Unmarshal(req.Tools, &tools); err != nil {
			log.Printf("[LLM] Warning: tools JSON parse error: %v", err)
		} else {
			payload["tools"] = tools
		}
	}

	if len(req.ToolChoice) > 0 {
		var toolChoice interface{}
		if err := json.Unmarshal(req.ToolChoice, &toolChoice); err != nil {
			log.Printf("[LLM] Warning: tool_choice JSON parse error: %v", err)
		} else {
			payload["tool_choice"] = toolChoice
		}
	}

	return json.Marshal(payload)
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"code":500,"message":"内部序列化错误","guess":"可能原因：内部服务器错误，请稍后重试"}`
	}
	return string(b)
}

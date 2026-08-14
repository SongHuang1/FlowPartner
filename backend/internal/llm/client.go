package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

const defaultChunkBufferSize = 64

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
	Data string
	Done bool
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

	if auth := req.Header.Get("Authorization"); auth != "" {
		log.Printf("[LLM] Authorization: Bearer ***")
	}

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

	chunkChan := make(chan StreamChunk, defaultChunkBufferSize)

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
	for {
		event, done, err := scanner.Scan()
		if err != nil {
			if ctx.Err() == context.DeadlineExceeded {
				return dataSent, TimeoutError()
			}
			return dataSent, fmt.Errorf("sse scan: %w", err)
		}

		if done {
			chunkChan <- StreamChunk{Done: true}
			return false, nil
		}

		if event.Data == "" {
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
		return `{"code":500,"message":"内部序列化错误","guess":"可能原因：服务器内部错误，请稍后重试"}`
	}
	return string(b)
}

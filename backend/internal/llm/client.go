package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/SongHuang1/FlowPartner/backend/proto"
	"golang.org/x/net/http/httpproxy"
)

// StreamRequest 流式请求参数。
// 请求体以 RawPayload 为唯一来源；Model/Temperature/ResponseFormat 由服务端配置
// 覆盖，其余字段原样透传。
type StreamRequest struct {
	RawPayload     []byte
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

// NewClient 创建 LLMClient。
func NewClient() *LLMClient {
	return &LLMClient{
		httpClient: &http.Client{
			Transport: &loggingTransport{base: newTransport()},
		},
	}
}

// newTransport 以 http.DefaultTransport 为基底构造传输层：提高单域名连接复用，
// 建连与握手快速失败。
func newTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 20
	transport.MaxIdleConnsPerHost = 20
	transport.IdleConnTimeout = 120 * time.Second
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.DialContext = (&net.Dialer{
		Timeout: 10 * time.Second,
	}).DialContext
	// 设置 DialContext 会关闭标准库的 HTTP/2 自动协商，需显式开启。
	transport.ForceAttemptHTTP2 = true
	// 环境变量优先，系统代理兜底；回环地址由 httpproxy 放行。
	transport.Proxy = envFirst(http.ProxyFromEnvironment, osProxy())
	return transport
}

// envFirst 环境变量优先，其次 osDefault；osDefault 为 nil 时直接用 env。
// 独立成函数，以便脱离真实环境变量与注册表验证优先级。
func envFirst(env, osDefault func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	if osDefault == nil {
		return env
	}
	return func(req *http.Request) (*url.URL, error) {
		if u, err := env(req); err != nil || u != nil {
			return u, err
		}
		return osDefault(req)
	}
}

// winINETProxyConfig 把注册表值转换为 httpproxy 配置。
// 例外列表复用 NO_PROXY 的匹配规则；Windows 专有的 <local> 不在该语义内，忽略。
func winINETProxyConfig(server, override string) *httpproxy.Config {
	httpProxy, httpsProxy := splitWinINETProxyServer(server)
	if httpProxy == "" && httpsProxy == "" {
		return nil
	}
	return &httpproxy.Config{
		HTTPProxy:  httpProxy,
		HTTPSProxy: httpsProxy,
		NoProxy:    strings.ReplaceAll(override, ";", ","),
	}
}

// splitWinINETProxyServer 解析 ProxyServer："host:port" 表示所有协议共用，
// "http=...;https=..." 表示按协议指定；只配一侧时另一侧沿用。
func splitWinINETProxyServer(server string) (httpProxy, httpsProxy string) {
	server = strings.TrimSpace(server)
	if !strings.Contains(server, "=") {
		return server, server
	}

	for _, part := range strings.Split(server, ";") {
		scheme, addr, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		addr = strings.TrimSpace(addr)
		switch strings.ToLower(strings.TrimSpace(scheme)) {
		case "http":
			httpProxy = addr
		case "https", "ssl":
			httpsProxy = addr
		}
	}

	if httpsProxy == "" {
		httpsProxy = httpProxy
	}
	if httpProxy == "" {
		httpProxy = httpsProxy
	}
	return httpProxy, httpsProxy
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

	// 0 或负值会让 context.WithTimeout 立即过期，按默认值兜底而非报错：
	// 超时属于配置项，不应让整条链路不可用。
	const defaultTimeout = 30 * time.Second
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = defaultTimeout
	}

	// 64：吸收网络突发，同时限制消费端变慢时的内存占用。
	const chunkBufferSize = 64

	chunkChan := make(chan StreamChunk, chunkBufferSize)

	go c.streamWithRetry(ctx, targetURL, body, req.APIKey, timeout, chunkChan)

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

// parseUsage 从 SSE chunk 中提取用量。
// OpenAI 字段名只在这一层转换为 proto 字段名。
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

// jsonObject 取出嵌套对象；缺失或类型不符时返回 nil。
func jsonObject(obj map[string]interface{}, key string) map[string]interface{} {
	nested, ok := obj[key].(map[string]interface{})
	if !ok {
		return nil
	}
	return nested
}

// jsonInt32 取出整数；encoding/json 把数字解成 float64，故按 float64 断言。
func jsonInt32(obj map[string]interface{}, key string) int32 {
	value, ok := obj[key].(float64)
	if !ok {
		return 0
	}
	return int32(value)
}

// buildRequestBody 组装请求体：RawPayload 为唯一来源，仅覆盖服务端权威字段
// （model / temperature / response_format / stream），其余原样透传。
func buildRequestBody(req StreamRequest) ([]byte, error) {
	if len(req.RawPayload) == 0 {
		return nil, MessagesEmptyError()
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(req.RawPayload, &payload); err != nil {
		return nil, InvalidJSONError()
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

	return json.Marshal(payload)
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"code":500,"message":"内部序列化错误","guess":"可能原因：内部服务器错误，请稍后重试"}`
	}
	return string(b)
}

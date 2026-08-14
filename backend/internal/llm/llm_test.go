package llm

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNormalizeChatCompletionsURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard openai", "https://api.openai.com/v1", "https://api.openai.com/v1/chat/completions"},
		{"trailing slash", "https://api.openai.com/v1/", "https://api.openai.com/v1/chat/completions"},
		{"ollama", "http://localhost:11434/api", "http://localhost:11434/api/chat/completions"},
		{"deepseek", "https://api.deepseek.com/v1", "https://api.deepseek.com/v1/chat/completions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NormalizeChatCompletionsURL(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNormalizeChatCompletionsURL_Invalid(t *testing.T) {
	_, err := NormalizeChatCompletionsURL("not-a-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestClassifyHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantCode   int
		wantMsg    string
	}{
		{"401 unauthorized", 401, CodeUnauthorized, "认证失败"},
		{"403 forbidden", 403, CodeForbidden, "权限不足"},
		{"404 not found", 404, CodeNotFound, "资源不存在"},
		{"429 rate limited", 429, CodeTooManyRequests, "请求过于频繁"},
		{"500 server error", 500, CodeServerError, "服务器内部错误"},
		{"502 bad gateway", 502, CodeBadGateway, "网关错误"},
		{"503 unavailable", 503, CodeServiceUnavailable, "服务不可用"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifyHTTPError(tt.statusCode, "")
			if err.Code != tt.wantCode {
				t.Errorf("code: got %d, want %d", err.Code, tt.wantCode)
			}
			if err.Message != tt.wantMsg {
				t.Errorf("message: got %q, want %q", err.Message, tt.wantMsg)
			}
		})
	}
}

func TestClassifyHTTPError_RetryAfter(t *testing.T) {
	err := ClassifyHTTPError(503, "30")
	if !strings.Contains(err.Guess, "建议 30 秒后重试") {
		t.Errorf("expected retry-after hint, got: %s", err.Guess)
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  *LLMError
		want bool
	}{
		{"network error", &LLMError{Code: CodeNetworkError}, true},
		{"500 error", &LLMError{Code: CodeServerError}, true},
		{"502 error", &LLMError{Code: CodeBadGateway}, true},
		{"401 not retryable", &LLMError{Code: CodeUnauthorized}, false},
		{"404 not retryable", &LLMError{Code: CodeNotFound}, false},
		{"429 not retryable", &LLMError{Code: CodeTooManyRequests}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryable(tt.err); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSSEScanner(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":"你好"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"世界"},"finish_reason":null}]}

data: [DONE]
`
	scanner := NewSSEScanner(strings.NewReader(sseData))

	var chunks []string
	for {
		event, done, err := scanner.Scan()
		if err != nil {
			t.Fatalf("scan error: %v", err)
		}
		if done {
			break
		}
		chunks = append(chunks, event.Data)
	}

	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	var first map[string]interface{}
	json.Unmarshal([]byte(chunks[0]), &first)
	choices := first["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	if delta["content"] != "你好" {
		t.Errorf("first chunk content: got %q, want %q", delta["content"], "你好")
	}
}

func TestSSEScanner_CommentsAndEmptyLines(t *testing.T) {
	sseData := `: this is a comment

data: {"choices":[{"delta":{"content":"test"}}]}

: another comment
data: [DONE]
`
	scanner := NewSSEScanner(strings.NewReader(sseData))

	var chunks []string
	for {
		event, done, _ := scanner.Scan()
		if done {
			break
		}
		chunks = append(chunks, event.Data)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestSSEScanner_NoDone(t *testing.T) {
	sseData := `data: {"choices":[{"delta":{"content":"test"}}]}
`
	scanner := NewSSEScanner(strings.NewReader(sseData))

	var chunks []string
	for {
		event, done, _ := scanner.Scan()
		if done {
			break
		}
		chunks = append(chunks, event.Data)
	}

	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
}

func TestStream_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"Hello"},"finish_reason":null}]}`)
		flusher.Flush()

		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":" World"},"finish_reason":null}]}`)
		flusher.Flush()

		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`)
		flusher.Flush()

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:    []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		Model:       "gpt-4",
		APIKey:      []byte("sk-test-not-a-real-key"),
		TargetURL:   server.URL + "/chat/completions",
		Timeout:     5 * time.Second,
	}

	chunkChan, err := client.Stream(t.Context(), req)
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}

	var chunks []string
	for chunk := range chunkChan {
		if !chunk.Done {
			chunks = append(chunks, chunk.Data)
		}
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
}

func TestStream_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]string{"message": "Invalid API key"},
		})
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:    []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		Model:       "gpt-4",
		APIKey:      []byte("sk-test-not-a-real-key"),
		TargetURL:   server.URL + "/chat/completions",
		Timeout:     5 * time.Second,
	}

	chunkChan, err := client.Stream(t.Context(), req)
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}

	var errorFound bool
	for chunk := range chunkChan {
		if chunk.Done && chunk.Data != "" {
			var llmErr LLMError
			json.Unmarshal([]byte(chunk.Data), &llmErr)
			if llmErr.Code == 401 {
				errorFound = true
			}
		}
	}

	if !errorFound {
		t.Fatal("expected 401 error chunk")
	}
}

func TestStream_RetryNetworkError(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			conn.Close()
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"ok"},"finish_reason":"stop"}]}`)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:    []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		Model:       "gpt-4",
		APIKey:      []byte("sk-test-not-a-real-key"),
		TargetURL:   server.URL + "/chat/completions",
		Timeout:     5 * time.Second,
	}

	chunkChan, _ := client.Stream(t.Context(), req)

	var gotData bool
	for chunk := range chunkChan {
		if !chunk.Done && strings.Contains(chunk.Data, "ok") {
			gotData = true
		}
	}

	if !gotData {
		t.Fatal("expected data after retry")
	}
}

func TestStream_NoRetryAfterChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"content":"partial"},"finish_reason":null}]}`)
		flusher.Flush()

		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("server does not support hijacking")
		}
		conn, _, _ := hj.Hijack()
		conn.Close()
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:    []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		Model:       "gpt-4",
		APIKey:      []byte("sk-test-not-a-real-key"),
		TargetURL:   server.URL + "/chat/completions",
		Timeout:     5 * time.Second,
	}

	chunkChan, _ := client.Stream(t.Context(), req)

	var errorFound bool
	for chunk := range chunkChan {
		if chunk.Done && chunk.Data != "" {
			var llmErr LLMError
			json.Unmarshal([]byte(chunk.Data), &llmErr)
			if llmErr.Code == CodeStreamInterrupted {
				errorFound = true
			}
		}
	}

	if !errorFound {
		t.Fatal("expected stream interrupted error, no retry")
	}
}

func TestStream_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		time.Sleep(3 * time.Second)
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:    []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		Model:       "gpt-4",
		APIKey:      []byte("sk-test-not-a-real-key"),
		TargetURL:   server.URL + "/chat/completions",
		Timeout:     1 * time.Second,
	}

	chunkChan, _ := client.Stream(t.Context(), req)

	var errorFound bool
	for chunk := range chunkChan {
		if chunk.Done && chunk.Data != "" {
			var llmErr LLMError
			json.Unmarshal([]byte(chunk.Data), &llmErr)
			if llmErr.Code == CodeTimeout {
				errorFound = true
			}
		}
	}

	if !errorFound {
		t.Fatal("expected timeout error")
	}
}

func TestStream_ResponseFormat(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:       []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		Model:          "gpt-4",
		ResponseFormat: "json_object",
		APIKey:         []byte("sk-test-not-a-real-key"),
		TargetURL:      server.URL + "/chat/completions",
		Timeout:        5 * time.Second,
	}

	chunkChan, _ := client.Stream(t.Context(), req)
	for range chunkChan {
	}

	rFormat, ok := receivedBody["response_format"].(map[string]interface{})
	if !ok {
		t.Fatal("expected response_format in request body")
	}
	if rFormat["type"] != "json_object" {
		t.Errorf("expected response_format.type=json_object, got %v", rFormat["type"])
	}
}

func TestStream_OverrideModel(t *testing.T) {
	var receivedBody map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:    []byte(`{"messages":[{"role":"user","content":"hi"}]}`),
		Model:       "gpt-4o",
		APIKey:      []byte("sk-test-not-a-real-key"),
		TargetURL:   server.URL + "/chat/completions",
		Timeout:     5 * time.Second,
	}

	chunkChan, _ := client.Stream(t.Context(), req)
	for range chunkChan {
	}

	if receivedBody["model"] != "gpt-4o" {
		t.Errorf("expected model=gpt-4o, got %v", receivedBody["model"])
	}
}

func TestStream_ToolCallsPassthrough(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)

		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"get_weather","arguments":""}}]},"finish_reason":null}]}`)
		flusher.Flush()

		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"location\":\"Beijing\"}"}}]},"finish_reason":null}]}`)
		flusher.Flush()

		fmt.Fprintf(w, "data: %s\n\n", `{"choices":[{"delta":{},"finish_reason":"tool_calls"}]}`)
		flusher.Flush()

		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewClient()
	req := StreamRequest{
		Messages:    []byte(`{"messages":[{"role":"user","content":"weather?"}]}`),
		Model:       "gpt-4",
		APIKey:      []byte("sk-test-not-a-real-key"),
		TargetURL:   server.URL + "/chat/completions",
		Timeout:     5 * time.Second,
	}

	chunkChan, _ := client.Stream(t.Context(), req)

	var chunks []string
	for chunk := range chunkChan {
		if !chunk.Done && chunk.Data != "" {
			chunks = append(chunks, chunk.Data)
		}
	}

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}

	var first map[string]interface{}
	json.Unmarshal([]byte(chunks[0]), &first)
	choices := first["choices"].([]interface{})
	delta := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
	toolCalls := delta["tool_calls"].([]interface{})
	tc := toolCalls[0].(map[string]interface{})
	if tc["id"] != "call_123" {
		t.Errorf("tool_call id: got %v, want call_123", tc["id"])
	}
}

func TestBuildRequestBody_MessagesEmpty(t *testing.T) {
	_, err := buildRequestBody(StreamRequest{
		Messages: []byte(`{"messages":[]}`),
		Model:    "gpt-4",
	})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

func TestBuildRequestBody_InvalidJSON(t *testing.T) {
	_, err := buildRequestBody(StreamRequest{
		Messages: []byte(`not json`),
		Model:    "gpt-4",
	})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

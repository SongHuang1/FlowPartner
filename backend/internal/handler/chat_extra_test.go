package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

// TestChat_GetAgentPort_FromFile 验证从 agent.port 文件读取端口
func TestChat_GetAgentPort_FromFile(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	// 写入有效端口
	os.WriteFile(portFile, []byte("9000"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 9000 {
		t.Errorf("expected port 9000, got %d", port)
	}
}

// TestChat_GetAgentPort_InvalidPort 验证无效端口使用默认值
func TestChat_GetAgentPort_InvalidPort(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	// 写入无效端口（非数字）
	os.WriteFile(portFile, []byte("not-a-number"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 8989 {
		t.Errorf("expected default port 8989 for invalid port, got %d", port)
	}
}

// TestChat_GetAgentPort_OutOfRange 验证超出范围的端口使用默认值
func TestChat_GetAgentPort_OutOfRange(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	// 写入超出范围的端口
	os.WriteFile(portFile, []byte("80"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 8989 {
		t.Errorf("expected default port 8989 for out-of-range port, got %d", port)
	}
}

// TestChat_GetAgentPort_TooHigh 验证过高端口使用默认值
func TestChat_GetAgentPort_TooHigh(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	// 写入超出范围的端口
	os.WriteFile(portFile, []byte("70000"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 8989 {
		t.Errorf("expected default port 8989 for too-high port, got %d", port)
	}
}

// TestChat_GetAgentPort_Whitespace 验证带空白字符的端口号
func TestChat_GetAgentPort_Whitespace(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	// 写入带空白字符的端口
	os.WriteFile(portFile, []byte("  9000  \n"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 9000 {
		t.Errorf("expected port 9000, got %d", port)
	}
}

// TestChat_GetAgentPort_BoundaryLow 验证边界端口 1024
func TestChat_GetAgentPort_BoundaryLow(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	os.WriteFile(portFile, []byte("1024"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 1024 {
		t.Errorf("expected port 1024, got %d", port)
	}
}

// TestChat_GetAgentPort_BoundaryHigh 验证边界端口 65535
func TestChat_GetAgentPort_BoundaryHigh(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	os.WriteFile(portFile, []byte("65535"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 65535 {
		t.Errorf("expected port 65535, got %d", port)
	}
}

// TestChat_GetAgentPort_BelowRange 验证端口 1023 使用默认值
func TestChat_GetAgentPort_BelowRange(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	portFile := filepath.Join(dir, "agent.port")

	os.WriteFile(portFile, []byte("1023"), 0600)
	defer os.Remove(portFile)

	h := &ChatHandler{}
	port := h.getAgentPort()

	if port != 8989 {
		t.Errorf("expected default port 8989 for port 1023, got %d", port)
	}
}

// TestChat_LoadConversationHistory_Truncate 验证对话历史超过 50 条时截断
func TestChat_LoadConversationHistory_Truncate(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	convPath := filepath.Join(dir, "conversations.json")

	// 创建 60 条消息的对话历史
	messages := make([]Message, 60)
	for i := range messages {
		messages[i] = Message{
			ID:        fmt.Sprintf("msg_%d", i),
			Role:      "user",
			Content:   fmt.Sprintf("message %d", i),
			Timestamp: int64(i),
		}
	}
	conv := Conversation{Messages: messages, UpdatedAt: 60}
	storage.WriteJSON("conversations.json", conv)
	defer os.Remove(convPath)

	h := &ChatHandler{}
	history := h.loadConversationHistory()

	if len(history) != 50 {
		t.Errorf("expected 50 messages (truncated), got %d", len(history))
	}

	// 验证保留的是最近的消息
	if history[0].ID != "msg_10" {
		t.Errorf("expected first message to be 'msg_10', got %q", history[0].ID)
	}
	if history[49].ID != "msg_59" {
		t.Errorf("expected last message to be 'msg_59', got %q", history[49].ID)
	}
}

// TestChat_LoadConversationHistory_Exactly50 验证恰好 50 条时不截断
func TestChat_LoadConversationHistory_Exactly50(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	messages := make([]Message, 50)
	for i := range messages {
		messages[i] = Message{
			ID:        fmt.Sprintf("msg_%d", i),
			Role:      "user",
			Content:   fmt.Sprintf("message %d", i),
			Timestamp: int64(i),
		}
	}
	conv := Conversation{Messages: messages, UpdatedAt: 50}
	storage.WriteJSON("conversations.json", conv)

	h := &ChatHandler{}
	history := h.loadConversationHistory()

	if len(history) != 50 {
		t.Errorf("expected 50 messages, got %d", len(history))
	}
}

// TestChat_LoadConversationHistory_LessThan50 验证少于 50 条时不截断
func TestChat_LoadConversationHistory_LessThan50(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	messages := make([]Message, 10)
	for i := range messages {
		messages[i] = Message{
			ID:        fmt.Sprintf("msg_%d", i),
			Role:      "user",
			Content:   fmt.Sprintf("message %d", i),
			Timestamp: int64(i),
		}
	}
	conv := Conversation{Messages: messages, UpdatedAt: 10}
	storage.WriteJSON("conversations.json", conv)

	h := &ChatHandler{}
	history := h.loadConversationHistory()

	if len(history) != 10 {
		t.Errorf("expected 10 messages, got %d", len(history))
	}
}

// TestChat_EmptyContent 验证空内容消息
func TestChat_EmptyContent(t *testing.T) {
	_, _ = setupChatTest()

	reqBody, _ := json.Marshal(ChatRequest{Content: ""})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(reqBody)))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	// 空内容不会立即返回错误，会尝试调用 Agent（返回 502）
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (agent unavailable), got %d: %s", w.Code, w.Body.String())
	}
}

// TestChat_LongContent 验证超长消息内容
func TestChat_LongContent(t *testing.T) {
	_, _ = setupChatTest()

	longContent := strings.Repeat("a", 10000)
	reqBody, _ := json.Marshal(ChatRequest{Content: longContent})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(reqBody)))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	// 超长内容不会立即返回错误，会尝试调用 Agent（返回 502）
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (agent unavailable), got %d: %s", w.Code, w.Body.String())
	}
}

// TestChat_UnicodeContent 验证 Unicode 消息内容
func TestChat_UnicodeContent(t *testing.T) {
	_, _ = setupChatTest()

	reqBody, _ := json.Marshal(ChatRequest{Content: "你好世界 🌍 日本語テスト"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(reqBody)))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	// Unicode 内容不会立即返回错误，会尝试调用 Agent（返回 502）
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (agent unavailable), got %d: %s", w.Code, w.Body.String())
	}
}

// TestSanitizeError_Token 验证 token 模式被净化
func TestSanitizeError_Token(t *testing.T) {
	err := fmt.Errorf("authentication failed with token=secret_token_value")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "secret_token_value") {
		t.Error("sanitized error should not contain the token value")
	}
}

// TestSanitizeError_Secret 验证 secret 模式被净化
func TestSanitizeError_Secret(t *testing.T) {
	err := fmt.Errorf("config error: secret=my-secret-value")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "my-secret-value") {
		t.Error("sanitized error should not contain the secret value")
	}
}

// TestSanitizeError_Password 验证 password 模式被净化
func TestSanitizeError_Password(t *testing.T) {
	err := fmt.Errorf("login failed: password=supersecret")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "supersecret") {
		t.Error("sanitized error should not contain the password")
	}
}

// TestSanitizeError_AuthorizationHeader 验证 Authorization header 被净化
func TestSanitizeError_AuthorizationHeader(t *testing.T) {
	err := fmt.Errorf("request failed: Authorization: Bearer sk-1234567890abcdef")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "sk-1234567890abcdef") {
		t.Error("sanitized error should not contain the API key")
	}
}

// TestSanitizeError_APIKeyVariant 验证 api-key 变体被净化
func TestSanitizeError_APIKeyVariant(t *testing.T) {
	err := fmt.Errorf("api-key: sk-abcdefghijklmnopqrstuvwxyz123456")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "sk-abc") {
		t.Error("sanitized error should not contain the API key")
	}
}

// TestSanitizeError_NoMatch 验证不匹配敏感模式的错误原样返回
func TestSanitizeError_NoMatch(t *testing.T) {
	original := "connection timeout after 30 seconds"
	err := fmt.Errorf("%s", original)
	sanitized := sanitizeError(err)

	if sanitized != original {
		t.Errorf("expected %q, got %q", original, sanitized)
	}
}

// TestSanitizeError_EmptyMessage 验证空错误消息
func TestSanitizeError_EmptyMessage(t *testing.T) {
	err := fmt.Errorf("")
	sanitized := sanitizeError(err)

	if sanitized != "" {
		t.Errorf("expected empty string, got %q", sanitized)
	}
}

// TestSanitizeError_MultiplePatterns 验证多个敏感模式同时存在
func TestSanitizeError_MultiplePatterns(t *testing.T) {
	err := fmt.Errorf("error: Bearer sk-key and api_key=sk-another-key")
	sanitized := sanitizeError(err)

	if strings.Contains(sanitized, "sk-key") || strings.Contains(sanitized, "sk-another-key") {
		t.Error("sanitized error should not contain any API keys")
	}
}

// TestChat_LockedResponseCode 验证锁定时返回正确的错误码
func TestChat_LockedResponseCode(t *testing.T) {
	keystore.Reset()
	ks := keystore.Instance()
	ks.SetAPIKeyConfigured(true)
	ks.Lock()

	reqBody, _ := json.Marshal(ChatRequest{Content: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(reqBody)))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodePermissionDenied {
		t.Errorf("expected code %d, got %d", response.CodePermissionDenied, resp.Code)
	}
}

// TestChat_InvalidJSONResponseCode 验证无效 JSON 返回正确的错误码
func TestChat_InvalidJSONResponseCode(t *testing.T) {
	_, _ = setupChatTest()

	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var resp response.Response
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.Code != response.CodeInvalidParam {
		t.Errorf("expected code %d, got %d", response.CodeInvalidParam, resp.Code)
	}
}

// TestChat_AgentUnavailable 验证 Agent 不可用时返回 502
func TestChat_AgentUnavailable(t *testing.T) {
	_, _ = setupChatTest()

	reqBody, _ := json.Marshal(ChatRequest{Content: "Hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(string(reqBody)))
	w := httptest.NewRecorder()

	h := &ChatHandler{}
	h.Post(w, req)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
}

// TestChat_LoadConversationHistory_CorruptedFile 验证对话文件损坏时返回空历史
func TestChat_LoadConversationHistory_CorruptedFile(t *testing.T) {
	keystore.Reset()
	storage.ResetDataDirCache()

	dir, _ := storage.DataDir()
	convPath := filepath.Join(dir, "conversations.json")
	os.WriteFile(convPath, []byte("{invalid json"), 0600)
	defer os.Remove(convPath)

	h := &ChatHandler{}
	history := h.loadConversationHistory()

	if len(history) != 0 {
		t.Errorf("expected empty history for corrupted file, got %d messages", len(history))
	}
}

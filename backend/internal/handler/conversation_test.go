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

	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func TestConversationHandler_Get_Empty(t *testing.T) {
	// 确保测试前文件不存在，避免被其他测试的数据污染
	dir, _ := storage.DataDir()
	os.Remove(filepath.Join(dir, "conversations.json"))

	handler := &ConversationHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages is not an array: %T", data["messages"])
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(messages))
	}
}

func TestConversationHandler_Post_And_Get(t *testing.T) {
	handler := &ConversationHandler{}

	body := `{"messages":[{"id":"msg_123","role":"user","content":"hello","timestamp":1700000000000}],"updated_at":1700000000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec = httptest.NewRecorder()

	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages is not an array: %T", data["messages"])
	}
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func TestConversationHandler_Get_NullMessages(t *testing.T) {
	handler := &ConversationHandler{}

	// 先写入一个 messages 为 null 的 JSON
	body := `{"messages":null,"updated_at":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.Post(rec, req)

	// 验证 GET 返回空数组
	req = httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec = httptest.NewRecorder()
	handler.Get(rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages should be an array, got: %T", data["messages"])
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages for null, got %d", len(messages))
	}
}

func TestConversationHandler_Post_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader("{bad"))
	rec := httptest.NewRecorder()

	convHandler := &ConversationHandler{}
	convHandler.Post(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestConversationHandler_Post_EmptyContent(t *testing.T) {
	handler := &ConversationHandler{}
	body := `{"messages":[{"id":"msg_1","role":"user","content":"  ","timestamp":1700000000000}],"updated_at":1700000000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestConversationHandler_Post_TooLongContent(t *testing.T) {
	handler := &ConversationHandler{}
	longContent := strings.Repeat("a", 10001)
	body := fmt.Sprintf(`{"messages":[{"id":"msg_1","role":"user","content":"%s","timestamp":1700000000000}],"updated_at":1700000000000}`, longContent)
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestConversationHandler_Post_InvalidRole(t *testing.T) {
	handler := &ConversationHandler{}
	body := `{"messages":[{"id":"msg_1","role":"admin","content":"hello","timestamp":1700000000000}],"updated_at":1700000000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestConversationHandler_ResponseFormat(t *testing.T) {
	handler := &ConversationHandler{}
	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()

	handler.Get(rec, req)

	var raw map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	required := []string{"code", "message", "data", "timestamp", "request_id"}
	for _, field := range required {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}
}

// TestConversationHandler_Post_EmptyMessagesArray 验证空消息数组可以保存
func TestConversationHandler_Post_EmptyMessagesArray(t *testing.T) {
	handler := &ConversationHandler{}
	body := `{"messages":[],"updated_at":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestConversationHandler_Post_MultipleMessages 验证多条消息保存和读取
func TestConversationHandler_Post_MultipleMessages(t *testing.T) {
	handler := &ConversationHandler{}
	body := `{"messages":[{"id":"msg_1","role":"user","content":"hello","timestamp":1700000000000},{"id":"msg_2","role":"assistant","content":"hi there","timestamp":1700000001000},{"id":"msg_3","role":"user","content":"how are you","timestamp":1700000002000}],"updated_at":1700000002000}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// GET 验证
	req = httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec = httptest.NewRecorder()
	handler.Get(rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages is not an array: %T", data["messages"])
	}
	if len(messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(messages))
	}
}

// TestConversationHandler_Post_ContentExactly10000Chars 验证恰好 10000 字符可以通过
func TestConversationHandler_Post_ContentExactly10000Chars(t *testing.T) {
	handler := &ConversationHandler{}
	longContent := strings.Repeat("a", 10000)
	body := fmt.Sprintf(`{"messages":[{"id":"msg_1","role":"user","content":"%s","timestamp":1700000000000}],"updated_at":1700000000000}`, longContent)
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for exactly 10000 chars, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestConversationHandler_Post_Content10001Chars 验证 10001 字符被拒绝
func TestConversationHandler_Post_Content10001Chars(t *testing.T) {
	handler := &ConversationHandler{}
	longContent := strings.Repeat("a", 10001)
	body := fmt.Sprintf(`{"messages":[{"id":"msg_1","role":"user","content":"%s","timestamp":1700000000000}],"updated_at":1700000000000}`, longContent)
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for 10001 chars, got %d", rec.Code)
	}
}

// TestConversationHandler_Post_ValidRoles 验证 user 和 assistant 角色都有效
func TestConversationHandler_Post_ValidRoles(t *testing.T) {
	handler := &ConversationHandler{}

	roles := []string{"user", "assistant"}
	for _, role := range roles {
		body := fmt.Sprintf(`{"messages":[{"id":"msg_1","role":"%s","content":"test","timestamp":1700000000000}],"updated_at":1700000000000}`, role)
		req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.Post(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("role %q: expected 200, got %d", role, rec.Code)
		}
	}
}

// TestConversationHandler_Post_EmptyJSONBody 验证空 JSON body 被拒绝
func TestConversationHandler_Post_EmptyJSONBody(t *testing.T) {
	handler := &ConversationHandler{}
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(""))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", rec.Code)
	}
}

// TestConversationHandler_Post_MissingMessagesField 验证缺少 messages 字段时 Go 使用零值
func TestConversationHandler_Post_MissingMessagesField(t *testing.T) {
	handler := &ConversationHandler{}
	body := `{"updated_at":12345}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	// 缺少 messages 字段时，Go 将其解码为零值（nil），应返回 200 并保存空消息
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for missing messages field (zero value), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestConversationHandler_Post_MessageWithSpecialChars 验证消息内容包含特殊字符
func TestConversationHandler_Post_MessageWithSpecialChars(t *testing.T) {
	handler := &ConversationHandler{}
	body := `{"messages":[{"id":"msg_1","role":"user","content":"Hello \"world\" \n\t中文 🎉","timestamp":1700000000000}],"updated_at":1700000000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for special chars, got %d: %s", rec.Code, rec.Body.String())
	}

	// 验证读取后内容完整
	req = httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec = httptest.NewRecorder()
	handler.Get(rec, req)

	var resp response.Response
	json.Unmarshal(rec.Body.Bytes(), &resp)
	data := resp.Data.(map[string]interface{})
	messages := data["messages"].([]interface{})
	firstMsg := messages[0].(map[string]interface{})
	if firstMsg["content"] != "Hello \"world\" \n\t中文 🎉" {
		t.Errorf("content mismatch: got %v", firstMsg["content"])
	}
}

// TestConversationHandler_Get_CorruptedFile 验证文件损坏时返回空对话
func TestConversationHandler_Get_CorruptedFile(t *testing.T) {
	handler := &ConversationHandler{}

	// 直接写入损坏的 JSON 到文件（绕过 POST 的 JSON 解析）
	dir, err := storage.DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	corruptPath := filepath.Join(dir, "conversations.json")
	if err := os.WriteFile(corruptPath, []byte("{invalid json"), 0600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}
	// 测试结束后清理
	defer os.Remove(corruptPath)

	// GET 应该返回空对话（因为文件内容损坏，解析失败）
	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()
	handler.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even with corrupted file, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}

	data, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data is not a map: %T", resp.Data)
	}

	messages, ok := data["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages should be an array for corrupted file, got: %T", data["messages"])
	}
	if len(messages) != 0 {
		t.Errorf("expected 0 messages for corrupted file, got %d", len(messages))
	}
}

// TestConversationHandler_Post_EmptyID 验证空 ID 仍然可以保存（ID 不是必填字段）
func TestConversationHandler_Post_EmptyID(t *testing.T) {
	handler := &ConversationHandler{}
	body := `{"messages":[{"id":"","role":"user","content":"test","timestamp":1700000000000}],"updated_at":1700000000000}`
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty ID, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestEmptyConversation 验证 EmptyConversation 返回正确的空结构
func TestEmptyConversation(t *testing.T) {
	conv := EmptyConversation()
	if conv.Messages == nil {
		t.Error("Messages should not be nil")
	}
	if len(conv.Messages) != 0 {
		t.Errorf("expected 0 messages, got %d", len(conv.Messages))
	}
	if conv.UpdatedAt != 0 {
		t.Errorf("expected UpdatedAt 0, got %d", conv.UpdatedAt)
	}
}

// TestConversationHandler_Post_Chinese10000Chars 验证中文恰好 10000 字符的消息应通过
func TestConversationHandler_Post_Chinese10000Chars(t *testing.T) {
	handler := &ConversationHandler{}
	// 10000 个中文字符 = 10000 个 rune，恰好等于限制
	chineseContent := strings.Repeat("中", 10000)
	body := fmt.Sprintf(`{"messages":[{"id":"msg_1","role":"user","content":"%s","timestamp":1700000000000}],"updated_at":1700000000000}`, chineseContent)
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for Chinese 10000 chars (exactly at limit), got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestConversationHandler_Post_Chinese10001Chars 验证中文 10001 字符的消息应被拒绝
func TestConversationHandler_Post_Chinese10001Chars(t *testing.T) {
	handler := &ConversationHandler{}
	// 10001 个中文字符 = 10001 个 rune，超过限制
	chineseContent := strings.Repeat("文", 10001)
	body := fmt.Sprintf(`{"messages":[{"id":"msg_1","role":"user","content":"%s","timestamp":1700000000000}],"updated_at":1700000000000}`, chineseContent)
	req := httptest.NewRequest(http.MethodPost, "/api/conversation", strings.NewReader(body))
	rec := httptest.NewRecorder()

	handler.Post(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for Chinese 10001 chars (exceeds limit), got %d: %s", rec.Code, rec.Body.String())
	}
}

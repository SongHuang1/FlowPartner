package response

import (
	"encoding/json"
	"testing"
	"time"
)

// TestError_SetsCorrectFields 验证 Error() 正确设置所有字段
func TestError_SetsCorrectFields(t *testing.T) {
	resp := Error(CodeInvalidParam, "参数错误")

	if resp.Code != CodeInvalidParam {
		t.Errorf("expected code %d, got %d", CodeInvalidParam, resp.Code)
	}
	if resp.Message != "参数错误" {
		t.Errorf("expected message '参数错误', got '%s'", resp.Message)
	}
	if resp.Data != nil {
		t.Errorf("expected nil data, got %v", resp.Data)
	}
}

// TestError_TimestampIsRecent 验证 Error() 的时间戳在合理范围内
func TestError_TimestampIsRecent(t *testing.T) {
	before := time.Now().Unix()
	resp := Error(CodeInternalError, "内部错误")
	after := time.Now().Unix()

	if resp.Timestamp < before || resp.Timestamp > after {
		t.Errorf("timestamp %d not in range [%d, %d]", resp.Timestamp, before, after)
	}
}

// TestSuccess_SetsCorrectFields 验证 Success() 正确设置所有字段
func TestSuccess_SetsCorrectFields(t *testing.T) {
	resp := Success(map[string]string{"key": "value"})

	if resp.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got '%s'", resp.Message)
	}
	if resp.Data == nil {
		t.Error("expected non-nil data")
	}
}

// TestSuccess_TimestampIsRecent 验证 Success() 的时间戳在合理范围内
func TestSuccess_TimestampIsRecent(t *testing.T) {
	before := time.Now().Unix()
	resp := Success("test")
	after := time.Now().Unix()

	if resp.Timestamp < before || resp.Timestamp > after {
		t.Errorf("timestamp %d not in range [%d, %d]", resp.Timestamp, before, after)
	}
}

// TestSuccess_WithNilData 验证 Success(nil) 正常工作
func TestSuccess_WithNilData(t *testing.T) {
	resp := Success(nil)

	if resp.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Code)
	}
	if resp.Message != "success" {
		t.Errorf("expected message 'success', got '%s'", resp.Message)
	}
	if resp.Data != nil {
		t.Errorf("expected nil data, got %v", resp.Data)
	}
}

// TestSuccess_WithComplexData 验证 Success() 能携带复杂数据结构
func TestSuccess_WithComplexData(t *testing.T) {
	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	data := User{Name: "张三", Age: 30}
	resp := Success(data)

	if resp.Code != CodeOK {
		t.Errorf("expected code %d, got %d", CodeOK, resp.Code)
	}

	// 验证 Data 可以被序列化
	jsonBytes, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal data: %v", err)
	}

	var result User
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		t.Fatalf("failed to unmarshal data: %v", err)
	}
	if result.Name != "张三" || result.Age != 30 {
		t.Errorf("data mismatch: got %+v", result)
	}
}

// TestResponse_JSONSerialization 验证 Response 结构体的 JSON 标签正确
func TestResponse_JSONSerialization(t *testing.T) {
	resp := Response{
		Code:      2001,
		Message:   "error",
		Data:      map[string]int{"count": 42},
		Timestamp: 1700000000,
		RequestID: "test-uuid",
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// 验证 JSON 字段名与标签一致
	var raw map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	expectedFields := []string{"code", "message", "data", "timestamp", "request_id"}
	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing JSON field: %s", field)
		}
	}
}

// TestResponse_JSONDeserialization 验证可以从 JSON 反序列化
func TestResponse_JSONDeserialization(t *testing.T) {
	jsonStr := `{"code":1001,"message":"参数错误","data":null,"timestamp":1700000000,"request_id":"abc-123"}`

	var resp Response
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if resp.Code != 1001 {
		t.Errorf("expected code 1001, got %d", resp.Code)
	}
	if resp.Message != "参数错误" {
		t.Errorf("expected message '参数错误', got '%s'", resp.Message)
	}
	if resp.Timestamp != 1700000000 {
		t.Errorf("expected timestamp 1700000000, got %d", resp.Timestamp)
	}
	if resp.RequestID != "abc-123" {
		t.Errorf("expected request_id 'abc-123', got '%s'", resp.RequestID)
	}
}

// TestError_ZeroCode 验证 Error() 允许 code 为 0（边界情况）
func TestError_ZeroCode(t *testing.T) {
	resp := Error(0, "zero code error")

	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
	if resp.Message != "zero code error" {
		t.Errorf("expected message 'zero code error', got '%s'", resp.Message)
	}
}

// TestError_NegativeCode 验证 Error() 允许负数 code（边界情况）
func TestError_NegativeCode(t *testing.T) {
	resp := Error(-1, "negative code")

	if resp.Code != -1 {
		t.Errorf("expected code -1, got %d", resp.Code)
	}
}

// TestError_EmptyMessage 验证 Error() 允许空消息
func TestError_EmptyMessage(t *testing.T) {
	resp := Error(CodeInternalError, "")

	if resp.Message != "" {
		t.Errorf("expected empty message, got '%s'", resp.Message)
	}
}

// TestSuccess_EmptyData 验证 Success() 携带空字符串数据
func TestSuccess_EmptyData(t *testing.T) {
	resp := Success("")

	if resp.Data != "" {
		t.Errorf("expected empty string data, got '%v'", resp.Data)
	}
}

// TestResponse_DataTypes 验证 Data 字段可以承载多种类型
func TestResponse_DataTypes(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{"string", "hello"},
		{"int", 42},
		{"float", 3.14},
		{"bool", true},
		{"slice", []int{1, 2, 3}},
		{"map", map[string]string{"a": "b"}},
		{"struct", struct{ X int }{X: 10}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := Success(tt.data)

			jsonBytes, err := json.Marshal(resp)
			if err != nil {
				t.Fatalf("failed to marshal response with %s data: %v", tt.name, err)
			}

			var raw struct {
				Code      int         `json:"code"`
				Message   string      `json:"message"`
				Data      interface{} `json:"data"`
				Timestamp int64       `json:"timestamp"`
				RequestID string      `json:"request_id"`
			}
			if err := json.Unmarshal(jsonBytes, &raw); err != nil {
				t.Fatalf("failed to unmarshal response with %s data: %v", tt.name, err)
			}

			if raw.Code != CodeOK {
				t.Errorf("expected code %d, got %d", CodeOK, raw.Code)
			}
		})
	}
}

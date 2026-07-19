package response

import "testing"

// TestCodeValues_Unchanged 验证错误码常量值未被意外修改
// 这些值一旦确定就是 API 契约的一部分，变更会导致客户端兼容性问题
func TestCodeValues_Unchanged(t *testing.T) {
	tests := []struct {
		name     string
		got      int
		expected int
	}{
		{"CodeOK", CodeOK, 0},
		{"CodeInvalidParam", CodeInvalidParam, 1001},
		{"CodeMissingParam", CodeMissingParam, 1002},
		{"CodeInternalError", CodeInternalError, 2001},
		{"CodeDangerousAction", CodeDangerousAction, 4001},
		{"CodePermissionDenied", CodePermissionDenied, 4002},
		{"CodeUserRejected", CodeUserRejected, 4003},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("%s = %d, want %d (changing error codes breaks API compatibility)", tt.name, tt.got, tt.expected)
			}
		})
	}
}

// TestCodeRanges 验证错误码在正确的范围内
func TestCodeRanges(t *testing.T) {
	tests := []struct {
		name      string
		code      int
		min, max  int
	}{
		{"CodeOK", CodeOK, 0, 0},
		{"CodeInvalidParam", CodeInvalidParam, 1000, 1999},
		{"CodeMissingParam", CodeMissingParam, 1000, 1999},
		{"CodeInternalError", CodeInternalError, 2000, 2999},
		{"CodeDangerousAction", CodeDangerousAction, 4000, 4999},
		{"CodePermissionDenied", CodePermissionDenied, 4000, 4999},
		{"CodeUserRejected", CodeUserRejected, 4000, 4999},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code < tt.min || tt.code > tt.max {
				t.Errorf("%s = %d, expected range [%d, %d]", tt.name, tt.code, tt.min, tt.max)
			}
		})
	}
}

// TestCodeUniqueness 验证所有错误码互不重复
func TestCodeUniqueness(t *testing.T) {
	codes := map[string]int{
		"CodeOK":              CodeOK,
		"CodeInvalidParam":    CodeInvalidParam,
		"CodeMissingParam":    CodeMissingParam,
		"CodeInternalError":   CodeInternalError,
		"CodeDangerousAction": CodeDangerousAction,
		"CodePermissionDenied": CodePermissionDenied,
		"CodeUserRejected":    CodeUserRejected,
	}

	// 用 map 检测重复值
	seen := make(map[int]string)
	for name, code := range codes {
		if prevName, exists := seen[code]; exists {
			t.Errorf("duplicate code %d: %s and %s share the same value", code, prevName, name)
		}
		seen[code] = name
	}
}

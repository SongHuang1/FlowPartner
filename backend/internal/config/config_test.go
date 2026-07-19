package config

import (
	"os"
	"testing"
)

// TestLoad_DefaultPort 验证未设置环境变量时使用默认端口
func TestLoad_DefaultPort(t *testing.T) {
	// 确保环境变量不存在
	os.Unsetenv("FP_HTTP_PORT")
	
	cfg := Load()
	
	if cfg.HTTPPort != ":8090" {
		t.Errorf("expected default port :8090, got %s", cfg.HTTPPort)
	}
}

// TestLoad_CustomPort 验证设置环境变量时使用自定义端口
func TestLoad_CustomPort(t *testing.T) {
	t.Setenv("FP_HTTP_PORT", ":9090")
	
	cfg := Load()
	
	if cfg.HTTPPort != ":9090" {
		t.Errorf("expected custom port :9090, got %s", cfg.HTTPPort)
	}
}

// TestLoad_EmptyEnvVar 验证环境变量设置为空字符串时的行为
func TestLoad_EmptyEnvVar(t *testing.T) {
	t.Setenv("FP_HTTP_PORT", "")
	
	cfg := Load()
	
	// os.LookupEnv 对已设置但为空的变量返回 true
	// 所以空字符串会被使用，而不是 fallback
	if cfg.HTTPPort != "" {
		t.Errorf("expected empty string when env var is set to empty, got %s", cfg.HTTPPort)
	}
}

// TestLoad_PortFormat 验证各种端口格式都能正确传递
func TestLoad_PortFormat(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected string
	}{
		{"standard port", ":8080", ":8080"},
		{"localhost port", "localhost:3000", "localhost:3000"},
		{"ip port", "127.0.0.1:8080", "127.0.0.1:8080"},
		{"no colon port", "8080", "8080"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("FP_HTTP_PORT", tt.envValue)
			
			cfg := Load()
			
			if cfg.HTTPPort != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, cfg.HTTPPort)
			}
		})
	}
}

// TestLoad_ReturnsNonNil 验证 Load 永远不返回 nil
func TestLoad_ReturnsNonNil(t *testing.T) {
	cfg := Load()
	
	if cfg == nil {
		t.Fatal("Load() returned nil, expected non-nil *Config")
	}
}

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_DefaultValues(t *testing.T) {
	os.Unsetenv("FP_HTTP_PORT")
	os.Unsetenv("FP_FRONTEND_DIR")
	os.Unsetenv("FP_DEV_MODE")

	cfg := Load()

	if cfg.HTTPPort != ":8080" {
		t.Errorf("expected default port :8080, got %s", cfg.HTTPPort)
	}
	if cfg.DevMode != false {
		t.Errorf("expected DevMode false, got %v", cfg.DevMode)
	}
}

func TestLoad_CustomPort(t *testing.T) {
	t.Setenv("FP_HTTP_PORT", ":9090")
	defer os.Unsetenv("FP_HTTP_PORT")

	cfg := Load()

	if cfg.HTTPPort != ":9090" {
		t.Errorf("expected custom port :9090, got %s", cfg.HTTPPort)
	}
}

func TestLoad_CustomFrontendDir(t *testing.T) {
	t.Setenv("FP_FRONTEND_DIR", filepath.VolumeName("D:") + `\custom\path`)
	defer os.Unsetenv("FP_FRONTEND_DIR")

	cfg := Load()

	if !filepath.IsAbs(cfg.FrontendDir) {
		t.Errorf("expected absolute path for FrontendDir, got %q", cfg.FrontendDir)
	}
	if cfg.FrontendDir != filepath.VolumeName("D:")+`\custom\path` {
		t.Errorf("expected custom path preserved, got %q", cfg.FrontendDir)
	}
}

func TestLoad_DevModeTrue(t *testing.T) {
	t.Setenv("FP_DEV_MODE", "true")
	defer os.Unsetenv("FP_DEV_MODE")

	cfg := Load()

	if !cfg.DevMode {
		t.Errorf("expected DevMode true, got false")
	}
}

func TestLoad_DevModeInvalidFallsBack(t *testing.T) {
	t.Setenv("FP_DEV_MODE", "invalid")
	defer os.Unsetenv("FP_DEV_MODE")

	cfg := Load()

	if cfg.DevMode {
		t.Errorf("expected DevMode false for invalid value, got true")
	}
}

func TestLoad_FrontendDir_CWDResolution(t *testing.T) {
	os.Unsetenv("FP_FRONTEND_DIR")

	cfg := Load()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	expected := filepath.Join(wd, "../frontend/dist")
	expectedAbs, _ := filepath.Abs(expected)

	if cfg.FrontendDir != expectedAbs {
		t.Errorf("expected FrontendDir %q, got %q", expectedAbs, cfg.FrontendDir)
	}
}

func TestLoad_ReturnsNonNil(t *testing.T) {
	cfg := Load()

	if cfg == nil {
		t.Fatal("Load() returned nil, expected non-nil *Config")
	}
}

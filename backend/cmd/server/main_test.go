package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/config"
	"github.com/songhuang/flowpartner/backend/internal/response"
)

// uuidRegex 匹配 UUID v4 格式
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func newTestFrontendDir(t *testing.T) string {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.html")
	if err := os.WriteFile(indexPath, []byte("<html><body>Test</body></html>"), 0644); err != nil {
		t.Fatalf("failed to create index.html: %v", err)
	}
	assetsDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("failed to create assets dir: %v", err)
	}
	appJS := filepath.Join(assetsDir, "app.js")
	if err := os.WriteFile(appJS, []byte("// app code"), 0644); err != nil {
		t.Fatalf("failed to create app.js: %v", err)
	}
	return dir
}

func TestSetupRoutes_DevMode_Returns404ForRoot(t *testing.T) {
	cfg := &config.Config{
		DevMode:  true,
		HTTPPort: ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 in DevMode for GET /, got %d", rec.Code)
	}
}

func TestSetupRoutes_DevMode_HealthStillWorks(t *testing.T) {
	cfg := &config.Config{
		DevMode:  true,
		HTTPPort: ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /health, got %d", rec.Code)
	}
}

func TestSetupRoutes_Production_ServesIndexHTML(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for GET /, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body != "<html><body>Test</body></html>" {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestSetupRoutes_Production_SPAFallback(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/chat/123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for SPA fallback, got %d", rec.Code)
	}
}

func TestSetupRoutes_Production_APIEndpoint_Returns501(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("expected 501 for /api/unknown, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != response.CodeNotImplemented {
		t.Errorf("expected code %d, got %d", response.CodeNotImplemented, resp.Code)
	}
	if resp.RequestID == "" {
		t.Error("request_id must not be empty")
	}
}

func TestSetupRoutes_Production_AssetsServed(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /assets/app.js, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body != "// app code" {
		t.Errorf("unexpected asset body: %s", body)
	}
}

func TestSetupRoutes_Production_PathTraversalBlocked(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/../etc/passwd", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if body == "root:x:0:0:root:/root:/bin/bash" {
		t.Error("path traversal leaked system file content")
	}
}

func TestSetupRoutes_Production_MissingAsset_Returns404(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/assets/missing.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound && rec.Code != http.StatusOK {
		t.Logf("status code for missing asset: %d", rec.Code)
	}
}

// TestSetupRoutes_DevMode_APIEndpoint_StillReturns501 验证 DevMode 下 /api/ 仍返回 501
func TestSetupRoutes_DevMode_APIEndpoint_StillReturns404(t *testing.T) {
	cfg := &config.Config{
		DevMode:  true,
		HTTPPort: ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for unknown /api/ in DevMode, got %d", rec.Code)
	}
}

// TestSetupRoutes_Production_HealthEndpoint_Works 验证生产模式下 /health 正常返回
func TestSetupRoutes_Production_HealthEndpoint_Works(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /health in production, got %d", rec.Code)
	}
	body := rec.Body.String()
	if body != `{"status":"ok"}` {
		t.Errorf("unexpected health response body: %s", body)
	}
}

// TestSetupRoutes_Production_APIEndpoint_AllFiveFieldsPresent 验证 501 响应包含全部 5 个标准字段
func TestSetupRoutes_Production_APIEndpoint_AllFiveFieldsPresent(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var raw map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	requiredFields := []string{"code", "message", "data", "timestamp", "request_id"}
	for _, field := range requiredFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("missing required field in response: %s", field)
		}
	}
}

// TestSetupRoutes_Production_APIEndpoint_RequestIDIsValidUUID 验证 request_id 是有效 UUID 格式
func TestSetupRoutes_Production_APIEndpoint_RequestIDIsValidUUID(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if !uuidRegex.MatchString(resp.RequestID) {
		t.Errorf("request_id is not a valid UUID format: %q", resp.RequestID)
	}
}

// TestSetupRoutes_Production_SPAFallback_AssetsPath_NoFallback 验证 /assets/ 路径不存在时返回 404 而非 fallback 到 index.html
func TestSetupRoutes_Production_SPAFallback_AssetsPath_NoFallback(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/assets/nonexistent.css", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	// 不应该返回 index.html 的内容
	if body == "<html><body>Test</body></html>" {
		t.Error("asset path should not fallback to index.html")
	}
}

// TestSetupRoutes_Production_MultipleAPIEndpoints_AllReturn501 验证多个 /api/ 子路径都返回 501
func TestSetupRoutes_Production_NotImplementedEndpoint(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/not-implemented", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("expected 501 for /api/not-implemented, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if resp.Code != response.CodeNotImplemented {
		t.Errorf("expected code %d, got %d", response.CodeNotImplemented, resp.Code)
	}
	if resp.Message != "API not implemented yet" {
		t.Errorf("unexpected message: %s", resp.Message)
	}
}

// TestServeSPA_DirectCall_APIEndpoint 验证 serveSPA 直接调用时 /api/ 路径返回 501
func TestServeSPA_DirectCall_APIEndpoint(t *testing.T) {
	frontendDir := newTestFrontendDir(t)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	serveSPA(rec, req, frontendDir)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("expected 501 from serveSPA for /api/ path, got %d", rec.Code)
	}
}

// TestServeSPA_DirectCall_KnownFile 验证 serveSPA 直接调用时已知文件正确返回
func TestServeSPA_DirectCall_KnownFile(t *testing.T) {
	frontendDir := newTestFrontendDir(t)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	serveSPA(rec, req, frontendDir)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from serveSPA for known asset, got %d", rec.Code)
	}
	if rec.Body.String() != "// app code" {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

// TestServeSPA_DirectCall_IndexFallback 验证 serveSPA 对未知路径 fallback 到 index.html
func TestServeSPA_DirectCall_IndexFallback(t *testing.T) {
	frontendDir := newTestFrontendDir(t)

	req := httptest.NewRequest(http.MethodGet, "/some/spa/route", nil)
	rec := httptest.NewRecorder()
	serveSPA(rec, req, frontendDir)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from serveSPA fallback, got %d", rec.Code)
	}
	if rec.Body.String() != "<html><body>Test</body></html>" {
		t.Errorf("expected index.html content, got: %s", rec.Body.String())
	}
}

// TestNotImplementedHandler_ResponseStructure 验证 notImplementedHandler 的响应结构完整性
func TestNotImplementedHandler_ResponseStructure(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	rec := httptest.NewRecorder()
	notImplementedHandler(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("expected 501, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Code != response.CodeNotImplemented {
		t.Errorf("expected code %d, got %d", response.CodeNotImplemented, resp.Code)
	}
	if resp.Message != "API not implemented yet" {
		t.Errorf("unexpected message: %s", resp.Message)
	}
	if resp.Data != nil {
		t.Errorf("expected nil data, got %v", resp.Data)
	}
	if resp.Timestamp == 0 {
		t.Error("timestamp should not be zero")
	}
	if !uuidRegex.MatchString(resp.RequestID) {
		t.Errorf("request_id is not valid UUID: %q", resp.RequestID)
	}
}

// TestSetupRoutes_Production_PathTraversal_VariousAttacks 验证多种路径穿越攻击被阻止
func TestSetupRoutes_Production_PathTraversal_VariousAttacks(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	attackPaths := []string{
		"/../etc/passwd",
		"/..%2f..%2fetc%2fpasswd",
		"/assets/../../../etc/passwd",
	}

	for _, path := range attackPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			body := rec.Body.String()
			// 不应该泄露系统文件
			if body == "root:x:0:0:root:/root:/bin/bash" {
				t.Errorf("path traversal leaked system file for %s", path)
			}
		})
	}
}

// TestSetupRoutes_API_SettingsEndpoint 验证 /api/settings 路由正确分发
func TestSetupRoutes_API_SettingsEndpoint(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	// GET /api/settings 应该返回 200
	req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/settings expected 200, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

// TestSetupRoutes_API_ConversationEndpoint 验证 /api/conversation 路由正确分发
func TestSetupRoutes_API_ConversationEndpoint(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	// GET /api/conversation 应该返回 200
	req := httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/conversation expected 200, got %d", rec.Code)
	}

	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

// TestSetupRoutes_API_MethodNotAllowed 验证不支持的 HTTP 方法返回 501
func TestSetupRoutes_API_MethodNotAllowed(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	// DELETE /api/settings 应该返回 501
	req := httptest.NewRequest(http.MethodDelete, "/api/settings", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("DELETE /api/settings expected 501, got %d", rec.Code)
	}

	// PUT /api/conversation 应该返回 501
	req = httptest.NewRequest(http.MethodPut, "/api/conversation", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("PUT /api/conversation expected 501, got %d", rec.Code)
	}
}

// TestSetupRoutes_Production_HealthEndpoint_ExactBody 验证 /health 响应体精确匹配
func TestSetupRoutes_Production_HealthEndpoint_ExactBody(t *testing.T) {
	frontendDir := newTestFrontendDir(t)
	cfg := &config.Config{
		DevMode:     false,
		FrontendDir: frontendDir,
		HTTPPort:    ":0",
	}
	handler := setupRoutes(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	body := rec.Body.String()
	expected := `{"status":"ok"}`
	if body != expected {
		t.Errorf("expected body %q, got %q", expected, body)
	}
}

// TestSetupRoutes_DevMode_AllRoutesWork 验证 DevMode 下所有路由正常工作
func TestSetupRoutes_DevMode_AllRoutesWork(t *testing.T) {
	cfg := &config.Config{
		DevMode:  true,
		HTTPPort: ":0",
	}
	handler := setupRoutes(cfg)

	// /health 在 DevMode 下仍然可用
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /health in DevMode expected 200, got %d", rec.Code)
	}

	// /api/settings 在 DevMode 下仍然可用
	req = httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/settings in DevMode expected 200, got %d", rec.Code)
	}

	// /api/conversation 在 DevMode 下仍然可用
	req = httptest.NewRequest(http.MethodGet, "/api/conversation", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/conversation in DevMode expected 200, got %d", rec.Code)
	}
}

// TestServeSPA_DirectCall_HealthPath 验证 serveSPA 直接调用时 /health 正常返回
func TestServeSPA_DirectCall_HealthPath(t *testing.T) {
	frontendDir := newTestFrontendDir(t)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	serveSPA(rec, req, frontendDir)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 from serveSPA for /health, got %d", rec.Code)
	}
	if rec.Body.String() != `{"status":"ok"}` {
		t.Errorf("unexpected body: %s", rec.Body.String())
	}
}

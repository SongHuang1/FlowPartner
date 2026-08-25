package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

func newTestAgentDefHandler() *AgentDefHandler {
	return NewAgentDefHandler(bridge.NewManager(), nil)
}

func decodeAgentDefResponse(t *testing.T, rec *httptest.ResponseRecorder) storage.AgentDef {
	t.Helper()
	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, rec.Body.String())
	}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var def storage.AgentDef
	if err := json.Unmarshal(data, &def); err != nil {
		t.Fatalf("data is not AgentDef: %v (%s)", err, data)
	}
	return def
}

func decodeListMetaResponse(t *testing.T, rec *httptest.ResponseRecorder) []ListMeta {
	t.Helper()
	var resp response.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp.Code != 0 {
		t.Fatalf("expected code 0, got %d: %s", resp.Code, rec.Body.String())
	}
	data, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal data: %v", err)
	}
	var metas []ListMeta
	if err := json.Unmarshal(data, &metas); err != nil {
		t.Fatalf("data is not []ListMeta: %v (%s)", err, data)
	}
	return metas
}

func TestAgentDefHandler_Create_Get_Update_Delete(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	// Create
	body := `{"name":"翻译官","description":"负责中英互译的专职智能体","system_prompt":"你是一个翻译专家。"}`
	rec := httptest.NewRecorder()
	h.Handle(rec, httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	def := decodeAgentDefResponse(t, rec)
	if def.ID == "" {
		t.Fatal("expected non-empty UUID id")
	}
	if def.Name != "翻译官" || def.Description != "负责中英互译的专职智能体" || def.SystemPrompt != "你是一个翻译专家。" {
		t.Errorf("unexpected def: %+v", def)
	}
	if def.CreatedAt == 0 || def.UpdatedAt == 0 {
		t.Errorf("expected timestamps, got %+v", def)
	}

	// Get（详情含 system_prompt）
	rec = httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodGet, "/api/agents/"+def.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", rec.Code)
	}
	got := decodeAgentDefResponse(t, rec)
	if got.SystemPrompt != "你是一个翻译专家。" {
		t.Errorf("detail should include system_prompt, got %q", got.SystemPrompt)
	}

	// Update
	rec = httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodPut, "/api/agents/"+def.ID,
		bytes.NewBufferString(`{"name":"翻译官","description":"新的描述","system_prompt":"新的提示词"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	updated := decodeAgentDefResponse(t, rec)
	if updated.Description != "新的描述" || updated.SystemPrompt != "新的提示词" {
		t.Errorf("update not applied: %+v", updated)
	}
	if updated.CreatedAt != def.CreatedAt {
		t.Errorf("created_at should be preserved, got %d want %d", updated.CreatedAt, def.CreatedAt)
	}

	// Delete
	rec = httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodDelete, "/api/agents/"+def.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE expected 200, got %d", rec.Code)
	}

	// Get after delete → 404
	rec = httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodGet, "/api/agents/"+def.ID, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET after delete expected 404, got %d", rec.Code)
	}
}

func TestAgentDefHandler_Create_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	storage.SetDataDirForTest(dir)
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	rec := httptest.NewRecorder()
	h.Handle(rec, httptest.NewRequest(http.MethodPost, "/api/agents",
		bytes.NewBufferString(`{"name":"A","description":"desc A","system_prompt":"prompt A"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// 落盘 JSON 正确（重新初始化后仍可读取）
	agents, err := storage.LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents: %v", err)
	}
	if len(agents) != 1 || agents[0].Name != "A" || agents[0].SystemPrompt != "prompt A" {
		t.Errorf("persisted agents mismatch: %+v", agents)
	}
}

func TestAgentDefHandler_List_ExcludesSystemPrompt(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	rec := httptest.NewRecorder()
	h.Handle(rec, httptest.NewRequest(http.MethodPost, "/api/agents",
		bytes.NewBufferString(`{"name":"B","description":"desc B","system_prompt":"secret prompt"}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST expected 201, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.Handle(rec, httptest.NewRequest(http.MethodGet, "/api/agents", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET list expected 200, got %d", rec.Code)
	}
	metas := decodeListMetaResponse(t, rec)
	if len(metas) != 2 { // main + B
		t.Fatalf("expected 2 entries (main + B), got %d", len(metas))
	}
	if metas[0].ID != "main" {
		t.Errorf("first entry should be built-in main, got %+v", metas[0])
	}
	// 列表接口不得携带 system_prompt
	raw := rec.Body.String()
	if strings.Contains(raw, "secret prompt") {
		t.Error("list response must not contain system_prompt")
	}
}

func TestAgentDefHandler_Validation(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	cases := []struct {
		name string
		body string
		want int
	}{
		{"empty name", `{"name":"","description":"d","system_prompt":"p"}`, http.StatusBadRequest},
		{"empty description", `{"name":"n","description":"","system_prompt":"p"}`, http.StatusBadRequest},
		{"empty prompt", `{"name":"n","description":"d","system_prompt":""}`, http.StatusBadRequest},
		{"invalid json", `{broken`, http.StatusBadRequest},
		{"name too long", `{"name":"` + strings.Repeat("x", 129) + `","description":"d","system_prompt":"p"}`, http.StatusBadRequest},
		{"description too long", `{"name":"n","description":"` + strings.Repeat("x", 2001) + `","system_prompt":"p"}`, http.StatusBadRequest},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.Handle(rec, httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(c.body)))
		if rec.Code != c.want {
			t.Errorf("%s: expected %d, got %d", c.name, c.want, rec.Code)
		}
	}

	// 校验失败不得落盘
	agents, err := storage.LoadAgents()
	if err != nil {
		t.Fatalf("LoadAgents: %v", err)
	}
	if len(agents) != 0 {
		t.Errorf("invalid definitions must not be persisted, got %d", len(agents))
	}
}

func TestAgentDefHandler_NameConflict(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	body := `{"name":"同名","description":"d","system_prompt":"p"}`
	rec := httptest.NewRecorder()
	h.Handle(rec, httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("first POST expected 201, got %d", rec.Code)
	}

	// 重名 → 409
	rec = httptest.NewRecorder()
	h.Handle(rec, httptest.NewRequest(http.MethodPost, "/api/agents", bytes.NewBufferString(body)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate name expected 409, got %d", rec.Code)
	}

	// 与内置 main 重名 → 409
	rec = httptest.NewRecorder()
	h.Handle(rec, httptest.NewRequest(http.MethodPost, "/api/agents",
		bytes.NewBufferString(`{"name":"主智能体","description":"d","system_prompt":"p"}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("name colliding with main expected 409, got %d", rec.Code)
	}
}

func TestAgentDefHandler_MainAgentNotEditableOrDeletable(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	rec := httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodPut, "/api/agents/main",
		bytes.NewBufferString(`{"name":"主智能体","description":"d","system_prompt":"p"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PUT main expected 400, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodDelete, "/api/agents/main", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("DELETE main expected 400, got %d", rec.Code)
	}

	// GET main 返回合成定义（含 system_prompt = 设置中的系统提示词）
	rec = httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodGet, "/api/agents/main", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET main expected 200, got %d", rec.Code)
	}
	main := decodeAgentDefResponse(t, rec)
	if main.ID != "main" || main.SystemPrompt == "" {
		t.Errorf("built-in main mismatch: %+v", main)
	}
}

func TestAgentDefHandler_Get_NotFound(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	rec := httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodGet, "/api/agents/no-such-id", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestAgentDefHandler_PathTraversalRejected(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	// 注意：http.NewRequest 会先经 net/url 规范化路径，../ 与 %2e%2e 在到达 handler 前已被解析，
	// 因此这里只覆盖能真实到达 HandleByID 的非法 ID 形式；ID 只用于内存匹配、不触碰文件系统。
	for _, path := range []string{"/api/agents/a/b", "/api/agents/with:colon"} {
		rec := httptest.NewRecorder()
		h.HandleByID(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("path %q expected 400, got %d", path, rec.Code)
		}
	}

	// 规范化后的畸形 ID 必须被拒绝（400）或视为不存在（404），不得返回 200
	for _, path := range []string{"/api/agents/%2e%2e", "/api/agents/.."} {
		rec := httptest.NewRecorder()
		h.HandleByID(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code == http.StatusOK || rec.Code == http.StatusCreated {
			t.Errorf("path %q must not resolve to an agent, got %d", path, rec.Code)
		}
	}
}

func TestAgentDefHandler_Update_NotFound(t *testing.T) {
	storage.SetDataDirForTest(t.TempDir())
	storage.ResetDataDirCache()
	h := newTestAgentDefHandler()

	rec := httptest.NewRecorder()
	h.HandleByID(rec, httptest.NewRequest(http.MethodPut, "/api/agents/no-such-id",
		bytes.NewBufferString(`{"name":"n","description":"d","system_prompt":"p"}`)))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
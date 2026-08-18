package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/internal/tools"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestAgentHandler(mgr *bridge.Manager) *AgentHandler {
	return NewAgentHandler(mgr, tools.NewApprovalManager())
}

// fakeCallLLMServer 实现 proto.FlowPartnerService_CallLLMServer
type fakeCallLLMServer struct {
	grpc.ServerStream
	ctx       context.Context
	responses []*proto.LLMResponse
	sendErr   error
}

func (f *fakeCallLLMServer) Send(resp *proto.LLMResponse) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.responses = append(f.responses, resp)
	return nil
}

func (f *fakeCallLLMServer) Context() context.Context { return f.ctx }

// fakeSyncServer 实现 proto.FlowPartnerService_SyncChannelServer
type fakeSyncServer struct {
	grpc.ServerStream
	ctx       context.Context
	events    []*proto.AgentEvent
	recvErrs  []error
	recvIdx   int
	mu        sync.Mutex
	commands  []*proto.ServerCommand
	sendErr   error
	sendCount int
	// stop 非空时，Recv 在事件/错误耗尽后阻塞，直到 stop 关闭（用于断言流仍在运行）
	stop chan struct{}
}

func (f *fakeSyncServer) Recv() (*proto.AgentEvent, error) {
	if f.recvIdx < len(f.recvErrs) {
		err := f.recvErrs[f.recvIdx]
		f.recvIdx++
		return nil, err
	}
	if f.recvIdx < len(f.events) {
		evt := f.events[f.recvIdx]
		f.recvIdx++
		return evt, nil
	}
	if f.stop != nil {
		<-f.stop
		return nil, io.EOF
	}
	return nil, io.EOF
}

func (f *fakeSyncServer) Send(cmd *proto.ServerCommand) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendCount++
	if f.sendErr != nil {
		return f.sendErr
	}
	f.commands = append(f.commands, cmd)
	return nil
}

func (f *fakeSyncServer) commandCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.commands)
}

func (f *fakeSyncServer) Context() context.Context { return f.ctx }

func writeSettingsJSON(t *testing.T, raw string) {
	t.Helper()
	if err := storage.WriteJSON("settings.json", json.RawMessage(raw)); err != nil {
		t.Fatalf("failed to write settings.json: %v", err)
	}
}

func TestAgentHandler_CallLLM_InvalidJSON(t *testing.T) {
	setupTestStorage(t)

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeCallLLMServer{ctx: context.Background()}

	err := h.CallLLM(&proto.LLMRequest{SessionId: "s1", JsonPayload: `{not json`}, stream)
	if err != nil {
		t.Fatalf("CallLLM should not return error, got %v", err)
	}

	if len(stream.responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(stream.responses))
	}
	resp := stream.responses[0]
	if !resp.IsError {
		t.Fatal("expected IsError=true for invalid JSON")
	}
	if resp.MessageId == "" {
		t.Fatal("expected non-empty MessageId")
	}
	var llmErr struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(resp.JsonResponse), &llmErr); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if llmErr.Code != 400 {
		t.Errorf("expected error code 400, got %d", llmErr.Code)
	}
}

func TestAgentHandler_CallLLM_EmptyMessages(t *testing.T) {
	setupTestStorage(t)

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeCallLLMServer{ctx: context.Background()}

	err := h.CallLLM(&proto.LLMRequest{SessionId: "s1", JsonPayload: `{"messages":[]}`}, stream)
	if err != nil {
		t.Fatalf("CallLLM should not return error, got %v", err)
	}

	if len(stream.responses) != 1 || !stream.responses[0].IsError {
		t.Fatal("expected an error response")
	}
}

func TestAgentHandler_CallLLM_NoActiveConfig(t *testing.T) {
	setupTestStorage(t)
	writeSettingsJSON(t, `{"model_configs":[],"active_config_id":""}`)

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeCallLLMServer{ctx: context.Background()}

	req := &proto.LLMRequest{
		SessionId:   "s1",
		JsonPayload: `{"messages":[{"role":"user","content":"hi"}]}`,
	}
	err := h.CallLLM(req, stream)
	if err != nil {
		t.Fatalf("CallLLM should not return error, got %v", err)
	}

	if len(stream.responses) != 1 || !stream.responses[0].IsError {
		t.Fatal("expected an error response")
	}
	if !strings.Contains(stream.responses[0].JsonResponse, "没有激活的模型配置") {
		t.Errorf("unexpected error payload: %s", stream.responses[0].JsonResponse)
	}
}

func TestAgentHandler_CallLLM_LockedKey(t *testing.T) {
	setupTestStorage(t)
	writeSettingsJSON(t, `{"model_configs":[{"id":"cfg-1","name":"Test","base_url":"https://api.example.com/v1","model_name":"gpt-4"}],"active_config_id":"cfg-1"}`)

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeCallLLMServer{ctx: context.Background()}

	req := &proto.LLMRequest{
		SessionId:   "s1",
		JsonPayload: `{"messages":[{"role":"user","content":"hi"}]}`,
	}
	err := h.CallLLM(req, stream)
	if err != nil {
		t.Fatalf("CallLLM should not return error, got %v", err)
	}

	if len(stream.responses) != 1 || !stream.responses[0].IsError {
		t.Fatal("expected an error response")
	}
	if !strings.Contains(stream.responses[0].JsonResponse, "模型配置已锁定") {
		t.Errorf("unexpected error payload: %s", stream.responses[0].JsonResponse)
	}
}

func TestAgentHandler_CallLLM_InvalidBaseURL(t *testing.T) {
	setupTestStorage(t)
	writeSettingsJSON(t, `{"model_configs":[{"id":"cfg-1","name":"Test","base_url":"not-a-url","model_name":"gpt-4"}],"active_config_id":"cfg-1"}`)

	ks := keystore.Instance()
	if err := ks.Unlock([]byte("sk-test-key")); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeCallLLMServer{ctx: context.Background()}

	req := &proto.LLMRequest{
		SessionId:   "s1",
		JsonPayload: `{"messages":[{"role":"user","content":"hi"}]}`,
	}
	err := h.CallLLM(req, stream)
	if err != nil {
		t.Fatalf("CallLLM should not return error, got %v", err)
	}

	if len(stream.responses) != 1 || !stream.responses[0].IsError {
		t.Fatal("expected an error response")
	}
	if !strings.Contains(stream.responses[0].JsonResponse, "接口地址格式无效") {
		t.Errorf("unexpected error payload: %s", stream.responses[0].JsonResponse)
	}
}

func TestAgentHandler_CallLLM_StreamingSuccess(t *testing.T) {
	setupTestStorage(t)

	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			return
		}
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer sseServer.Close()

	cfgJSON := `{"model_configs":[{"id":"cfg-1","name":"Test","base_url":%q,"model_name":"gpt-4","temperature":0.7,"timeout_secs":5}],"active_config_id":"cfg-1"}`
	writeSettingsJSON(t, fmt.Sprintf(cfgJSON, sseServer.URL))

	ks := keystore.Instance()
	if err := ks.Unlock([]byte("sk-test-key")); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeCallLLMServer{ctx: context.Background()}

	req := &proto.LLMRequest{
		SessionId:   "s1",
		JsonPayload: `{"messages":[{"role":"user","content":"hi"}]}`,
	}
	err := h.CallLLM(req, stream)
	if err != nil {
		t.Fatalf("CallLLM failed: %v", err)
	}

	var streamed string
	var errored bool
	for _, resp := range stream.responses {
		if resp.IsError {
			errored = true
			break
		}
		streamed += resp.JsonResponse
	}
	if errored {
		t.Fatalf("unexpected error response: %v", stream.responses)
	}
	if !strings.Contains(streamed, "Hello") || !strings.Contains(streamed, "world") {
		t.Errorf("expected streamed chunks containing Hello and world, got %q", streamed)
	}

	messageIDs := map[string]bool{}
	for _, resp := range stream.responses {
		messageIDs[resp.MessageId] = true
	}
	if len(messageIDs) != 1 {
		t.Errorf("all responses should share one MessageId, got %d", len(messageIDs))
	}
}

func TestAgentHandler_CallLLM_HTTPError(t *testing.T) {
	setupTestStorage(t)

	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer sseServer.Close()

	cfgJSON := `{"model_configs":[{"id":"cfg-1","name":"Test","base_url":%q,"model_name":"gpt-4","temperature":0.7,"timeout_secs":5}],"active_config_id":"cfg-1"}`
	writeSettingsJSON(t, fmt.Sprintf(cfgJSON, sseServer.URL))

	ks := keystore.Instance()
	if err := ks.Unlock([]byte("sk-test-key")); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeCallLLMServer{ctx: context.Background()}

	req := &proto.LLMRequest{
		SessionId:   "s1",
		JsonPayload: `{"messages":[{"role":"user","content":"hi"}]}`,
	}
	err := h.CallLLM(req, stream)
	if err != nil {
		t.Fatalf("CallLLM should not return error, got %v", err)
	}

	if len(stream.responses) != 1 || !stream.responses[0].IsError {
		t.Fatal("expected an error response for 401")
	}
	var llmErr struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal([]byte(stream.responses[0].JsonResponse), &llmErr); err != nil {
		t.Fatalf("error response is not valid JSON: %v", err)
	}
	if llmErr.Code != 401 {
		t.Errorf("expected error code 401, got %d", llmErr.Code)
	}
}

func TestAgentHandler_SyncChannel_EOF(t *testing.T) {
	setupTestStorage(t)

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeSyncServer{ctx: context.Background()}

	err := h.SyncChannel(stream)
	if err != nil {
		t.Fatalf("SyncChannel should return nil on EOF, got %v", err)
	}
}

func TestAgentHandler_SyncChannel_RecvError(t *testing.T) {
	setupTestStorage(t)

	h := newTestAgentHandler(bridge.NewManager())
	stream := &fakeSyncServer{
		ctx:      context.Background(),
		recvErrs: []error{fmt.Errorf("boom")},
	}

	err := h.SyncChannel(stream)
	if err == nil {
		t.Fatal("SyncChannel should return an error on Recv failure")
	}
	if status.Code(err) != codes.Internal {
		t.Errorf("expected Internal status code, got %v", status.Code(err))
	}
}

func TestAgentHandler_SyncChannel_ForwardsCommands(t *testing.T) {
	setupTestStorage(t)

	mgr := bridge.NewManager()
	h := newTestAgentHandler(mgr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &fakeSyncServer{ctx: ctx, stop: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- h.SyncChannel(stream)
	}()

	cmd := &proto.ServerCommand{SessionId: "s1", CommandType: "start_chat", Payload: `{"content":"hi"}`}
	mgr.CmdChan <- cmd

	deadline := time.After(2 * time.Second)
	for stream.commandCount() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for command to be forwarded")
		case <-time.After(5 * time.Millisecond):
		}
	}

	stream.mu.Lock()
	got := make([]*proto.ServerCommand, len(stream.commands))
	copy(got, stream.commands)
	stream.mu.Unlock()
	if len(got) != 1 {
		t.Fatalf("expected 1 forwarded command, got %d", len(got))
	}
	if got[0].CommandType != "start_chat" {
		t.Errorf("unexpected command type %q", got[0].CommandType)
	}
	if got[0].SessionId != "s1" {
		t.Errorf("unexpected session id %q", got[0].SessionId)
	}

	select {
	case err := <-done:
		t.Fatalf("SyncChannel should still be running, got %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(stream.stop)
	if err := <-done; err != nil {
		t.Fatalf("SyncChannel should return nil after stream stop, got %v", err)
	}
}

func TestExecuteTool_UnknownTool(t *testing.T) {
	setupTestStorage(t)

	h := newTestAgentHandler(bridge.NewManager())
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "s1",
		ToolName:  "nonexistent",
		Arguments: "{}",
	})
	if err != nil {
		t.Fatalf("ExecuteTool should not return gRPC error, got %v", err)
	}
	if resp.Success {
		t.Fatal("expected failure for unknown tool")
	}
	if resp.ErrorCode != "TOOL_NOT_FOUND" {
		t.Errorf("expected error code TOOL_NOT_FOUND, got %s", resp.ErrorCode)
	}
	if !strings.Contains(resp.Result, "未找到工具") {
		t.Errorf("expected Chinese error, got %q", resp.Result)
	}
}

func TestExecuteTool_InvalidJSON(t *testing.T) {
	setupTestStorage(t)

	h := newTestAgentHandler(bridge.NewManager())
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "s1",
		ToolName:  "read",
		Arguments: `{not json`,
	})
	if err != nil {
		t.Fatalf("ExecuteTool should not return gRPC error, got %v", err)
	}
	if resp.Success {
		t.Fatal("expected failure for invalid JSON")
	}
	if resp.ErrorCode != "TOOL_ERROR" {
		t.Errorf("expected error code TOOL_ERROR, got %s", resp.ErrorCode)
	}
}

func TestExecuteTool_ReadFile(t *testing.T) {
	setupTestStorage(t)
	// 设置工作目录为 temp dir
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello world"), 0644)

	h := newTestAgentHandler(bridge.NewManager())
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "s1",
		ToolName:  "read",
		Arguments: fmt.Sprintf(`{"path":%q}`, testFile),
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got: %s", resp.Result)
	}
	if resp.Result != "hello world" {
		t.Errorf("got %q, want %q", resp.Result, "hello world")
	}
}

func TestExecuteTool_PathOutsideWorkspace(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	// 尝试读取工作目录外的文件——应返回 needs_permission=true
	outsideFile := filepath.Join(filepath.Dir(tmpDir), "outside.txt")
	h := newTestAgentHandler(bridge.NewManager())
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "s1",
		ToolName:  "read",
		Arguments: fmt.Sprintf(`{"path":%q}`, outsideFile),
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if !resp.NeedsPermission {
		t.Fatal("expected NeedsPermission=true for outside workspace path")
	}
	if resp.RequestId == "" {
		t.Fatal("expected non-empty RequestId")
	}
}

func TestExecuteTool_WorkingDirFallback(t *testing.T) {
	setupTestStorage(t)
	// 不设置 working_directory（空字符串）
	writeSettingsJSON(t, `{}`)

	h := newTestAgentHandler(bridge.NewManager())
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "s1",
		ToolName:  "bash",
		Arguments: `{"command":"echo fallback_test"}`,
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success with fallback, got: %s", resp.Result)
	}
	if !strings.Contains(resp.Result, "fallback_test") {
		t.Errorf("expected output to contain 'fallback_test', got %q", resp.Result)
	}
}

func TestExecuteTool_EditMatchCountError(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	testFile := filepath.Join(tmpDir, "edit_test.txt")
	os.WriteFile(testFile, []byte("abc abc abc"), 0644)

	h := newTestAgentHandler(bridge.NewManager())
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "s1",
		ToolName:  "edit",
		Arguments: fmt.Sprintf(`{"path":%q,"old_string":"abc","new_string":"xyz"}`, testFile),
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if resp.Success {
		t.Fatal("expected failure for multiple matches")
	}
	if resp.ErrorCode != "EDIT_MATCH_COUNT_ERROR" {
		t.Errorf("expected error code EDIT_MATCH_COUNT_ERROR, got %s", resp.ErrorCode)
	}
	if !strings.Contains(resp.Result, "匹配数 3 大于 1") {
		t.Errorf("expected match count error, got %q", resp.Result)
	}
}

func TestAgentHandler_SyncChannel_ConcurrentCommandSend(t *testing.T) {
	setupTestStorage(t)

	mgr := bridge.NewManager()
	h := newTestAgentHandler(mgr)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &fakeSyncServer{ctx: ctx, stop: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- h.SyncChannel(stream)
	}()

	var wg sync.WaitGroup
	const senders = 4
	for i := 0; i < senders; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			mgr.CmdChan <- &proto.ServerCommand{
				SessionId:   fmt.Sprintf("sess_%d", n),
				CommandType: "start_chat",
				Payload:     `{}`,
			}
		}(i)
	}

	deadline := time.After(2 * time.Second)
	for stream.commandCount() < senders {
		select {
		case <-deadline:
			t.Fatalf("timed out, got %d commands, want %d", stream.commandCount(), senders)
		case <-time.After(5 * time.Millisecond):
		}
	}

	wg.Wait()

	select {
	case err := <-done:
		t.Fatalf("SyncChannel should still be running, got %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(stream.stop)
	if err := <-done; err != nil {
		t.Fatalf("SyncChannel should return nil after stream stop, got %v", err)
	}
}

// --- ExecuteTool 审批流程测试 ---

func TestExecuteTool_ApprovalFlow_FullCycle(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	// 创建测试文件
	outsideFile := filepath.Join(filepath.Dir(tmpDir), "approval_test.txt")
	os.WriteFile(outsideFile, []byte("approval content"), 0644)

	h := newTestAgentHandler(bridge.NewManager())

	// 第一步：路径越权，应返回 needs_permission=true
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "sess-1",
		ToolName:  "read",
		Arguments: fmt.Sprintf(`{"path":%q}`, outsideFile),
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if !resp.NeedsPermission {
		t.Fatal("expected NeedsPermission=true")
	}
	if resp.RequestId == "" {
		t.Fatal("expected non-empty RequestId")
	}

	// 第二步：带 approval_id 重试——审批已自动创建但状态为 Pending，应拒绝
	resp2, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId:  "sess-1",
		ToolName:   "read",
		Arguments:  fmt.Sprintf(`{"path":%q}`, outsideFile),
		ApprovalId: resp.RequestId,
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if resp2.Success {
		t.Fatal("expected failure for pending (not yet approved) approval")
	}
	if !strings.Contains(resp2.Result, "审批无效") {
		t.Errorf("expected 审批无效 error, got %q", resp2.Result)
	}
}

func TestExecuteTool_ApprovalFlow_WithResolve(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	outsideFile := filepath.Join(filepath.Dir(tmpDir), "approval_resolve.txt")
	os.WriteFile(outsideFile, []byte("resolve content"), 0644)

	h := newTestAgentHandler(bridge.NewManager())

	// 第一步：路径越权
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "sess-1",
		ToolName:  "read",
		Arguments: fmt.Sprintf(`{"path":%q}`, outsideFile),
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if !resp.NeedsPermission {
		t.Fatal("expected NeedsPermission=true")
	}

	// 手动批准
	h.approvalManager.Resolve("sess-1", resp.RequestId, "allow")

	// 第二步：带 approval_id 重试——已批准，应执行成功
	resp2, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId:  "sess-1",
		ToolName:   "read",
		Arguments:  fmt.Sprintf(`{"path":%q}`, outsideFile),
		ApprovalId: resp.RequestId,
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if !resp2.Success {
		t.Fatalf("expected success after approval, got: %s", resp2.Result)
	}
	if resp2.Result != "resolve content" {
		t.Errorf("got %q, want %q", resp2.Result, "resolve content")
	}
}

func TestExecuteTool_ApprovalFlow_OneTimeUse(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	outsideFile := filepath.Join(filepath.Dir(tmpDir), "approval_onetime.txt")
	os.WriteFile(outsideFile, []byte("onetime content"), 0644)

	h := newTestAgentHandler(bridge.NewManager())

	// 第一次：越权 → 批准 → 执行成功
	resp, _ := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "sess-1",
		ToolName:  "read",
		Arguments: fmt.Sprintf(`{"path":%q}`, outsideFile),
	})
	h.approvalManager.Resolve("sess-1", resp.RequestId, "allow")

	resp2, _ := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId:  "sess-1",
		ToolName:   "read",
		Arguments:  fmt.Sprintf(`{"path":%q}`, outsideFile),
		ApprovalId: resp.RequestId,
	})
	if !resp2.Success {
		t.Fatalf("expected first approval to succeed, got: %s", resp2.Result)
	}

	// 第二次：同一 approval_id 再用——已被消费，应失败（一次性）
	resp3, _ := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId:  "sess-1",
		ToolName:   "read",
		Arguments:  fmt.Sprintf(`{"path":%q}`, outsideFile),
		ApprovalId: resp.RequestId,
	})
	if resp3.Success {
		t.Fatal("expected second use of same approval to fail (one-time)")
	}
	if !strings.Contains(resp3.Result, "审批无效") {
		t.Errorf("expected 审批无效, got %q", resp3.Result)
	}
}

func TestExecuteTool_ApprovalFlow_ToolMismatch(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	outsideFile := filepath.Join(filepath.Dir(tmpDir), "approval_mismatch.txt")
	os.WriteFile(outsideFile, []byte("mismatch"), 0644)

	h := newTestAgentHandler(bridge.NewManager())

	// 用 read 申请
	resp, _ := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "sess-1",
		ToolName:  "read",
		Arguments: fmt.Sprintf(`{"path":%q}`, outsideFile),
	})
	h.approvalManager.Resolve("sess-1", resp.RequestId, "allow")

	// 用 write 重试——工具不匹配
	resp2, _ := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId:  "sess-1",
		ToolName:   "write",
		Arguments:  fmt.Sprintf(`{"path":%q,"content":"overwritten"}`, outsideFile),
		ApprovalId: resp.RequestId,
	})
	if resp2.Success {
		t.Fatal("expected failure for tool mismatch")
	}
	if !strings.Contains(resp2.Result, "审批无效") {
		t.Errorf("expected 审批无效, got %q", resp2.Result)
	}
}

func TestExecuteTool_ApprovalFlow_SessionMismatch(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	outsideFile := filepath.Join(filepath.Dir(tmpDir), "approval_session.txt")
	os.WriteFile(outsideFile, []byte("session"), 0644)

	h := newTestAgentHandler(bridge.NewManager())

	// sess-1 申请
	resp, _ := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "sess-1",
		ToolName:  "read",
		Arguments: fmt.Sprintf(`{"path":%q}`, outsideFile),
	})
	h.approvalManager.Resolve("sess-1", resp.RequestId, "allow")

	// sess-2 用同一 approval_id 重试——session 不匹配
	resp2, _ := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId:  "sess-2",
		ToolName:   "read",
		Arguments:  fmt.Sprintf(`{"path":%q}`, outsideFile),
		ApprovalId: resp.RequestId,
	})
	if resp2.Success {
		t.Fatal("expected failure for session mismatch")
	}
	if !strings.Contains(resp2.Result, "审批无效") {
		t.Errorf("expected 审批无效, got %q", resp2.Result)
	}
}

func TestExecuteTool_ApprovalFlow_BashSkipsApproval(t *testing.T) {
	setupTestStorage(t)
	tmpDir := newPersistentTestDir(t)
	writeSettingsJSON(t, fmt.Sprintf(`{"working_directory":%q}`, tmpDir))

	h := newTestAgentHandler(bridge.NewManager())

	// bash 工具应直接执行，不走审批
	resp, err := h.ExecuteTool(context.Background(), &proto.ToolRequest{
		SessionId: "sess-1",
		ToolName:  "bash",
		Arguments: `{"command":"echo hello"}`,
	})
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	if resp.NeedsPermission {
		t.Fatal("bash should not trigger approval flow")
	}
	if !resp.Success {
		t.Fatalf("expected bash success, got: %s", resp.Result)
	}
}
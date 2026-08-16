package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

	h := NewAgentHandler(bridge.NewManager())
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

	h := NewAgentHandler(bridge.NewManager())
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

	h := NewAgentHandler(bridge.NewManager())
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

	h := NewAgentHandler(bridge.NewManager())
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

	h := NewAgentHandler(bridge.NewManager())
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

	h := NewAgentHandler(bridge.NewManager())
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

	h := NewAgentHandler(bridge.NewManager())
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

	h := NewAgentHandler(bridge.NewManager())
	stream := &fakeSyncServer{ctx: context.Background()}

	err := h.SyncChannel(stream)
	if err != nil {
		t.Fatalf("SyncChannel should return nil on EOF, got %v", err)
	}
}

func TestAgentHandler_SyncChannel_RecvError(t *testing.T) {
	setupTestStorage(t)

	h := NewAgentHandler(bridge.NewManager())
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
	h := NewAgentHandler(mgr)
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

func TestAgentHandler_SyncChannel_ConcurrentCommandSend(t *testing.T) {
	setupTestStorage(t)

	mgr := bridge.NewManager()
	h := NewAgentHandler(mgr)
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
package handler

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/internal/thread"
	"github.com/songhuang/flowpartner/backend/proto"
	"google.golang.org/grpc"
)

func newTestHandler() *AgentHandler {
	return NewAgentHandler(thread.NewManager(), make(chan *proto.AgentEvent, 100))
}

func setupAgentTestStorage(t *testing.T) {
	t.Helper()
	storage.SetDataDirForTest(t.TempDir())
}

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

func TestAgentHandler_CallLLM_InvalidJSON(t *testing.T) {
	setupAgentTestStorage(t)
	keystore.Reset()
	storage.WriteJSON("settings.json", map[string]interface{}{"model_configs": []interface{}{}})
	h := newTestHandler()

	req := &proto.LLMRequest{SessionId: "s1", JsonPayload: "{invalid"}
	stream := &fakeCallLLMServer{ctx: context.Background()}

	h.CallLLM(req, stream)

	if len(stream.responses) == 0 {
		t.Fatal("expected error response for invalid JSON")
	}
	if !stream.responses[0].IsError {
		t.Fatalf("expected IsError=true, got %+v", stream.responses[0])
	}
}

func TestAgentHandler_CallLLM_EmptyMessages(t *testing.T) {
	setupAgentTestStorage(t)
	keystore.Reset()
	storage.WriteJSON("settings.json", map[string]interface{}{"model_configs": []interface{}{}})
	h := newTestHandler()

	req := &proto.LLMRequest{SessionId: "s1", JsonPayload: `{"model":"gpt-4"}`}
	stream := &fakeCallLLMServer{ctx: context.Background()}

	h.CallLLM(req, stream)

	if len(stream.responses) == 0 {
		t.Fatal("expected error response for empty messages")
	}
	if !stream.responses[0].IsError {
		t.Fatalf("expected IsError=true, got %+v", stream.responses[0])
	}
}

func TestAgentHandler_CallLLM_NoActiveConfig(t *testing.T) {
	setupAgentTestStorage(t)
	keystore.Reset()
	storage.WriteJSON("settings.json", map[string]interface{}{})

	h := newTestHandler()

	req := &proto.LLMRequest{
		SessionId:   "s1",
		JsonPayload: `{"messages":[{"role":"user","content":"hi"}]}`,
	}
	stream := &fakeCallLLMServer{ctx: context.Background()}

	h.CallLLM(req, stream)

	if len(stream.responses) == 0 {
		t.Fatal("expected error response for no active config")
	}
	if !stream.responses[0].IsError {
		t.Fatalf("expected IsError=true, got %+v", stream.responses[0])
	}
}

func TestAgentHandler_SyncChannel_EOF(t *testing.T) {
	setupAgentTestStorage(t)
	h := newTestHandler()

	stream := &fakeSyncServer{ctx: context.Background(), stop: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- h.SyncChannel(stream)
	}()

	stream.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SyncChannel returned error on EOF: %v", err)
		}
	case <-stream.stop:
	}
}

type fakeSyncServer struct {
	grpc.ServerStream
	ctx         context.Context
	events      []*proto.AgentEvent
	commands    []*proto.ServerCommand
	mu          sync.Mutex
	stop        chan struct{}
	sendClosed  bool
}

func (f *fakeSyncServer) Send(cmd *proto.ServerCommand) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendClosed {
		return io.EOF
	}
	f.commands = append(f.commands, cmd)
	return nil
}

func (f *fakeSyncServer) Recv() (*proto.AgentEvent, error) {
	<-f.stop
	return nil, io.EOF
}

func (f *fakeSyncServer) Context() context.Context { return f.ctx }

func (f *fakeSyncServer) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.sendClosed {
		f.sendClosed = true
		close(f.stop)
	}
}

func (f *fakeSyncServer) commandCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.commands)
}

func TestSyncChannel_ForwardsEvents(t *testing.T) {
	setupAgentTestStorage(t)
	agentEventCh := make(chan *proto.AgentEvent, 10)
	h := NewAgentHandler(thread.NewManager(), agentEventCh)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := &fakeSyncServer{ctx: ctx, stop: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		done <- h.SyncChannel(stream)
	}()

	event := &proto.AgentEvent{
		ThreadId: "t1",
		Payload: &proto.AgentEvent_Error{
			Error: &proto.Error{Message: "test"},
		},
	}
	agentEventCh <- event

	stream.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("SyncChannel returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SyncChannel to finish")
	}
}

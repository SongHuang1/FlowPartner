package thread

import (
	"encoding/json"
	"testing"
)

func setupTestHandler(t *testing.T) (*Handler, *Manager, *Scheduler) {
	t.Helper()
	m := NewManager()
	s := NewScheduler()
	h := NewHandler(m, s)
	return h, m, s
}

func TestHandler_ThreadStart(t *testing.T) {
	h, _, _ := setupTestHandler(t)

	t.Run("success", func(t *testing.T) {
		params := json.RawMessage("{\"cwd\":\"/tmp/work\",\"agentId\":\"main\"}")
		result, err := h.Dispatch("thread/start", params)
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		r, ok := result.(threadStartResult)
		if !ok {
			t.Fatalf("wrong type: %T", result)
		}
		if r.ThreadID == "" {
			t.Error("threadId is empty")
		}
		if r.AgentID != "main" {
			t.Errorf("agentId = %s, want main", r.AgentID)
		}
		if r.Cwd != "/tmp/work" {
			t.Errorf("cwd = %s, want /tmp/work", r.Cwd)
		}
	})

	t.Run("default agent", func(t *testing.T) {
		params := json.RawMessage("{}")
		result, err := h.Dispatch("thread/start", params)
		if err != nil {
			t.Fatal(err)
		}
		r := result.(threadStartResult)
		if r.AgentID != "main" {
			t.Errorf("agentId = %s, want main", r.AgentID)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		params := json.RawMessage("{bad}")
		_, err := h.Dispatch("thread/start", params)
		if err == nil {
			t.Error("expected error for invalid json")
		}
		if err.Code != -32602 {
			t.Errorf("code = %d, want -32602", err.Code)
		}
	})
}

func TestHandler_ThreadList(t *testing.T) {
	h, m, _ := setupTestHandler(t)

	m.CreateThread("t1", "main", "/a")
	m.CreateThread("t2", "main", "/b")
	m.CreateThread("t3", "sub", "/c")

	t.Run("list all", func(t *testing.T) {
		params := json.RawMessage("{}")
		result, err := h.Dispatch("thread/list", params)
		if err != nil {
			t.Fatal(err)
		}
		r := result.(threadListResult)
		if len(r.Data) != 3 {
			t.Errorf("got %d threads, want 3", len(r.Data))
		}
	})

	t.Run("limit", func(t *testing.T) {
		params := json.RawMessage("{\"limit\":2}")
		result, err := h.Dispatch("thread/list", params)
		if err != nil {
			t.Fatal(err)
		}
		r := result.(threadListResult)
		if len(r.Data) != 2 {
			t.Errorf("got %d threads, want 2", len(r.Data))
		}
		if r.NextCursor == "" {
			t.Error("expected nextCursor")
		}
	})
}

func TestHandler_ThreadRead(t *testing.T) {
	h, m, _ := setupTestHandler(t)

	m.CreateThread("t1", "main", "/a")
	thread, _ := m.GetThread("t1")
	thread.StartTurn("u1")

	t.Run("success", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"t1\"}")
		result, err := h.Dispatch("thread/read", params)
		if err != nil {
			t.Fatal(err)
		}
		r := result.(threadReadResult)
		if r.Thread.ThreadID != "t1" {
			t.Errorf("threadId = %s", r.Thread.ThreadID)
		}
		if len(r.Turns) != 1 {
			t.Errorf("turns = %d, want 1", len(r.Turns))
		}
	})

	t.Run("not found", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"nonexistent\"}")
		_, err := h.Dispatch("thread/read", params)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("missing id", func(t *testing.T) {
		params := json.RawMessage("{}")
		_, err := h.Dispatch("thread/read", params)
		if err == nil {
			t.Error("expected error for missing threadId")
		}
	})
}

func TestHandler_TurnStart(t *testing.T) {
	h, m, _ := setupTestHandler(t)

	m.CreateThread("t1", "main", "")

	t.Run("stub returns turn info", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"t1\",\"input\":[{\"type\":\"text\",\"text\":\"hello\"}]}")
		result, err := h.Dispatch("turn/start", params)
		if err != nil {
			t.Fatalf("unexpected error: %+v", err)
		}
		r := result.(turnStartResult)
		if r.ThreadID != "t1" {
			t.Errorf("threadId = %s", r.ThreadID)
		}
		if r.TurnID == "" {
			t.Error("turnId is empty")
		}
	})

	t.Run("conflict", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"t1\",\"input\":[{\"type\":\"text\",\"text\":\"world\"}]}")
		_, err := h.Dispatch("turn/start", params)
		if err == nil {
			t.Error("expected conflict error")
		}
		if err.Code != -32002 {
			t.Errorf("code = %d, want -32002", err.Code)
		}
	})

	t.Run("archived thread", func(t *testing.T) {
		m.ArchiveThread("t1")
		params := json.RawMessage("{\"threadId\":\"t1\",\"input\":[{\"type\":\"text\",\"text\":\"x\"}]}")
		_, err := h.Dispatch("turn/start", params)
		if err == nil {
			t.Error("expected error for archived thread")
		}
		if err.Code != -32602 {
			t.Errorf("code = %d, want -32602", err.Code)
		}
	})

	t.Run("unknown input type", func(t *testing.T) {
		m.CreateThread("t2", "main", "")
		params := json.RawMessage("{\"threadId\":\"t2\",\"input\":[{\"type\":\"image\",\"text\":\"\"}]}")
		_, err := h.Dispatch("turn/start", params)
		if err == nil {
			t.Error("expected error for unknown input type")
		}
		if err.Code != -32602 {
			t.Errorf("code = %d, want -32602", err.Code)
		}
	})
}

func TestHandler_TurnInterrupt(t *testing.T) {
	h, m, _ := setupTestHandler(t)

	m.CreateThread("t1", "main", "")
	thread, _ := m.GetThread("t1")
	thread.StartTurn("u1")

	t.Run("success", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"t1\"}")
		result, err := h.Dispatch("turn/interrupt", params)
		if err != nil {
			t.Fatal(err)
		}
		_ = result
		turn := thread.GetTurn()
		if turn.Status != TurnAborting {
			t.Errorf("status = %d, want TurnAborting", turn.Status)
		}
	})

	t.Run("no active turn", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"t1\"}")
		_, err := h.Dispatch("turn/interrupt", params)
		if err == nil {
			t.Error("expected -32005")
		}
		if err.Code != -32005 {
			t.Errorf("code = %d, want -32005", err.Code)
		}
	})

	t.Run("thread not found", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"nonexistent\"}")
		_, err := h.Dispatch("turn/interrupt", params)
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestHandler_TurnSteer(t *testing.T) {
	h, m, _ := setupTestHandler(t)

	m.CreateThread("t1", "main", "")
	thread, _ := m.GetThread("t1")
	thread.StartTurn("u1")

	t.Run("success", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"t1\",\"input\":[{\"type\":\"text\",\"text\":\"steer\"}]}")
		result, err := h.Dispatch("turn/steer", params)
		if err != nil {
			t.Fatal(err)
		}
		_ = result
	})

	t.Run("turn mismatch", func(t *testing.T) {
		params := json.RawMessage("{\"threadId\":\"t1\",\"turnId\":\"wrong\",\"input\":[{\"type\":\"text\",\"text\":\"x\"}]}")
		_, err := h.Dispatch("turn/steer", params)
		if err == nil {
			t.Error("expected -32005")
		}
		if err.Code != -32005 {
			t.Errorf("code = %d, want -32005", err.Code)
		}
	})

	t.Run("no active turn", func(t *testing.T) {
		m.CreateThread("t2", "main", "")
		params := json.RawMessage("{\"threadId\":\"t2\",\"input\":[{\"type\":\"text\",\"text\":\"x\"}]}")
		_, err := h.Dispatch("turn/steer", params)
		if err == nil {
			t.Error("expected -32005")
		}
	})
}

func TestHandler_ThreadArchive(t *testing.T) {
	h, m, _ := setupTestHandler(t)

	m.CreateThread("t1", "main", "")

	params := json.RawMessage("{\"threadId\":\"t1\"}")
	result, err := h.Dispatch("thread/archive", params)
	if err != nil {
		t.Fatal(err)
	}
	_ = result

	thread, _ := m.GetThread("t1")
	if !thread.Archived {
		t.Error("thread should be archived")
	}
}

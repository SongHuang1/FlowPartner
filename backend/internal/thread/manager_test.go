package thread

import (
	"sync"
	"testing"
	"time"
)

// mockConn is a test double for Connection.
type mockConn struct {
	mu            sync.Mutex
	notifications []notification
}

type notification struct {
	Method string
	Params interface{}
}

func (c *mockConn) SendNotification(method string, params interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.notifications = append(c.notifications, notification{method, params})
	return nil
}

func (c *mockConn) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.notifications)
}

func TestManager_CreateThread(t *testing.T) {
	m := NewManager()
	defer m.Close()

	t.Run("success", func(t *testing.T) {
		th, err := m.CreateThread("t1", "main", "/tmp/work")
		if err != nil {
			t.Fatal(err)
		}
		if th.ID != "t1" {
			t.Errorf("id = %s", th.ID)
		}
		if th.AgentID != "main" {
			t.Errorf("agentID = %s", th.AgentID)
		}
		if th.Cwd != "/tmp/work" {
			t.Errorf("cwd = %s", th.Cwd)
		}
	})

	t.Run("duplicate", func(t *testing.T) {
		_, err := m.CreateThread("t1", "main", "")
		if err == nil {
			t.Error("expected error for duplicate")
		}
	})

	t.Run("invalid id", func(t *testing.T) {
		_, err := m.CreateThread("invalid id!", "main", "")
		if err == nil {
			t.Error("expected error for invalid id")
		}
	})

	t.Run("empty id", func(t *testing.T) {
		_, err := m.CreateThread("", "main", "")
		if err == nil {
			t.Error("expected error for empty id")
		}
	})
}

func TestManager_GetThread(t *testing.T) {
	m := NewManager()
	defer m.Close()
	m.CreateThread("t1", "main", "")

	t.Run("exists", func(t *testing.T) {
		th, ok := m.GetThread("t1")
		if !ok || th == nil {
			t.Fatal("expected to find t1")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, ok := m.GetThread("nonexistent")
		if ok {
			t.Error("expected not found")
		}
	})
}

func TestManager_ListThreads(t *testing.T) {
	m := NewManager()
	defer m.Close()
	m.CreateThread("t1", "main", "")
	m.CreateThread("t2", "main", "")
	m.CreateThread("t3", "main", "")

	t.Run("all", func(t *testing.T) {
		threads, _ := m.ListThreads("", 100, nil)
		if len(threads) != 3 {
			t.Errorf("got %d threads, want 3", len(threads))
		}
	})

	t.Run("limit", func(t *testing.T) {
		threads, cursor := m.ListThreads("", 2, nil)
		if len(threads) != 2 {
			t.Errorf("got %d threads, want 2", len(threads))
		}
		if cursor == "" {
			t.Error("expected cursor")
		}
	})

	t.Run("cursor", func(t *testing.T) {
		threads, _ := m.ListThreads("t1", 2, nil)
		if len(threads) != 2 {
			t.Errorf("got %d threads, want 2", len(threads))
		}
		if threads[0].ID != "t2" {
			t.Errorf("first = %s, want t2", threads[0].ID)
		}
	})
}

func TestManager_ArchiveThread(t *testing.T) {
	m := NewManager()
	defer m.Close()
	m.CreateThread("t1", "main", "")

	err := m.ArchiveThread("t1")
	if err != nil {
		t.Fatal(err)
	}

	th, _ := m.GetThread("t1")
	if !th.Archived {
		t.Error("thread should be archived")
	}
}

func TestThread_AttachDetach(t *testing.T) {
	m := NewManager()
	defer m.Close()
	th, _ := m.CreateThread("t1", "main", "")

	conn1 := &mockConn{}
	conn2 := &mockConn{}

	th.AttachConn("c1", conn1)
	th.AttachConn("c2", conn2)

	if th.ConnectionCount() != 2 {
		t.Errorf("count = %d, want 2", th.ConnectionCount())
	}

	th.DetachConn("c1")
	if th.ConnectionCount() != 1 {
		t.Errorf("count = %d, want 1", th.ConnectionCount())
	}
}

func TestThread_Fanout(t *testing.T) {
	m := NewManager()
	defer m.Close()
	th, _ := m.CreateThread("t1", "main", "")

	conn1 := &mockConn{}
	conn2 := &mockConn{}

	th.AttachConn("c1", conn1)
	th.AttachConn("c2", conn2)

	th.Fanout("item/started", map[string]string{"threadId": "t1"})

	if conn1.count() != 1 {
		t.Errorf("conn1 got %d notifications, want 1", conn1.count())
	}
	if conn2.count() != 1 {
		t.Errorf("conn2 got %d notifications, want 1", conn2.count())
	}
}

func TestThread_StartTurn(t *testing.T) {
	m := NewManager()
	defer m.Close()
	th, _ := m.CreateThread("t1", "main", "")

	t.Run("success", func(t *testing.T) {
		err := th.StartTurn("u1")
		if err != nil {
			t.Fatal(err)
		}
		turn := th.GetTurn()
		if turn == nil || turn.ID != "u1" {
			t.Errorf("turn = %+v", turn)
		}
	})

	t.Run("conflict", func(t *testing.T) {
		err := th.StartTurn("u2")
		if err == nil {
			t.Error("expected conflict error")
		}
	})

	t.Run("end and restart", func(t *testing.T) {
		th.EndTurn()
		err := th.StartTurn("u3")
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestManager_ServerRequest(t *testing.T) {
	m := NewManager()
	defer m.Close()
	th, _ := m.CreateThread("t1", "main", "")

	conn := &mockConn{}
	th.AttachConn("c1", conn)

	t.Run("create and resolve", func(t *testing.T) {
		sr := m.CreateServerRequest("t1", "item/commandExecution/requestApproval",
			map[string]string{"command": "rm -rf"},
			[]string{"approved", "denied"}, KindPermission)

		if sr.ID != 1 {
			t.Errorf("id = %d, want 1", sr.ID)
		}

		go func() {
			time.Sleep(50 * time.Millisecond)
			m.ResolveServerRequest(sr.ID, "approved")
		}()

		result := sr.WaitForResponse()
		if result.Err != nil {
			t.Errorf("unexpected error: %v", result.Err)
		}
		if result.Decision != "approved" {
			t.Errorf("decision = %s, want approved", result.Decision)
		}

		// resolved notification should be sent to attached conns
		time.Sleep(50 * time.Millisecond)
		if conn.count() < 1 {
			t.Errorf("expected at least 1 notification (resolved), got %d", conn.count())
		}
	})

	t.Run("invalid decision", func(t *testing.T) {
		sr := m.CreateServerRequest("t1", "item/commandExecution/requestApproval",
			map[string]string{"command": "rm"},
			[]string{"approved", "denied"}, KindPermission)

		m.ResolveServerRequest(sr.ID, "invalid_choice")

		result := sr.WaitForResponse()
		if result.Err == nil {
			t.Error("expected error for invalid decision")
		}
	})

	t.Run("timeout", func(t *testing.T) {
		// Override timeout for testing
		origTimeout := kindTimeouts[KindPermission]
		kindTimeouts[KindPermission] = 50 * time.Millisecond
		defer func() { kindTimeouts[KindPermission] = origTimeout }()

		sr := m.CreateServerRequest("t1", "test/timeout",
			map[string]string{}, []string{"ok"}, KindPermission)

		result := sr.WaitForResponse()
		if result.Err == nil {
			t.Error("expected timeout error")
		}
	})
}

func TestManager_AbortPending(t *testing.T) {
	m := NewManager()
	defer m.Close()
	th, _ := m.CreateThread("t1", "main", "")

	conn := &mockConn{}
	th.AttachConn("c1", conn)

	sr := m.CreateServerRequest("t1", "test/abort",
		map[string]string{}, []string{"ok"}, KindPermission)

	th.AbortPendingRequests("test_abort")

	result := sr.WaitForResponse()
	if result.Err == nil {
		t.Error("expected abort error")
	}
}

func TestManager_ReplayPending(t *testing.T) {
	m := NewManager()
	defer m.Close()
	th, _ := m.CreateThread("t1", "main", "")

	conn := &mockConn{}
	th.AttachConn("c1", conn)

	m.CreateServerRequest("t1", "test/replay",
		map[string]string{"key": "val"}, []string{"ok"}, KindPermission)

	// Replay should resend pending requests to the connection
	m.ReplayPendingForConnection("t1", conn)

	time.Sleep(50 * time.Millisecond)
	if conn.count() != 1 {
		t.Errorf("expected 1 notification from replay, got %d", conn.count())
	}
}

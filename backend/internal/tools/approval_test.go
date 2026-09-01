package tools

import (
	"sync"
	"testing"
)

func TestApprovalManager_Create(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/outside.txt", "/tmp/outside.txt")

	if requestID == "" {
		t.Fatal("expected non-empty request_id")
	}

	// 验证记录已创建
	pending := m.GetPendingRequestID("sess-1")
	if pending != requestID {
		t.Errorf("expected pending request %q, got %q", requestID, pending)
	}
}

func TestApprovalManager_Create_UniqueIDs(t *testing.T) {
	m := NewApprovalManager()
	id1 := m.Create("sess-1", "read", "/a", "/a")
	id2 := m.Create("sess-1", "read", "/a", "/a")

	if id1 == id2 {
		t.Error("two Create calls should return different request IDs")
	}
}

func TestApprovalManager_Resolve_Allow(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")

	ok := m.Resolve("sess-1", requestID, "allow")
	if !ok {
		t.Fatal("expected Resolve to succeed")
	}

	// 已批准后不再出现在 pending 中
	pending := m.GetPendingRequestID("sess-1")
	if pending != "" {
		t.Errorf("expected no pending request after resolve, got %q", pending)
	}
}

func TestApprovalManager_Resolve_Deny(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")

	ok := m.Resolve("sess-1", requestID, "deny")
	if !ok {
		t.Fatal("expected Resolve to succeed")
	}

	pending := m.GetPendingRequestID("sess-1")
	if pending != "" {
		t.Errorf("expected no pending request after deny, got %q", pending)
	}
}

func TestApprovalManager_Resolve_NotFound(t *testing.T) {
	m := NewApprovalManager()
	ok := m.Resolve("sess-1", "nonexistent-id", "allow")
	if ok {
		t.Error("expected Resolve to fail for nonexistent request_id")
	}
}

func TestApprovalManager_Resolve_SessionMismatch(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")

	ok := m.Resolve("sess-2", requestID, "allow")
	if ok {
		t.Error("expected Resolve to fail for mismatched session")
	}
}

func TestApprovalManager_Consume_Success(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
	m.Resolve("sess-1", requestID, "allow")

	granted := m.Consume("sess-1", requestID, "read", "/tmp/out.txt")
	if !granted {
		t.Fatal("expected Consume to succeed")
	}
}

func TestApprovalManager_Consume_OneTimeUse(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
	m.Resolve("sess-1", requestID, "allow")

	// 第一次消费成功
	granted := m.Consume("sess-1", requestID, "read", "/tmp/out.txt")
	if !granted {
		t.Fatal("expected first Consume to succeed")
	}

	// 第二次消费失败（一次性）
	granted = m.Consume("sess-1", requestID, "read", "/tmp/out.txt")
	if granted {
		t.Error("expected second Consume to fail (one-time use)")
	}
}

func TestApprovalManager_Consume_ToolMismatch(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
	m.Resolve("sess-1", requestID, "allow")

	granted := m.Consume("sess-1", requestID, "write", "/tmp/out.txt")
	if granted {
		t.Error("expected Consume to fail for tool mismatch")
	}
}

func TestApprovalManager_Consume_PathMismatch(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
	m.Resolve("sess-1", requestID, "allow")

	granted := m.Consume("sess-1", requestID, "read", "/tmp/other.txt")
	if granted {
		t.Error("expected Consume to fail for path mismatch")
	}
}

func TestApprovalManager_Consume_SessionMismatch(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
	m.Resolve("sess-1", requestID, "allow")

	granted := m.Consume("sess-2", requestID, "read", "/tmp/out.txt")
	if granted {
		t.Error("expected Consume to fail for session mismatch")
	}
}

func TestApprovalManager_Consume_NotFound(t *testing.T) {
	m := NewApprovalManager()
	granted := m.Consume("sess-1", "nonexistent", "read", "/tmp/out.txt")
	if granted {
		t.Error("expected Consume to fail for nonexistent approval")
	}
}

func TestApprovalManager_Consume_NotGranted(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
	// 未 Resolve（仍为 Pending 状态）

	granted := m.Consume("sess-1", requestID, "read", "/tmp/out.txt")
	if granted {
		t.Error("expected Consume to fail when status is Pending (not Granted)")
	}
}

func TestApprovalManager_Consume_Denied(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
	m.Resolve("sess-1", requestID, "deny")

	granted := m.Consume("sess-1", requestID, "read", "/tmp/out.txt")
	if granted {
		t.Error("expected Consume to fail when status is Denied")
	}
}

func TestApprovalManager_CancelThread(t *testing.T) {
	m := NewApprovalManager()
	id1 := m.Create("sess-1", "read", "/tmp/a.txt", "/tmp/a.txt")
	id2 := m.Create("sess-1", "read", "/tmp/b.txt", "/tmp/b.txt")
	m.Create("sess-2", "read", "/tmp/c.txt", "/tmp/c.txt")

	m.CancelThread("sess-1")

	// sess-1 的审批已取消，不能再消费
	if m.Consume("sess-1", id1, "read", "/tmp/a.txt") {
		t.Error("expected Consume to fail after CancelThread")
	}
	if m.Consume("sess-1", id2, "read", "/tmp/b.txt") {
		t.Error("expected Consume to fail after CancelThread")
	}

	// sess-2 不受影响
	pending := m.GetPendingRequestID("sess-2")
	if pending == "" {
		t.Error("expected sess-2 to still have pending request")
	}
}

func TestApprovalManager_CancelThread_OnlyPending(t *testing.T) {
	m := NewApprovalManager()
	requestID := m.Create("sess-1", "read", "/tmp/a.txt", "/tmp/a.txt")
	m.Resolve("sess-1", requestID, "allow")

	m.CancelThread("sess-1")

	// 已批准的记录不受 CancelThread 影响（只有 Pending 状态才被取消）
	granted := m.Consume("sess-1", requestID, "read", "/tmp/a.txt")
	if !granted {
		t.Error("expected Consume to succeed for already-granted approval")
	}
}

func TestApprovalManager_GetPendingRequestID_None(t *testing.T) {
	m := NewApprovalManager()
	pending := m.GetPendingRequestID("nonexistent-session")
	if pending != "" {
		t.Errorf("expected empty, got %q", pending)
	}
}

func TestApprovalManager_ConcurrentCreateAndConsume(t *testing.T) {
	m := NewApprovalManager()
	const goroutines = 50

	var wg sync.WaitGroup
	ids := make(chan string, goroutines)

	// 并发创建
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := m.Create("sess-concurrent", "read", "/tmp/out.txt", "/tmp/out.txt")
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)

	created := make([]string, 0, goroutines)
	for id := range ids {
		created = append(created, id)
	}

	// 逐一批准
	for _, id := range created {
		if !m.Resolve("sess-concurrent", id, "allow") {
			t.Errorf("expected Resolve to succeed for %s", id)
		}
	}

	// 并发消费——每个记录是独立的，全部应成功
	var successes sync.WaitGroup
	successCount := make(chan int, goroutines)
	for _, id := range created {
		successes.Add(1)
		go func(approvalID string) {
			defer successes.Done()
			if m.Consume("sess-concurrent", approvalID, "read", "/tmp/out.txt") {
				successCount <- 1
			}
		}(id)
	}
	successes.Wait()
	close(successCount)

	count := 0
	for range successCount {
		count++
	}
	if count != goroutines {
		t.Errorf("expected %d successful Consume, got %d", goroutines, count)
	}
}

func TestApprovalManager_ConcurrentCreate(t *testing.T) {
	m := NewApprovalManager()
	const goroutines = 100

	var wg sync.WaitGroup
	ids := make(chan string, goroutines)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id := m.Create("sess-1", "read", "/tmp/out.txt", "/tmp/out.txt")
			ids <- id
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[string]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate request_id: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != goroutines {
		t.Errorf("expected %d unique IDs, got %d", goroutines, len(seen))
	}
}

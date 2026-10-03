package snapshot

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// waitFor 轮询等待条件成立。
func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("等待条件超时")
}

// TestManager_ConfigureAndManualSnapshot 启用后手动快照产出 manifest。
func TestManager_ConfigureAndManualSnapshot(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")
	mgr := NewManager(nil, nil)
	if err := mgr.Configure(workingDir, snapshotDir, true, false, 60, 15, 30, 5120); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	mgr.TriggerManual()
	waitFor(t, 5*time.Second, func() bool {
		list, _ := ListSnapshots(snapshotDir, mgr.ProjectID())
		return len(list) == 1 && list[0].Reason == ReasonManual
	})
}

func TestManager_ManualQueuedWhileSnapshotting(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")
	mgr := NewManager(nil, nil)
	if err := mgr.Configure(workingDir, snapshotDir, true, false, 60, 15, 30, 5120); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// 模拟快照进行中。
	mgr.mu.Lock()
	mgr.snapshotting = true
	mgr.mu.Unlock()

	mgr.TriggerManual()

	mgr.mu.Lock()
	queued := mgr.pendingManual
	statusQueued := mgr.status.Queued
	mgr.mu.Unlock()
	if !queued {
		t.Fatal("进行中触发手动快照应置 pendingManual")
	}
	if !statusQueued {
		t.Fatal("状态应显示手动快照已排队")
	}

	// 模拟当前快照结束 → 排队的手动快照应执行并清除标志。
	mgr.mu.Lock()
	mgr.snapshotting = false
	mgr.mu.Unlock()
	mgr.finishSnapshot(ReasonDebounce, snapshotDir, mgr.ProjectID())

	// 排队的手动快照应实际执行完毕（manifest 出现）。
	waitFor(t, 5*time.Second, func() bool {
		list, _ := ListSnapshots(snapshotDir, mgr.ProjectID())
		for _, m := range list {
			if m.Reason == ReasonManual {
				return true
			}
		}
		return false
	})
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.pendingManual || mgr.status.Queued {
		t.Error("手动快照执行后排队标志应清除")
	}
}

func TestManager_RearmOnce(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")
	mgr := NewManager(nil, nil)
	if err := mgr.Configure(workingDir, snapshotDir, true, false, 60, 15, 30, 5120); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	mgr.mu.Lock()
	mgr.snapshotting = true
	mgr.mu.Unlock()
	mgr.trigger(ReasonDebounce)

	mgr.mu.Lock()
	rearm := mgr.rearmAfter
	mgr.mu.Unlock()
	if !rearm {
		t.Fatal("进行中触发应置 rearmAfter")
	}

	mgr.mu.Lock()
	mgr.snapshotting = false
	mgr.mu.Unlock()
	mgr.finishSnapshot(ReasonDebounce, snapshotDir, mgr.ProjectID())

	// 补一次的快照应实际产出。
	waitFor(t, 5*time.Second, func() bool {
		list, _ := ListSnapshots(snapshotDir, mgr.ProjectID())
		return len(list) >= 1
	})
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.rearmAfter {
		t.Error("补一次后 rearmAfter 应清除")
	}
}

// TestManager_DisabledNoSnapshot 禁用时不产生快照。
func TestManager_DisabledNoSnapshot(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")
	mgr := NewManager(nil, nil)
	if err := mgr.Configure(workingDir, snapshotDir, false, false, 60, 15, 30, 5120); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()
	mgr.TriggerManual()
	time.Sleep(500 * time.Millisecond)
	list, _ := ListSnapshots(snapshotDir, mgr.ProjectID())
	if len(list) != 0 {
		t.Error("禁用状态不应产生快照")
	}
}

func TestManager_NestingRejected(t *testing.T) {
	base := t.TempDir()
	workingDir := filepath.Join(base, "work")
	snapshotDir := filepath.Join(base, "work", "snaps")
	if err := mkdirAll(workingDir); err != nil {
		t.Fatal(err)
	}
	// statusFunc 由 pushStatusLocked 在新 goroutine 中异步调用，须加锁保护 lastStatus。
	var mu sync.Mutex
	var lastStatus Status
	mgr := NewManager(func(s Status) {
		mu.Lock()
		defer mu.Unlock()
		lastStatus = s
	}, nil)
	if err := mgr.Configure(workingDir, snapshotDir, true, false, 60, 15, 30, 5120); err == nil {
		t.Fatal("嵌套配置应被拒绝")
	}
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastStatus.Phase == "error" && lastStatus.Error != ""
	})
	mu.Lock()
	defer mu.Unlock()
	if lastStatus.Phase != "error" || lastStatus.Error == "" {
		t.Errorf("状态应为 error，got %+v", lastStatus)
	}
}

func TestManager_WorkingDirMissing(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "not_exist")
	snapshotDir := filepath.Join(base, "snaps")
	mgr := NewManager(nil, nil)
	if err := mgr.Configure(missing, snapshotDir, true, false, 60, 15, 30, 5120); err == nil {
		t.Fatal("工作区根不存在应报错")
	}
	if mgr.Enabled() {
		t.Error("不应启用")
	}
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if mgr.status.Phase != "error" {
		t.Errorf("状态应为 error，got %q", mgr.status.Phase)
	}
}

func mkdirAll(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

func TestManager_PhaseReturnsToIdleAfterSnapshot(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")

	var mu sync.Mutex
	var lastPhase string
	mgr := NewManager(func(s Status) {
		mu.Lock()
		lastPhase = s.Phase
		mu.Unlock()
	}, nil)
	if err := mgr.Configure(workingDir, snapshotDir, true, false, 60, 15, 30, 5120); err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	mgr.TriggerManual()
	waitFor(t, 5*time.Second, func() bool {
		list, _ := ListSnapshots(snapshotDir, mgr.ProjectID())
		return len(list) == 1
	})

	// 状态回调异步推送，等待最终状态落地。
	waitFor(t, 2*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return lastPhase == "idle"
	})
}

package thread

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestScheduler_GlobalSerializes(t *testing.T) {
	s := NewScheduler()
	var counter int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Execute(Scope{Type: ScopeGlobal, Key: "test"}, func() {
				atomic.AddInt32(&counter, 1)
			})
		}()
	}

	wg.Wait()
	if atomic.LoadInt32(&counter) != 10 {
		t.Errorf("counter = %d, want 10", counter)
	}
}

func TestScheduler_ThreadSerializesSameKey(t *testing.T) {
	s := NewScheduler()
	var concurrent int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Execute(ScopeKeyThread("t1"), func() {
				c := atomic.AddInt32(&concurrent, 1)
				for {
					m := atomic.LoadInt32(&maxConcurrent)
					if c <= m || atomic.CompareAndSwapInt32(&maxConcurrent, m, c) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&concurrent, -1)
			})
		}()
	}

	wg.Wait()
	if maxConcurrent > 1 {
		t.Errorf("max concurrent for same thread = %d, want 1", maxConcurrent)
	}
}

func TestScheduler_ThreadParallelizesDifferentKeys(t *testing.T) {
	s := NewScheduler()
	var concurrent int32
	var maxConcurrent int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			s.Execute(ScopeKeyThread(string(rune('a'+id))), func() {
				c := atomic.AddInt32(&concurrent, 1)
				for {
					m := atomic.LoadInt32(&maxConcurrent)
					if c <= m || atomic.CompareAndSwapInt32(&maxConcurrent, m, c) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				atomic.AddInt32(&concurrent, -1)
			})
		}(i)
	}

	wg.Wait()
	if maxConcurrent < 2 {
		t.Errorf("max concurrent for different threads = %d, want >= 2", maxConcurrent)
	}
}

func TestScheduler_PanicRecovery(t *testing.T) {
	s := NewScheduler()
	done := make(chan struct{})

	s.Execute(Scope{Type: ScopeGlobal, Key: "panic_test"}, func() {
		panic("test panic")
	})

	s.Execute(Scope{Type: ScopeGlobal, Key: "panic_test"}, func() {
		close(done)
	})

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Error("scheduler blocked after panic")
	}
}

func TestMethodScope(t *testing.T) {
	tests := []struct {
		method string
		want   Scope
	}{
		{"initialize", Scope{Type: ScopeGlobal, Key: "global"}},
		{"thread/start", Scope{Type: ScopeThread, Key: "thread_id"}},
		{"thread/list", Scope{Type: ScopeGlobalSharedRead, Key: "thread_list"}},
		{"unknown/method", Scope{Type: ScopeThread, Key: "unknown/method"}},
	}
	for _, tt := range tests {
		got := MethodScope(tt.method)
		if got.Type != tt.want.Type {
			t.Errorf("MethodScope(%s).Type = %v, want %v", tt.method, got.Type, tt.want.Type)
		}
	}
}

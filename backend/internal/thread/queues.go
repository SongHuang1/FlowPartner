package thread

import (
	"fmt"
	"log"
	"sync"
)

// ScopeType defines the serialization semantics for a method.
type ScopeType int

const (
	// ScopeGlobal serializes all methods with the same key (one executor goroutine per key).
	ScopeGlobal ScopeType = iota
	// ScopeGlobalSharedRead allows parallel reads but exclusive writes per key.
	ScopeGlobalSharedRead
	// ScopeThread serializes per thread, parallel across threads.
	ScopeThread
)

// Scope describes the concurrency scope for a method.
type Scope struct {
	Type ScopeType
	Key  string
}

// SerialExecutor serializes function execution for a single key.
type SerialExecutor struct {
	mu   sync.Mutex
	done chan struct{}
}

func newSerialExecutor() *SerialExecutor {
	return &SerialExecutor{done: make(chan struct{}, 1)}
}

func (e *SerialExecutor) execute(fn func()) {
	e.mu.Lock()
	defer e.mu.Unlock()
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[SerialExecutor] panic recovered: %v", r)
		}
	}()
	fn()
}

// Scheduler manages method serialization scopes.
type Scheduler struct {
	mu        sync.Mutex
	executors map[string]*SerialExecutor
	rwMu      map[string]*sync.RWMutex
}

// NewScheduler creates a new Scheduler.
func NewScheduler() *Scheduler {
	return &Scheduler{
		executors: make(map[string]*SerialExecutor),
		rwMu:      make(map[string]*sync.RWMutex),
	}
}

// Execute runs fn under the given scope.
func (s *Scheduler) Execute(scope Scope, fn func()) {
	s.execute(scope, fn, false)
}

// ExecuteWrite runs fn under the given scope, marking it as a write operation.
// For ScopeGlobalSharedRead, this acquires an exclusive write lock.
func (s *Scheduler) ExecuteWrite(scope Scope, fn func()) {
	s.execute(scope, fn, true)
}

func (s *Scheduler) execute(scope Scope, fn func(), isWrite bool) {
	switch scope.Type {
	case ScopeGlobal:
		s.execGlobal(scope.Key, fn)
	case ScopeGlobalSharedRead:
		s.execGlobalSharedRead(scope.Key, fn, isWrite)
	case ScopeThread:
		s.execThread(scope.Key, fn)
	default:
		fn()
	}
}

func (s *Scheduler) execGlobal(key string, fn func()) {
	s.mu.Lock()
	e, ok := s.executors[key]
	if !ok {
		e = newSerialExecutor()
		s.executors[key] = e
	}
	s.mu.Unlock()
	e.execute(fn)
}

func (s *Scheduler) execGlobalSharedRead(key string, fn func(), isWrite bool) {
	s.mu.Lock()
	mu, ok := s.rwMu[key]
	if !ok {
		mu = &sync.RWMutex{}
		s.rwMu[key] = mu
	}
	s.mu.Unlock()
	if isWrite {
		mu.Lock()
		defer mu.Unlock()
	} else {
		mu.RLock()
		defer mu.RUnlock()
	}
	fn()
}

func (s *Scheduler) execThread(key string, fn func()) {
	s.mu.Lock()
	mu, ok := s.rwMu[key]
	if !ok {
		mu = &sync.RWMutex{}
		s.rwMu[key] = mu
	}
	s.mu.Unlock()
	mu.Lock()
	defer mu.Unlock()
	fn()
}

// MethodScope returns the scope for a given method name.
func MethodScope(method string) Scope {
	if s, ok := methodScopes[method]; ok {
		return s
	}
	return Scope{Type: ScopeThread, Key: method}
}

// methodScopes maps method names to their concurrency scopes.
var methodScopes = map[string]Scope{
	"initialize": {Type: ScopeGlobal, Key: "global"},
	"initialized": {Type: ScopeGlobal, Key: "global"},
	"thread/start": {Type: ScopeThread, Key: "thread_id"},
	"thread/list":  {Type: ScopeGlobalSharedRead, Key: "thread_list"},
	"thread/read":  {Type: ScopeGlobalSharedRead, Key: "thread_read"},
	"thread/archive": {Type: ScopeThread, Key: "thread_id"},
	"thread/delete":  {Type: ScopeThread, Key: "thread_id"},
	"turn/start":  {Type: ScopeThread, Key: "thread_id"},
	"turn/interrupt": {Type: ScopeThread, Key: "thread_id"},
	"turn/steer":  {Type: ScopeThread, Key: "thread_id"},
}

// ScopeKeyThread returns a thread-scoped key for a given thread ID.
func ScopeKeyThread(threadID string) Scope {
	return Scope{Type: ScopeThread, Key: fmt.Sprintf("thread:%s", threadID)}
}

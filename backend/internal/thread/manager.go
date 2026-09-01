package thread

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/storage"
)

type TurnStatus int

const (
	TurnIdle     TurnStatus = iota // 无活跃回合
	TurnActive                     // 回合进行中
	TurnAborting                   // 中断中（等待 Python 确认）
)

type Connection interface {
	SendNotification(method string, params interface{}) error
}

type Thread struct {
	ID        string    `json:"id"`
	AgentID   string    `json:"agentId"`
	Cwd       string    `json:"cwd"`
	CreatedAt time.Time `json:"createdAt"`
	Archived  bool      `json:"archived"`

	mu                     sync.RWMutex
	ActiveTurn             *TurnInfo
	pendingServerRequestID *int64
	connIDs                map[string]struct{}
	conns                  map[string]Connection
	manager                *Manager
}

// TurnInfo tracks the active turn within a thread.
type TurnInfo struct {
	ID        string     `json:"id"`
	Status    TurnStatus `json:"status"`
	StartedAt time.Time  `json:"startedAt"`
}

// Manager owns the thread registry and fanout routing.
type Manager struct {
	mu      sync.RWMutex
	threads map[string]*Thread

	reqMu     sync.Mutex
	nextReqID int64
	pending   map[int64]*ServerRequest

	done chan struct{}
}

// NewManager creates a thread Manager.
func NewManager() *Manager {
	return &Manager{
		threads:   make(map[string]*Thread),
		pending:   make(map[int64]*ServerRequest),
		nextReqID: 1,
		done:      make(chan struct{}),
	}
}

// Close signals the manager to stop background goroutines.
func (m *Manager) Close() {
	select {
	case <-m.done:
	default:
		close(m.done)
	}
}

// CreateThread creates a new thread and registers it.
func (m *Manager) CreateThread(id, agentID, cwd string) (*Thread, error) {
	if id == "" {
		return nil, fmt.Errorf("thread id 不能为空")
	}
	if !storage.ValidSessionID(id) {
		return nil, fmt.Errorf("无效的 thread id: %s", id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.threads[id]; exists {
		return nil, fmt.Errorf("thread 已存在: %s", id)
	}

	t := &Thread{
		ID:        id,
		AgentID:   agentID,
		Cwd:       cwd,
		CreatedAt: time.Now(),
		Archived:  false,
		connIDs:   make(map[string]struct{}),
		conns:     make(map[string]Connection),
		manager:   m,
	}
	m.threads[id] = t
	return t, nil
}

// GetThread retrieves a thread by id.
func (m *Manager) GetThread(id string) (*Thread, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.threads[id]
	return t, ok
}

func (m *Manager) ListThreads(cursor string, limit int, archived *bool) ([]*Thread, string) {
	m.mu.RLock()
	all := make([]*Thread, 0, len(m.threads))
	for _, t := range m.threads {
		all = append(all, t)
	}
	m.mu.RUnlock()

	if archived != nil {
		filtered := make([]*Thread, 0, len(all))
		for _, t := range all {
			if t.Archived == *archived {
				filtered = append(filtered, t)
			}
		}
		all = filtered
	}

	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	found := cursor == ""
	result := make([]*Thread, 0, limit)
	var nextCursor string
	for _, t := range all {
		if !found {
			if t.ID == cursor {
				found = true
			}
			continue
		}
		if len(result) >= limit {
			nextCursor = t.ID
			break
		}
		result = append(result, t)
	}
	return result, nextCursor
}

// ArchiveThread marks a thread as archived and aborts pending requests.
func (m *Manager) ArchiveThread(id string) error {
	m.mu.Lock()
	t, ok := m.threads[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("thread 不存在: %s", id)
	}
	t.mu.Lock()
	t.Archived = true
	t.mu.Unlock()

	go t.AbortPendingRequests(turnAbortedReason)
	return nil
}

// DeleteThread marks a thread as deleted (metadata only).
func (m *Manager) DeleteThread(id string) error {
	m.mu.Lock()
	t, ok := m.threads[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("thread 不存在: %s", id)
	}

	go t.AbortPendingRequests(turnAbortedReason)
	return nil
}

// --- Connection attach/detach ---

// AttachConn attaches a connection to a thread for event fanout.
func (t *Thread) AttachConn(connID string, conn Connection) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connIDs[connID] = struct{}{}
	t.conns[connID] = conn
}

// DetachConn removes a connection from a thread.
func (t *Thread) DetachConn(connID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.connIDs, connID)
	delete(t.conns, connID)
}

// ConnectionCount returns the number of attached connections.
func (t *Thread) ConnectionCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.conns)
}

// Fanout sends a notification to all connections attached to this thread.
func (t *Thread) Fanout(method string, params interface{}) {
	t.mu.RLock()
	conns := make(map[string]Connection, len(t.conns))
	for id, c := range t.conns {
		conns[id] = c
	}
	t.mu.RUnlock()

	for _, conn := range conns {
		if err := conn.SendNotification(method, params); err != nil {
			log.Printf("[Thread:%s] fanout failed: %v", t.ID, err)
		}
	}
}

// --- Turn management ---

// StartTurn activates a new turn on the thread. Returns error if busy.
func (t *Thread) StartTurn(turnID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Archived {
		return fmt.Errorf("thread 已归档")
	}
	if t.ActiveTurn != nil && t.ActiveTurn.Status != TurnIdle {
		return fmt.Errorf("turn conflict")
	}

	t.ActiveTurn = &TurnInfo{
		ID:        turnID,
		Status:    TurnActive,
		StartedAt: time.Now(),
	}
	return nil
}

// EndTurn clears the active turn.
func (t *Thread) EndTurn() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.ActiveTurn = nil
}

// GetTurn returns the current active turn.
func (t *Thread) GetTurn() *TurnInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.ActiveTurn
}

// SetTurnStatus updates the active turn's status.
func (t *Thread) SetTurnStatus(status TurnStatus) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ActiveTurn != nil {
		t.ActiveTurn.Status = status
	}
}

// PendingServerRequestID returns the pending request id for the thread, if any.
func (t *Thread) PendingServerRequestID() *int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.pendingServerRequestID
}

// SetPendingServerRequestID updates the pending request id on the thread.
func (t *Thread) SetPendingServerRequestID(id *int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pendingServerRequestID = id
}

// --- Pending request abort ---

const turnAbortedReason = "turn_aborted"

// AbortPendingRequests releases all pending requests on this thread with -32003.
func (t *Thread) AbortPendingRequests(reason string) {
	pendingID := t.PendingServerRequestID()
	if pendingID == nil {
		return
	}
	t.SetPendingServerRequestID(nil)
	if t.manager != nil {
		t.manager.resolveRequest(*pendingID, nil, fmt.Errorf("%w: %s", ErrApprovalReleased, reason))
	}
}

// ErrApprovalReleased indicates a pending approval was released due to abort/lock.
var ErrApprovalReleased = fmt.Errorf("approval pending released")

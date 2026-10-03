package thread

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ServerRequestKind categorizes reverse requests for timeout behavior.
type ServerRequestKind string

const (
	KindPermission ServerRequestKind = "permission"
)

var kindTimeouts = map[ServerRequestKind]time.Duration{
	KindPermission: 120 * time.Second,
}

// KindTimeout returns the timeout duration for a given kind.
func KindTimeout(kind ServerRequestKind) time.Duration {
	if d, ok := kindTimeouts[kind]; ok {
		return d
	}
	return 120 * time.Second
}

// ServerRequest represents a pending reverse request to a client.
type ServerRequest struct {
	ID             int64
	ThreadID       string
	Method         string
	Params         interface{}
	AvailableDecisions []string
	Kind           ServerRequestKind
	Deadline       time.Time
	ResponseCh     chan ServerRequestResult
}

// ServerRequestResult is the result of a reverse request.
type ServerRequestResult struct {
	Decision string
	Err      error
}

// CreateServerRequest allocates a new server request and registers it in the pending table.
func (m *Manager) CreateServerRequest(threadID, method string, params interface{}, decisions []string, kind ServerRequestKind) *ServerRequest {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()

	id := m.nextReqID
	m.nextReqID++

	sr := &ServerRequest{
		ID:                 id,
		ThreadID:           threadID,
		Method:             method,
		Params:             params,
		AvailableDecisions: decisions,
		Kind:               kind,
		Deadline:           time.Now().Add(KindTimeout(kind)),
		ResponseCh:         make(chan ServerRequestResult, 1),
	}
	m.pending[id] = sr

	thread, ok := m.threads[threadID]
	if ok {
		thread.SetPendingServerRequestID(&id)
	}

	log.Printf("[ServerRequest] created id=%d thread=%s method=%s kind=%s", id, threadID, method, kind)
	return sr
}

// resolveRequest resolves a pending request and notifies the caller.
// If decision is nil, it means timeout or abort.
func (m *Manager) resolveRequest(id int64, decision *string, err error) {
	m.reqMu.Lock()
	sr, ok := m.pending[id]
	if ok {
		delete(m.pending, id)
	}
	m.reqMu.Unlock()

	if !ok {
		return
	}

	m.mu.RLock()
	thread, threadOK := m.threads[sr.ThreadID]
	m.mu.RUnlock()
	if threadOK {
		thread.SetPendingServerRequestID(nil)
	}

	// Send resolved notification to all attached connections
	if threadOK {
		thread.Fanout("serverRequest/resolved", map[string]interface{}{
			"threadId":   sr.ThreadID,
			"requestId":  id,
		})
	}

	result := ServerRequestResult{Err: err}
	if decision != nil {
		result.Decision = *decision
	}
	select {
	case sr.ResponseCh <- result:
	default:
	}

	log.Printf("[ServerRequest] resolved id=%d thread=%s decision=%v err=%v", id, sr.ThreadID, decision, err)
}

// ResolveServerRequest resolves a request with a valid decision from the client.
func (m *Manager) ResolveServerRequest(id int64, decision string) error {
	m.reqMu.Lock()
	sr, ok := m.pending[id]
	m.reqMu.Unlock()
	if !ok {
		return fmt.Errorf("request %d not found", id)
	}

	// Validate decision is in availableDecisions
	valid := false
	for _, d := range sr.AvailableDecisions {
		if d == decision {
			valid = true
			break
		}
	}
	if !valid {
		// Invalid decision = treat as timeout (fail-closed)
		m.resolveRequest(id, nil, fmt.Errorf("invalid decision: %s", decision))
		return nil
	}

	m.resolveRequest(id, &decision, nil)
	return nil
}

// GetPendingRequest returns a pending request by id.
func (m *Manager) GetPendingRequest(id int64) (*ServerRequest, bool) {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	sr, ok := m.pending[id]
	return sr, ok
}

// GetAllPendingForThread returns all pending requests for a thread.
func (m *Manager) GetAllPendingForThread(threadID string) []*ServerRequest {
	m.reqMu.Lock()
	defer m.reqMu.Unlock()
	var result []*ServerRequest
	for _, sr := range m.pending {
		if sr.ThreadID == threadID {
			result = append(result, sr)
		}
	}
	return result
}

// ReplayPendingForConnection resends pending requests to a reattached connection (idempotent).
func (m *Manager) ReplayPendingForConnection(threadID string, conn Connection) {
	pending := m.GetAllPendingForThread(threadID)
	for _, sr := range pending {
		if err := conn.SendNotification(sr.Method, sr.Params); err != nil {
			log.Printf("[ServerRequest] replay failed id=%d: %v", sr.ID, err)
		}
	}
}

// AbortAllPending releases all pending requests (e.g., on shutdown or keystore lock).
func (m *Manager) AbortAllPending(reason string) {
	m.reqMu.Lock()
	all := make(map[int64]*ServerRequest, len(m.pending))
	for id, sr := range m.pending {
		all[id] = sr
	}
	m.reqMu.Unlock()

	for id := range all {
		m.resolveRequest(id, nil, fmt.Errorf("%w: %s", ErrApprovalReleased, reason))
	}
}

// WaitForResponse waits for the response to a server request with timeout.
func (sr *ServerRequest) WaitForResponse() ServerRequestResult {
	select {
	case result := <-sr.ResponseCh:
		return result
	default:
	}
	select {
	case result := <-sr.ResponseCh:
		return result
	case <-time.After(time.Until(sr.Deadline)):
		return ServerRequestResult{Err: fmt.Errorf("request timeout")}
	}
}

// ServerRequestIdPtr returns a pointer to an int64 value.
func ServerRequestIdPtr(id int64) *int64 {
	return &id
}

// GlobalMutex exposes the initialize/settings global mutex for serialization_scope.
func (m *Manager) GlobalMutex() *sync.Mutex {
	return &m.reqMu
}

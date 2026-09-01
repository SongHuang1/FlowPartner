package tools

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
)

type ApprovalStatus int

const (
	ApprovalPending  ApprovalStatus = iota
	ApprovalGranted
	ApprovalDenied
)

type Approval struct {
	ThreadID     string
	ToolName     string
	RawPath      string
	ResolvedPath string
	Status       ApprovalStatus
	Consumed     bool
}

type ApprovalManager struct {
	mu      sync.RWMutex
	records map[string]*Approval
}

func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		records: make(map[string]*Approval),
	}
}

func (m *ApprovalManager) Create(threadID, toolName, rawPath, resolvedPath string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	requestID := uuid.New().String()
	m.records[requestID] = &Approval{
		ThreadID:     threadID,
		ToolName:     toolName,
		RawPath:      rawPath,
		ResolvedPath: resolvedPath,
		Status:       ApprovalPending,
		Consumed:     false,
	}

	log.Printf("[Approval] Created request_id=%s thread=%s tool=%s path=%s", requestID, threadID, toolName, rawPath)
	return requestID
}

func (m *ApprovalManager) Resolve(threadID, requestID, decision string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, exists := m.records[requestID]
	if !exists {
		log.Printf("[Approval] Resolve failed: request_id=%s not found", requestID)
		return false
	}
	if approval.ThreadID != threadID {
		log.Printf("[Approval] Resolve failed: thread mismatch, request_id=%s expected=%s got=%s", requestID, approval.ThreadID, threadID)
		return false
	}

	if decision == "allow" {
		approval.Status = ApprovalGranted
		log.Printf("[Approval] Granted request_id=%s", requestID)
	} else {
		approval.Status = ApprovalDenied
		log.Printf("[Approval] Denied request_id=%s", requestID)
	}

	return true
}

func (m *ApprovalManager) Consume(threadID, approvalID, toolName, resolvedPath string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, exists := m.records[approvalID]
	if !exists {
		log.Printf("[Approval] Consume failed: approval_id=%s not found", approvalID)
		return false
	}
	if approval.ThreadID != threadID {
		log.Printf("[Approval] Consume failed: thread mismatch, approval_id=%s expected=%s got=%s", approvalID, approval.ThreadID, threadID)
		return false
	}
	if approval.ToolName != toolName {
		log.Printf("[Approval] Consume failed: tool mismatch, approval_id=%s expected=%s got=%s", approvalID, approval.ToolName, toolName)
		return false
	}
	if approval.ResolvedPath != resolvedPath {
		log.Printf("[Approval] Consume failed: path mismatch, approval_id=%s expected=%s got=%s", approvalID, approval.ResolvedPath, resolvedPath)
		return false
	}
	if approval.Status != ApprovalGranted {
		log.Printf("[Approval] Consume failed: not granted, approval_id=%s status=%d", approvalID, approval.Status)
		return false
	}
	if approval.Consumed {
		log.Printf("[Approval] Consume failed: already consumed, approval_id=%s", approvalID)
		return false
	}

	approval.Consumed = true
	delete(m.records, approvalID)
	log.Printf("[Approval] Consumed approval_id=%s", approvalID)
	return true
}

func (m *ApprovalManager) CancelThread(threadID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, approval := range m.records {
		if approval.ThreadID == threadID && approval.Status == ApprovalPending {
			approval.Status = ApprovalDenied
			log.Printf("[Approval] Cancelled: request_id=%s thread=%s", id, threadID)
		}
	}
}

func (m *ApprovalManager) GetPendingRequestID(threadID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, approval := range m.records {
		if approval.ThreadID == threadID && approval.Status == ApprovalPending {
			return id
		}
	}
	return ""
}

func DeniedMessage(path string) string {
	return fmt.Sprintf("用户拒绝了此操作：%s", path)
}

package tools

import (
	"fmt"
	"log"
	"sync"

	"github.com/google/uuid"
)

// ApprovalStatus 审批记录状态。
type ApprovalStatus int

const (
	ApprovalPending  ApprovalStatus = iota // 待审批
	ApprovalGranted                        // 已批准
	ApprovalDenied                         // 已拒绝
)

// Approval 单条审批记录。
type Approval struct {
	SessionID   string
	ToolName    string
	RawPath     string // 请求时的原始路径（用于比较）
	ResolvedPath string // PathGuard.Resolve 后的绝对路径（用于比较）
	Status      ApprovalStatus
	Consumed    bool
}

// trustKey 会话级信任的复合键。
type trustKey struct {
	SessionID    string
	ToolName     string
	ResolvedPath string
}

// ApprovalManager 管理所有审批记录（内存态，不落盘）与会话级信任。
type ApprovalManager struct {
	mu      sync.RWMutex
	records map[string]*Approval
	trusts  map[trustKey]struct{}
}

// NewApprovalManager 创建审批管理器。
func NewApprovalManager() *ApprovalManager {
	return &ApprovalManager{
		records: make(map[string]*Approval),
		trusts:  make(map[trustKey]struct{}),
	}
}

// Create 创建一条待审批记录，返回 request_id（UUID v4）。
func (m *ApprovalManager) Create(sessionID, toolName, rawPath, resolvedPath string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	requestID := uuid.New().String()
	m.records[requestID] = &Approval{
		SessionID:    sessionID,
		ToolName:     toolName,
		RawPath:      rawPath,
		ResolvedPath: resolvedPath,
		Status:       ApprovalPending,
		Consumed:     false,
	}

	log.Printf("[Approval] Created request_id=%s session=%s tool=%s path=%s", requestID, sessionID, toolName, rawPath)
	return requestID
}

// Resolve 更新审批记录状态（用户做出决定时调用）。
func (m *ApprovalManager) Resolve(sessionID, requestID, decision string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, exists := m.records[requestID]
	if !exists {
		log.Printf("[Approval] Resolve failed: request_id=%s not found", requestID)
		return false
	}
	if approval.SessionID != sessionID {
		log.Printf("[Approval] Resolve failed: session mismatch, request_id=%s expected=%s got=%s", requestID, approval.SessionID, sessionID)
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

// Consume 校验审批记录并标记为已消费（一次性：成功后删除记录）。
func (m *ApprovalManager) Consume(sessionID, approvalID, toolName, resolvedPath string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	approval, exists := m.records[approvalID]
	if !exists {
		log.Printf("[Approval] Consume failed: approval_id=%s not found", approvalID)
		return false
	}
	if approval.SessionID != sessionID {
		log.Printf("[Approval] Consume failed: session mismatch, approval_id=%s expected=%s got=%s", approvalID, approval.SessionID, sessionID)
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

// AddTrust 直接添加会话级信任（由 ws.go 在 permission_response scope=session 时调用）。
// 不消费审批记录，审批记录由 agent.go 的 Consume 处理。
func (m *ApprovalManager) AddTrust(sessionID, toolName, resolvedPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := trustKey{SessionID: sessionID, ToolName: toolName, ResolvedPath: resolvedPath}
	m.trusts[key] = struct{}{}
	log.Printf("[Trust] Added session trust: session=%s tool=%s path=%s", sessionID, toolName, resolvedPath)
}

// GetApproval 查找审批记录（用于 ws.go 在 scope=session 时获取 toolName 和 resolvedPath）。
func (m *ApprovalManager) GetApproval(sessionID, approvalID string) (toolName, resolvedPath string, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	approval, exists := m.records[approvalID]
	if !exists {
		return "", "", false
	}
	if approval.SessionID != sessionID {
		return "", "", false
	}
	return approval.ToolName, approval.ResolvedPath, true
}

// CancelSession 将指定 session 的所有待审批记录置为已拒绝，并清除该会话的全部会话级信任（断连时调用）。
func (m *ApprovalManager) CancelSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, approval := range m.records {
		if approval.SessionID == sessionID && approval.Status == ApprovalPending {
			approval.Status = ApprovalDenied
			log.Printf("[Approval] Cancelled by disconnect: request_id=%s session=%s", id, sessionID)
		}
	}

	for key := range m.trusts {
		if key.SessionID == sessionID {
			delete(m.trusts, key)
		}
	}
	log.Printf("[Trust] Cleared all trusts for session=%s", sessionID)
}

// GetPendingRequestID 获取指定 session 的待审批 request_id。
func (m *ApprovalManager) GetPendingRequestID(sessionID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for id, approval := range m.records {
		if approval.SessionID == sessionID && approval.Status == ApprovalPending {
			return id
		}
	}
	return ""
}

// IsTrusted 检查指定 (session, tool, resolvedPath) 组合是否有会话级信任。
func (m *ApprovalManager) IsTrusted(sessionID, toolName, resolvedPath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := trustKey{SessionID: sessionID, ToolName: toolName, ResolvedPath: resolvedPath}
	_, trusted := m.trusts[key]
	if trusted {
		log.Printf("[Trust] Hit: session=%s tool=%s path=%s", sessionID, toolName, resolvedPath)
	}
	return trusted
}

// DeniedMessage 返回拒绝文案。
func DeniedMessage(path string) string {
	return fmt.Sprintf("用户拒绝了此操作：%s", path)
}

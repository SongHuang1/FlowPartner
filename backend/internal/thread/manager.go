package thread

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/songhuang/flowpartner/backend/internal/bridge"
	"github.com/songhuang/flowpartner/backend/internal/tools"
	"github.com/songhuang/flowpartner/backend/proto"
)

type TurnStatus int

const (
	TurnIdle     TurnStatus = iota // 无活跃回合
	TurnActive                     // 回合进行中
	TurnAborting                   // 中断中（等待 Python 确认）
)

type TurnInfo struct {
	ThreadID  string
	TurnID    string
	Status    TurnStatus
	StartedAt time.Time
}

type ClearTrustFunc func(sessionID string)

type TurnManager struct {
	manager     *bridge.Manager
	approvalMgr *tools.ApprovalManager
	clearTrust  ClearTrustFunc

	mu    sync.RWMutex
	turns map[string]*TurnInfo // session_id -> turn info
}

func NewTurnManager(m *bridge.Manager, am *tools.ApprovalManager) *TurnManager {
	return &TurnManager{
		manager:     m,
		approvalMgr: am,
		turns:       make(map[string]*TurnInfo),
	}
}

func (tm *TurnManager) SetClearTrustFunc(fn ClearTrustFunc) {
	tm.clearTrust = fn
}

func (tm *TurnManager) StartTurn(sessionID, threadID, turnID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.turns[sessionID] = &TurnInfo{
		ThreadID:  threadID,
		TurnID:    turnID,
		Status:    TurnActive,
		StartedAt: time.Now(),
	}
}

func (tm *TurnManager) GetTurn(sessionID string) *TurnInfo {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.turns[sessionID]
}

func (tm *TurnManager) EndTurn(sessionID string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	delete(tm.turns, sessionID)
}

func (tm *TurnManager) SteerInput(sessionID, threadID, turnID, content string) int {
	tm.mu.RLock()
	info, ok := tm.turns[sessionID]
	tm.mu.RUnlock()

	if !ok {
		log.Printf("[TurnManager] Steer rejected: no active turn for session=%s", sessionID)
		return -32005
	}

	if info.Status != TurnActive {
		log.Printf("[TurnManager] Steer rejected: turn not active (status=%d) for session=%s", info.Status, sessionID)
		return -32005
	}

	if turnID != "" && info.TurnID != turnID {
		log.Printf("[TurnManager] Steer rejected: turn_id mismatch (expected=%s got=%s)", info.TurnID, turnID)
		return -32005
	}

	payload := map[string]interface{}{
		"content":   content,
		"thread_id": threadID,
		"turn_id":   info.TurnID,
	}

	payloadBytes, _ := jsonMarshal(payload)
	cmd := &proto.ServerCommand{
		SessionId:   sessionID,
		CommandType: "steer_input",
		Payload:     string(payloadBytes),
	}

	select {
	case tm.manager.CmdChan <- cmd:
		log.Printf("[TurnManager] Steer input sent: session=%s turn=%s", sessionID, info.TurnID)
		return 0
	default:
		log.Printf("[TurnManager] Steer failed: CmdChan full for session=%s", sessionID)
		return -32001
	}
}

func (tm *TurnManager) InterruptTurn(sessionID, reason string) error {
	tm.mu.Lock()
	info, ok := tm.turns[sessionID]
	if ok && info.Status == TurnAborting {
		tm.mu.Unlock()
		return nil // 已在中断中
	}

	if ok {
		info.Status = TurnAborting
	}
	threadID := ""
	turnID := ""
	if ok {
		threadID = info.ThreadID
		turnID = info.TurnID
	}
	tm.mu.Unlock()

	tm.approvalMgr.CancelSession(sessionID)

	if tm.clearTrust != nil {
		tm.clearTrust(sessionID)
	}

	payload := map[string]interface{}{
		"thread_id": threadID,
		"turn_id":   turnID,
		"reason":    reason,
	}
	payloadBytes, _ := jsonMarshal(payload)
	cmd := &proto.ServerCommand{
		SessionId:   sessionID,
		CommandType: "abort_turn",
		Payload:     string(payloadBytes),
	}

	select {
	case tm.manager.CmdChan <- cmd:
		log.Printf("[TurnManager] AbortTurn sent: session=%s turn=%s reason=%s", sessionID, turnID, reason)
	default:
		log.Printf("[TurnManager] AbortTurn failed: CmdChan full for session=%s", sessionID)
	}

	if ok {
		go tm.forceAbortAfterTimeout(sessionID, 5*time.Second)
	}

	return nil
}

func (tm *TurnManager) forceAbortAfterTimeout(sessionID string, timeout time.Duration) {
	time.Sleep(timeout)

	tm.mu.RLock()
	info, ok := tm.turns[sessionID]
	tm.mu.RUnlock()

	if !ok {
		return
	}

	if info.Status == TurnAborting {
		log.Printf("[TurnManager] Force abort after timeout: session=%s turn=%s", sessionID, info.TurnID)
		tm.EndTurn(sessionID)
	}
}

func jsonMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

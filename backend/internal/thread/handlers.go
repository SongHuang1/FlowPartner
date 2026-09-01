package thread

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// HandlerError is a structured error from a thread/turn handler.
type HandlerError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *HandlerError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// Handler implements the thread/* and turn/* method handlers.
type Handler struct {
	manager   *Manager
	scheduler *Scheduler
	methods   map[string]func(json.RawMessage) (interface{}, *HandlerError)
}

// NewHandler creates a new Handler.
func NewHandler(m *Manager, s *Scheduler) *Handler {
	h := &Handler{manager: m, scheduler: s, methods: make(map[string]func(json.RawMessage) (interface{}, *HandlerError))}
	h.register()
	return h
}

func (h *Handler) register() {
	h.methods["thread/start"] = h.handleThreadStart
	h.methods["thread/list"] = h.handleThreadList
	h.methods["thread/read"] = h.handleThreadRead
	h.methods["thread/archive"] = h.handleThreadArchive
	h.methods["thread/delete"] = h.handleThreadDelete
	h.methods["turn/start"] = h.handleTurnStart
	h.methods["turn/interrupt"] = h.handleTurnInterrupt
	h.methods["turn/steer"] = h.handleTurnSteer
}

// Dispatch dispatches a method call.
func (h *Handler) Dispatch(method string, params json.RawMessage) (interface{}, *HandlerError) {
	fn, ok := h.methods[method]
	if !ok {
		return nil, &HandlerError{Code: -32601, Message: "未知方法: " + method}
	}
	return fn(params)
}

// Methods returns the registered method names.
func (h *Handler) Methods() map[string]struct{} {
	result := make(map[string]struct{}, len(h.methods))
	for m := range h.methods {
		result[m] = struct{}{}
	}
	return result
}

// MethodNames returns the list of registered method names.
func (h *Handler) MethodNames() []string {
	names := make([]string, 0, len(h.methods))
	for m := range h.methods {
		names = append(names, m)
	}
	return names
}

// --- thread/start ---

type threadStartParams struct {
	Cwd          *string `json:"cwd,omitempty"`
	AgentID      *string `json:"agentId,omitempty"`
	Model        *string `json:"model,omitempty"`
	ApprovalMode *string `json:"approvalMode,omitempty"`
}

type threadStartResult struct {
	ThreadID  string `json:"threadId"`
	AgentID   string `json:"agentId"`
	Cwd       string `json:"cwd"`
	CreatedAt int64  `json:"createdAtMs"`
}

func (h *Handler) handleThreadStart(params json.RawMessage) (interface{}, *HandlerError) {
	var p threadStartParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("thread/start 参数格式错误: %v", err)}
		}
	}

	threadID := generateThreadID()
	agentID := "main"
	if p.AgentID != nil && *p.AgentID != "" {
		agentID = *p.AgentID
	}
	cwd := ""
	if p.Cwd != nil {
		cwd = *p.Cwd
	}

	thread, err := h.manager.CreateThread(threadID, agentID, cwd)
	if err != nil {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("创建 thread 失败: %v", err)}
	}

	return threadStartResult{
		ThreadID:  thread.ID,
		AgentID:   thread.AgentID,
		Cwd:       thread.Cwd,
		CreatedAt: thread.CreatedAt.UnixMilli(),
	}, nil
}

// --- thread/list ---

type threadListParams struct {
	Cursor   *string `json:"cursor,omitempty"`
	Limit    *int    `json:"limit,omitempty"`
	Archived *bool   `json:"archived,omitempty"`
}

type threadListResult struct {
	Data       []threadListItem `json:"data"`
	NextCursor string           `json:"nextCursor,omitempty"`
}

type threadListItem struct {
	ThreadID  string `json:"threadId"`
	AgentID   string `json:"agentId"`
	Cwd       string `json:"cwd"`
	CreatedAt int64  `json:"createdAtMs"`
	Archived  bool   `json:"archived"`
}

func (h *Handler) handleThreadList(params json.RawMessage) (interface{}, *HandlerError) {
	var p threadListParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("thread/list 参数格式错误: %v", err)}
		}
	}

	limit := 50
	if p.Limit != nil && *p.Limit > 0 {
		limit = *p.Limit
	}
	cursor := ""
	if p.Cursor != nil {
		cursor = *p.Cursor
	}

	threads, nextCursor := h.manager.ListThreads(cursor, limit, p.Archived)
	items := make([]threadListItem, 0, len(threads))
	for _, t := range threads {
		items = append(items, threadListItem{
			ThreadID:  t.ID,
			AgentID:   t.AgentID,
			Cwd:       t.Cwd,
			CreatedAt: t.CreatedAt.UnixMilli(),
			Archived:  t.Archived,
		})
	}

	return threadListResult{Data: items, NextCursor: nextCursor}, nil
}

// --- thread/read ---

type threadReadParams struct {
	ThreadID string `json:"threadId"`
}

type threadReadResult struct {
	Thread threadDetail `json:"thread"`
	Turns  []turnDetail `json:"turns"`
}

type threadDetail struct {
	ThreadID  string `json:"threadId"`
	AgentID   string `json:"agentId"`
	Cwd       string `json:"cwd"`
	CreatedAt int64  `json:"createdAtMs"`
	Archived  bool   `json:"archived"`
}

type turnDetail struct {
	TurnID    string `json:"turnId"`
	Status    string `json:"status"`
	StartedAt int64  `json:"startedAtMs"`
}

func (h *Handler) handleThreadRead(params json.RawMessage) (interface{}, *HandlerError) {
	var p threadReadParams
	if err := json.Unmarshal(params, &p); err != nil || p.ThreadID == "" {
		return nil, &HandlerError{Code: -32602, Message: "thread/read 缺少 threadId 参数"}
	}

	thread, ok := h.manager.GetThread(p.ThreadID)
	if !ok {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("thread 不存在: %s", p.ThreadID)}
	}

	turns := []turnDetail{}
	if thread.GetTurn() != nil {
		t := thread.GetTurn()
		turns = append(turns, turnDetail{
			TurnID:    t.ID,
			Status:    turnStatusString(t.Status),
			StartedAt: t.StartedAt.UnixMilli(),
		})
	}

	return threadReadResult{
		Thread: threadDetail{
			ThreadID:  thread.ID,
			AgentID:   thread.AgentID,
			Cwd:       thread.Cwd,
			CreatedAt: thread.CreatedAt.UnixMilli(),
			Archived:  thread.Archived,
		},
		Turns: turns,
	}, nil
}

// --- thread/archive ---

type threadArchiveParams struct {
	ThreadID string `json:"threadId"`
}

func (h *Handler) handleThreadArchive(params json.RawMessage) (interface{}, *HandlerError) {
	var p threadArchiveParams
	if err := json.Unmarshal(params, &p); err != nil || p.ThreadID == "" {
		return nil, &HandlerError{Code: -32602, Message: "thread/archive 缺少 threadId 参数"}
	}

	if err := h.manager.ArchiveThread(p.ThreadID); err != nil {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("归档失败: %v", err)}
	}

	return map[string]string{"threadId": p.ThreadID, "status": "archived"}, nil
}

// --- thread/delete ---

type threadDeleteParams struct {
	ThreadID string `json:"threadId"`
}

func (h *Handler) handleThreadDelete(params json.RawMessage) (interface{}, *HandlerError) {
	var p threadDeleteParams
	if err := json.Unmarshal(params, &p); err != nil || p.ThreadID == "" {
		return nil, &HandlerError{Code: -32602, Message: "thread/delete 缺少 threadId 参数"}
	}

	if err := h.manager.DeleteThread(p.ThreadID); err != nil {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("删除失败: %v", err)}
	}

	return map[string]string{"threadId": p.ThreadID, "status": "deleted"}, nil
}

// --- turn/start ---

type turnStartParams struct {
	ThreadID  string      `json:"threadId"`
	Input     []UserInput `json:"input"`
	Overrides *map[string]interface{} `json:"overrides,omitempty"`
}

// UserInput represents a user input item.
type UserInput struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type turnStartResult struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

func (h *Handler) handleTurnStart(params json.RawMessage) (interface{}, *HandlerError) {
	var p turnStartParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("turn/start 参数格式错误: %v", err)}
	}

	if p.ThreadID == "" {
		return nil, &HandlerError{Code: -32602, Message: "turn/start 缺少 threadId 参数"}
	}
	if len(p.Input) == 0 {
		return nil, &HandlerError{Code: -32602, Message: "turn/start 缺少 input 参数"}
	}
	for _, input := range p.Input {
		if input.Type != "text" {
			return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("不支持的 input type: %s（首批仅支持 text）", input.Type)}
		}
	}

	thread, ok := h.manager.GetThread(p.ThreadID)
	if !ok {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("thread 不存在: %s", p.ThreadID)}
	}
	if thread.Archived {
		return nil, &HandlerError{Code: -32602, Message: "该会话已归档，无法启动新回合"}
	}

	turnID := generateTurnID()
	if err := thread.StartTurn(turnID); err != nil {
		return nil, &HandlerError{Code: -32002, Message: fmt.Sprintf("回合冲突: %v", err)}
	}

	return turnStartResult{
		ThreadID: p.ThreadID,
		TurnID:   turnID,
	}, nil
}

// --- turn/interrupt ---

type turnInterruptParams struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId,omitempty"`
}

func (h *Handler) handleTurnInterrupt(params json.RawMessage) (interface{}, *HandlerError) {
	var p turnInterruptParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("turn/interrupt 参数格式错误: %v", err)}
	}
	if p.ThreadID == "" {
		return nil, &HandlerError{Code: -32602, Message: "turn/interrupt 缺少 threadId 参数"}
	}

	thread, ok := h.manager.GetThread(p.ThreadID)
	if !ok {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("thread 不存在: %s", p.ThreadID)}
	}

	turn := thread.GetTurn()
	if turn == nil || turn.Status != TurnActive {
		return nil, &HandlerError{Code: -32005, Message: "该线程没有活跃的回合"}
	}

	thread.SetTurnStatus(TurnAborting)
	thread.AbortPendingRequests("user_interrupt")

	return map[string]string{"threadId": p.ThreadID, "status": "aborting"}, nil
}

// --- turn/steer ---

type turnSteerParams struct {
	ThreadID       string      `json:"threadId"`
	ExpectedTurnID string      `json:"turnId,omitempty"`
	Input          []UserInput `json:"input"`
}

func (h *Handler) handleTurnSteer(params json.RawMessage) (interface{}, *HandlerError) {
	var p turnSteerParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("turn/steer 参数格式错误: %v", err)}
	}
	if p.ThreadID == "" {
		return nil, &HandlerError{Code: -32602, Message: "turn/steer 缺少 threadId 参数"}
	}

	thread, ok := h.manager.GetThread(p.ThreadID)
	if !ok {
		return nil, &HandlerError{Code: -32602, Message: fmt.Sprintf("thread 不存在: %s", p.ThreadID)}
	}

	turn := thread.GetTurn()
	if turn == nil || turn.Status != TurnActive {
		return nil, &HandlerError{Code: -32005, Message: "该线程没有活跃的回合"}
	}
	if p.ExpectedTurnID != "" && turn.ID != p.ExpectedTurnID {
		return nil, &HandlerError{Code: -32005, Message: fmt.Sprintf("turnId 不匹配: expected=%s actual=%s", p.ExpectedTurnID, turn.ID)}
	}

	return map[string]string{"threadId": p.ThreadID, "turnId": turn.ID, "status": "steered"}, nil
}

// --- helpers ---

func turnStatusString(s TurnStatus) string {
	switch s {
	case TurnIdle:
		return "idle"
	case TurnActive:
		return "active"
	case TurnAborting:
		return "aborting"
	default:
		return "unknown"
	}
}

var threadCounter int64

func generateThreadID() string {
	threadCounter++
	return fmt.Sprintf("t_%d_%d", time.Now().UnixNano(), threadCounter)
}

var turnCounter int64

func generateTurnID() string {
	turnCounter++
	return fmt.Sprintf("u_%d_%d", time.Now().UnixNano(), turnCounter)
}

var _ = log.Printf

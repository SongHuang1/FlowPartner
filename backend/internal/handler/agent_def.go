package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
	"github.com/songhuang/flowpartner/backend/internal/thread"
	"github.com/songhuang/flowpartner/backend/proto"
)

const (
	// mainAgentID 内置主智能体的固定 ID（不可删除、不可创建同名）。
	mainAgentID = "main"

	// maxAgentNameRunes
	maxAgentNameRunes = 128

	maxAgentDescriptionRunes = 2000
	maxAgentPromptRunes      = 200000

	// defaultMainPrompt 内置主智能体的默认系统提示词（与历史硬编码一致）。
	defaultMainPrompt = "你是一个强大的本地 AI 助手FlowPartner。你可以使用工具读取文件、写入文件、浏览目录等，帮助用户完成各种任务。请根据用户需求合理使用工具。删除任何文件或目录时，必须使用 trash 工具（移入回收站，可恢复），禁止使用 shell 删除命令。仅当用户明确要求永久删除回收站内容时，才使用 purge 工具（该操作不可逆且每次都需要用户审批）。"
)

// AgentDefHandler 处理 /api/agents 的 REST CRUD。
// sendCmd 用于向 Python 侧发送 agents_changed 指令。
// notifyFrontend 用于向所有前端广播 agents_changed 事件。
type AgentDefHandler struct {
	threadMgr     *thread.Manager
	sendCmd       func(cmd *proto.ServerCommand)
	notifyFrontend func(eventType, payload string)
}

// NewAgentDefHandler 创建智能体定义处理器。
func NewAgentDefHandler(threadMgr *thread.Manager, sendCmd func(cmd *proto.ServerCommand), notifyFrontend func(eventType, payload string)) *AgentDefHandler {
	return &AgentDefHandler{threadMgr: threadMgr, sendCmd: sendCmd, notifyFrontend: notifyFrontend}
}

// BuiltinMainAgent 构造内置主智能体定义。
// system_prompt 取自设置（settings.SystemPrompt），未设置时使用默认提示词，
// 与历史行为保持一致——主智能体的提示词仍由设置页「系统提示词」字段管理。
func BuiltinMainAgent() storage.AgentDef {
	prompt := LoadSettings().SystemPrompt
	if strings.TrimSpace(prompt) == "" {
		prompt = defaultMainPrompt
	}
	return storage.AgentDef{
		ID:           mainAgentID,
		Name:         "主智能体",
		Description:  "主智能体：负责与用户直接对话，统筹协调各子智能体完成任务。可调用除自身外的全部子智能体。",
		SystemPrompt: prompt,
	}
}

// ListMeta 列表条目（不含 system_prompt，私有数据不出现在列表接口）。
type ListMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Handle 分发 GET/POST 到 /api/agents。
func (h *AgentDefHandler) Handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.List(w, r)
	case http.MethodPost:
		h.Create(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// HandleByID 分发 GET/PUT/DELETE 到 /api/agents/{id}。
func (h *AgentDefHandler) HandleByID(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.Get(w, r)
	case http.MethodPut:
		h.Update(w, r)
	case http.MethodDelete:
		h.Delete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// List 返回全部智能体元数据（内置 main + 用户定义），不含 system_prompt。
func (h *AgentDefHandler) List(w http.ResponseWriter, r *http.Request) {
	agents, err := storage.LoadAgents()
	if err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "读取智能体定义失败"))
		return
	}
	result := make([]ListMeta, 0, len(agents)+1)
	main := BuiltinMainAgent()
	result = append(result, ListMeta{ID: main.ID, Name: main.Name, Description: main.Description})
	for _, a := range agents {
		result = append(result, ListMeta{ID: a.ID, Name: a.Name, Description: a.Description})
	}
	response.WriteJSON(w, http.StatusOK, response.Success(result))
}

// Get 返回单个智能体完整定义（含 system_prompt，供编辑界面使用）。
func (h *AgentDefHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := agentIDFromPath(r.URL.Path)
	if id == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "智能体 ID 不能为空"))
		return
	}
	if id == mainAgentID {
		response.WriteJSON(w, http.StatusOK, response.Success(BuiltinMainAgent()))
		return
	}

	agents, err := storage.LoadAgents()
	if err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "读取智能体定义失败"))
		return
	}
	for _, a := range agents {
		if a.ID == id {
			response.WriteJSON(w, http.StatusOK, response.Success(a))
			return
		}
	}
	response.WriteJSON(w, http.StatusNotFound,
		response.Error(response.CodeNotFound, "智能体不存在"))
}

// Create 创建智能体定义（ID 由后端生成 UUID v4，名称全局唯一）。
func (h *AgentDefHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "请求体格式错误"))
		return
	}

	def := storage.AgentDef{
		ID:           uuid.NewString(),
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		SystemPrompt: strings.TrimSpace(req.SystemPrompt),
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	}
	if err := validateAgentDef(&def); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, err.Error()))
		return
	}

	agents, err := storage.LoadAgents()
	if err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "读取智能体定义失败"))
		return
	}
	if agentNameExists(def.Name, agents, "") {
		response.WriteJSON(w, http.StatusConflict,
			response.Error(response.CodeNameConflict, fmt.Sprintf("智能体名称「%s」已存在", def.Name)))
		return
	}

	agents = append(agents, def)
	if err := storage.SaveAgents(agents); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "保存智能体定义失败"))
		return
	}

	h.audit("create", def.ID)
	h.notifyAgentsChanged()
	response.WriteJSON(w, http.StatusCreated, response.Success(def))
}

// Update 更新智能体定义（名称唯一性校验排除自身）。
func (h *AgentDefHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := agentIDFromPath(r.URL.Path)
	if id == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "智能体 ID 不能为空"))
		return
	}
	if id == mainAgentID {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "内置主智能体不可编辑，请在「智能体」设置中修改系统提示词"))
		return
	}

	var req struct {
		Name         string `json:"name"`
		Description  string `json:"description"`
		SystemPrompt string `json:"system_prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "请求体格式错误"))
		return
	}

	agents, err := storage.LoadAgents()
	if err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "读取智能体定义失败"))
		return
	}
	found := false
	for i := range agents {
		if agents[i].ID != id {
			continue
		}
		def := storage.AgentDef{
			ID:           id,
			Name:         strings.TrimSpace(req.Name),
			Description:  strings.TrimSpace(req.Description),
			SystemPrompt: strings.TrimSpace(req.SystemPrompt),
			CreatedAt:    agents[i].CreatedAt,
			UpdatedAt:    time.Now().UnixMilli(),
		}
		if err := validateAgentDef(&def); err != nil {
			response.WriteJSON(w, http.StatusBadRequest,
				response.Error(response.CodeInvalidParam, err.Error()))
			return
		}
		if agentNameExists(def.Name, agents, id) {
			response.WriteJSON(w, http.StatusConflict,
				response.Error(response.CodeNameConflict, fmt.Sprintf("智能体名称「%s」已存在", def.Name)))
			return
		}
		agents[i] = def
		found = true
		break
	}
	if !found {
		response.WriteJSON(w, http.StatusNotFound,
			response.Error(response.CodeNotFound, "智能体不存在"))
		return
	}

	if err := storage.SaveAgents(agents); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "保存智能体定义失败"))
		return
	}

	h.audit("update", id)
	h.notifyAgentsChanged()
	for _, a := range agents {
		if a.ID == id {
			response.WriteJSON(w, http.StatusOK, response.Success(a))
			return
		}
	}
	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{"id": id}))
}

// Delete 删除智能体定义（不影响正在运行的调用，后续调用因定义缺失失败）。
func (h *AgentDefHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := agentIDFromPath(r.URL.Path)
	if id == "" {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "智能体 ID 不能为空"))
		return
	}
	if id == mainAgentID {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "内置主智能体不可删除"))
		return
	}

	agents, err := storage.LoadAgents()
	if err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "读取智能体定义失败"))
		return
	}
	newAgents := make([]storage.AgentDef, 0, len(agents))
	found := false
	for _, a := range agents {
		if a.ID == id {
			found = true
			continue
		}
		newAgents = append(newAgents, a)
	}
	if !found {
		response.WriteJSON(w, http.StatusNotFound,
			response.Error(response.CodeNotFound, "智能体不存在"))
		return
	}

	if err := storage.SaveAgents(newAgents); err != nil {
		response.WriteJSON(w, http.StatusInternalServerError,
			response.Error(response.CodeInternalError, "保存智能体定义失败"))
		return
	}

	h.audit("delete", id)
	h.notifyAgentsChanged()
	response.WriteJSON(w, http.StatusOK, response.Success(map[string]string{
		"message": "智能体已删除",
	}))
}

// validateAgentDef 校验智能体定义字段（3.6 创建/更新校验）。
func validateAgentDef(def *storage.AgentDef) error {
	if def.Name == "" {
		return fmt.Errorf("名称不能为空")
	}
	if utf8.RuneCountInString(def.Name) > maxAgentNameRunes {
		return fmt.Errorf("名称长度不能超过 %d 个字符", maxAgentNameRunes)
	}
	if def.Description == "" {
		return fmt.Errorf("对外描述不能为空")
	}
	if utf8.RuneCountInString(def.Description) > maxAgentDescriptionRunes {
		return fmt.Errorf("对外描述长度不能超过 %d 个字符", maxAgentDescriptionRunes)
	}
	if def.SystemPrompt == "" {
		return fmt.Errorf("系统提示词不能为空")
	}
	if utf8.RuneCountInString(def.SystemPrompt) > maxAgentPromptRunes {
		return fmt.Errorf("系统提示词长度不能超过 %d 个字符", maxAgentPromptRunes)
	}
	return nil
}

// agentNameExists 检查名称是否已被其他智能体占用。
func agentNameExists(name string, agents []storage.AgentDef, excludeID string) bool {
	if name == BuiltinMainAgent().Name && excludeID != mainAgentID {
		return true
	}
	for _, a := range agents {
		if a.ID != excludeID && a.Name == name {
			return true
		}
	}
	return false
}

// agentIDFromPath 从 /api/agents/{id} 提取 ID，拒绝路径穿越字符。
func agentIDFromPath(path string) string {
	id := strings.TrimPrefix(path, "/api/agents/")
	if id == "" || strings.ContainsAny(id, "/\\:%") {
		return ""
	}
	return id
}

// audit 记录智能体定义变更审计日志（操作类型 + id + 时间戳）。
func (h *AgentDefHandler) audit(op, id string) {
	log.Printf("[Audit] agent_def %s id=%s at=%d", op, id, time.Now().UnixMilli())
}

// notifyAgentsChanged 广播智能体定义失效事件：
// 1. Python 侧收到后立即刷新缓存
// 2. 前端 WebSocket 收到后立即刷新智能体列表
func (h *AgentDefHandler) notifyAgentsChanged() {
	cmd := &proto.ServerCommand{
		CommandType: "agents_changed",
		Payload:     "{}",
	}
	if h.sendCmd != nil {
		h.sendCmd(cmd)
	}
	if h.notifyFrontend != nil {
		h.notifyFrontend("agents_changed", "{}")
	}
}

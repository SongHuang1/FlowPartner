package storage

// AgentDef 用户定义的智能体（agents.json 单一真相源）。
// system_prompt 为私有字段：列表接口/事件日志不得携带，仅详情接口与 gRPC 全量拉取返回。
type AgentDef struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// LoadAgents 读取全部用户自定义智能体定义；文件不存在时返回空列表。
func LoadAgents() ([]AgentDef, error) {
	var agents []AgentDef
	err := ReadJSON("agents.json", &agents)
	if err != nil {
		if IsNotFound(err) {
			return []AgentDef{}, nil
		}
		return nil, err
	}
	if agents == nil {
		agents = []AgentDef{}
	}
	return agents, nil
}

// SaveAgents 原子写入全部智能体定义。
func SaveAgents(agents []AgentDef) error {
	if agents == nil {
		agents = []AgentDef{}
	}
	return WriteJSON("agents.json", agents)
}
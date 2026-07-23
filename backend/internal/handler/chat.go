package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	flowcrypto "github.com/songhuang/flowpartner/backend/internal/crypto"
	"github.com/songhuang/flowpartner/backend/internal/keystore"
	"github.com/songhuang/flowpartner/backend/internal/response"
	"github.com/songhuang/flowpartner/backend/internal/storage"
)

type ChatRequest struct {
	Content string `json:"content"`
}

type ChatResponse struct {
	Content string `json:"content"`
}

type AgentChatRequest struct {
	Content             string   `json:"content"`
	ModelName           string   `json:"model_name"`
	SystemPrompt        string   `json:"system_prompt"`
	Temperature         float64  `json:"temperature"`
	ConversationHistory []Message `json:"conversation_history"`
}

type AgentChatResponse struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type LLMRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type LLMResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type ChatHandler struct{}

func (h *ChatHandler) Post(w http.ResponseWriter, r *http.Request) {
	ks := keystore.Instance()
	apiKeyBytes, unlocked := ks.GetKey()
	if !unlocked {
		response.WriteJSON(w, http.StatusForbidden,
			response.Error(response.CodePermissionDenied, "请先解锁 API Key"))
		return
	}
	defer flowcrypto.ZeroBytes(apiKeyBytes)

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, http.StatusBadRequest,
			response.Error(response.CodeInvalidParam, "Invalid JSON body"))
		return
	}

	settings := LoadSettings()

	agentReq := AgentChatRequest{
		Content:             req.Content,
		ModelName:           settings.ModelName,
		SystemPrompt:        settings.SystemPrompt,
		Temperature:         settings.Temperature,
		ConversationHistory: h.loadConversationHistory(),
	}

	agentResp, err := h.callAgent(agentReq)
	if err != nil {
		response.WriteJSON(w, http.StatusBadGateway,
			response.Error(response.CodeInternalError, "Agent 服务不可用"))
		return
	}

	llmReq := LLMRequest{
		Model:       agentResp.Model,
		Messages:    agentResp.Messages,
		Temperature: agentResp.Temperature,
	}

	llmResp, err := h.callLLM(llmReq, string(apiKeyBytes), settings.BaseURL)
	if err != nil {
		safeErr := sanitizeError(err)
		response.WriteJSON(w, http.StatusBadGateway,
			response.Error(response.CodeInternalError, safeErr))
		return
	}

	response.WriteJSON(w, http.StatusOK, response.Success(ChatResponse{
		Content: llmResp,
	}))
}

func (h *ChatHandler) loadConversationHistory() []Message {
	var conv Conversation
	err := storage.ReadJSON("conversations.json", &conv)
	if err != nil {
		return []Message{}
	}
	const maxHistoryLen = 50
	if len(conv.Messages) > maxHistoryLen {
		return conv.Messages[len(conv.Messages)-maxHistoryLen:]
	}
	return conv.Messages
}

func (h *ChatHandler) callAgent(req AgentChatRequest) (*AgentChatResponse, error) {
	agentPort := h.getAgentPort()
	url := fmt.Sprintf("http://127.0.0.1:%d/chat", agentPort)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal agent request: %w", err)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create agent HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if authToken := os.Getenv("AGENT_AUTH_TOKEN"); authToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("agent unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("agent authentication failed")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("agent returned %d", resp.StatusCode)
	}

	var agentResp AgentChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&agentResp); err != nil {
		return nil, fmt.Errorf("agent response parse error: %w", err)
	}
	return &agentResp, nil
}

func (h *ChatHandler) callLLM(req LLMRequest, apiKey, baseURL string) (string, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	url := fmt.Sprintf("%s/chat/completions", baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request: %w", err)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("LLM API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM API returned %d", resp.StatusCode)
	}

	var llmResp LLMResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("LLM response parse error: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return llmResp.Choices[0].Message.Content, nil
}

func (h *ChatHandler) getAgentPort() int {
	dir, err := storage.DataDir()
	if err != nil {
		return 8989
	}
	portFile := filepath.Join(dir, "agent.port")
	data, err := os.ReadFile(portFile)
	if err != nil {
		return 8989
	}
	port, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || port < 1024 || port > 65535 {
		return 8989
	}
	return port
}

// 预编译正则表达式，避免每次调用 sanitizeError 时重复编译
var sensitivePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)bearer\s+\S+`),
	regexp.MustCompile(`(?i)api[_-]key\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)token\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)sk-[a-zA-Z0-9]{20,}`),
	regexp.MustCompile(`(?i)secret\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)password\s*[:=]\s*\S+`),
	regexp.MustCompile(`(?i)authorization[:\s]+\S+`),
}

func sanitizeError(err error) string {
	msg := err.Error()
	for _, re := range sensitivePatterns {
		if re.MatchString(msg) {
			return "API 调用失败（已隐藏敏感信息）"
		}
	}
	return msg
}

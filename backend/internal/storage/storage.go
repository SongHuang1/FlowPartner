package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	ErrNotFound       = errors.New("file not found")
	ErrInvalidFilename = errors.New("invalid filename: must not contain path separators or '..'")
)

var dataDirCache string
var testDataDir string

func DataDir() (string, error) {
	if dataDirCache != "" {
		return dataDirCache, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home dir: %w", err)
	}
	dir := filepath.Join(home, ".flowpartner")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create data dir: %w", err)
	}
	dataDirCache = dir
	return dir, nil
}

func ResetDataDirCache() {
	dataDirCache = testDataDir
}

func SetDataDirForTest(dir string) {
	testDataDir = dir
	dataDirCache = dir
}

func validateFilename(filename string) error {
	if filename == "" {
		return ErrInvalidFilename
	}
	if strings.ContainsAny(filename, `/\`) || strings.Contains(filename, "..") {
		return ErrInvalidFilename
	}
	return nil
}

func ReadJSON(filename string, dest interface{}) error {
	if err := validateFilename(filename); err != nil {
		return err
	}
	dir, err := DataDir()
	if err != nil {
		return err
	}
	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to read %s: %w", filename, err)
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to parse %s: %w", filename, err)
	}
	return nil
}

func WriteJSON(filename string, src interface{}) error {
	if err := validateFilename(filename); err != nil {
		return err
	}
	dir, err := DataDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", filename, err)
	}
	tmpPath := filepath.Join(dir, filename+".tmp")
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp file for %s: %w", filename, err)
	}
	finalPath := filepath.Join(dir, filename)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			log.Printf("Warning: failed to clean up temp file %s: %v", tmpPath, removeErr)
		}
		return fmt.Errorf("failed to rename temp file for %s: %w", filename, err)
	}
	return nil
}

// ToolCallParam LLM 返回的工具调用参数。
type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall 单个工具调用。
type ToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function ToolCallFunction   `json:"function"`
}

// HistoryMessage 历史会话中的单条消息，支持结构化工具上下文。
type HistoryMessage struct {
	Role     string     `json:"role"`
	Content  string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string   `json:"tool_call_id,omitempty"`
	Name      string     `json:"name,omitempty"`
}

// HistoryEntry 历史会话列表条目
type HistoryEntry struct {
	SessionID    string `json:"session_id"`
	Title        string `json:"title"`
	UpdatedAt    int64  `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

func ValidSessionID(sessionID string) bool {
	if len(sessionID) == 0 || len(sessionID) > 128 {
		return false
	}
	for _, r := range sessionID {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' && r != '-' {
			return false
		}
	}
	return true
}

// ValidSnapshotID 校验快照 id 格式（YYYYMMDD-HHMMSS，可带 -N 后缀）。
func ValidSnapshotID(snapshotID string) bool {
	if len(snapshotID) < 15 || len(snapshotID) > 20 {
		return false
	}
	for _, r := range snapshotID {
		if !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	return true
}

func HistoryDir() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	historyDir := filepath.Join(dir, "history")
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create history dir: %w", err)
	}
	return historyDir, nil
}

func ListHistory() ([]HistoryEntry, error) {
	historyDir, err := HistoryDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read history dir: %w", err)
	}
	result := make([]HistoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		sessionID := strings.TrimSuffix(entry.Name(), ".json")
		if !ValidSessionID(sessionID) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		msgs, err := ReadHistory(sessionID)
		if err != nil {
			continue
		}
		title := ""
		for _, m := range msgs {
			if m.Role == "user" {
				title = truncateRunes(m.Content, 50)
				break
			}
		}
		result = append(result, HistoryEntry{
			SessionID:    sessionID,
			Title:        title,
			UpdatedAt:    info.ModTime().UnixMilli(),
			MessageCount: len(msgs),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].UpdatedAt > result[j].UpdatedAt })
	return result, nil
}

// ReadHistory 读取历史会话的全部消息。
// 兼容两种格式：旧"成对 [user,assistant] 数组"与新"单条消息对象"（JSONL）。
func ReadHistory(sessionID string) ([]HistoryMessage, error) {
	if !ValidSessionID(sessionID) {
		return nil, ErrInvalidFilename
	}
	historyDir, err := HistoryDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(historyDir, sessionID+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read history %s: %w", sessionID, err)
	}

	result := make([]HistoryMessage, 0)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 先尝试旧格式：成对 [user,assistant] 数组
		var pair []HistoryMessage
		if err := json.Unmarshal([]byte(line), &pair); err == nil && len(pair) > 0 {
			valid := true
			for _, m := range pair {
				if m.Role != "user" && m.Role != "assistant" {
					valid = false
					break
				}
			}
			if valid {
				result = append(result, pair...)
				continue
			}
		}

		// 再尝试新格式：单条消息对象
		var msg HistoryMessage
		if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Role != "" {
			result = append(result, msg)
			continue
		}

		// 两种格式都失败，跳过损坏行
		log.Printf("[Storage] Skipping malformed history line in %s", sessionID)
	}
	return result, nil
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

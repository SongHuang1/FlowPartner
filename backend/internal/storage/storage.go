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
	// ErrNotFound 表示文件不存在
	ErrNotFound = errors.New("file not found")
	// ErrInvalidFilename 表示文件名包含非法字符（路径遍历风险）
	ErrInvalidFilename = errors.New("invalid filename: must not contain path separators or '..'")
)

// dataDirCache 缓存 DataDir 结果，避免重复 syscall
var dataDirCache string

// testDataDir 测试期间指定的数据目录（仅测试使用），ResetDataDirCache 后仍生效
var testDataDir string

// DataDir 返回用户数据目录路径，若不存在则创建。结果缓存，首次调用后后续直接返回缓存值。
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

// ResetDataDirCache 重置缓存（仅测试使用）。若已通过 SetDataDirForTest 指定目录，重置后回到该目录。
func ResetDataDirCache() {
	dataDirCache = testDataDir
}

// SetDataDirForTest 指定数据目录，避免测试污染真实用户目录（仅测试使用）
func SetDataDirForTest(dir string) {
	testDataDir = dir
	dataDirCache = dir
}

// validateFilename 校验文件名安全性，防止路径遍历
func validateFilename(filename string) error {
	if filename == "" {
		return ErrInvalidFilename
	}
	if strings.ContainsAny(filename, `/\`) || strings.Contains(filename, "..") {
		return ErrInvalidFilename
	}
	return nil
}

// ReadJSON 读取 JSON 文件并反序列化到 dest（dest 必须为指针）
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

// WriteJSON 将数据序列化为 JSON 并原子写入文件（temp file + rename），权限 0600
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

// HistoryMessage 历史会话中的单条消息（由 Python Agent 写入，仅含 role/content）
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// HistoryEntry 历史会话列表条目
type HistoryEntry struct {
	SessionID    string `json:"session_id"`
	Title        string `json:"title"`
	UpdatedAt    int64  `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

// ValidSessionID 校验会话 ID 是否安全（仅允许字母数字、下划线、连字符，防止路径遍历）
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

// HistoryDir 返回历史记录目录路径，若不存在则创建
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

// ListHistory 列出全部历史会话，按更新时间倒序排列
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

// ReadHistory 读取历史会话的全部消息（JSONL：每行一个 [user, assistant] 数组）
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
		var pair []HistoryMessage
		if err := json.Unmarshal([]byte(line), &pair); err != nil {
			// 跳过损坏行，保证部分损坏不影响整体读取
			continue
		}
		result = append(result, pair...)
	}
	return result, nil
}

// truncateRunes 按字符数截断字符串（避免按字节截断破坏 UTF-8）
func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

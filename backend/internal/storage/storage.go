package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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

// ResetDataDirCache 重置缓存（仅测试使用）
func ResetDataDirCache() {
	dataDirCache = ""
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

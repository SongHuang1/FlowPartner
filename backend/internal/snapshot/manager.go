package snapshot

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// 触发间隔常量
const (
	debounceInterval = 60 * time.Second
	tickerInterval   = 15 * time.Minute
)

// Status 快照状态（通过 WebSocket snapshot_status 事件下发给前端）。
// phase: idle（监听中）/ snapshotting（快照中）/ error。
type Status struct {
	Phase          string     `json:"phase"`
	LastAt         *time.Time `json:"last_at,omitempty"`
	Count          int        `json:"count"`
	SizeBytes      int64      `json:"size_bytes"`
	SkippedFiles   int        `json:"skipped_files"`
	Queued         bool       `json:"queued,omitempty"`
	Error          string     `json:"error,omitempty"`
	LastSnapshotID string     `json:"last_snapshot_id,omitempty"`
}

// StatusFunc 状态回调（由 WebSocket handler 注入，广播到前端）。
type StatusFunc func(Status)

// Message 面向用户的快照消息（还原结果、排队提示等）。
type Message struct {
	Type string `json:"type"` // info | warning | error
	Text string `json:"text"`
}

// MessageFunc 消息回调（由 WebSocket handler 注入，广播到前端）。
type MessageFunc func(Message)

// Manager 快照管理器：文件监听 + 三路触发（防抖/周期/锁屏）+ 手动排队 + 并发守卫。
type Manager struct {
	mu             sync.Mutex
	opMu           sync.Mutex // 串行化快照与清理操作（防止启动清理删除进行中的快照）
	statusFunc     StatusFunc
	messageFunc    MessageFunc
	workingDir     string
	snapshotDir    string
	includeSecrets bool
	enabled        bool
	projectID      string

	watcher  *fsnotify.Watcher
	watchCtx context.CancelFunc

	debounceTimer *time.Timer
	ticker        *time.Ticker

	snapshotting  bool
	rearmAfter    bool
	pendingManual bool

	status Status
}

// NewManager 创建快照管理器。
func NewManager(statusFunc StatusFunc, messageFunc MessageFunc) *Manager {
	return &Manager{statusFunc: statusFunc, messageFunc: messageFunc}
}

// RestoreAsync 异步执行一键还原（WebSocket restore 指令入口）。
// 与快照/清理串行执行；结果通过消息回调推送，并刷新状态统计。
func (m *Manager) RestoreAsync(snapshotID string, deleteExtras bool) {
	go func() {
		m.opMu.Lock()
		defer m.opMu.Unlock()

		m.mu.Lock()
		workingDir, snapshotDir, projectID := m.workingDir, m.snapshotDir, m.projectID
		includeSecrets := m.includeSecrets
		m.mu.Unlock()

		if workingDir == "" || snapshotDir == "" || projectID == "" {
			m.sendMessage("error", "快照未启用，无法还原")
			return
		}
		result, err := Restore(context.Background(), RestoreOptions{
			WorkingDir:     workingDir,
			SnapshotDir:    snapshotDir,
			ProjectID:      projectID,
			SnapshotID:     snapshotID,
			DeleteExtras:   deleteExtras,
			IncludeSecrets: includeSecrets,
		})
		if err != nil {
			m.sendMessage("error", "还原失败："+err.Error())
			return
		}
		text := fmt.Sprintf("还原完成：写回 %d 个文件", result.RestoredFiles)
		if len(result.DeletedFiles) > 0 {
			text += fmt.Sprintf("，删除 %d 个多余文件", len(result.DeletedFiles))
		}
		if result.PreSnapshotID != "" {
			text += fmt.Sprintf("（还原前已自动快照：%s）", result.PreSnapshotID)
		}
		if len(result.SymlinkFailures) > 0 {
			m.sendMessage("warning", text+fmt.Sprintf("；%d 个符号链接重建失败已跳过", len(result.SymlinkFailures)))
		} else {
			m.sendMessage("info", text)
		}
		m.refreshStats()
	}()
}

// sendMessage 推送用户可见消息（锁外调用，回调异步执行）。
func (m *Manager) sendMessage(msgType, text string) {
	if m.messageFunc == nil {
		return
	}
	m.messageFunc(Message{Type: msgType, Text: text})
}

// SnapshotDir 返回储存目录。
func (m *Manager) SnapshotDir() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotDir
}

// ProjectID 返回当前项目标识。
func (m *Manager) ProjectID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.projectID
}

// Enabled 返回是否已启用。
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.enabled
}

// IncludeSecrets 返回当前是否包含敏感文件。
func (m *Manager) IncludeSecrets() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.includeSecrets
}

// Configure 应用配置。工作区根与储存目录任一变更为均整体重建监听。
// enabled=false 时停止监听与定时器，保留状态展示。
// 返回错误时不会启用（工作区根不存在等），并置 phase=error。
func (m *Manager) Configure(workingDir, snapshotDir string, enabled, includeSecrets bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.includeSecrets = includeSecrets

	// 停掉旧监听与定时器（幂等）。
	m.stopWatchLocked()
	m.stopTimersLocked()

	m.workingDir = filepath.Clean(workingDir)
	m.snapshotDir = filepath.Clean(snapshotDir)
	m.enabled = false
	m.snapshotting = false
	m.rearmAfter = false
	m.pendingManual = false
	m.status.Queued = false
	m.status.Error = ""

	normalized, err := NormalizeRoot(m.workingDir)
	if err != nil {
		m.status.Phase = "error"
		m.status.Error = "无法解析工作区根: " + err.Error()
		m.pushStatusLocked()
		return fmt.Errorf("解析工作区根失败: %w", err)
	}
	m.projectID = ProjectID(normalized)

	if !enabled || m.snapshotDir == "" {
		m.status.Phase = "idle"
		m.pushStatusLocked()
		return nil
	}

	if err := ValidateNoNesting(m.workingDir, m.snapshotDir); err != nil {
		m.status.Phase = "error"
		m.status.Error = err.Error()
		m.pushStatusLocked()
		return err
	}

	if fi, err := os.Stat(m.workingDir); err != nil || !fi.IsDir() {
		m.status.Phase = "error"
		m.status.Error = "工作区根不存在或不是文件夹: " + m.workingDir
		m.pushStatusLocked()
		return fmt.Errorf("工作区根不存在: %s", m.workingDir)
	}

	projectDir := filepath.Join(m.snapshotDir, m.projectID)
	if err := os.MkdirAll(projectDir, 0o700); err != nil {
		m.status.Phase = "error"
		m.status.Error = "无法创建快照目录: " + err.Error()
		m.pushStatusLocked()
		return fmt.Errorf("创建快照目录失败: %w", err)
	}

	m.enabled = true
	m.status.Phase = "idle"

	// 启动文件监听（事件驱动，非轮询）。
	ctx, cancel := context.WithCancel(context.Background())
	m.watchCtx = cancel
	if err := m.startWatchLocked(ctx); err != nil {
		m.enabled = false
		m.status.Phase = "error"
		m.status.Error = "无法启动文件监听: " + err.Error()
		m.pushStatusLocked()
		cancel()
		return err
	}

	m.ticker = time.NewTicker(tickerInterval)
	go m.tickerLoop(ctx, m.ticker)
	go m.cleanupAndRefresh()
	m.pushStatusLocked()
	return nil
}

// Close 停止监听与定时器。
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopWatchLocked()
	m.stopTimersLocked()
	m.enabled = false
}

func (m *Manager) TriggerManual() {
	m.trigger(ReasonManual)
}

// TriggerLock 系统锁屏 flush（§2.4）。
func (m *Manager) TriggerLock() {
	m.trigger(ReasonLock)
}

// trigger 汇聚三路触发 + 手动，带并发守卫。
func (m *Manager) trigger(reason Reason) {
	m.mu.Lock()
	if !m.enabled || m.workingDir == "" {
		m.mu.Unlock()
		return
	}
	if m.snapshotting {
		if reason == ReasonManual {
			m.pendingManual = true
			m.status.Queued = true
			log.Printf("[snapshot] 快照进行中，手动快照已排队")
			m.pushStatusLocked()
		} else {
			m.rearmAfter = true
			log.Printf("[snapshot] 快照进行中，触发 %s 已跳过，结束后补一次", reason)
		}
		m.mu.Unlock()
		return
	}
	m.snapshotting = true
	m.mu.Unlock()
	go m.runSnapshot(reason)
}

func (m *Manager) runSnapshot(reason Reason) {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	m.mu.Lock()
	workingDir, snapshotDir, projectID := m.workingDir, m.snapshotDir, m.projectID
	includeSecrets := m.includeSecrets
	m.mu.Unlock()

	m.setStatus(func(s *Status) {
		s.Phase = "snapshotting"
		s.Error = ""
	})
	log.Printf("[snapshot] 开始快照 reason=%s", reason)

	manifest, err := Capture(context.Background(), workingDir, snapshotDir, projectID, reason, includeSecrets)
	if err != nil {
		m.setStatus(func(s *Status) {
			s.Phase = "error"
			s.Error = "快照失败: " + err.Error()
		})
		log.Printf("[snapshot] 快照失败: %v", err)
		m.finishSnapshot(reason, snapshotDir, projectID)
		return
	}
	if manifest != nil {
		m.setStatus(func(s *Status) {
			s.LastAt = &manifest.CreatedAt
			s.LastSnapshotID = manifest.SnapshotID
			s.SkippedFiles = len(manifest.SkippedFiles)
		})
	}

	m.finishSnapshot(reason, snapshotDir, projectID)
}

// finishSnapshot 快照结束：执行保留策略清理、重算统计、处理补触发与手动排队。
func (m *Manager) finishSnapshot(reason Reason, snapshotDir, projectID string) {
	projectDir := filepath.Join(snapshotDir, projectID)
	if deleted, err := Cleanup(projectDir, time.Now()); err != nil {
		log.Printf("[snapshot] 清理失败: %v", err)
	} else if len(deleted) > 0 {
		log.Printf("[snapshot] 清理完成，删除 %d 个快照", len(deleted))
	}
	count, size := countAndSize(projectDir)

	m.mu.Lock()
	if m.status.Phase != "error" {
		// 成功路径：恢复空闲态（错误路径已在 runSnapshot 中置为 error，保持展示）
		m.status.Phase = "idle"
	}
	m.snapshotting = false
	m.status.Count = count
	m.status.SizeBytes = size
	m.pushStatusLocked()

	if m.pendingManual {
		m.pendingManual = false
		m.status.Queued = false
		m.snapshotting = true
		m.mu.Unlock()
		go m.runSnapshot(ReasonManual)
		return
	}
	if m.rearmAfter {
		m.rearmAfter = false
		m.snapshotting = true
		m.mu.Unlock()
		go m.runSnapshot(ReasonDebounce)
		return
	}
	m.mu.Unlock()

	log.Printf("[snapshot] 触发 %s 结束", reason)
}

// setStatus 在锁内修改状态并推送副本。
func (m *Manager) setStatus(mutate func(*Status)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mutate(&m.status)
	m.pushStatusLocked()
}

// pushStatusLocked 推送当前状态（调用方须持有 m.mu）。
func (m *Manager) pushStatusLocked() {
	if m.statusFunc == nil {
		return
	}
	status := m.status
	// 锁外推送，避免阻塞快照流程。
	go m.statusFunc(status)
}

// startWatchLocked 递归监听工作区根（调用方须持有 m.mu）。
func (m *Manager) startWatchLocked(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建文件监听器失败: %w", err)
	}
	m.watcher = watcher
	go m.watchLoop(ctx, watcher)

	if err := m.watchDir(m.workingDir, false); err != nil {
		watcher.Close()
		return err
	}
	return nil
}

// watchDir 监听单个目录并递归其子目录（排除目录跳过）。
func (m *Manager) watchDir(dir string, isNested bool) error {
	ex := NewExcluder(m.includeSecrets, filepath.Join(m.snapshotDir, m.projectID))
	if isNested && ex.IsExcludedDir(dir) {
		return nil
	}
	if err := m.watcher.Add(dir); err != nil {
		return fmt.Errorf("监听目录 %s 失败: %w", dir, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if ex.IsExcludedDir(sub) {
			continue
		}
		if err := m.watchDir(sub, true); err != nil {
			log.Printf("[snapshot] 监听子目录 %s 失败: %v", sub, err)
		}
	}
	return nil
}

func (m *Manager) watchLoop(ctx context.Context, watcher *fsnotify.Watcher) {
	for {
		select {
		case <-ctx.Done():
			watcher.Close()
			return
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Printf("[snapshot] 文件监听错误: %v", err)
		case evt, ok := <-watcher.Events:
			if !ok {
				return
			}
			m.handleWatchEvent(watcher, evt)
		}
	}
}

// handleWatchEvent 处理单个监听事件。
func (m *Manager) handleWatchEvent(watcher *fsnotify.Watcher, evt fsnotify.Event) {
	// 排除目录内的事件忽略（如 node_modules 高频变更）。
	ex := NewExcluder(m.includeSecrets, filepath.Join(m.snapshotDir, m.projectID))
	for _, dir := range parentChain(evt.Name) {
		if dir == "" {
			break
		}
		if ex.IsExcludedDir(dir) {
			return
		}
	}

	if evt.Op&(fsnotify.Create) != 0 {
		if fi, err := os.Stat(evt.Name); err == nil && fi.IsDir() {
			if !ex.IsExcludedDir(evt.Name) {
				if err := watcher.Add(evt.Name); err != nil {
					log.Printf("[snapshot] 动态补 watch 失败 %s: %v", evt.Name, err)
				}
			}
		}
	}
	if evt.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		watcher.Remove(evt.Name)
	}

	m.mu.Lock()
	if !m.enabled {
		m.mu.Unlock()
		return
	}
	if m.debounceTimer == nil {
		m.debounceTimer = time.AfterFunc(debounceInterval, func() {
			m.mu.Lock()
			m.debounceTimer = nil
			m.mu.Unlock()
			m.trigger(ReasonDebounce)
		})
	} else {
		m.debounceTimer.Reset(debounceInterval)
	}
	m.mu.Unlock()
}

// parentChain 返回 path 自身及其逐级父目录。
func parentChain(path string) []string {
	var chain []string
	for {
		chain = append(chain, path)
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return chain
}

// tickerLoop 周期兜底。
func (m *Manager) tickerLoop(ctx context.Context, ticker *time.Ticker) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.trigger(ReasonTicker)
		}
	}
}

// cleanupAndRefresh 启动时后台清理 + 刷新状态统计。
// opMu 与快照互斥：进行中的快照结束时会自行清理，这里直接等待并串行执行。
func (m *Manager) cleanupAndRefresh() {
	m.opMu.Lock()
	defer m.opMu.Unlock()

	m.mu.Lock()
	snapshotDir, projectID := m.snapshotDir, m.projectID
	m.mu.Unlock()
	projectDir := filepath.Join(snapshotDir, projectID)
	if deleted, err := Cleanup(projectDir, time.Now()); err != nil {
		log.Printf("[snapshot] 启动清理失败: %v", err)
	} else if len(deleted) > 0 {
		log.Printf("[snapshot] 启动清理完成，删除 %d 个快照", len(deleted))
	}
	m.refreshStats()
}

// refreshStats 重算完整快照数量与总占用并推送状态（仅快照/清理/还原后调用）。
func (m *Manager) refreshStats() {
	m.mu.Lock()
	snapshotDir, projectID := m.snapshotDir, m.projectID
	m.mu.Unlock()
	count, size := countAndSize(filepath.Join(snapshotDir, projectID))
	m.mu.Lock()
	if m.snapshotDir == snapshotDir && m.projectID == projectID {
		m.status.Count = count
		m.status.SizeBytes = size
	}
	m.pushStatusLocked()
	m.mu.Unlock()
}

// stopWatchLocked 停止监听（调用方须持有 m.mu）。
func (m *Manager) stopWatchLocked() {
	if m.watchCtx != nil {
		m.watchCtx()
		m.watchCtx = nil
	}
	if m.watcher != nil {
		m.watcher.Close()
		m.watcher = nil
	}
}

// stopTimersLocked 停止防抖与周期定时器（调用方须持有 m.mu）。
func (m *Manager) stopTimersLocked() {
	if m.debounceTimer != nil {
		m.debounceTimer.Stop()
		m.debounceTimer = nil
	}
	if m.ticker != nil {
		m.ticker.Stop()
		m.ticker = nil
	}
}

// 储存目录不得嵌套在工作区根内，工作区根也不得嵌套在储存目录内。
func ValidateNoNesting(workingDir, snapshotDir string) error {
	wd, err := NormalizeRoot(workingDir)
	if err != nil {
		return err
	}
	sd, err := NormalizeRoot(snapshotDir)
	if err != nil {
		return err
	}
	sep := string(filepath.Separator)
	wdWithSep := wd + sep
	sdWithSep := sd + sep
	switch {
	case wd == sd || strings.HasPrefix(sd, wdWithSep):
		return fmt.Errorf("快照储存目录不能位于工作区根目录之内，否则快照会包含自身并无限增长")
	case strings.HasPrefix(wd, sdWithSep):
		return fmt.Errorf("工作区根目录不能位于快照储存目录之内，否则快照会包含自身并无限增长")
	}
	return nil
}

func countAndSize(projectDir string) (int, int64) {
	infos, err := listSnapshotDirs(projectDir)
	if err != nil {
		return 0, 0
	}
	count := 0
	var size int64
	for _, s := range infos {
		if s.complete {
			count++
			size += s.size
		}
	}
	return count, size
}

func ListSnapshots(snapshotDir, projectID string) ([]Manifest, error) {
	projectDir := filepath.Join(snapshotDir, projectID)
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []Manifest
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(projectDir, e.Name())
		mf, err := LoadManifest(path)
		if err != nil || !mf.Complete {
			continue
		}
		result = append(result, *mf)
	}
	// 时间从新到旧
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].CreatedAt.After(result[i].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result, nil
}

// ListSnapshots 返回该项目的完整快照列表（按时间从新到旧）。
func (m *Manager) ListSnapshots() ([]Manifest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return ListSnapshots(m.snapshotDir, m.projectID)
}

// SnapshotDetail 单个快照详情（含受保护文件清单，供还原确认框展示）。
type SnapshotDetail struct {
	Manifest       Manifest         `json:"manifest"`
	ProtectedFiles []ProtectedEntry `json:"protected_files"`
}

// SnapshotDetail 返回快照详情；仅接受完整快照。
func (m *Manager) GetSnapshotDetail(snapshotID string) (*SnapshotDetail, error) {
	path := filepath.Join(m.snapshotDir, m.projectID, snapshotID)
	mf, err := LoadManifest(path)
	if err != nil {
		return nil, fmt.Errorf("快照不存在: %v", err)
	}
	if !mf.Complete {
		return nil, fmt.Errorf("该快照未完成，不可用")
	}
	protected, err := ProtectedFiles(m.workingDir, m.includeSecrets)
	if err != nil {
		log.Printf("[snapshot] 计算受保护文件失败: %v", err)
	}
	return &SnapshotDetail{Manifest: *mf, ProtectedFiles: protected}, nil
}

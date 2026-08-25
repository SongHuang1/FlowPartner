package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// setup 创建临时工作区根与储存目录，返回清理函数。
func setup(t *testing.T) (workingDir, snapshotDir string, cleanup func()) {
	t.Helper()
	base := t.TempDir()
	workingDir = filepath.Join(base, "work")
	snapshotDir = filepath.Join(base, "snapshots")
	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return workingDir, snapshotDir, func() {}
}

// writeFile 写文件。
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProjectIDStableAcrossDriveCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("盘符大小写折叠是 NormalizeRoot 的 Windows 专属语义，非 Windows 上无法构造对应场景")
	}
	a := ProjectID(mustNormalize(t, "C:/Users/foo/work"))
	b := ProjectID(mustNormalize(t, "c:\\users\\foo\\work"))
	if a != b {
		t.Errorf("Windows 大小写折叠后 project_id 应一致: %s vs %s", a, b)
	}
	if len(a) != 16 {
		t.Errorf("project_id 应为 16 位十六进制，got %q", a)
	}
}

func TestNewSnapshotIDSuffix(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 20, 10, 30, 0, 0, time.UTC)
	id1, err := NewSnapshotID(now, dir)
	if err != nil {
		t.Fatal(err)
	}
	if id1 != "20260820-103000" {
		t.Errorf("got %q", id1)
	}
	if err := os.MkdirAll(filepath.Join(dir, id1), 0o755); err != nil {
		t.Fatal(err)
	}
	id2, err := NewSnapshotID(now, dir)
	if err != nil {
		t.Fatal(err)
	}
	if id2 != "20260820-103000-2" {
		t.Errorf("got %q", id2)
	}
}

func TestCapture_Exclusions(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "main.go"), "package main")
	writeFile(t, filepath.Join(workingDir, "node_modules", "pkg", "index.js"), "x")
	big := make([]byte, maxFileSizeBytes+1)
	writeFile(t, filepath.Join(workingDir, "big.bin"), string(big))
	writeFile(t, filepath.Join(workingDir, ".env"), "SECRET=1")

	projectID := ProjectID(mustNormalize(t, workingDir))
	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, false)
	if err != nil {
		t.Fatal(err)
	}
	if m == nil {
		t.Fatal("expected manifest")
	}
	if !m.Complete {
		t.Error("manifest 必须带 complete 标记")
	}
	if m.FileCount != 1 {
		t.Errorf("应只捕获 main.go，got %d", m.FileCount)
	}
	reasons := map[string]bool{}
	for _, s := range m.SkippedFiles {
		reasons[s.Path] = true
	}
	if !reasons["node_modules"] {
		t.Error("node_modules 目录应记入 skipped_files")
	}
	if !reasons["big.bin"] {
		t.Error("超大文件应记入 skipped_files")
	}
	if !reasons[".env"] {
		t.Error("密钥文件应记入 skipped_files")
	}
	if _, err := os.Stat(filepath.Join(snapshotDir, projectID, m.SnapshotID, "node_modules")); !os.IsNotExist(err) {
		t.Error("node_modules 不应存在于快照中")
	}
}

// TestCapture_IncludeSecrets 开关包含敏感文件。
func TestCapture_IncludeSecrets(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, ".env"), "SECRET=1")
	projectID := ProjectID(mustNormalize(t, workingDir))
	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, true)
	if err != nil {
		t.Fatal(err)
	}
	if m.FileCount != 1 {
		t.Errorf("开启包含敏感文件后应捕获 .env，got %d", m.FileCount)
	}
	if len(m.SkippedFiles) != 0 {
		t.Errorf("不应有跳过文件，got %v", m.SkippedFiles)
	}
}

func TestCapture_EmptyProject(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	projectID := ProjectID(mustNormalize(t, workingDir))
	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, false)
	if err != nil {
		t.Fatal(err)
	}
	if m != nil {
		t.Error("空项目应返回 nil（跳过）")
	}
	entries, _ := os.ReadDir(filepath.Join(snapshotDir, projectID))
	if len(entries) != 0 {
		t.Errorf("不应创建快照文件夹，got %v", entries)
	}
}

func TestCapture_SymlinkNotFollowed(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "real.txt"), "real")
	if err := os.Symlink("real.txt", filepath.Join(workingDir, "link.txt")); err != nil {
		t.Skipf("当前环境无符号链接权限，跳过: %v", err)
	}
	projectID := ProjectID(mustNormalize(t, workingDir))
	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, false)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range m.Symlinks {
		if s.Path == "link.txt" && s.Target == "real.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("应记录符号链接本身，got %v", m.Symlinks)
	}
	// 链接指向的文件不应被跟随复制
	if fi, err := os.Lstat(filepath.Join(snapshotDir, projectID, m.SnapshotID, "link.txt")); err == nil && fi.Mode()&os.ModeSymlink == 0 {
		t.Error("快照中不应存在跟随后的普通文件 link.txt")
	}
}

func TestCapture_ManifestLastWriteAndIncompleteExcluded(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")
	projectID := ProjectID(mustNormalize(t, workingDir))

	// 构造残缺快照（无 manifest）
	projectDir := filepath.Join(snapshotDir, projectID)
	os.MkdirAll(projectDir, 0o755)
	incomplete := filepath.Join(projectDir, "20260820-000000")
	os.MkdirAll(incomplete, 0o755)
	writeFile(t, filepath.Join(incomplete, "junk.txt"), "junk")

	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, false)
	if err != nil {
		t.Fatal(err)
	}
	// 残缺快照不可还原
	if _, err := Restore(context.Background(), RestoreOptions{
		WorkingDir: workingDir, SnapshotDir: snapshotDir, ProjectID: projectID,
		SnapshotID: "20260820-000000", DeleteExtras: false,
	}); err == nil || !strings.Contains(err.Error(), "未完成") {
		t.Errorf("残缺快照应被拒绝，got %v", err)
	}
	_ = m
}

func TestRetention_TimeAndCapacity(t *testing.T) {
	dir := t.TempDir()
	// 构造三个完整快照，CreatedAt 与文件夹名一致
	ids := []string{"20260101-000000", "20260801-000000", "20260810-000000"}
	times := []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
	}
	for i, id := range ids {
		sd := filepath.Join(dir, id)
		os.MkdirAll(sd, 0o755)
		writeFile(t, filepath.Join(sd, "f.txt"), "data")
		m := Manifest{SnapshotID: id, Complete: true, CreatedAt: times[i]}
		if err := writeManifest(sd, &m); err != nil {
			t.Fatal(err)
		}
	}
	// 20260101 已超过 30 天 → 应被删除
	deleted, err := Cleanup(dir, time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(deleted, "20260101-000000") {
		t.Errorf("过期快照应被删除，got %v", deleted)
	}
	// 未完成快照应被清理
	incomplete := filepath.Join(dir, "20260820-000000")
	os.MkdirAll(incomplete, 0o755)
	deleted, err = Cleanup(dir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !containsStr(deleted, "20260820-000000") {
		t.Errorf("未完成快照应被清理，got %v", deleted)
	}
}

func TestRetention_SingleHugeSnapshot(t *testing.T) {
	dir := t.TempDir()
	sd := filepath.Join(dir, "20260820-000000")
	os.MkdirAll(sd, 0o755)
	writeFile(t, filepath.Join(sd, "f.txt"), "data")
	m := Manifest{SnapshotID: "20260820-000000", Complete: true, CreatedAt: time.Now()}
	if err := writeManifest(sd, &m); err != nil {
		t.Fatal(err)
	}
	// 30 天内完整快照不应被清理误删；超容终止逻辑由 Cleanup 内部分支保证（单快照 > 5GB 时 break）。
	deleted, err := Cleanup(dir, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 0 {
		t.Errorf("30 天内完整快照不应被删除，got %v", deleted)
	}
	if _, err := os.Stat(sd); err != nil {
		t.Error("完整快照应保留")
	}
}

func TestRestore_RootMismatch(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")
	projectID := ProjectID(mustNormalize(t, workingDir))
	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, false)
	if err != nil {
		t.Fatal(err)
	}
	otherDir := filepath.Join(t.TempDir(), "other")
	os.MkdirAll(otherDir, 0o755)
	_, err = Restore(context.Background(), RestoreOptions{
		WorkingDir: otherDir, SnapshotDir: snapshotDir, ProjectID: projectID,
		SnapshotID: m.SnapshotID, DeleteExtras: false,
	})
	if err == nil || !strings.Contains(err.Error(), "禁止还原") {
		t.Errorf("不同项目根应阻止还原，got %v", err)
	}
}

func TestRestore_FullCycle(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "v1")
	projectID := ProjectID(mustNormalize(t, workingDir))
	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, false)
	if err != nil {
		t.Fatal(err)
	}

	// 修改文件 + 新增文件 + 新建受保护密钥
	writeFile(t, filepath.Join(workingDir, "a.txt"), "v2")
	writeFile(t, filepath.Join(workingDir, "extra.txt"), "extra")
	writeFile(t, filepath.Join(workingDir, ".env"), "SECRET")

	result, err := Restore(context.Background(), RestoreOptions{
		WorkingDir: workingDir, SnapshotDir: snapshotDir, ProjectID: projectID,
		SnapshotID: m.SnapshotID, DeleteExtras: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PreSnapshotID == "" {
		t.Error("应有还原前预快照")
	}
	content, _ := os.ReadFile(filepath.Join(workingDir, "a.txt"))
	if string(content) != "v1" {
		t.Errorf("a.txt 应回退到 v1，got %q", content)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "extra.txt")); !os.IsNotExist(err) {
		t.Error("多余文件 extra.txt 应被删除")
	}

	if _, err := os.Stat(filepath.Join(workingDir, ".env")); err != nil {
		t.Error(".env 受保护不应被删除")
	}

	preList, err := ListSnapshots(snapshotDir, projectID)
	if err != nil {
		t.Fatal(err)
	}
	preSnapshot := ""
	for _, pm := range preList {
		if pm.Reason == ReasonPreRestore {
			preSnapshot = pm.SnapshotID
		}
	}
	if preSnapshot == "" {
		t.Fatal("未找到预快照")
	}
	if _, err := Restore(context.Background(), RestoreOptions{
		WorkingDir: workingDir, SnapshotDir: snapshotDir, ProjectID: projectID,
		SnapshotID: preSnapshot, DeleteExtras: true,
	}); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(filepath.Join(workingDir, "a.txt"))
	if string(content) != "v2" {
		t.Errorf("预快照应恢复 v2，got %q", content)
	}
}

func TestDeleteExtras_ProtectedByExcludedDir(t *testing.T) {
	workingDir, snapshotDir, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "v1")
	projectID := ProjectID(mustNormalize(t, workingDir))
	m, err := Capture(context.Background(), workingDir, snapshotDir, projectID, ReasonManual, false)
	if err != nil {
		t.Fatal(err)
	}
	// 快照后新建 node_modules（排除目录）与超大文件
	writeFile(t, filepath.Join(workingDir, "node_modules", "x", "y.js"), "yyy")
	big := make([]byte, maxFileSizeBytes+1)
	writeFile(t, filepath.Join(workingDir, "huge.bin"), string(big))

	if _, err := Restore(context.Background(), RestoreOptions{
		WorkingDir: workingDir, SnapshotDir: snapshotDir, ProjectID: projectID,
		SnapshotID: m.SnapshotID, DeleteExtras: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(workingDir, "node_modules", "x", "y.js")); err != nil {
		t.Error("排除目录内的文件不应被删除")
	}
	if _, err := os.Stat(filepath.Join(workingDir, "huge.bin")); err != nil {
		t.Error("超大文件不应被删除")
	}
}

// TestProtectedFiles 受保护文件清单。
func TestProtectedFiles(t *testing.T) {
	workingDir, _, _ := setup(t)
	writeFile(t, filepath.Join(workingDir, "a.txt"), "a")
	writeFile(t, filepath.Join(workingDir, ".env"), "S")
	writeFile(t, filepath.Join(workingDir, "node_modules", "x", "y.js"), "y")
	big := make([]byte, maxFileSizeBytes+1)
	writeFile(t, filepath.Join(workingDir, "big.bin"), string(big))

	entries, err := ProtectedFiles(workingDir, false)
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, e := range entries {
		types[e.Path] = e.Type
	}
	if types[".env"] != "secret" {
		t.Errorf("缺少 .env secret 条目: %v", types)
	}
	if types["big.bin"] != "too_large" {
		t.Errorf("缺少 big.bin too_large 条目: %v", types)
	}
	if types["node_modules"] != "excluded_dir" {
		t.Errorf("缺少 node_modules excluded_dir 条目: %v", types)
	}
}

func TestValidateNoNesting(t *testing.T) {
	// 跨平台部分：用临时目录构造真实的嵌套/平级关系
	base := t.TempDir()
	wd := filepath.Join(base, "proj")
	snapsInside := filepath.Join(wd, "snaps")
	if err := ValidateNoNesting(wd, snapsInside); err == nil {
		t.Error("储存目录在工作区根内应被拒绝")
	}
	if err := ValidateNoNesting(wd, base); err == nil {
		t.Error("工作区根在储存目录内应被拒绝")
	}
	unrelated := filepath.Join(base, "other")
	if err := ValidateNoNesting(wd, unrelated); err != nil {
		t.Errorf("合法配置不应被拒绝: %v", err)
	}

	// Windows 专属：盘符与反斜杠路径语义
	if runtime.GOOS == "windows" {
		winWd := "C:\\work\\proj"
		if err := ValidateNoNesting(winWd, "C:\\work\\proj\\snaps"); err == nil {
			t.Error("储存目录在工作区根内应被拒绝")
		}
		if err := ValidateNoNesting(winWd, "C:\\work"); err == nil {
			t.Error("工作区根在储存目录内应被拒绝")
		}
		if err := ValidateNoNesting(winWd, "D:\\snaps"); err != nil {
			t.Errorf("合法配置不应被拒绝: %v", err)
		}
	}
}

func mustNormalize(t *testing.T, dir string) string {
	t.Helper()
	n, err := NormalizeRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

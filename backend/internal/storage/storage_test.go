package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func setTestDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "history"), 0755); err != nil {
		t.Fatalf("failed to create history dir: %v", err)
	}
	SetDataDirForTest(dir)
	t.Cleanup(func() {
		testDataDir = ""
		ResetDataDirCache()
	})
}

func TestReadSubAgentsReturnsEmptyWhenFileMissing(t *testing.T) {
	setTestDir(t)

	runs, err := ReadSubAgents("sess_1")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("expected empty runs, got %d", len(runs))
	}
}

func TestReadSubAgentsParsesRuns(t *testing.T) {
	setTestDir(t)
	historyDir, err := HistoryDir()
	if err != nil {
		t.Fatal(err)
	}
	content := `{"version":1,"session_id":"sess_1","runs":{"span-1":{"span_id":"span-1","agent_name":"翻译官","task":"翻译","status":"done","result":"译文","steps":[{"step_type":"thinking","content":"先理解"}]}}}`
	if err := os.WriteFile(filepath.Join(historyDir, "sess_1.subagents.json"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	runs, err := ReadSubAgents("sess_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	run, ok := runs["span-1"]
	if !ok {
		t.Fatal("expected run span-1 to exist")
	}
	if run.AgentName != "翻译官" || run.Status != "done" || run.Result != "译文" {
		t.Fatalf("unexpected run fields: %+v", run)
	}
	if len(run.Steps) != 1 || run.Steps[0].Content != "先理解" {
		t.Fatalf("unexpected steps: %+v", run.Steps)
	}
}

func TestDeleteHistoryRemovesSubAgentsFile(t *testing.T) {
	setTestDir(t)
	historyDir, err := HistoryDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sess_1.json", "sess_1.subagents.json"} {
		if err := os.WriteFile(filepath.Join(historyDir, name), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	if err := DeleteHistory("sess_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{"sess_1.json", "sess_1.subagents.json"} {
		if _, err := os.Stat(filepath.Join(historyDir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected %s to be removed", name)
		}
	}
}

func TestListHistoryIgnoresSubAgentsFiles(t *testing.T) {
	setTestDir(t)
	historyDir, err := HistoryDir()
	if err != nil {
		t.Fatal(err)
	}
	line := `{"role":"user","content":"你好"}`
	if err := os.WriteFile(filepath.Join(historyDir, "sess_1.json"), []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(historyDir, "sess_1.subagents.json"), []byte(`{"version":1,"runs":{}}`), 0600); err != nil {
		t.Fatal(err)
	}

	entries, err := ListHistory()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 entry, got %d", len(entries))
	}
	if entries[0].SessionID != "sess_1" {
		t.Fatalf("unexpected session id: %s", entries[0].SessionID)
	}
}

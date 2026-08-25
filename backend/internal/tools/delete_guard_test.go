package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyzeDeletion_Blocked(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"rm", `rm file.txt`},
		{"rm absolute", `rm /abs/path`},
		{"rm -rf", `rm -rf dir`},
		{"rmdir", `rmdir dir`},
		{"del windows", `del file.txt`},
		{"erase windows", `erase file.txt`},
		{"rd windows", `rd dir`},
		{"rd /s /q", `rd /s /q dir`},
		{"deltree", `deltree dir`},
		{"Remove-Item", `Remove-Item x`},
		{"Remove-Item alias ri", `ri x`},
		{"find -delete", `find . -name '*.log' -delete`},
		{"sudo rm", `sudo rm x`},
		{"sudo -u root rm", `sudo -u root rm x`},
		{"sudo -E rm", `sudo -E rm x`},
		{"env -i rm", `env -i rm x`},
		{"env -u VAR rm", `env -u PATH rm x`},
		{"nice -n 5 rm", `nice -n 5 rm x`},
		{"time rm", `time rm x`},
		{"busybox rm", `busybox rm x`},
		{"sh -c quoted", `sh -c "rm x"`},
		{"bash -c quoted", `bash -c 'rm -rf /tmp/x'`},
		{"cmd /c quoted", `cmd /c "del x"`},
		{"powershell -c", `powershell -c "Remove-Item x"`},
		{"powershell -Command", `powershell -Command "Remove-Item x"`},
		{"shred", `shred file`},
		{"srm", `srm file`},
		{"rm uppercase", `RM file`},
		{"sudo -u root -- rm", `sudo -u root -- rm x`},
		{"semicolon separator", `echo hi; rm x`},
		{"pipe", `echo x | rm y`},
		{"assignment then rm", `FOO=bar rm x`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := AnalyzeDeletion(tc.cmd)
			if !res.Blocked {
				t.Errorf("expected blocked for %q", tc.cmd)
			}
			if res.Tool == "" {
				t.Errorf("expected tool name for %q", tc.cmd)
			}
		})
	}
}

func TestAnalyzeDeletion_NotBlocked(t *testing.T) {
	cases := []struct {
		name string
		cmd  string
	}{
		{"echo quoted", `echo "rm file"`},
		{"echo single quoted", `echo 'rm file'`},
		{"cat heredoc inline", "cat <<EOF rm -rf EOF"},
		{"cat heredoc multiline", "cat <<EOF\nrm -rf /\nEOF"},
		{"comment", `# rm x`},
		{"assignment", `rm=foo`},
		{"assignment with value", `rm=foo bar`},
		{"rm in middle of word", `echo arm`},
		{"echo with rm token", `echo rm`},
		{"touch", `touch rm`},
		{"mkdir", `mkdir dir`},
		{"find without delete", `find . -name '*.log' -print`},
		{"find quoted delete", `find . -name '-delete' -print`},
		{"cp", `cp a b`},
		{"mv", `mv a b`},
		{"variable expansion in quotes", `echo "$rm"`},
		{"quoted command word", `"echo" hi`},
		{"backtick content", "echo `rm file`"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := AnalyzeDeletion(tc.cmd)
			if res.Blocked {
				t.Errorf("expected NOT blocked for %q, got tool=%s paths=%v", tc.cmd, res.Tool, res.Paths)
			}
		})
	}
}

func TestAnalyzeDeletion_Paths(t *testing.T) {
	res := AnalyzeDeletion(`rm -rf old.log backup/`)
	if !res.Blocked {
		t.Fatal("expected blocked")
	}
	if len(res.Paths) == 0 {
		t.Fatal("expected best-effort paths")
	}
	found := false
	for _, p := range res.Paths {
		if p == "old.log" || p == "backup/" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected paths to include source paths, got %v", res.Paths)
	}
}

func TestBash_DeletionBlocked_NoSideEffect(t *testing.T) {
	executor, dir := makeExecutor(t)
	testFile := filepath.Join(dir, "keep.txt")
	os.WriteFile(testFile, []byte("data"), 0644)

	var cmds []string
	if isWindows() {
		cmds = []string{"del keep.txt", "erase keep.txt", "rd sub"}
	} else {
		cmds = []string{"rm keep.txt", "rmdir sub"}
	}

	for _, cmd := range cmds {
		args := mustArgs(t, map[string]interface{}{"command": cmd})
		result := executor.Execute(context.Background(), "s1", "bash", args)
		if result.Success {
			t.Fatalf("expected failure for command %q", cmd)
		}
		if result.ErrorCode != ErrDeletionBlocked {
			t.Errorf("command %q: expected error code %s, got %s", cmd, ErrDeletionBlocked, result.ErrorCode)
		}
		if !strings.Contains(result.Result, "改用 trash") {
			t.Errorf("command %q: expected guidance to use trash tool, got %q", cmd, result.Result)
		}
		// 文件必须仍然存在
		if _, err := os.Stat(testFile); err != nil {
			t.Fatalf("command %q: file should still exist after interception, err=%v", cmd, err)
		}
	}
}

func TestBash_DeletionBlocked_BeforePathValidation(t *testing.T) {
	// F1/AC1：rm /abs/path 应返回 TOOL_DELETION_BLOCKED 而非 PATH_OUTSIDE_WORKSPACE
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"command": "rm /abs/path"})
	result := executor.Execute(context.Background(), "s1", "bash", args)
	if result.ErrorCode != ErrDeletionBlocked {
		t.Errorf("expected %s, got %s (result=%s)", ErrDeletionBlocked, result.ErrorCode, result.Result)
	}
}

func TestBash_NonDeletionCommandsStillRun(t *testing.T) {
	executor, _ := makeExecutor(t)
	args := mustArgs(t, map[string]interface{}{"command": "echo hello"})
	result := executor.Execute(context.Background(), "s1", "bash", args)
	if !result.Success {
		t.Fatalf("expected success for non-deletion command, got: %s", result.Result)
	}
}

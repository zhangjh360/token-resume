package monitor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRateLimitHintCN(t *testing.T) {
	raw := "API Error: Request rejected (429) · 已达到 5 小时的使用上限。您的限额将在 2026-04-29 03:44:40 重置。"
	hint := parseRateLimitHint(raw)
	if hint == nil {
		t.Fatalf("expected hint, got nil")
	}
	if !hint.IsLimited {
		t.Fatalf("expected limited=true")
	}
	if hint.ResetAt.IsZero() {
		t.Fatalf("expected non-zero reset time")
	}
}

func TestParseRateLimitHintMiss(t *testing.T) {
	raw := "all good no limit here"
	hint := parseRateLimitHint(raw)
	if hint != nil {
		t.Fatalf("expected nil hint")
	}
}

func TestDetectFromClaudeProjectLogs(t *testing.T) {
	dir := t.TempDir()
	projectDir := filepath.Join(dir, "proj")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	logPath := filepath.Join(projectDir, "session.jsonl")
	line := `{"msg":"API Error: Request rejected (429) · 已达到 5 小时的使用上限。您的限额将在 2026-04-29 03:44:40 重置。"}`
	if err := os.WriteFile(logPath, []byte(line), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}
	hint, err := detectFromClaudeProjectLogs(dir)
	if err != nil {
		t.Fatalf("detect error: %v", err)
	}
	if hint == nil || !hint.IsLimited {
		t.Fatalf("expected limited hint from claude project logs")
	}
}

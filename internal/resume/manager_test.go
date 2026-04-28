package resume

import (
	"path/filepath"
	"testing"

	"tokenresume/internal/config"
	"tokenresume/pkg/logger"
)

func TestManagerRemoveSnapshot(t *testing.T) {
	cfg := config.ResumeConfig{
		Strategy:     "restart",
		SnapshotPath: filepath.Join(t.TempDir(), "snapshots.json"),
	}
	m := NewManager(cfg, logger.New("error"))
	if err := m.SaveSnapshot(&TaskSnapshot{PID: 42, CommandLine: "echo hi"}); err != nil {
		t.Fatalf("save snapshot failed: %v", err)
	}
	if got := len(m.ListSnapshots()); got != 1 {
		t.Fatalf("snapshot count mismatch before remove: %d", got)
	}
	if err := m.RemoveSnapshot(42); err != nil {
		t.Fatalf("remove snapshot failed: %v", err)
	}
	if got := len(m.ListSnapshots()); got != 0 {
		t.Fatalf("snapshot count mismatch after remove: %d", got)
	}
}

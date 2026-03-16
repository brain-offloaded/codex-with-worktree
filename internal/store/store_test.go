package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreUpsertAndList(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	row := WorktreeRecord{
		Path:            "/repo--feature-login",
		RepoRoot:        "/repo",
		Branch:          "feat/login-timeout",
		SessionCount:    2,
		LaunchCount:     1,
		LastSeenAt:      &now,
		LastSelectedAt:  &now,
		LastCodexTurnAt: &now,
	}
	if err := s.UpsertWorktree(row); err != nil {
		t.Fatalf("UpsertWorktree returned error: %v", err)
	}

	rows, err := s.ListWorktrees()
	if err != nil {
		t.Fatalf("ListWorktrees returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Branch != "feat/login-timeout" {
		t.Fatalf("unexpected branch: %q", rows[0].Branch)
	}
}

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

func TestStoreTracksDeletedRowsSeparately(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	rows := []WorktreeRecord{
		{Path: "/repo", RepoRoot: "/repo", Branch: "main", IsMain: true, LastSeenAt: &now},
		{Path: "/repo--old", RepoRoot: "/repo", Branch: "feat/old", LastSeenAt: &now},
	}
	for _, row := range rows {
		if err := s.UpsertWorktree(row); err != nil {
			t.Fatalf("UpsertWorktree returned error: %v", err)
		}
	}

	if err := s.MarkMissingWorktreesDeleted("/repo", []string{"/repo"}, now); err != nil {
		t.Fatalf("MarkMissingWorktreesDeleted returned error: %v", err)
	}

	active, err := s.ListActiveWorktrees("/repo")
	if err != nil {
		t.Fatalf("ListActiveWorktrees returned error: %v", err)
	}
	if len(active) != 1 || active[0].Path != "/repo" {
		t.Fatalf("unexpected active rows: %#v", active)
	}

	deleted, err := s.ListDeletedWorktrees("/repo")
	if err != nil {
		t.Fatalf("ListDeletedWorktrees returned error: %v", err)
	}
	if len(deleted) != 1 || deleted[0].Path != "/repo--old" {
		t.Fatalf("unexpected deleted rows: %#v", deleted)
	}
	if deleted[0].DeletedAt == nil {
		t.Fatalf("expected deleted_at to be recorded")
	}
}

func TestStoreRefreshesWorktreeSessionStats(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	first := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	last := first.Add(2 * time.Hour)
	if err := s.UpsertWorktree(WorktreeRecord{Path: "/repo--feature", RepoRoot: "/repo", Branch: "feat/db"}); err != nil {
		t.Fatalf("UpsertWorktree returned error: %v", err)
	}
	if err := s.UpsertSession("sess-1", "/repo--feature", first, last, 3); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}
	if err := s.UpsertSession("sess-2", "/repo--feature", first, last.Add(time.Hour), 2); err != nil {
		t.Fatalf("UpsertSession returned error: %v", err)
	}

	if err := s.RefreshWorktreeSessionStats("/repo", last.Add(time.Hour)); err != nil {
		t.Fatalf("RefreshWorktreeSessionStats returned error: %v", err)
	}

	active, err := s.ListActiveWorktrees("/repo")
	if err != nil {
		t.Fatalf("ListActiveWorktrees returned error: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected one active row, got %d", len(active))
	}
	if active[0].SessionCount != 2 {
		t.Fatalf("expected session_count=2, got %d", active[0].SessionCount)
	}
	if active[0].LastCodexTurnAt == nil || !active[0].LastCodexTurnAt.Equal(last.Add(time.Hour)) {
		t.Fatalf("unexpected last_codex_turn_at: %#v", active[0].LastCodexTurnAt)
	}
}

func TestStoreRecordsEvents(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "index.sqlite")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer s.Close()

	ts := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	if err := s.RecordEvent(EventRecord{
		Timestamp: ts,
		Kind:      "worktree_selected",
		CWD:       "/repo--feature",
		SessionID: "sess-1",
		Payload:   `{"source":"picker"}`,
	}); err != nil {
		t.Fatalf("RecordEvent returned error: %v", err)
	}

	events, err := s.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != "worktree_selected" {
		t.Fatalf("unexpected event kind: %q", events[0].Kind)
	}
}

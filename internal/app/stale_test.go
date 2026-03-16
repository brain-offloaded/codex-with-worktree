package app

import (
	"testing"
	"time"
)

func TestIsStaleCandidate(t *testing.T) {
	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	threshold := 30 * 24 * time.Hour
	old := now.Add(-45 * 24 * time.Hour)

	candidate := WorktreeView{
		Path:            "/repo--old",
		LastSelectedAt:  &old,
		LastCodexTurnAt: &old,
	}
	if !IsStaleCandidate(candidate, now, threshold) {
		t.Fatalf("expected stale candidate")
	}

	candidate.IsMain = true
	if IsStaleCandidate(candidate, now, threshold) {
		t.Fatalf("main worktree must not be stale candidate")
	}
}

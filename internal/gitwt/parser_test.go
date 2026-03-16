package gitwt

import (
	"strings"
	"testing"
)

func TestParsePorcelain(t *testing.T) {
	input := strings.TrimSpace(`
worktree /repo
HEAD abcdef1234567890
branch refs/heads/main

worktree /repo--feature-login
HEAD 1111111111111111
branch refs/heads/feat/login-timeout

worktree /repo--old
HEAD 2222222222222222
detached
locked stale branch
prunable gitdir file points to non-existent location
`)

	items, err := ParsePorcelain(input)
	if err != nil {
		t.Fatalf("ParsePorcelain returned error: %v", err)
	}

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	if got := items[0].Path; got != "/repo" {
		t.Fatalf("unexpected main path: %q", got)
	}
	if !items[0].IsMain {
		t.Fatalf("expected first worktree to be main")
	}
	if got := items[1].Branch; got != "feat/login-timeout" {
		t.Fatalf("unexpected branch: %q", got)
	}
	if items[2].Branch != "" {
		t.Fatalf("expected detached worktree branch to be empty, got %q", items[2].Branch)
	}
	if !items[2].Locked {
		t.Fatalf("expected detached worktree to be locked")
	}
	if !items[2].Prunable {
		t.Fatalf("expected detached worktree to be prunable")
	}
}

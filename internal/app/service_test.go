package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/brain-offloaded/codex-with-worktree/internal/store"
)

func TestSyncReconcilesToDBAndRecordsEvents(t *testing.T) {
	repoRoot, worktreePath := initGitRepoWithWorktree(t)
	codexHome := writeCodexSessionLog(t, worktreePath)

	st, err := store.Open(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	service := &Service{Now: func() time.Time { return now }}

	views, err := service.Sync(repoRoot, st, codexHome)
	if err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if len(views) != 2 {
		t.Fatalf("expected 2 views, got %d", len(views))
	}

	active, err := st.ListActiveWorktrees(repoRoot)
	if err != nil {
		t.Fatalf("ListActiveWorktrees returned error: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active worktrees, got %d", len(active))
	}

	var secondary store.WorktreeRecord
	for _, row := range active {
		if row.Path == worktreePath {
			secondary = row
			break
		}
	}
	if secondary.Path == "" {
		t.Fatalf("expected secondary worktree to be present")
	}
	if secondary.SessionCount != 1 {
		t.Fatalf("expected session_count=1, got %d", secondary.SessionCount)
	}
	if secondary.LastCodexTurnAt == nil {
		t.Fatalf("expected last_codex_turn_at to be populated")
	}

	events, err := st.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected reconcile events to be recorded, got %d", len(events))
	}
}

func TestSyncMarksMissingWorktreesDeleted(t *testing.T) {
	repoRoot, worktreePath := initGitRepoWithWorktree(t)
	codexHome := t.TempDir()

	st, err := store.Open(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	service := &Service{Now: func() time.Time { return now }}

	if _, err := service.Sync(repoRoot, st, codexHome); err != nil {
		t.Fatalf("initial Sync returned error: %v", err)
	}

	runGit(t, repoRoot, "worktree", "remove", worktreePath)

	if _, err := service.Sync(repoRoot, st, codexHome); err != nil {
		t.Fatalf("second Sync returned error: %v", err)
	}

	deleted, err := st.ListDeletedWorktrees(repoRoot)
	if err != nil {
		t.Fatalf("ListDeletedWorktrees returned error: %v", err)
	}
	if len(deleted) != 1 || deleted[0].Path != worktreePath {
		t.Fatalf("unexpected deleted rows: %#v", deleted)
	}
}

func initGitRepoWithWorktree(t *testing.T) (string, string) {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.name", "Codex")
	runGit(t, root, "config", "user.email", "codex@example.com")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-m", "init")

	worktreePath := filepath.Join(t.TempDir(), "repo--feature")
	runGit(t, root, "worktree", "add", "-b", "feat/db", worktreePath)
	return root, worktreePath
}

func writeCodexSessionLog(t *testing.T, cwd string) string {
	t.Helper()

	codexHome := t.TempDir()
	sessionsDir := filepath.Join(codexHome, "sessions", "2026", "03", "17")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := `{"timestamp":"2026-03-16T15:00:00Z","type":"session_meta","payload":{"id":"sess-1","cwd":"` + cwd + `"}}
{"timestamp":"2026-03-16T15:02:00Z","type":"assistant_message","payload":{}}
`
	if err := os.WriteFile(filepath.Join(sessionsDir, "rollout.jsonl"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return codexHome
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

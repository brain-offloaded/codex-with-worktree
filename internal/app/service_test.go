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

func TestSelectWorktreeRecordsSelectionEvent(t *testing.T) {
	repoRoot, worktreePath := initGitRepoWithWorktree(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	now := time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC)
	service := &Service{Now: func() time.Time { return now }}
	if _, err := service.Sync(repoRoot, st, t.TempDir()); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	if err := service.SelectWorktree(st, worktreePath); err != nil {
		t.Fatalf("SelectWorktree returned error: %v", err)
	}

	active, err := st.ListActiveWorktrees(repoRoot)
	if err != nil {
		t.Fatalf("ListActiveWorktrees returned error: %v", err)
	}
	var selected store.WorktreeRecord
	for _, row := range active {
		if row.Path == worktreePath {
			selected = row
			break
		}
	}
	if selected.LastSelectedAt == nil {
		t.Fatalf("expected last_selected_at to be set")
	}

	events, err := st.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Kind == "worktree_selected" && event.CWD == worktreePath {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected worktree_selected event")
	}
}

func TestCleanupRecordsCandidateEvents(t *testing.T) {
	repoRoot, worktreePath := initGitRepoWithWorktree(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := st.UpsertWorktree(store.WorktreeRecord{
		Path:            worktreePath,
		RepoRoot:        repoRoot,
		Branch:          "feat/db",
		LastSelectedAt:  &old,
		LastCodexTurnAt: &old,
	}); err != nil {
		t.Fatalf("UpsertWorktree returned error: %v", err)
	}

	service := &Service{Now: func() time.Time { return time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC) }}
	views := []WorktreeView{{Path: worktreePath, Branch: "feat/db", LastSelectedAt: &old, LastCodexTurnAt: &old}}
	if err := service.Cleanup(st, repoRoot, views, 30, false); err != nil {
		t.Fatalf("Cleanup returned error: %v", err)
	}

	events, err := st.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Kind == "cleanup_candidate" && event.CWD == worktreePath {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cleanup_candidate event")
	}
}

func TestCreateTrackedWorktreeRecordsCreateEvent(t *testing.T) {
	repoRoot, _ := initGitRepoWithWorktree(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	service := &Service{Now: func() time.Time { return time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC) }}
	targetPath, err := service.CreateTrackedWorktree(st, repoRoot, "feature-two", "feat/two", "")
	if err != nil {
		t.Fatalf("CreateTrackedWorktree returned error: %v", err)
	}
	if _, err := service.Sync(repoRoot, st, t.TempDir()); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}

	events, err := st.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Kind == "worktree_created" && event.CWD == targetPath {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected worktree_created event")
	}
}

func TestRemoveTrackedWorktreeMarksDeletedAndRecordsEvent(t *testing.T) {
	repoRoot, worktreePath := initGitRepoWithWorktree(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "index.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer st.Close()

	service := &Service{Now: func() time.Time { return time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC) }}
	if _, err := service.Sync(repoRoot, st, t.TempDir()); err != nil {
		t.Fatalf("Sync returned error: %v", err)
	}
	if err := service.RemoveTrackedWorktree(st, repoRoot, worktreePath, false, "worktree_removed"); err != nil {
		t.Fatalf("RemoveTrackedWorktree returned error: %v", err)
	}

	deleted, err := st.ListDeletedWorktrees(repoRoot)
	if err != nil {
		t.Fatalf("ListDeletedWorktrees returned error: %v", err)
	}
	if len(deleted) != 1 || deleted[0].Path != worktreePath {
		t.Fatalf("unexpected deleted rows: %#v", deleted)
	}

	events, err := st.ListEvents()
	if err != nil {
		t.Fatalf("ListEvents returned error: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Kind == "worktree_removed" && event.CWD == worktreePath {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected worktree_removed event")
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

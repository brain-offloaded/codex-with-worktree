package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type WorktreeRecord struct {
	Path             string
	RepoRoot         string
	Branch           string
	IsMain           bool
	IsLocked         bool
	IsPrunable       bool
	CreatedAt        *time.Time
	LastSelectedAt   *time.Time
	LastCodexTurnAt  *time.Time
	SessionCount     int
	LaunchCount      int
	LastSeenAt       *time.Time
	DeletedAt        *time.Time
	LastReconciledAt *time.Time
}

type EventRecord struct {
	ID        int64
	Timestamp time.Time
	Kind      string
	CWD       string
	SessionID string
	Payload   string
}

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
CREATE TABLE IF NOT EXISTS worktrees (
  path TEXT PRIMARY KEY,
  repo_root TEXT NOT NULL,
  branch TEXT NOT NULL DEFAULT '',
  is_main INTEGER NOT NULL DEFAULT 0,
  is_locked INTEGER NOT NULL DEFAULT 0,
  is_prunable INTEGER NOT NULL DEFAULT 0,
  created_at TEXT,
  last_selected_at TEXT,
  last_codex_turn_at TEXT,
  session_count INTEGER NOT NULL DEFAULT 0,
  launch_count INTEGER NOT NULL DEFAULT 0,
  last_seen_at TEXT,
  deleted_at TEXT,
  last_reconciled_at TEXT
);

CREATE TABLE IF NOT EXISTS sessions (
  session_id TEXT PRIMARY KEY,
  cwd TEXT NOT NULL,
  first_seen_at TEXT,
  last_seen_at TEXT,
  turn_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  ts TEXT NOT NULL,
  kind TEXT NOT NULL,
  cwd TEXT NOT NULL DEFAULT '',
  session_id TEXT NOT NULL DEFAULT '',
  payload_json TEXT NOT NULL DEFAULT '{}'
);
`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}
	for _, stmt := range []string{
		`ALTER TABLE worktrees ADD COLUMN deleted_at TEXT`,
		`ALTER TABLE worktrees ADD COLUMN last_reconciled_at TEXT`,
	} {
		if _, err := s.db.Exec(stmt); err != nil && !isDuplicateColumnError(err) {
			return err
		}
	}
	return nil
}

func (s *Store) UpsertWorktree(row WorktreeRecord) error {
	query := `
INSERT INTO worktrees (
  path, repo_root, branch, is_main, is_locked, is_prunable, created_at,
  last_selected_at, last_codex_turn_at, session_count, launch_count, last_seen_at,
  deleted_at, last_reconciled_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(path) DO UPDATE SET
  repo_root = excluded.repo_root,
  branch = excluded.branch,
  is_main = excluded.is_main,
  is_locked = excluded.is_locked,
  is_prunable = excluded.is_prunable,
  created_at = COALESCE(worktrees.created_at, excluded.created_at),
  last_selected_at = COALESCE(excluded.last_selected_at, worktrees.last_selected_at),
  last_codex_turn_at = COALESCE(excluded.last_codex_turn_at, worktrees.last_codex_turn_at),
  session_count = excluded.session_count,
  launch_count = CASE
    WHEN excluded.launch_count > worktrees.launch_count THEN excluded.launch_count
    ELSE worktrees.launch_count
  END,
  last_seen_at = COALESCE(excluded.last_seen_at, worktrees.last_seen_at),
  deleted_at = excluded.deleted_at,
  last_reconciled_at = COALESCE(excluded.last_reconciled_at, worktrees.last_reconciled_at)
`
	_, err := s.db.Exec(
		query,
		row.Path,
		row.RepoRoot,
		row.Branch,
		boolToInt(row.IsMain),
		boolToInt(row.IsLocked),
		boolToInt(row.IsPrunable),
		toNullableTime(row.CreatedAt),
		toNullableTime(row.LastSelectedAt),
		toNullableTime(row.LastCodexTurnAt),
		row.SessionCount,
		row.LaunchCount,
		toNullableTime(row.LastSeenAt),
		toNullableTime(row.DeletedAt),
		toNullableTime(row.LastReconciledAt),
	)
	return err
}

func (s *Store) ListWorktrees() ([]WorktreeRecord, error) {
	return s.listWorktrees(`
SELECT path, repo_root, branch, is_main, is_locked, is_prunable, created_at,
       last_selected_at, last_codex_turn_at, session_count, launch_count, last_seen_at,
       deleted_at, last_reconciled_at
FROM worktrees
ORDER BY path
`)
}

func (s *Store) ListActiveWorktrees(repoRoot string) ([]WorktreeRecord, error) {
	return s.listWorktrees(`
SELECT path, repo_root, branch, is_main, is_locked, is_prunable, created_at,
       last_selected_at, last_codex_turn_at, session_count, launch_count, last_seen_at,
       deleted_at, last_reconciled_at
FROM worktrees
WHERE repo_root = ? AND deleted_at IS NULL
ORDER BY path
`, repoRoot)
}

func (s *Store) ListDeletedWorktrees(repoRoot string) ([]WorktreeRecord, error) {
	return s.listWorktrees(`
SELECT path, repo_root, branch, is_main, is_locked, is_prunable, created_at,
       last_selected_at, last_codex_turn_at, session_count, launch_count, last_seen_at,
       deleted_at, last_reconciled_at
FROM worktrees
WHERE repo_root = ? AND deleted_at IS NOT NULL
ORDER BY path
`, repoRoot)
}

func (s *Store) RecordSelection(path string, when time.Time) error {
	_, err := s.db.Exec(`
UPDATE worktrees
SET last_selected_at = ?, launch_count = launch_count + 1
WHERE path = ?
`, when.UTC().Format(time.RFC3339), path)
	return err
}

func (s *Store) UpsertSession(sessionID, cwd string, firstSeenAt, lastSeenAt time.Time, turnCount int) error {
	_, err := s.db.Exec(`
INSERT INTO sessions (session_id, cwd, first_seen_at, last_seen_at, turn_count)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(session_id) DO UPDATE SET
  cwd = excluded.cwd,
  first_seen_at = CASE
    WHEN sessions.first_seen_at IS NULL THEN excluded.first_seen_at
    ELSE sessions.first_seen_at
  END,
  last_seen_at = excluded.last_seen_at,
  turn_count = excluded.turn_count
`, sessionID, cwd, firstSeenAt.UTC().Format(time.RFC3339), lastSeenAt.UTC().Format(time.RFC3339), turnCount)
	return err
}

func (s *Store) MarkMissingWorktreesDeleted(repoRoot string, livePaths []string, when time.Time) error {
	query := `
UPDATE worktrees
SET deleted_at = ?, last_reconciled_at = ?
WHERE repo_root = ?
  AND deleted_at IS NULL
`
	args := []any{when.UTC().Format(time.RFC3339), when.UTC().Format(time.RFC3339), repoRoot}
	if len(livePaths) > 0 {
		query += " AND path NOT IN (" + placeholders(len(livePaths)) + ")"
		for _, livePath := range livePaths {
			args = append(args, livePath)
		}
	}
	_, err := s.db.Exec(query, args...)
	return err
}

func (s *Store) MarkWorktreeDeleted(repoRoot, path string, when time.Time) error {
	_, err := s.db.Exec(`
UPDATE worktrees
SET deleted_at = ?, last_reconciled_at = ?
WHERE repo_root = ? AND path = ?
`, when.UTC().Format(time.RFC3339), when.UTC().Format(time.RFC3339), repoRoot, path)
	return err
}

func (s *Store) RefreshWorktreeSessionStats(repoRoot string, reconciledAt time.Time) error {
	_, err := s.db.Exec(`
UPDATE worktrees
SET session_count = COALESCE((
      SELECT COUNT(*)
      FROM sessions
      WHERE sessions.cwd = worktrees.path
    ), 0),
    last_codex_turn_at = (
      SELECT MAX(last_seen_at)
      FROM sessions
      WHERE sessions.cwd = worktrees.path
    ),
    last_reconciled_at = ?
WHERE repo_root = ?
`, reconciledAt.UTC().Format(time.RFC3339), repoRoot)
	return err
}

func (s *Store) RecordEvent(event EventRecord) error {
	if event.Payload == "" {
		event.Payload = "{}"
	}
	_, err := s.db.Exec(`
INSERT INTO events (ts, kind, cwd, session_id, payload_json)
VALUES (?, ?, ?, ?, ?)
`, event.Timestamp.UTC().Format(time.RFC3339), event.Kind, event.CWD, event.SessionID, event.Payload)
	return err
}

func (s *Store) ListEvents() ([]EventRecord, error) {
	rows, err := s.db.Query(`
SELECT id, ts, kind, cwd, session_id, payload_json
FROM events
ORDER BY id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []EventRecord
	for rows.Next() {
		var rec EventRecord
		var ts string
		if err := rows.Scan(&rec.ID, &ts, &rec.Kind, &rec.CWD, &rec.SessionID, &rec.Payload); err != nil {
			return nil, err
		}
		parsed, err := time.Parse(time.RFC3339, ts)
		if err != nil {
			return nil, err
		}
		rec.Timestamp = parsed
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (s *Store) listWorktrees(query string, args ...any) ([]WorktreeRecord, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WorktreeRecord
	for rows.Next() {
		var rec WorktreeRecord
		var isMain, isLocked, isPrunable int
		var createdAt, lastSelectedAt, lastCodexTurnAt, lastSeenAt, deletedAt, lastReconciledAt sql.NullString
		if err := rows.Scan(
			&rec.Path,
			&rec.RepoRoot,
			&rec.Branch,
			&isMain,
			&isLocked,
			&isPrunable,
			&createdAt,
			&lastSelectedAt,
			&lastCodexTurnAt,
			&rec.SessionCount,
			&rec.LaunchCount,
			&lastSeenAt,
			&deletedAt,
			&lastReconciledAt,
		); err != nil {
			return nil, err
		}
		rec.IsMain = isMain == 1
		rec.IsLocked = isLocked == 1
		rec.IsPrunable = isPrunable == 1
		rec.CreatedAt = parseNullableTime(createdAt)
		rec.LastSelectedAt = parseNullableTime(lastSelectedAt)
		rec.LastCodexTurnAt = parseNullableTime(lastCodexTurnAt)
		rec.LastSeenAt = parseNullableTime(lastSeenAt)
		rec.DeletedAt = parseNullableTime(deletedAt)
		rec.LastReconciledAt = parseNullableTime(lastReconciledAt)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func toNullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func parseNullableTime(v sql.NullString) *time.Time {
	if !v.Valid || v.String == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, v.String)
	if err != nil {
		return nil
	}
	return &parsed
}

func placeholders(n int) string {
	values := make([]string, n)
	for i := range values {
		values[i] = "?"
	}
	return strings.Join(values, ",")
}

func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate column name") || strings.Contains(msg, "already exists")
}

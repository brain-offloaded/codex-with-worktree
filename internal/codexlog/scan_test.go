package codexlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanSessionsAggregatesByCWD(t *testing.T) {
	root := t.TempDir()
	sessionsDir := filepath.Join(root, "sessions", "2026", "03", "17")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(sessionsDir, "rollout-1.jsonl")
	content := `{"timestamp":"2026-03-16T15:00:00Z","type":"session_meta","payload":{"id":"sess-1","cwd":"/repo--feature-login"}}
{"timestamp":"2026-03-16T15:01:00Z","type":"user_message","payload":{}}
{"timestamp":"2026-03-16T15:02:00Z","type":"assistant_message","payload":{}}
`
	if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	agg, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	wt, ok := agg.ByCWD["/repo--feature-login"]
	if !ok {
		t.Fatalf("expected aggregate for cwd")
	}
	if wt.SessionCount != 1 {
		t.Fatalf("expected session_count=1, got %d", wt.SessionCount)
	}
	if wt.TurnCount != 2 {
		t.Fatalf("expected turn_count=2, got %d", wt.TurnCount)
	}
	want := time.Date(2026, 3, 16, 15, 2, 0, 0, time.UTC)
	if !wt.LastActivityAt.Equal(want) {
		t.Fatalf("expected last activity %s, got %s", want, wt.LastActivityAt)
	}
}

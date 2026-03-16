package picker

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/brain-offloaded/codex-with-worktree/internal/app"
)

func TestRunSelect(t *testing.T) {
	in := strings.NewReader("1\n")
	out := &bytes.Buffer{}
	p := &Picker{
		In:  in,
		Out: out,
		Now: func() time.Time { return time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC) },
	}

	result, err := p.Run([]app.WorktreeView{{Path: "/repo"}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Action != ActionSelect || result.Index != 0 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunCreate(t *testing.T) {
	in := strings.NewReader("c\nfeature-login\n\n")
	out := &bytes.Buffer{}
	p := &Picker{
		In:  in,
		Out: out,
		Now: func() time.Time { return time.Date(2026, 3, 17, 0, 0, 0, 0, time.UTC) },
	}

	result, err := p.Run([]app.WorktreeView{{Path: "/repo"}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.Action != ActionCreate || result.Name != "feature-login" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

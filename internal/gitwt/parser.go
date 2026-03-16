package gitwt

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type Worktree struct {
	Path           string
	Head           string
	Branch         string
	Detached       bool
	Locked         bool
	LockReason     string
	Prunable       bool
	PrunableReason string
	IsMain         bool
}

func ParsePorcelain(input string) ([]Worktree, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}

	blocks := strings.Split(input, "\n\n")
	items := make([]Worktree, 0, len(blocks))
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		var wt Worktree
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "worktree "):
				wt.Path = strings.TrimSpace(strings.TrimPrefix(line, "worktree "))
			case strings.HasPrefix(line, "HEAD "):
				wt.Head = strings.TrimSpace(strings.TrimPrefix(line, "HEAD "))
			case strings.HasPrefix(line, "branch refs/heads/"):
				wt.Branch = strings.TrimSpace(strings.TrimPrefix(line, "branch refs/heads/"))
			case line == "detached":
				wt.Detached = true
				wt.Branch = ""
			case strings.HasPrefix(line, "locked"):
				wt.Locked = true
				wt.LockReason = strings.TrimSpace(strings.TrimPrefix(line, "locked"))
			case strings.HasPrefix(line, "prunable"):
				wt.Prunable = true
				wt.PrunableReason = strings.TrimSpace(strings.TrimPrefix(line, "prunable"))
			}
		}
		if wt.Path == "" {
			return nil, fmt.Errorf("invalid porcelain block: missing path in %q", block)
		}
		items = append(items, wt)
	}
	if len(items) > 0 {
		mainPath := items[0].Path
		for i := range items {
			items[i].IsMain = items[i].Path == mainPath
		}
	}
	return items, nil
}

func RepoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func List(repoRoot string) ([]Worktree, error) {
	cmd := exec.Command("git", "-C", repoRoot, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	items, err := ParsePorcelain(string(out))
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, errors.New("no worktrees found")
	}
	return items, nil
}

func DefaultCreatePath(repoRoot, name string) string {
	repoName := filepath.Base(repoRoot)
	return filepath.Join(filepath.Dir(repoRoot), repoName+"--"+name)
}

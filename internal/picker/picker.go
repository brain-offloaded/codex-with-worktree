package picker

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/brain-offloaded/codex-with-worktree/internal/app"
)

type Action int

const (
	ActionSelect Action = iota
	ActionCreate
	ActionDelete
	ActionCancel
	ActionRefresh
)

type Result struct {
	Action Action
	Index  int
	Name   string
	Branch string
	Force  bool
}

type Picker struct {
	In  io.Reader
	Out io.Writer
	Now func() time.Time
}

func (p *Picker) Run(items []app.WorktreeView) (Result, error) {
	if p.Now == nil {
		p.Now = time.Now
	}
	reader := bufio.NewReader(p.In)
	for {
		if _, err := fmt.Fprintln(p.Out, "Worktrees:"); err != nil {
			return Result{}, err
		}
		for i, item := range items {
			if _, err := fmt.Fprintf(
				p.Out,
				"  [%d] %s | branch=%s | sessions=%d | selected=%s | codex=%s | state=%s\n",
				i+1,
				item.Path,
				displayOrDash(item.Branch),
				item.SessionCount,
				ageString(p.Now(), item.LastSelectedAt),
				ageString(p.Now(), item.LastCodexTurnAt),
				strings.Join(stateTags(item, p.Now()), ","),
			); err != nil {
				return Result{}, err
			}
		}
		if _, err := fmt.Fprintln(p.Out, "Commands: <number> select, c create, d<number> delete, r refresh, q quit"); err != nil {
			return Result{}, err
		}
		if _, err := fmt.Fprint(p.Out, "> "); err != nil {
			return Result{}, err
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			return Result{}, err
		}
		line = strings.TrimSpace(line)
		switch {
		case line == "q":
			return Result{Action: ActionCancel}, nil
		case line == "r":
			return Result{Action: ActionRefresh}, nil
		case line == "c":
			if _, err := fmt.Fprint(p.Out, "name: "); err != nil {
				return Result{}, err
			}
			name, err := reader.ReadString('\n')
			if err != nil {
				return Result{}, err
			}
			name = strings.TrimSpace(name)
			if _, err := fmt.Fprint(p.Out, "branch (blank = use name): "); err != nil {
				return Result{}, err
			}
			branch, err := reader.ReadString('\n')
			if err != nil {
				return Result{}, err
			}
			return Result{Action: ActionCreate, Name: strings.TrimSpace(name), Branch: strings.TrimSpace(branch)}, nil
		case strings.HasPrefix(line, "d"):
			n, err := strconv.Atoi(strings.TrimPrefix(line, "d"))
			if err != nil || n < 1 || n > len(items) {
				if _, err := fmt.Fprintln(p.Out, "invalid delete selection"); err != nil {
					return Result{}, err
				}
				continue
			}
			if _, err := fmt.Fprint(p.Out, "force delete? [y/N]: "); err != nil {
				return Result{}, err
			}
			confirm, err := reader.ReadString('\n')
			if err != nil {
				return Result{}, err
			}
			return Result{Action: ActionDelete, Index: n - 1, Force: strings.EqualFold(strings.TrimSpace(confirm), "y")}, nil
		default:
			n, err := strconv.Atoi(line)
			if err != nil || n < 1 || n > len(items) {
				if _, err := fmt.Fprintln(p.Out, "invalid selection"); err != nil {
					return Result{}, err
				}
				continue
			}
			return Result{Action: ActionSelect, Index: n - 1}, nil
		}
	}
}

func stateTags(v app.WorktreeView, now time.Time) []string {
	var tags []string
	if v.IsMain {
		tags = append(tags, "main")
	}
	if v.IsLocked {
		tags = append(tags, "locked")
	}
	if v.IsPrunable {
		tags = append(tags, "prunable")
	}
	if app.IsStaleCandidate(v, now, 30*24*time.Hour) {
		tags = append(tags, "stale")
	}
	if len(tags) == 0 {
		tags = append(tags, "active")
	}
	return tags
}

func ageString(now time.Time, ts *time.Time) string {
	if ts == nil {
		return "-"
	}
	d := now.Sub(*ts)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return strconv.Itoa(int(d.Minutes())) + "m"
	}
	if d < 24*time.Hour {
		return strconv.Itoa(int(d.Hours())) + "h"
	}
	return strconv.Itoa(int(d.Hours()/24)) + "d"
}

func displayOrDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}

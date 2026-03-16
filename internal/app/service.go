package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/brain-offloaded/codex-with-worktree/internal/codexlog"
	"github.com/brain-offloaded/codex-with-worktree/internal/gitwt"
	"github.com/brain-offloaded/codex-with-worktree/internal/store"
)

type Service struct {
	Now func() time.Time
}

func NewService() *Service {
	return &Service{Now: time.Now}
}

func DefaultStateDBPath() string {
	if base := os.Getenv("XDG_STATE_HOME"); base != "" {
		return filepath.Join(base, "cwt", "index.sqlite")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "cwt", "index.sqlite")
}

func DefaultCodexHome() string {
	if v := os.Getenv("CODEX_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".codex")
}

func (s *Service) Sync(repoRoot string, st *store.Store, codexHome string) ([]WorktreeView, error) {
	items, err := gitwt.List(repoRoot)
	if err != nil {
		return nil, err
	}
	commonRoot := items[0].Path
	logs, err := codexlog.Scan(codexHome)
	if err != nil {
		return nil, err
	}

	for _, sess := range logs.BySession {
		_ = st.UpsertSession(sess.SessionID, sess.CWD, sess.FirstSeenAt, sess.LastActivityAt, sess.TurnCount)
	}

	now := s.Now().UTC()
	for _, item := range items {
		record := store.WorktreeRecord{
			Path:       item.Path,
			RepoRoot:   commonRoot,
			Branch:     item.Branch,
			IsMain:     item.IsMain,
			IsLocked:   item.Locked,
			IsPrunable: item.Prunable,
			LastSeenAt: &now,
		}
		if agg, ok := logs.ByCWD[item.Path]; ok {
			record.SessionCount = agg.SessionCount
			record.LastCodexTurnAt = nullableTime(agg.LastActivityAt)
		}
		if err := st.UpsertWorktree(record); err != nil {
			return nil, err
		}
	}

	rows, err := st.ListWorktrees()
	if err != nil {
		return nil, err
	}
	rowByPath := map[string]store.WorktreeRecord{}
	for _, row := range rows {
		rowByPath[row.Path] = row
	}

	views := make([]WorktreeView, 0, len(items))
	for _, item := range items {
		row := rowByPath[item.Path]
		view := WorktreeView{
			Path:            item.Path,
			Branch:          firstNonEmpty(item.Branch, row.Branch),
			IsMain:          item.IsMain,
			IsLocked:        item.Locked,
			IsPrunable:      item.Prunable,
			SessionCount:    row.SessionCount,
			LaunchCount:     row.LaunchCount,
			LastSelectedAt:  row.LastSelectedAt,
			LastCodexTurnAt: row.LastCodexTurnAt,
		}
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) CreateWorktree(repoRoot, name, branch, path string) (string, error) {
	if name == "" && branch == "" {
		return "", errors.New("worktree name or branch is required")
	}
	if name == "" {
		name = sanitizeBranchName(branch)
	}
	if branch == "" {
		branch = name
	}
	baseRoot := repoRoot
	if items, err := gitwt.List(repoRoot); err == nil && len(items) > 0 {
		baseRoot = items[0].Path
	}
	if path == "" {
		path = gitwt.DefaultCreatePath(baseRoot, name)
	}

	args := []string{"-C", repoRoot, "worktree", "add"}
	exists, err := branchExists(repoRoot, branch)
	if err != nil {
		return "", err
	}
	if exists {
		args = append(args, path, branch)
	} else {
		args = append(args, "-b", branch, path)
	}
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return path, nil
}

func (s *Service) RemoveWorktree(repoRoot, path string, force bool) error {
	args := []string{"-C", repoRoot, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	cmd := exec.Command("git", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (s *Service) ExecCodex(dir string, args []string) error {
	cmd := exec.Command("codex", args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func (s *Service) PrintList(views []WorktreeView) {
	tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "PATH\tBRANCH\tSESSIONS\tLAST_SELECTED\tLAST_CODEX\tSTATE")
	now := s.Now()
	for _, v := range views {
		_, _ = fmt.Fprintf(
			tw,
			"%s\t%s\t%d\t%s\t%s\t%s\n",
			v.Path,
			displayOrDash(v.Branch),
			v.SessionCount,
			ageString(now, v.LastSelectedAt),
			ageString(now, v.LastCodexTurnAt),
			strings.Join(stateTags(v, now), ","),
		)
	}
	_ = tw.Flush()
}

func (s *Service) Cleanup(repoRoot string, views []WorktreeView, staleDays int, apply bool) error {
	now := s.Now()
	threshold := time.Duration(staleDays) * 24 * time.Hour
	for _, v := range views {
		if !IsStaleCandidate(v, now, threshold) {
			continue
		}
		if !apply {
			fmt.Printf("stale\t%s\tbranch=%s\n", v.Path, displayOrDash(v.Branch))
			continue
		}
		if err := s.RemoveWorktree(repoRoot, v.Path, false); err != nil {
			return err
		}
		fmt.Printf("removed\t%s\n", v.Path)
	}
	return nil
}

func branchExists(repoRoot, branch string) (bool, error) {
	cmd := exec.Command("git", "-C", repoRoot, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return false, nil
	}
	return false, err
}

func stateTags(v WorktreeView, now time.Time) []string {
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
	if IsStaleCandidate(v, now, 30*24*time.Hour) {
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

func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func sanitizeBranchName(v string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", "\t", "-", "_", "-")
	return replacer.Replace(v)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

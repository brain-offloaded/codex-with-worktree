package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/brain-offloaded/codex-with-worktree/internal/app"
	"github.com/brain-offloaded/codex-with-worktree/internal/gitwt"
	"github.com/brain-offloaded/codex-with-worktree/internal/picker"
	"github.com/brain-offloaded/codex-with-worktree/internal/store"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	service := app.NewService()
	plan := app.PlanCommand(args)
	if plan.Mode == app.ModeDirectCodex {
		if err := service.ExecCodex("", plan.CodexArgs); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	}

	repoRoot, err := gitwt.RepoRoot(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cwt must run inside a git repository or worktree")
		return 1
	}
	if items, err := gitwt.List(repoRoot); err == nil && len(items) > 0 {
		repoRoot = items[0].Path
	}

	st, err := store.Open(app.DefaultStateDBPath())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer st.Close()

	switch plan.Mode {
	case app.ModeInternal:
		if err := runInternal(service, st, repoRoot, plan); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	case app.ModePickAndExec:
		if err := runPickAndExec(service, st, repoRoot, plan.CodexArgs); err != nil {
			if errors.Is(err, errCancelled) {
				return 130
			}
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintln(os.Stderr, "unsupported mode")
		return 1
	}
}

var errCancelled = errors.New("cancelled")

func runPickAndExec(service *app.Service, st *store.Store, repoRoot string, args []string) error {
	for {
		views, err := service.Sync(repoRoot, st, app.DefaultCodexHome())
		if err != nil {
			return err
		}
		p := &picker.Picker{In: os.Stdin, Out: os.Stdout}
		result, err := p.Run(views)
		if err != nil {
			return err
		}
		switch result.Action {
		case picker.ActionCancel:
			return errCancelled
		case picker.ActionRefresh:
			continue
		case picker.ActionDelete:
			target := views[result.Index]
			if err := service.RemoveTrackedWorktree(st, repoRoot, target.Path, result.Force, "worktree_removed"); err != nil {
				return err
			}
			continue
		case picker.ActionCreate:
			targetPath, err := service.CreateTrackedWorktree(st, repoRoot, result.Name, result.Branch, "")
			if err != nil {
				return err
			}
			if _, err := service.Sync(repoRoot, st, app.DefaultCodexHome()); err != nil {
				return err
			}
			if err := service.SelectWorktree(st, targetPath); err != nil {
				return err
			}
			return service.ExecCodex(targetPath, args)
		case picker.ActionSelect:
			target := views[result.Index]
			if err := service.SelectWorktree(st, target.Path); err != nil {
				return err
			}
			return service.ExecCodex(target.Path, args)
		}
	}
}

func runInternal(service *app.Service, st *store.Store, repoRoot string, plan app.CommandPlan) error {
	switch plan.InternalCommand {
	case "list":
		views, err := service.Sync(repoRoot, st, app.DefaultCodexHome())
		if err != nil {
			return err
		}
		service.PrintList(views)
		return nil
	case "create":
		var name, branch, path string
		rest := plan.CodexArgs
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case "--branch":
				i++
				if i < len(rest) {
					branch = rest[i]
				}
			case "--path":
				i++
				if i < len(rest) {
					path = rest[i]
				}
			default:
				if name == "" {
					name = rest[i]
				}
			}
		}
		targetPath, err := service.CreateTrackedWorktree(st, repoRoot, name, branch, path)
		if err != nil {
			return err
		}
		fmt.Println(targetPath)
		return nil
	case "remove":
		if len(plan.CodexArgs) == 0 {
			return errors.New("usage: cwt remove <path> [--force]")
		}
		force := len(plan.CodexArgs) > 1 && plan.CodexArgs[1] == "--force"
		return service.RemoveTrackedWorktree(st, repoRoot, plan.CodexArgs[0], force, "worktree_removed")
	case "cleanup":
		staleDays := 30
		apply := false
		for i := 0; i < len(plan.CodexArgs); i++ {
			switch plan.CodexArgs[i] {
			case "--apply":
				apply = true
			case "--stale-days":
				i++
				if i < len(plan.CodexArgs) {
					v, err := strconv.Atoi(plan.CodexArgs[i])
					if err != nil {
						return err
					}
					staleDays = v
				}
			}
		}
		views, err := service.Sync(repoRoot, st, app.DefaultCodexHome())
		if err != nil {
			return err
		}
		return service.Cleanup(st, repoRoot, views, staleDays, apply)
	case "doctor":
		fmt.Println("repo_root:", repoRoot)
		fmt.Println("state_db:", app.DefaultStateDBPath())
		fmt.Println("codex_home:", app.DefaultCodexHome())
		views, err := service.Sync(repoRoot, st, app.DefaultCodexHome())
		if err != nil {
			return err
		}
		active, err := st.ListActiveWorktrees(repoRoot)
		if err != nil {
			return err
		}
		deleted, err := st.ListDeletedWorktrees(repoRoot)
		if err != nil {
			return err
		}
		events, err := st.ListEvents()
		if err != nil {
			return err
		}
		fmt.Println("worktrees:", len(views))
		fmt.Println("active_rows:", len(active))
		fmt.Println("deleted_rows:", len(deleted))
		fmt.Println("events:", len(events))
		return nil
	default:
		return fmt.Errorf("unsupported internal command: %s", plan.InternalCommand)
	}
}

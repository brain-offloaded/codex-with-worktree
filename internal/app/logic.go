package app

import (
	"strings"
	"time"
)

type Mode string

const (
	ModePickAndExec Mode = "pick_exec"
	ModeDirectCodex Mode = "direct_codex"
	ModeInternal    Mode = "internal"
)

type CommandPlan struct {
	Mode            Mode
	CodexArgs       []string
	InternalCommand string
}

var internalCommands = map[string]bool{
	"list":    true,
	"create":  true,
	"remove":  true,
	"cleanup": true,
	"doctor":  true,
}

type WorktreeView struct {
	Path            string
	Branch          string
	IsMain          bool
	IsLocked        bool
	IsPrunable      bool
	SessionCount    int
	LaunchCount     int
	LastSelectedAt  *time.Time
	LastCodexTurnAt *time.Time
}

func PlanCommand(args []string) CommandPlan {
	if len(args) == 0 {
		return CommandPlan{Mode: ModePickAndExec}
	}
	first := args[0]
	if first == "--help" || first == "help" || first == "--version" || first == "version" {
		return CommandPlan{Mode: ModeDirectCodex, CodexArgs: args}
	}
	if internalCommands[first] {
		return CommandPlan{Mode: ModeInternal, InternalCommand: first, CodexArgs: args[1:]}
	}
	if strings.HasPrefix(first, "-") || first == "resume" || first == "exec" || first == "run" || first == "chat" {
		return CommandPlan{Mode: ModePickAndExec, CodexArgs: args}
	}
	return CommandPlan{Mode: ModePickAndExec, CodexArgs: args}
}

func IsStaleCandidate(w WorktreeView, now time.Time, threshold time.Duration) bool {
	if w.IsMain || w.IsLocked || w.IsPrunable {
		return false
	}
	cutoff := now.Add(-threshold)
	if w.LastSelectedAt != nil && w.LastSelectedAt.After(cutoff) {
		return false
	}
	if w.LastCodexTurnAt != nil && w.LastCodexTurnAt.After(cutoff) {
		return false
	}
	return true
}

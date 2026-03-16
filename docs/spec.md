# CWT Specification

## Summary

`cwt` is a Go-based wrapper around `codex` for Git worktree workflows.
It must behave like `codex` for normal execution, while adding a worktree-aware selection and lifecycle layer.

The first release target is `v0.0.1`.

## Product Goals

1. Allow users to launch `codex` or `codex resume` inside an existing Git worktree selected from a picker.
2. Allow users to create a new worktree from the picker flow.
3. Track useful local metadata for worktrees, including:
   - current branch
   - whether the worktree is the main checkout
   - locked / prunable state
   - last selected time via `cwt`
   - launch count via `cwt`
   - Codex session count discovered from `CODEX_HOME/sessions`
   - last Codex activity time inferred from session logs
4. Surface stale / cleanup candidates and support worktree removal.
5. Ship as a single WSL-friendly Linux binary.

## Explicit Constraints From Shared Conversation

1. The project is implemented in Go, not Python.
2. SQLite is used for local state.
3. The tool is more than a shell wrapper: it stores metadata and exposes useful management commands.
4. `cwt resume` must support a worktree picker.
5. Outside of worktree-specific behavior, command execution should mirror `codex` by delegating to the real binary with the original arguments.
6. The repository must include:
   - TDD-driven implementation
   - a clear spec document
   - CI
   - dependency / install guidance
   - English README
   - Korean README
   - release `0.0.1`

## Command Model

### Passthrough commands

These commands select a worktree, then execute the real `codex` binary in that directory:

- `cwt`
- `cwt exec ...`
- `cwt run ...`
- `cwt chat ...`
- `cwt resume ...`
- `cwt -...`

Rules:

1. `cwt resume ...` opens the picker, then runs `codex resume ...` in the selected worktree.
2. `cwt --help`, `cwt help`, `cwt --version`, and `cwt version` do not open the picker.
3. Arguments after the command are passed through unchanged to `codex`.

### Management commands

- `cwt list`
- `cwt create [--branch NAME] [--path PATH] [NAME]`
- `cwt remove <path> [--force]`
- `cwt cleanup [--stale-days N] [--apply]`
- `cwt doctor`

## Picker Requirements

The picker must:

1. List all worktrees from `git worktree list --porcelain`.
2. Show at least:
   - path
   - branch
   - session count
   - last selected time
   - last Codex activity time
   - state tags such as `main`, `locked`, `prunable`, `stale`
3. Allow selecting an existing worktree.
4. Allow creating a new worktree.
5. Allow deleting a selected worktree.

For `v0.0.1`, a terminal-driven interactive picker is sufficient. It does not need a full-screen TUI as long as it is interactive and reliable.

## Worktree Creation Rules

1. If `--branch` is provided, create or attach to that branch explicitly.
2. If `NAME` is provided without `--branch`, use the name both as:
   - the default directory suffix
   - the default branch name
3. If neither is provided, prompt interactively for a new worktree name.
4. Default target path format:
   - sibling directory of repo root
   - `<repo-name>--<name>`
5. Creation is done with `git worktree add` or `git worktree add -b`.

## Metadata Model

For `v0.0.2`, SQLite is not only a cache. It is the operational state store for:

1. active worktree listing
2. stale cleanup decisions
3. launch and selection history
4. session aggregation
5. audit history of key actions

Git worktree state and Codex session logs remain external inputs, but they must be reconciled into SQLite before `cwt` renders picker/list/cleanup output.

SQLite database location:

- default: `$XDG_STATE_HOME/cwt/index.sqlite`
- fallback: `~/.local/state/cwt/index.sqlite`

Tables:

### `worktrees`

- `path` primary key
- `repo_root`
- `branch`
- `is_main`
- `is_locked`
- `is_prunable`
- `created_at`
- `last_selected_at`
- `last_codex_turn_at`
- `session_count`
- `launch_count`
- `last_seen_at`
- `deleted_at`
- `last_reconciled_at`

### `sessions`

- `session_id` primary key
- `cwd`
- `first_seen_at`
- `last_seen_at`
- `turn_count`

### `events`

- `id` primary key
- `ts`
- `kind`
- `cwd`
- `session_id`
- `payload_json`

Event kinds must include at least:

- `reconcile_started`
- `reconcile_completed`
- `session_observed`
- `worktree_selected`
- `worktree_created`
- `worktree_removed`
- `cleanup_candidate`
- `cleanup_removed`

## Session Discovery

`cwt` must discover Codex session metadata by scanning JSONL files under:

- `$CODEX_HOME/sessions`
- fallback: `~/.codex/sessions`

The implementation must:

1. Read `session_meta` records to map session id to `cwd`.
2. Read event timestamps in each file to infer:
   - session existence
   - last activity time
   - approximate turn count
3. Upsert session rows into SQLite.
4. Recompute worktree session aggregates from SQLite, not only in memory.

## Reconcile Rules

Every command that depends on worktree state must run a reconcile phase first.

Reconcile steps:

1. record `reconcile_started`
2. collect live Git worktrees
3. upsert live worktrees into `worktrees`
4. mark previously known but now missing worktrees as deleted by setting `deleted_at`
5. scan Codex session logs and upsert `sessions`
6. recompute `worktrees.session_count` and `worktrees.last_codex_turn_at` from `sessions`
7. set `last_reconciled_at`
8. record `reconcile_completed`

The picker, `list`, `cleanup`, and `doctor` commands must read from SQLite-backed query methods after reconcile completes.

## Cleanup Rules

`cwt cleanup` marks a worktree as stale when all are true:

1. it is not the main worktree
2. it is not locked
3. it is not prunable metadata only
4. its last selected time and last Codex activity are both older than the stale threshold

Default stale threshold: `30` days.

`--apply` removes matching worktrees with `git worktree remove`.
Without `--apply`, the command performs a dry run.

Both dry-run and apply modes must write cleanup events to SQLite.

## Out Of Scope For `v0.0.1`

1. Codex notify hook integration.
2. Cross-machine sync.
3. Rich full-screen TUI.
4. Automatic branch renaming from issue analysis.
5. Windows native binary support.

## Test Strategy

The project is developed with tests first for core behavior:

1. porcelain parser tests
2. session log parser tests
3. stale classification tests
4. command dispatch / passthrough planning tests
5. SQLite migration and query tests
6. reconcile tests for:
   - active worktree upsert
   - deleted worktree marking
   - session aggregate refresh
   - event recording
7. integration tests using temporary Git repositories for create/remove/cleanup flows where practical

# cwt

`cwt` is a Go-based worktree launcher for Codex CLI.

It wraps `codex` with three extra capabilities:

1. Pick an existing Git worktree before launching Codex.
2. Create or remove worktrees from the picker flow.
3. Track local metadata such as last selection time, Codex session count, and last Codex activity in SQLite.

The initial target is WSL-friendly Linux binary distribution.

See the formal specification in [docs/spec.md](docs/spec.md).

## Features

- `cwt` opens a worktree picker, then runs `codex` in the selected worktree.
- `cwt resume ...` opens the same picker, then runs `codex resume ...`.
- `cwt list` prints worktree metadata with branch, sessions, and stale hints.
- `cwt create` creates a sibling worktree using `git worktree add`.
- `cwt remove` removes a worktree.
- `cwt cleanup` identifies or removes stale worktrees.
- SQLite state is stored under `$XDG_STATE_HOME/cwt/index.sqlite` or `~/.local/state/cwt/index.sqlite`.
- Codex session metadata is inferred from `$CODEX_HOME/sessions` or `~/.codex/sessions`.

## Requirements

- Git
- Codex CLI available on `PATH`
- Go 1.24.0 for local development
- `goenv` recommended for development parity

## Development Setup

```bash
goenv install 1.24.0
goenv local 1.24.0
go mod tidy
go test ./...
go build ./cmd/cwt
```

## Usage

Start Codex in a selected worktree:

```bash
cwt
```

Resume inside a selected worktree:

```bash
cwt resume --last
```

Create a worktree directly:

```bash
cwt create feature-login
cwt create --branch feat/login-timeout feature-login
```

List worktrees and metadata:

```bash
cwt list
```

Dry-run stale cleanup:

```bash
cwt cleanup --stale-days 30
```

Apply stale cleanup:

```bash
cwt cleanup --stale-days 30 --apply
```

Remove a worktree:

```bash
cwt remove ../repo--feature-login
```

## Picker Controls

- `<number>`: select a worktree
- `c`: create a worktree
- `d<number>`: delete a listed worktree
- `r`: refresh
- `q`: quit

## Data Model

SQLite tables:

- `worktrees`
- `sessions`
- `events`

Tracked worktree fields include:

- path
- branch
- main / locked / prunable state
- last selected timestamp
- last Codex activity timestamp
- Codex session count
- launch count

## Testing

```bash
go test ./...
```

Current automated coverage includes:

- `git worktree --porcelain` parsing
- Codex session log scanning
- stale worktree classification
- command dispatch planning
- SQLite persistence
- picker interaction parsing

## CI and Release

- CI runs `go mod tidy`, `go test ./...`, and `go build ./cmd/cwt`.
- Tagging `v*` triggers a GitHub release workflow.
- Release artifacts include:
  - `cwt-linux-amd64`
  - `cwt-linux-amd64.tar.gz`
  - `sha256sum.txt`

## Notes

- `cwt --help` and `cwt --version` are delegated directly to `codex`.
- For `v0.0.1`, the picker is a line-oriented terminal UI rather than a full-screen TUI.

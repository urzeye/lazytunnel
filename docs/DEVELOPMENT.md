# Development Plan

This document turns the product roadmap into an execution plan.

## Current Decisions

- language: Go
- target shape: local-first terminal application
- primary use cases:
  - SSH local forwarding with `ssh -L`
  - Kubernetes port-forwarding with `kubectl port-forward`
- release goal for v0.1:
  - downloadable binaries from GitHub Releases
  - mise installation
  - optional `go install` for Go users
  - Homebrew installation deferred to v0.1.x

## Suggested Tech Stack

- CLI entry: `cobra`
- TUI: `bubbletea`, `lipgloss`, and `bubbles` as needed
- logging: `slog` or `zap`
- config format: YAML for app config, generated commands for runtime execution
- releases: `goreleaser`

## Engineering Principles

- wrap trusted system commands instead of reimplementing them
- keep the runtime local-first and process-based
- avoid requiring a daemon or web server for v0.1
- keep the data model small until the first workflows feel solid
- prioritize observability and recoverability over feature count

## v0.1 Scope

The first version should support one excellent happy path:

- save tunnel profiles
- start and stop profiles from the TUI
- support `ssh -L`
- support `kubectl port-forward`
- detect local port conflicts before launch
- auto-reconnect unexpected exits
- show live state, local port, target, uptime, and recent logs
- group profiles into named stacks

## Proposed Project Structure

```text
cmd/lazytunnel/
internal/app/
internal/domain/
internal/runtime/
internal/adapters/ssh/
internal/adapters/kubernetes/
internal/storage/
internal/tui/
pkg/
```

Suggested responsibilities:

- `cmd/lazytunnel/`: process startup and CLI flags
- `internal/app/`: application services and orchestration
- `internal/domain/`: profiles, stacks, runtime state, validation rules
- `internal/runtime/`: process supervision, restart policy, logs, events
- `internal/adapters/ssh/`: SSH command building and validation
- `internal/adapters/kubernetes/`: `kubectl port-forward` command building and validation
- `internal/storage/`: config load and save
- `internal/tui/`: Bubble Tea models and views

## Milestones

### Milestone 1: Bootstrap

- initialize Go module
- choose core libraries
- add `justfile`
- add formatter and lint commands
- add a sample config file

Exit criteria:

- `go test ./...` runs cleanly
- `just run` starts a placeholder TUI

### Milestone 2: Domain Model

- define tunnel profile model
- define stack model
- define runtime state model
- define restart policy and validation rules

Exit criteria:

- profiles can be parsed from disk
- validation catches invalid ports and incomplete definitions

### Milestone 3: Process Runtime

- start and stop child processes
- capture stdout and stderr logs
- track PID, status, start time, exit reason
- implement restart with backoff

Exit criteria:

- a mocked process can be supervised and restarted
- runtime state transitions are covered by tests

### Milestone 4: SSH Support

- generate `ssh -L` commands from a profile
- validate host, target, and local port
- surface common launch failures clearly

Exit criteria:

- a saved SSH local-forward profile can be started from the app layer

### Milestone 5: Kubernetes Support

- generate `kubectl port-forward` commands from a profile
- support context, namespace, and target resource
- surface missing context and namespace errors clearly

Exit criteria:

- a saved Kubernetes profile can be started from the app layer

### Milestone 6: TUI

- profile list
- detail panel
- status badges
- start and stop actions
- logs panel
- stack start action

Exit criteria:

- the full v0.1 happy path is usable from the terminal UI

### Milestone 7: Packaging

- add `goreleaser`
- publish binaries for macOS, Linux, and Windows
- verify `mise` installation
- record Homebrew as post-v0.1.0 packaging work

Exit criteria:

- a tagged release can be installed without building from source

## Recommended Build Order

1. bootstrap repo and libraries
2. lock the domain model
3. build the runtime supervisor
4. add SSH adapter
5. add Kubernetes adapter
6. build the TUI on top of real runtime events
7. polish releases and installation

## Definition of Done for v0.1

v0.1 is ready when all of the following are true:

- a user can install the binary from a release
- a user can save at least one SSH profile and one Kubernetes profile
- both profile types can be launched from the TUI
- dropped processes can reconnect automatically
- local port collisions are detected before launch
- recent logs are visible in the UI
- the README includes installation and a short demo

## v0.1.0 Release Decision

The recommended `v0.1.0` release scope is:

- GitHub Releases binaries
- `go install`
- `mise` install from GitHub Releases

The following should not block `v0.1.0`:

- Homebrew
- `ssh -R`
- `ssh -D`
- imports from `~/.ssh/config` and Kubernetes contexts
- deeper log formatting, styling, and filtering polish
- an explicit manual restart action

## Immediate Next Steps

- improve log formatting, visual hierarchy, and filtering in the TUI
- add stack-focused keyboard polish and clearer action hints
- import from `~/.ssh/config` and Kubernetes contexts
- evaluate whether an explicit manual restart action is still needed after log/import polish
- add release automation with `goreleaser`

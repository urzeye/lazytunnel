# Roadmap

This document tracks the early product direction for LazyTunnel.

Execution plan: [Development Plan](DEVELOPMENT.md)

## MVP Scope

The first usable version should focus on a small but high-value slice:

- create and save tunnel profiles
- support `ssh -L`, `ssh -R`, and `ssh -D`
- support `kubectl port-forward`
- detect local port conflicts
- auto-reconnect when a process exits unexpectedly
- show status, local port, target, and recent logs
- group related tunnels into named stacks

## Phases

### v0.1

- local-first TUI
- SSH local, remote, and dynamic forwards
- Kubernetes port-forwards
- stack startup
- reconnect and basic logs

### v0.1.x

- Homebrew support
- `aqua` / registry integration
- log formatting, styling, and filtering polish in the TUI
- additional TUI interaction polish

### v0.2

- better health checks

### v0.3

- presets for common developer workflows
- launch hooks
- deeper integrations with existing SSH and Kubernetes contexts

## Current Focus

- improve the logs inspector with better formatting, styling, and filtering
- keep SSH and Kubernetes workflows feeling fast and low-friction in daily use

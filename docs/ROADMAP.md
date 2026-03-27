# Roadmap

This document tracks the early product direction for LazyTunnel.

Execution plan: [Development Plan](DEVELOPMENT.md)

## MVP Scope

The first usable version should focus on a small but high-value slice:

- create and save tunnel profiles
- support `ssh -L`
- support `kubectl port-forward`
- detect local port conflicts
- auto-reconnect when a process exits unexpectedly
- show status, local port, target, and recent logs
- group related tunnels into named stacks

## Phases

### v0.1

- local-first TUI
- SSH local forwards
- Kubernetes port-forwards
- stack startup
- reconnect and logs

### v0.2

- `ssh -R`
- `ssh -D`
- richer import flows
- better health checks

### v0.3

- presets for common developer workflows
- launch hooks
- deeper integrations with existing SSH and Kubernetes contexts

## Current Focus

- nail the tunnel profile model
- design the main TUI layout
- make the first two workflows feel great: `ssh -L` and `kubectl port-forward`

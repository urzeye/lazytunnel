English | [简体中文](README.zh-CN.md)

# LazyTunnel

> A terminal UI for managing SSH tunnels and Kubernetes port-forwards.

LazyTunnel is a keyboard-first workspace for the tunnels you use every day:

- SSH local forwards with `ssh -L`
- SSH remote forwards with `ssh -R`
- SSH SOCKS proxies with `ssh -D`
- Kubernetes port-forwards with `kubectl port-forward`

Instead of retyping long commands, remembering ports, and reopening dropped sessions by hand, you manage everything from one terminal UI.

## What It Helps With

Tunnel commands are powerful, but they are also annoying to manage in real life:

- the commands are long and repetitive
- each project tends to need multiple tunnels
- ports collide all the time
- sessions die when your network changes
- you often forget which tunnel is still running
- switching between SSH and Kubernetes flows feels fragmented

LazyTunnel aims to make these workflows feel as smooth as `lazygit` and `lazydocker`, but for local development tunnels and temporary access paths.

## Key Features

LazyTunnel is designed around a few strong workflows:

- save tunnel profiles instead of retyping commands
- start, stop, and restart tunnels with one key
- monitor status, uptime, ports, and recent errors in one place
- auto-reconnect dropped tunnels with backoff
- group multiple tunnels into a stack and start them together
- copy local URLs, host:port pairs, or connection strings quickly

## Supported Workflows

- SSH local forward: `ssh -L`
- SSH remote forward: `ssh -R`
- SSH dynamic proxy: `ssh -D`
- Kubernetes port-forward for `pod`, `service`, and `deployment`

## Planned Capabilities

- saved tunnel profiles
- stacks for project-based startup
- environment labels such as `dev`, `staging`, and `prod`
- port conflict detection with suggestions
- startup dependencies and preflight validation
- import from `~/.ssh/config`
- import from kube contexts and namespaces

### Monitoring

- live process status
- uptime and reconnect counters
- recent logs and exit reasons
- health indicators for active tunnels

### Convenience

- copy local endpoint
- copy DSN or connection snippets
- open local URLs in the browser
- fuzzy search across names, targets, and labels

## Non-Goals

LazyTunnel is intentionally not trying to be:

- a public tunnel SaaS
- a web dashboard that requires a server
- a replacement for OpenSSH or `kubectl`
- a secret manager
- a full cloud control plane

It is a local-first terminal tool that wraps the commands you already trust.

## Screenshots

Screenshots and demo GIFs will be added once the first interactive prototype is ready.

## Status

This project is in an early stage.

Roadmap: [English](docs/ROADMAP.md) | [简体中文](docs/ROADMAP.zh-CN.md)

## Feedback

Early feedback is welcome, especially on:

- which tunnel workflows you use most often
- which commands are the most annoying to repeat
- which status details you need visible at a glance

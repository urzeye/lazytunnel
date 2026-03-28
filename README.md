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

## Installation

Install from source right now with Go. Tagged releases will also publish prebuilt archives on GitHub Releases, support `mise`, and include Linux `.deb` and `.rpm` packages.

### Go

```bash
go install github.com/urzeye/lazytunnel/cmd/lazytunnel@latest
```

### GitHub Releases

Tagged releases publish archives for macOS, Linux, and Windows on the
[GitHub Releases page](https://github.com/urzeye/lazytunnel/releases).

### mise

If you use `mise`, tagged releases can be installed directly from GitHub:

```bash
mise use -g github:urzeye/lazytunnel
```

### Linux Packages

Each tagged release also includes `.deb` and `.rpm` assets for Linux distributions that prefer native packages.

## Quick Start

Initialize an empty config:

```bash
lazytunnel init
```

Or start from the bundled sample config:

```bash
lazytunnel init --sample
```

Add an SSH local-forward profile:

```bash
lazytunnel profile add ssh-local \
  --name prod-db \
  --host bastion-prod \
  --remote-host db.internal \
  --remote-port 5432 \
  --local-port 5432
```

Add a Kubernetes port-forward profile:

```bash
lazytunnel profile add kubernetes \
  --name api-debug \
  --context dev-cluster \
  --namespace backend \
  --resource-type service \
  --resource api \
  --remote-port 80 \
  --local-port 8080
```

Validate your config:

```bash
lazytunnel validate
```

Launch the terminal UI:

```bash
lazytunnel
```

## Key Features

LazyTunnel is designed around a few strong workflows:

- save tunnel profiles instead of retyping commands
- start, stop, and restart tunnels with one key
- monitor status, uptime, ports, and recent errors in one place
- group multiple tunnels into a stack and start them together
- detect local port conflicts before startup

## Supported Workflows

- SSH local forward: `ssh -L`
- SSH remote forward: `ssh -R`
- SSH dynamic proxy: `ssh -D`
- Kubernetes port-forward for `pod`, `service`, and `deployment`

## Near-Term Roadmap

- richer runtime status, reconnect insight, and log surfaces in the TUI
- better project stacks with labels and preflight validation
- guided import flows for common SSH and Kubernetes setups

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

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

Add an SSH remote-forward profile:

```bash
lazytunnel profile add ssh-remote \
  --name public-api \
  --host bastion-prod \
  --bind-address 0.0.0.0 \
  --bind-port 9000 \
  --target-host 127.0.0.1 \
  --target-port 8080
```

Add an SSH dynamic SOCKS profile:

```bash
lazytunnel profile add ssh-dynamic \
  --name dev-socks \
  --host bastion-prod \
  --local-port 1080
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

Clone an existing profile for a nearby environment:

```bash
lazytunnel profile clone prod-db \
  --name staging-db \
  --local-port 15432 \
  --description "Staging database tunnel"
```

Edit a saved profile in place:

```bash
lazytunnel profile edit staging-db \
  --remote-host staging-db.internal \
  --label staging \
  --label db
```

Or walk through the same edit interactively:

```bash
lazytunnel profile edit staging-db --interactive
lazytunnel stack edit backend-dev --interactive
```

Import draft profiles from your existing SSH config:

```bash
lazytunnel profile import ssh-config
```

Import draft profiles from your kubeconfig contexts:

```bash
lazytunnel profile import kube-contexts
```

Use a custom config path or overwrite existing names when needed:

```bash
lazytunnel --config ~/.config/lazytunnel/config.yaml profile import ssh-config --overwrite
lazytunnel profile import kube-contexts --kubeconfig ~/.kube/config --overwrite
```

Imported profiles are created as editable drafts. SSH imports use a placeholder
forward target and Kubernetes imports use a placeholder resource target, so
you'll usually want to refine them before connecting. In the TUI, press `e` to
finish the selected draft in the built-in form editor, start with `Quick Fill`
to apply a common SSH or Kubernetes recipe for imported drafts, then adjust the
real target fields. Press `E` to jump to raw YAML instead. If the TUI is
already open when you import from the CLI, press `g` to reload the config after
importing.

Validate your config:

```bash
lazytunnel validate
```

Run preflight checks before you start a profile or stack:

```bash
lazytunnel profile check prod-db
lazytunnel stack check backend-dev
```

Preflight checks report `Ready`, `Warning`, or `Blocked`, and include actionable
hints for local port conflicts, missing `ssh` or `kubectl` binaries, SSH alias
and `IdentityFile` issues, plus Kubernetes context, namespace, and resource
verification.

Launch the terminal UI:

```bash
lazytunnel
```

Inside the TUI:

- press `i` to open the import prompt for `~/.ssh/config`, kube contexts, or both
- press `a` to choose a profile preset for SSH local, SSH remote, SOCKS, or Kubernetes, then finish it in the guided form
- press `A` to choose a stack preset from the current selection, visible profiles, or running profiles
- press `s` to seed the sample config when the workspace is empty
- press `e` to open the guided form editor for the selected profile or stack; imported drafts open on `Quick Fill` first
- press `E` to open the raw YAML config in your external editor
- inside details, press `y` to copy the selected profile command, or the selected stack member command
- inside the logs inspector, use `f` to follow/pause, `t` / `T` to cycle sources, `w` to wrap or truncate, `n` / `N` to jump between filter hits, `y` to copy visible logs, `o` to export them, and `x` to clear them
- inside stack details, use `[` / `]` to pick a member, `S` to start or stop it, `R` to restart it, and `p` to open that member profile
- inside the stack form editor, use `,` or paste comma/newline-separated profile names to expand member rows, `+` to add a member below the current row, `Ctrl+X` to remove it, and `[` / `]` to reorder members

## Key Features

LazyTunnel is designed around a few strong workflows:

- save tunnel profiles instead of retyping commands
- start and stop tunnels from the TUI
- monitor status, uptime, ports, recent errors, and recent logs in one place
- group multiple tunnels into a stack and start them together
- filter profiles and stacks by name, label, target, and port
- filter logs by text and source, highlight matches, and jump across hits from the logs inspector
- create new profiles and stacks from guided presets instead of starting from a blank YAML draft
- import draft profiles from `~/.ssh/config` and kubeconfig contexts from the CLI or TUI
- finish imported drafts in a built-in TUI form editor with `Quick Fill` recipes, or from `profile edit --interactive`
- run preflight checks for local port conflicts, missing `ssh` / `kubectl`, SSH alias and key-path issues, and Kubernetes target mismatches before startup
- show actionable validation hints that point to the next fix
- copy generated exec commands, copy or export visible logs, and clear captured logs directly from the TUI
- manage config from the CLI with add, clone, edit, and remove commands
- control individual stack members directly from stack details and reorder members in the guided stack form
- delete profiles and stacks directly from the TUI with confirmation
- switch the TUI between English and Simplified Chinese instantly

## Supported Today

- SSH local forward: `ssh -L`
- SSH remote forward: `ssh -R`
- SSH dynamic proxy: `ssh -D`
- Kubernetes port-forward for `pod`, `service`, and `deployment`

## Near-Term Roadmap

- lower configuration friction with stronger presets, draft completion, and stack editing flows
- keep deepening preflight validation for SSH and Kubernetes targets
- continue runtime observability, reconnect insight, and log ergonomics in the TUI
- polish tagged-release installation paths and package distribution

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

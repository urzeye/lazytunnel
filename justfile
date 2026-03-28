default:
  @just --list

fmt:
  find . -type f -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w

test:
  go test ./...

vet:
  go vet ./...

tidy:
  go mod tidy

run *args:
  go run ./cmd/lazytunnel {{args}}

build:
  #!/usr/bin/env bash
  set -euo pipefail
  mkdir -p bin
  version="${VERSION:-}"
  if [ -z "$version" ]; then
    version="$(git describe --tags --exact-match 2>/dev/null || echo dev)"
  fi
  commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
  date="${DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
  dirty="${DIRTY:-}"
  if [ -z "$dirty" ]; then
    dirty="false"
    if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
      if [ -n "$(git status --porcelain --untracked-files=normal 2>/dev/null)" ]; then
        dirty="true"
      fi
    fi
  fi
  go build -trimpath \
    -ldflags "-s -w -X github.com/urzeye/lazytunnel/internal/buildinfo.Version=$version -X github.com/urzeye/lazytunnel/internal/buildinfo.Commit=$commit -X github.com/urzeye/lazytunnel/internal/buildinfo.Date=$date -X github.com/urzeye/lazytunnel/internal/buildinfo.Dirty=$dirty" \
    -o bin/lazytunnel ./cmd/lazytunnel

check:
  just test
  just vet

release-check:
  #!/usr/bin/env bash
  set -euo pipefail
  if ! command -v goreleaser >/dev/null 2>&1; then
    echo "goreleaser is required; install it with: go install github.com/goreleaser/goreleaser/v2@latest" >&2
    exit 1
  fi
  goreleaser check

release-snapshot:
  #!/usr/bin/env bash
  set -euo pipefail
  if ! command -v goreleaser >/dev/null 2>&1; then
    echo "goreleaser is required; install it with: go install github.com/goreleaser/goreleaser/v2@latest" >&2
    exit 1
  fi
  goreleaser release --snapshot --clean

init-config:
  #!/usr/bin/env bash
  set -euo pipefail
  config_path="${XDG_CONFIG_HOME:-$HOME/.config}/lazytunnel/config.yaml"
  mkdir -p "$(dirname "$config_path")"
  if [ ! -f "$config_path" ]; then
    cp config.example.yaml "$config_path"
    echo "created $config_path"
  else
    echo "config already exists at $config_path"
  fi

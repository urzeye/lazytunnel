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
  version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
  commit="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
  date="${DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
  go build -trimpath \
    -ldflags "-s -w -X github.com/urzeye/lazytunnel/internal/buildinfo.Version=$version -X github.com/urzeye/lazytunnel/internal/buildinfo.Commit=$commit -X github.com/urzeye/lazytunnel/internal/buildinfo.Date=$date" \
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

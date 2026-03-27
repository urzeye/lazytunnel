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
  mkdir -p bin
  go build -o bin/lazytunnel ./cmd/lazytunnel

check:
  just test
  just vet

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

#!/usr/bin/env bash
# Make the Go toolchain that CI uses (go.mod's `go` line) available, so a cloud
# agent (Claude Code on the web / Codex Cloud) can run `make test`.
# Idempotent. Dependencies come from vendor/, so only the toolchain needs network.
#
#   bash scripts/agent/setup.sh
#
# Order: mise (if installed) -> an existing go (it switches itself to the
# go.mod version via GOTOOLCHAIN=auto, fetching from proxy.golang.org) ->
# a tarball from go.dev into .tools/ (git-ignored). The last case prints the
# PATH to export.

set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"

log() { printf '[agent-setup] %s\n' "$*"; }

want=$(awk '$1 == "go" { print $2; exit }' go.mod)
log "go.mod wants go $want"

if command -v mise >/dev/null 2>&1; then
  log "using mise"
  mise exec "go@$want" -- go version
  exit 0
fi

if command -v go >/dev/null 2>&1; then
  log "using $(command -v go) ($(go version))"
  # Older toolchains download go$want themselves because go.mod requires it.
  GOTOOLCHAIN=auto go version
  exit 0
fi

dest="$ROOT/.tools/go"
if [ ! -x "$dest/bin/go" ]; then
  arch=$(uname -m)
  case "$arch" in
    x86_64) arch=amd64 ;;
    aarch64 | arm64) arch=arm64 ;;
  esac
  url="https://go.dev/dl/go${want}.linux-${arch}.tar.gz"
  log "downloading $url"
  mkdir -p "$ROOT/.tools"
  curl -fsSL "$url" | tar -xz -C "$ROOT/.tools"
fi
"$dest/bin/go" version
log "add to PATH: export PATH=$dest/bin:\$PATH"

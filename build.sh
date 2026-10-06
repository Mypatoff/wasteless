#!/usr/bin/env bash
# Builds both release binaries into dist/. Run from anywhere; cd's to this
# script's own directory first so it always builds this repo, not whatever
# the caller's shell happened to be sitting in.
set -euo pipefail
cd "$(dirname "$0")"

mkdir -p dist

echo "Building dist/wasteless-linux (linux/amd64)..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o dist/wasteless-linux ./cmd/server

echo "Building dist/WasteLess.exe (windows/amd64)..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/WasteLess.exe ./cmd/server

echo "Done. Binaries are in dist/"

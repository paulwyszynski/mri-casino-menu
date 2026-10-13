#!/usr/bin/env bash
# Build menubot for the Pi and install it over SSH.
# Usage: ./deploy.sh <ssh-host>
set -euo pipefail

host="${1:?usage: ./deploy.sh pi@192.168.0.3}"
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p build
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o build/menubot ./cmd/menubot

remote=/tmp/menubot-deploy
ssh "$host" "rm -rf $remote && mkdir -p $remote"
scp build/menubot deploy/* "$host:$remote/"
ssh -t "$host" "sudo bash $remote/install.sh"

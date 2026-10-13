#!/usr/bin/env bash
# Build menubot for the Pi and install it over SSH.
#
# Usage: ./deploy.sh <user@ip>      e.g. ./deploy.sh pi@192.168.0.3
#        ./deploy.sh <ssh-alias>    e.g. ./deploy.sh pi  (Host entry in ~/.ssh/config)
#
# All SSH steps share one connection, so a password is asked for only once
# (plus the sudo password on the Pi, if your user needs one).
set -euo pipefail

usage="usage: ./deploy.sh <user@ip>   e.g. ./deploy.sh pi@192.168.0.3"
if [ $# -ne 1 ] || [ -z "$1" ]; then
  echo "$usage" >&2
  exit 2
fi
host="$1"
cd "$(dirname "$0")"

go vet ./...
go test ./...

mkdir -p build
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o build/menubot ./cmd/menubot

# Reuse one SSH connection for every step below.
ctl_dir="$(mktemp -d)"
ssh_opts=(-o ControlMaster=auto -o "ControlPath=$ctl_dir/s" -o ControlPersist=60)
cleanup() {
  ssh "${ssh_opts[@]}" -O exit "$host" 2>/dev/null || true
  rmdir "$ctl_dir" 2>/dev/null || true
}
trap cleanup EXIT

remote=/tmp/menubot-deploy
ssh "${ssh_opts[@]}" "$host" "rm -rf $remote && mkdir -p $remote"
scp "${ssh_opts[@]}" build/menubot deploy/* "$host:$remote/"
ssh "${ssh_opts[@]}" -t "$host" "sudo bash $remote/install.sh"

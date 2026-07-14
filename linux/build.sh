#!/usr/bin/env sh
set -eu

LINUX_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$LINUX_ROOT/.." && pwd)
cd "$REPO_ROOT"

mkdir -p "$LINUX_ROOT/bin"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o "$LINUX_ROOT/bin/sms-core" ./linux/cmd/sms
chmod +x "$LINUX_ROOT/init" "$LINUX_ROOT/sms" "$LINUX_ROOT/build.sh" "$LINUX_ROOT/bin/sms-core"
echo "Built $LINUX_ROOT/bin/sms-core"

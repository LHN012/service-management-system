#!/usr/bin/env sh
set -eu

LINUX_ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$LINUX_ROOT"

go test ./...
VERSION=${VERSION:-dev}
GOARCH=${GOARCH:-amd64}
CGO_ENABLED=0 GOOS=linux GOARCH="$GOARCH" go build -trimpath -ldflags="-s -w -X main.version=$VERSION" -o sms .
chmod +x sms

echo "Built $LINUX_ROOT/sms"

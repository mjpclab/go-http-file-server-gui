#!/usr/bin/env bash
set -e

cd "$(dirname "$0")/../"

CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" .

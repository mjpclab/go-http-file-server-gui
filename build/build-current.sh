#!/usr/bin/env bash
set -e

cd "$(dirname "$0")/../"

# Version shown on the About tab. Unlike gen_syso.sh the leading `v` is kept —
# nothing parses this string. The sed drops git's commit count and the `g` it
# prefixes to the hash: v0.0.9-3-g888df4e becomes v0.0.9-888df4e. Empty when
# there is no git history (source tarball); the binary then falls back to the
# version recorded in the build info. Override with VERSION=.
VERSION="${VERSION:-$(git describe --tags 2>/dev/null | sed -e 's/-[0-9]*-g/-/')}"

CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.appVersion=$VERSION" .

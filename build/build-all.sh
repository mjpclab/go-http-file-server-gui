#!/usr/bin/env bash
set -e

cd "$(dirname "$0")/../"
mkdir -p output
rm -f output/*

# Version shown on the About tab. Unlike gen_syso.sh the leading `v` is kept —
# nothing parses this string. The sed drops git's commit count and the `g` it
# prefixes to the hash: v0.0.9-3-g888df4e becomes v0.0.9-888df4e. Empty when
# there is no git history (source tarball); the binary then falls back to the
# version recorded in the build info. Override with VERSION=.
VERSION="${VERSION:-$(git describe --tags 2>/dev/null | sed -e 's/-[0-9]*-g/-/')}"

build() {
	local goos="$1" goarch="$2"
	local ext ldflags="-s -w -X main.appVersion=$VERSION"
	ext=$(GOOS="$goos" go env GOEXE)
	# Suppress the console window for the Windows GUI binary.
	[ "$goos" = "windows" ] && ldflags="$ldflags -H windowsgui"
	echo "building ${goos}/${goarch}"
	GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
		go build -trimpath -ldflags="$ldflags" \
		-o "output/ghfs-gui-${goos}-${goarch}${ext}" .
}

build linux   amd64
build linux   arm64
#build windows 386
build windows amd64
build windows arm64
build darwin  amd64
build darwin  arm64

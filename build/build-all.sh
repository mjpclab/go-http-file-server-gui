#!/usr/bin/env bash
set -e

cd "$(dirname "$0")/../"
mkdir -p output

build() {
	local goos="$1" goarch="$2"
	local ext ldflags="-s -w"
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

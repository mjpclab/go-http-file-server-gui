#!/usr/bin/env bash
# Regenerate the Windows resource (.syso) files. Each .syso embeds Icon.ico
# plus a VERSIONINFO block (version, product name) into the PE's .rsrc section.
# Run this after editing Icon.ico or whenever the git tag advances.
set -e

cd "$(dirname "$0")/../"

# `goversioninfo`'s parser requires the version to start with x.y.z digits,
# so the leading `v` from git tags is stripped here. No `--abbrev=0`: the
# pre-release suffix is kept. Override with VERSION= for untagged builds.
VERSION="${VERSION:-$(git describe --tags | sed -e 's/^v//' | sed -e 's/-[0-9]*-g/-/')}"

for arch in 386 amd64 arm64; do
	bits64=false arm=false
	[[ $arch == *64 ]] && bits64=true
	[[ $arch == arm* ]] && arm=true
	go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest \
		-64="$bits64" \
		-arm="$arm" \
		-icon Icon.ico \
		-product-name "Go HTTP File Server GUI" \
		-product-version "$VERSION" \
		-file-version "$VERSION" \
		-propagate-ver-strings \
		-o "rc_windows_${arch}.syso" \
		<(echo '{}')
done

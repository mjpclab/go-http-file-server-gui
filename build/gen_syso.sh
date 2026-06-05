#!/usr/bin/env bash
# Regenerate the Windows resource (.syso) files. Each .syso embeds Icon.ico
# plus a VERSIONINFO block (version, product name) into the PE's .rsrc section.
# Run this after editing Icon.ico or whenever the git tag advances.
set -e

cd "$(dirname "$0")/../"

# `goversioninfo`'s parser requires the version to start with x.y.z digits,
# so the leading `v` from git tags is stripped here.
version=$(git describe --tags | sed -e 's/^v//' | sed -e 's/-[0-9]*-g/-/')

for arch in 386 amd64; do
	bits=true
	[ "$arch" = "386" ] && bits=false
	go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest \
		-64="$bits" \
		-icon Icon.ico \
		-product-name "Go HTTP File Server GUI" \
		-product-version "$version" \
		-file-version "$version" \
		-propagate-ver-strings \
		-o "rc_windows_${arch}.syso" \
		<(echo '{}')
done

# xattr -dr com.apple.quarantine ghfs-gui-darwin-ARCH.app

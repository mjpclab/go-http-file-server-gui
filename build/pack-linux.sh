#!/usr/bin/env bash
# Wrap the built Linux binaries into distro packages (.deb / .rpm / Arch / apk)
# via nfpm, which needs no dpkg/rpmbuild/makepkg on the build host.
# Prerequisites: run build/build-all.sh first (produces output/ghfs-gui-linux-*).
set -e

cd "$(dirname "$0")/../"

# nfpm from PATH, else from GOPATH/bin, else installed with the Go toolchain.
NFPM="${NFPM:-$(command -v nfpm || true)}"
if [ -z "$NFPM" ]; then
	NFPM="$(go env GOPATH)/bin/nfpm"
	if [ ! -x "$NFPM" ]; then
		echo "nfpm not found, installing" >&2
		go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest
	fi
fi

# Package version: the latest tag with the leading `v` stripped. `--abbrev=0`
# keeps it clean (rpm and apk reject `-` in a version); override with VERSION=
# for untagged builds.
VERSION="${VERSION:-$(git describe --tags --abbrev=0 | sed -e 's/^v//')}"
export VERSION

trap 'rm -rf pkgroot' EXIT

pack() {
	local arch="$1"
	local bin_path="output/ghfs-gui-linux-${arch}"

	if [ ! -f "$bin_path" ]; then
		echo "skipping ${arch}: ${bin_path} not found (run build/build-all.sh)" >&2
		return
	fi

	# nfpm reads `src` paths relative to the working directory; stage the
	# binary under its installed name so dst/src stay in sync.
	rm -rf pkgroot
	mkdir -p pkgroot
	install -m 0755 "$bin_path" pkgroot/ghfs-gui

	export PKG_ARCH="$arch"
	local packager
	for packager in deb rpm archlinux apk; do
		echo "packaging ${arch} ${packager}"
		"$NFPM" package -f build/nfpm.yaml -p "$packager" -t output/
	done
}

pack amd64
pack arm64

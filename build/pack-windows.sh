#!/usr/bin/env bash
# Wrap the built Windows binaries into NSIS installers: Start Menu shortcut,
# uninstall entry, and a per-user (no UAC) / all-users choice at install time.
# Prerequisites: run build/build-all.sh first (produces output/ghfs-gui-windows-*).
set -e

cd "$(dirname "$0")/../"

# Unlike nfpm in pack-linux.sh, NSIS is not a Go tool and cannot be fetched on
# demand, so it has to be on PATH already.
if ! command -v makensis >/dev/null; then
	echo "makensis not found; install NSIS (Debian/Ubuntu: apt install nsis)" >&2
	exit 1
fi

# Installer version: the latest tag with the leading `v` stripped; override with
# VERSION= for untagged builds.
VERSION="${VERSION:-$(git describe --tags --abbrev=0 | sed -e 's/^v//')}"

# VIProductVersion accepts exactly four numeric components, so drop any
# pre-release suffix and zero-fill what's missing: 0.0.6 -> 0.0.6.0.
IFS=. read -r major minor patch build <<<"${VERSION%%-*}"
viversion="${major:-0}.${minor:-0}.${patch:-0}.${build:-0}"

pack() {
	local arch="$1"
	local src="output/ghfs-gui-windows-${arch}.exe"

	if [ ! -f "$src" ]; then
		echo "skipping ${arch}: ${src} not found (run build/build-all.sh)" >&2
		return
	fi

	echo "packaging ${arch} installer"
	# Paths are passed relative to the repository root; installer.nsi resolves
	# them through its own ${ROOT}, since makensis treats relative paths as
	# relative to the script's directory.
	makensis -V2 \
		-DARCH="$arch" \
		-DVERSION="$VERSION" \
		-DVIVERSION="$viversion" \
		-DSRCEXE="$src" \
		-DOUTFILE="output/ghfs-gui-windows-${arch}-setup.exe" \
		build/installer.nsi
}

pack amd64
pack arm64

#!/usr/bin/env bash
# Wrap the built macOS binaries into double-clickable .app bundles so they
# launch as windowed apps (a bare Mach-O opens a Terminal window in Finder;
# Prerequisites: run build/build-all.sh first (produces output/ghfs-gui-darwin-*)
# and ensure Icon.icns exists (see build/gen_icns.sh).
set -e

cd "$(dirname "$0")/../"

if [ ! -f Icon.icns ]; then
	echo "Icon.icns missing; run build/gen_icns.sh first" >&2
	exit 1
fi

# Same version derivation as build/gen_syso.sh: strip pre-release suffix and
# the leading `v` from the latest git tag.
version=$(git describe --tags | sed -e 's/^v//' | sed -e 's/-[0-9]*-g/-/')

pack() {
	local arch="$1"
	local bin_name="ghfs-gui-darwin-${arch}"
	local bin_path="output/${bin_name}"

	local app_name='Go HTTP File Server GUI.app'
	local app_path="output/${app_name}"

	local zip_name="${bin_name}.zip"

	if [ ! -f "$bin_path" ]; then
		echo "skipping ${arch}: ${bin_path} not found (run build/build-all.sh)" >&2
		return
	fi

	rm -rf "$app_path"
	mkdir -p "$app_path/Contents/MacOS" "$app_path/Contents/Resources"

	install -m 0755 "$bin_path" "$app_path/Contents/MacOS/ghfs-gui"
	cp Icon.icns "$app_path/Contents/Resources/Icon.icns"

	cat > "$app_path/Contents/Info.plist" <<-PLIST
		<?xml version="1.0" encoding="UTF-8"?>
		<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
		<plist version="1.0">
		<dict>
			<key>CFBundlePackageType</key><string>APPL</string>
			<key>CFBundleExecutable</key><string>ghfs-gui</string>
			<key>CFBundleName</key><string>ghfs-gui</string>
			<key>CFBundleDisplayName</key><string>${app_name%.app}</string>
			<key>CFBundleIdentifier</key><string>dev.mjpclab.ghfs-gui</string>
			<key>CFBundleIconFile</key><string>Icon</string>
			<key>CFBundleShortVersionString</key><string>${version}</string>
			<key>CFBundleVersion</key><string>${version}</string>
			<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
			<key>NSHighResolutionCapable</key><true/>
			<key>LSMinimumSystemVersion</key><string>11.0</string>
		</dict>
		</plist>
	PLIST

	(cd output/ && rm -f "${zip_name}" && \
		zip -1 -r -y -q "${zip_name}" "${app_name}")
	rm -rf "$app_path" "$bin_path"
}

pack amd64
pack arm64

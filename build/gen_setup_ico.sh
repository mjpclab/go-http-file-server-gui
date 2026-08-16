#!/usr/bin/env bash
# Regenerate the NSIS installer's and uninstaller's icons: Icon.ico with NSIS's
# stock emblem badged into the bottom-right corner. The output is committed and
# consumed by build/installer.nsi; run this after editing Icon.ico.
#
# Bottom-right knowingly: both executables ask for elevation, so Windows draws
# its UAC shield over that corner, hiding the emblem — which is also the only
# thing telling the installer's icon from the uninstaller's.
set -e

cd "$(dirname "$0")/../"

if ! command -v magick >/dev/null; then
	echo "magick not found; install ImageMagick 7" >&2
	exit 1
fi

if [ -z "$NSISDIR" ]; then
	if ! command -v makensis >/dev/null; then
		echo "makensis not found; install NSIS or set NSISDIR=" >&2
		exit 1
	fi
	NSISDIR=$(makensis -HDRINFO | tr ',' '\n' | sed -n 's/^NSISDIR=//p')
fi
icons="${NSISDIR}/Contrib/Graphics/Icons"

# Last match wins: the 4-bit and 8-bit palette frames precede the 32-bit one
# that has a real alpha channel.
frame() {
	magick identify "$1" |
		awk -v want="$2x$2" '$2 == "ICO" && $3 == want { gsub(/.*\[|\].*/, "", $1); print $1 }' |
		tail -1
}

appsizes=$(magick identify Icon.ico | awk '$2 == "ICO" { split($3, d, "x"); print d[1] }' | sort -nu)

# The smallest Icon.ico frame at or above $1, falling back to the largest one
# it has. Downscaling beats upscaling, and a higher-resolution frame added to
# Icon.ico later is used as soon as it exists, without an edit here.
srcsize() {
	echo "$appsizes" |
		awk -v want="$1" '$1 >= want { print $1; found = 1; exit } { last = $1 } END { if (!found) print last }'
}

compose() {
	local emblem="$1" out="$2"
	local tmp
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' RETURN

	# The stock icons' 16px frame is the bare emblem, with no package drawn
	# around it.
	emblem="${emblem}[$(frame "$emblem" 16)]"

	for size in 16 32 48; do
		# Nearest neighbour only when upscaling — the smooth filters turn the
		# pixel art to mush, but a downscale needs one.
		local src scale=()
		src=$(srcsize "$size")
		[ "$size" -gt "$src" ] && scale=(-filter point)
		magick "Icon.ico[$(frame Icon.ico "$src")]" \
			"${scale[@]}" -resize "${size}x${size}" "PNG32:$tmp/base.png"
		magick "$emblem" -resize "$((size / 2))x$((size / 2))" "PNG32:$tmp/emblem.png"
		magick "$tmp/base.png" "$tmp/emblem.png" \
			-gravity southeast -composite "PNG32:$tmp/$size.png"
	done

	# Both the PNG32 intermediates and -type TrueColorAlpha are load-bearing.
	# Icon.ico carries no colour of its own, so a plain PNG intermediate comes
	# out greyscale and desaturates the emblem composited onto it; and left to
	# itself the ICO encoder takes the palette path, rewriting every partially
	# transparent pixel to black.
	magick "$tmp/16.png" "$tmp/32.png" "$tmp/48.png" -type TrueColorAlpha "$out"
}

mkdir -p build/icons
compose "$icons/orange-install.ico" build/icons/setup.ico
compose "$icons/orange-uninstall.ico" build/icons/uninstall.ico

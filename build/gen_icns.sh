#!/usr/bin/env bash
# Regenerate the macOS icon resource (Icon.icns) from Icon.png. The generated
# Icon.icns is committed to the repo and consumed by build/pack-darwin.sh.
# Run this manually after editing Icon.png.
set -e

cd "$(dirname "$0")/../"

go run github.com/jackmordaunt/icns/cmd/icnsify@latest -i Icon.png -o Icon.icns

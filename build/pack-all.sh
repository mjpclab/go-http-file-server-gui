#!/usr/bin/env bash

set -e

cd "$(dirname "$0")"

self="$(basename "$0")"

for pack in pack-*.sh; do
	if [ "$pack" != "$self" ]; then
		"./$pack"
	fi
done
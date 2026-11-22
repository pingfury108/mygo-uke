#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
platform=${MYGO_UKE_RESOURCE_PLATFORM:-$(cd "$root/app" && go env GOOS)-$(cd "$root/app" && go env GOARCH)}
bundle=${1:-"$root/app/build/$platform/MyGo Uke.app"}
if [ ! -e "$bundle" ]; then
	echo "bundle not found: $bundle" >&2
	exit 1
fi

size_kb=$(du -sk "$bundle" | awk '{print $1}')
limit_kb=$((100 * 1024))
echo "$bundle: $((size_kb / 1024)) MB"
if [ "$size_kb" -gt "$limit_kb" ]; then
	echo "over the 100 MB distribution limit" >&2
	exit 1
fi

#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
platform=${MYGO_UKE_RESOURCE_PLATFORM:-$(cd "$root/app" && go env GOOS)-$(cd "$root/app" && go env GOARCH)}
dest="$root/app/resources/$platform"

mkdir -p "$dest"

ffmpeg_src=${MYGO_UKE_FFMPEG_SOURCE:-$(command -v ffmpeg || true)}
if [ -z "$ffmpeg_src" ] || [ ! -x "$ffmpeg_src" ]; then
	echo "ffmpeg not found: set MYGO_UKE_FFMPEG_SOURCE=/path/to/ffmpeg" >&2
	exit 1
fi
cp -f "$ffmpeg_src" "$dest/ffmpeg"
chmod 755 "$dest/ffmpeg"

font_src=${MYGO_UKE_FONT_SOURCE:-}
if [ -n "$font_src" ]; then
	if [ ! -f "$font_src" ]; then
		echo "font not found: $font_src" >&2
		exit 1
	fi
	mkdir -p "$root/app/resources/fonts"
	cp -f "$font_src" "$root/app/resources/fonts/NotoSansSC-Regular.otf"
elif [ -f "$root/app/resources/fonts/NotoSansSC-Regular.otf" ]; then
	echo "font: bundled NotoSansSC-Regular.otf"
else
	echo "font skipped: system font will be used" >&2
fi

echo "resources: $dest"
du -sh "$dest"

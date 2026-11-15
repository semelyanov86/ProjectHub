#!/usr/bin/env bash
# Renders the committed PNG icons from the SVG sources in build/icons/.
# The 16-24 px sizes and the tray icon use the simplified small design.
set -euo pipefail
cd "$(dirname "$0")/.."

render() { # render <svg> <size> <png>
	mkdir -p "$(dirname "$3")"
	if command -v rsvg-convert >/dev/null; then
		rsvg-convert -w "$2" -h "$2" "$1" -o "$3"
	elif command -v inkscape >/dev/null; then
		inkscape -w "$2" -h "$2" "$1" -o "$3" >/dev/null 2>&1
	else
		echo "rsvg-convert (librsvg2-bin) or inkscape is required" >&2
		exit 1
	fi
	echo "rendered $3"
}

hicolor=build/linux/share/icons/hicolor
for size in 16 22 24; do
	render build/icons/projecthub-small.svg "$size" "$hicolor/${size}x${size}/apps/projecthub.png"
done
for size in 32 48 64 128 256 512; do
	render build/icons/projecthub.svg "$size" "$hicolor/${size}x${size}/apps/projecthub.png"
done
render build/icons/projecthub.svg 512 build/appicon.png
render build/icons/projecthub-small.svg 32 build/icons/tray.png
cp build/icons/projecthub-small.svg frontend/public/projecthub.svg

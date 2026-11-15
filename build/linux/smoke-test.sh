#!/usr/bin/env bash
# Installs a ProjectHub .deb into clean Ubuntu containers and runs the smoke
# tests in build/linux/smoke/ (package install/removal, X11 and Wayland GUI,
# tray menu, single instance, close behaviour). Requires Docker.
#
# Usage: build/linux/smoke-test.sh <package.deb> [image ...]
#        (default images: ubuntu:24.04 ubuntu:26.04)
# Logs and screenshots are written to dist/smoke/<image>/.
set -euo pipefail

deb=$(realpath -e "${1:?usage: smoke-test.sh <package.deb> [image ...]}")
shift
if [[ ! -f $deb || $deb != *.deb ]]; then
	echo "not a .deb file: $deb" >&2
	exit 1
fi
images=("$@")
((${#images[@]})) || images=(ubuntu:24.04 ubuntu:26.04)
here=$(cd "$(dirname "$0")" && pwd)
name=$(basename "$deb")

for image in "${images[@]}"; do
	out="$(dirname "$deb")/smoke/${image//[:\/]/-}"
	rm -rf "$out"
	mkdir -p "$out"
	echo "######## $image"
	# WebKitGTK sandboxes its web processes with bubblewrap, which needs user
	# namespaces; Docker's default seccomp/AppArmor profiles forbid them (an
	# Ubuntu desktop allows them) and mask /proc, so the container runs without
	# those restrictions.
	docker run --rm \
		--security-opt seccomp=unconfined --security-opt apparmor=unconfined --security-opt systempaths=unconfined \
		-e HOST_UID="$(id -u)" -e HOST_GID="$(id -g)" \
		-v "$deb:/pkg/$name:ro" -v "$here/smoke:/smoke:ro" -v "$out:/out" \
		"$image" bash /smoke/in-container.sh "/pkg/$name" /out
done

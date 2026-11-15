#!/usr/bin/env bash
# Reports whether the tools needed to develop, test and package ProjectHub are present.
set -uo pipefail

missing=0
have() { # have <required|optional> <command> <hint>
	local path
	if path=$(command -v "$2"); then
		printf 'ok        %-22s %s\n' "$2" "$path"
	elif [[ $1 == required ]]; then
		printf 'MISSING   %-22s %s\n' "$2" "$3"
		missing=$((missing + 1))
	else
		printf 'optional  %-22s %s\n' "$2" "$3"
	fi
}

echo "== Toolchain"
go version 2>/dev/null || true
printf 'node %s, npm %s\n' "$(node --version 2>/dev/null)" "$(npm --version 2>/dev/null)"
task --version 2>/dev/null || true
have required go "https://go.dev/dl/ (Go 1.26+)"
have required node "Node.js 22+ (https://nodejs.org/)"
have required npm "ships with Node.js"
have required task "go install github.com/go-task/task/v3/cmd/task@latest"
have required wails3 "go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0, then put \$(go env GOPATH)/bin first on PATH"
have required golangci-lint "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0"
have required git "sudo apt install git"
have optional govulncheck "go install golang.org/x/vuln/cmd/govulncheck@latest (task audit)"

echo
echo "== GTK4 / WebKitGTK 6.0 development libraries"
if command -v pkg-config >/dev/null && pkg-config --exists gtk4 webkitgtk-6.0; then
	echo "ok        gtk4 $(pkg-config --modversion gtk4), webkitgtk-6.0 $(pkg-config --modversion webkitgtk-6.0)"
else
	echo "MISSING   sudo apt install build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev"
	missing=$((missing + 1))
fi

echo
echo "== Packaging and verification (task deb)"
have required dpkg-deb "sudo apt install dpkg"
have optional dpkg-shlibdeps "sudo apt install dpkg-dev (checks runtime dependencies)"
have optional desktop-file-validate "sudo apt install desktop-file-utils"
have optional appstreamcli "sudo apt install appstream"
have optional lintian "sudo apt install lintian"
have optional dbus-daemon "sudo apt install dbus (tray tests use a private bus)"

echo
if command -v wails3 >/dev/null; then
	echo "== wails3 doctor"
	wails3 doctor || missing=$((missing + 1))
fi

echo
if ((missing > 0)); then
	echo "$missing required item(s) missing."
	exit 1
fi
echo "Environment is ready."

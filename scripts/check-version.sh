#!/usr/bin/env bash
# Verifies that the VERSION file matches every place that records the version.
# Usage: scripts/check-version.sh [vX.Y.Z]   (or TAG=vX.Y.Z in the environment;
#        the optional tag must equal v$VERSION)
set -euo pipefail
cd "$(dirname "$0")/.."

version=$(tr -d '[:space:]' <VERSION)
failures=0
check() { # check <description> <actual>
	if [[ $2 == "$version" ]]; then
		echo "ok    $1: $2"
	else
		echo "FAIL  $1: '$2', expected '$version'" >&2
		failures=$((failures + 1))
	fi
}

if [[ ! $version =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
	echo "FAIL  VERSION '$version' is not a semantic version" >&2
	exit 1
fi

check "build/config.yml info.version" "$(sed -nE 's/^  version: "([^"]*)".*/\1/p' build/config.yml)"
check "frontend/package.json" "$(sed -nE 's/^  "version": "([^"]*)".*/\1/p' frontend/package.json | head -1)"
check "frontend/package-lock.json" "$(sed -nE 's/^  "version": "([^"]*)".*/\1/p' frontend/package-lock.json | head -1)"
check "Debian changelog" "$(sed -nE '1s/^projecthub \(([^)]*)\).*/\1/p' build/linux/share/doc/projecthub/changelog)"
check "man page" "$(sed -nE 's/^\.TH PROJECTHUB 1 "[^"]*" "ProjectHub ([^"]*)".*/\1/p' build/linux/share/man/man1/projecthub.1)"
# "|| true": a missing entry must be reported by check(), not abort the script.
check "AppStream newest release" "$({ grep -m1 -oE '<release version="[^"]*"' build/linux/share/metainfo/io.github.semelyanov86.projecthub.metainfo.xml || true; } | cut -d'"' -f2)"
check "CHANGELOG.md newest release" "$({ grep -m1 -oE '^## \[[0-9][^]]*\]' CHANGELOG.md || true; } | tr -d '#[] ')"
tag=${1:-${TAG:-}}
if [[ -n $tag ]]; then
	check "git tag" "${tag#v}"
	[[ $tag == v* ]] || {
		echo "FAIL  tag '$tag' must start with v" >&2
		failures=$((failures + 1))
	}
fi

if ((failures > 0)); then
	echo "$failures version check(s) failed" >&2
	exit 1
fi
echo "Version $version is consistent."

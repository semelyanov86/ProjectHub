#!/usr/bin/env bash
# Verifies a ProjectHub .deb before it is published: control metadata,
# installed files and permissions, runtime dependencies (against
# dpkg-shlibdeps), desktop entry, AppStream metadata, and the binary itself.
#
# Usage: build/linux/verify-deb.sh <package.deb> <expected-version>
# Optional tools (checks are skipped with a note when missing):
#   desktop-file-validate (desktop-file-utils), appstreamcli (appstream),
#   dpkg-shlibdeps (dpkg-dev), lintian.
set -euo pipefail

deb=${1:?usage: verify-deb.sh <package.deb> <expected-version>}
version=${2:?usage: verify-deb.sh <package.deb> <expected-version>}
app_id=io.github.semelyanov86.projecthub
failures=0

fail() {
	echo "FAIL  $*" >&2
	failures=$((failures + 1))
}
pass() { echo "ok    $*"; }
skip() { echo "skip  $*"; }

if [[ ! -f $deb ]]; then
	echo "FAIL  $deb does not exist" >&2
	exit 1
fi

field() { dpkg-deb --field "$deb" "$1"; }

# --- Control metadata -------------------------------------------------------
arch=$(field Architecture)
[[ $(field Package) == projecthub ]] && pass "Package: projecthub" || fail "Package is '$(field Package)'"
[[ $(field Version) == "$version" ]] && pass "Version: $version" || fail "Version is '$(field Version)', expected $version"
case $arch in
amd64 | arm64) pass "Architecture: $arch" ;;
*) fail "unexpected Architecture '$arch'" ;;
esac
[[ $(basename "$deb") == "projecthub_${version}_${arch}.deb" ]] &&
	pass "file name matches projecthub_${version}_${arch}.deb" ||
	fail "file name $(basename "$deb") does not match projecthub_${version}_${arch}.deb"
for name in Maintainer Description Homepage Section Priority Installed-Size; do
	value=$(field "$name")
	[[ -n $value ]] && pass "$name: ${value%%$'\n'*}" || fail "$name is missing"
done
[[ $(field Maintainer) =~ ^[^\<]+\ \<[^@\>]+@[^\>]+\>$ ]] || fail "Maintainer is not 'Name <email>'"

depends=$(field Depends)
declared=$(tr ',' '\n' <<<"$depends" | sed -E 's/^ +//; s/ .*$//; s/:.*$//' | sort -u)
for required in libc6 libglib2.0-0t64 libgtk-4-1 libwebkitgtk-6.0-4 libjavascriptcoregtk-6.0-1 libsoup-3.0-0; do
	grep -qx "$required" <<<"$declared" && pass "Depends on $required" || fail "Depends lacks $required"
done

# --- Contents ---------------------------------------------------------------
contents=$(dpkg-deb --contents "$deb")
expect() { # expect <path> <mode>
	local line
	line=$(awk -v p=".$1" '$6 == p' <<<"$contents")
	if [[ -z $line ]]; then
		fail "missing $1"
	elif [[ $(awk '{print $1, $2}' <<<"$line") != "$2 root/root" ]]; then
		fail "$1 has $(awk '{print $1, $2}' <<<"$line"), expected $2 root/root"
	else
		pass "$1 ($2)"
	fi
}
expect /usr/bin/projecthub -rwxr-xr-x
expect "/usr/share/applications/$app_id.desktop" -rw-r--r--
expect "/usr/share/metainfo/$app_id.metainfo.xml" -rw-r--r--
expect /usr/share/icons/hicolor/scalable/apps/projecthub.svg -rw-r--r--
for size in 16 22 24 32 48 64 128 256 512; do
	expect "/usr/share/icons/hicolor/${size}x${size}/apps/projecthub.png" -rw-r--r--
done
expect /usr/share/man/man1/projecthub.1.gz -rw-r--r--
expect /usr/share/doc/projecthub/copyright -rw-r--r--
expect /usr/share/doc/projecthub/changelog.gz -rw-r--r--

if grep -Eq ' \./(home|root|tmp|usr/local|etc/skel|var)(/|$)' <<<"$contents"; then
	fail "package installs files outside system locations:"
	grep -E ' \./(home|root|tmp|usr/local|etc/skel|var)(/|$)' <<<"$contents" >&2
else
	pass "no files in home directories, /usr/local, /etc/skel or /var"
fi
if dpkg-deb --ctrl-tarfile "$deb" | tar -t | grep -Eq '(pre|post)(inst|rm)$'; then
	fail "unexpected maintainer scripts"
else
	pass "no maintainer scripts (dpkg triggers refresh menus and icon caches)"
fi

# --- Extracted files --------------------------------------------------------
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
dpkg-deb -x "$deb" "$work/root"
desktop="$work/root/usr/share/applications/$app_id.desktop"
metainfo="$work/root/usr/share/metainfo/$app_id.metainfo.xml"

grep -qx 'Exec=projecthub' "$desktop" && pass "desktop entry runs projecthub from PATH" || fail "desktop Exec is not 'projecthub'"
grep -qx 'Icon=projecthub' "$desktop" && pass "desktop entry uses the projecthub icon" || fail "desktop Icon is not 'projecthub'"
grep -qx 'Terminal=false' "$desktop" && pass "desktop entry launches without a terminal" || fail "desktop Terminal is not false"
grep -q "<release version=\"$version\"" "$metainfo" && pass "metainfo lists release $version" || fail "metainfo has no <release version=\"$version\">"
# sed reads the whole stream, so zcat never dies of SIGPIPE under pipefail.
first_line=$(zcat "$work/root/usr/share/doc/projecthub/changelog.gz" | sed -n '1p')
[[ $first_line == "projecthub ($version)"* ]] &&
	pass "changelog.gz starts with projecthub ($version)" || fail "changelog.gz does not start with projecthub ($version)"

if command -v desktop-file-validate >/dev/null; then
	if out=$(desktop-file-validate "$desktop" 2>&1) && ! grep -q 'error:' <<<"$out"; then
		pass "desktop-file-validate${out:+ (${out##*: })}"
	else
		fail "desktop-file-validate: $out"
	fi
else
	skip "desktop-file-validate not installed"
fi
if command -v appstreamcli >/dev/null; then
	if out=$(appstreamcli validate --no-net "$metainfo" 2>&1); then
		pass "appstreamcli validate"
	else
		fail "appstreamcli validate: $out"
	fi
else
	skip "appstreamcli not installed"
fi

host_arch=$(dpkg --print-architecture)
binary="$work/root/usr/bin/projecthub"
if [[ $arch == "$host_arch" ]]; then
	if out=$("$binary" --version 2>&1) && [[ $out == "ProjectHub $version" ]]; then
		pass "projecthub --version prints '$out'"
	else
		fail "projecthub --version printed '$out'"
	fi
	if missing=$(ldd "$binary" | grep 'not found'); then
		fail "unresolved libraries: $missing"
	else
		pass "ldd resolves every library on this host"
	fi
	if command -v dpkg-shlibdeps >/dev/null; then
		mkdir -p "$work/shlibs/debian"
		printf 'Source: projecthub\n\nPackage: projecthub\nArchitecture: any\n' >"$work/shlibs/debian/control"
		if ! shlibs=$(cd "$work/shlibs" && dpkg-shlibdeps -O "$binary" 2>"$work/shlibs.err"); then
			fail "dpkg-shlibdeps failed: $(cat "$work/shlibs.err")"
		else
			computed=$(sed -n 's/^shlibs:Depends=//p' <<<"$shlibs")
			missing_deps=""
			for dep in $(tr ',' '\n' <<<"$computed" | sed -E 's/^ +//; s/ .*$//'); do
				grep -qx "$dep" <<<"$declared" || missing_deps+=" $dep"
			done
			if [[ -z $computed ]]; then
				fail "dpkg-shlibdeps reported no dependencies for the binary"
			elif [[ -n $missing_deps ]]; then
				fail "dpkg-shlibdeps requires packages missing from Depends:$missing_deps (computed: $computed)"
			else
				pass "Depends covers dpkg-shlibdeps: $computed"
			fi
		fi
	else
		skip "dpkg-shlibdeps not installed (dpkg-dev)"
	fi
else
	skip "binary checks: package is $arch, host is $host_arch"
fi

if command -v lintian >/dev/null; then
	# Informational: tags are printed for review; only errors fail the check.
	lint=$(lintian --tag-display-limit 0 "$deb" 2>&1 || true)
	if grep -q '^E:' <<<"$lint"; then
		fail "lintian reported errors:"
		echo "$lint" >&2
	else
		pass "lintian: no errors${lint:+ (other tags below)}"
		[[ -n $lint ]] && sed 's/^/      /' <<<"$lint"
	fi
else
	skip "lintian not installed"
fi

echo
echo "Package: $deb"
echo "Size:    $(stat -c %s "$deb") bytes"
echo "SHA256:  $(sha256sum "$deb" | cut -d' ' -f1)"
if ((failures > 0)); then
	echo "$failures check(s) failed" >&2
	exit 1
fi
echo "All package checks passed."

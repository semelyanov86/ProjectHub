#!/usr/bin/env bash
# Installs a ProjectHub .deb into a clean Ubuntu container and smoke-tests it:
# package installation and removal, the installed files, and the GUI under X11
# (Xvfb + Openbox) and Wayland (headless Weston), including the tray menu
# (against a stand-in StatusNotifierWatcher), single-instance focusing and the
# close-to-tray behaviour. Run by build/linux/smoke-test.sh; not meant for a
# developer machine.
#
# Usage (as root inside the container): in-container.sh <package.deb> <output-dir>
set -euo pipefail

deb=$1
out=$2
work=/tmp/smoke-out
export DEBIAN_FRONTEND=noninteractive
# Logs and screenshots are copied to the mounted output directory and handed
# to the invoking host user, also when a check fails.
publish() {
	if [[ -d $work ]]; then
		cp -r "$work/." "$out/" 2>/dev/null || true
	fi
	chown -R "${HOST_UID:-0}:${HOST_GID:-0}" "$out" 2>/dev/null || true
}
trap publish EXIT
step() { printf '\n== %s\n' "$*"; }
pass() { echo "ok    $*"; }
die() {
	echo "FAIL  $*" >&2
	exit 1
}

step "System"
grep PRETTY_NAME /etc/os-release

# Docker's minimal Ubuntu images tell dpkg to skip man pages and docs; a
# desktop installation does not, so the test installs like a desktop does.
rm -f /etc/dpkg/dpkg.cfg.d/excludes

step "Install $(basename "$deb") with apt (dependencies from the Ubuntu archive)"
apt-get update -qq
apt-get install -y -qq --no-install-recommends "$deb" >/tmp/apt-install.log 2>&1 ||
	{
		cat /tmp/apt-install.log
		die "apt-get install failed"
	}
dpkg-query -W -f='${Package} ${Version} ${Status}\n' projecthub
[[ $(command -v projecthub) == /usr/bin/projecthub ]] && pass "projecthub is /usr/bin/projecthub" || die "projecthub not on PATH"
[[ $(projecthub --version) == "ProjectHub $(dpkg-query -W -f='${Version}' projecthub)" ]] &&
	pass "projecthub --version: $(projecthub --version)" || die "unexpected --version output"
for file in /usr/share/applications/io.github.semelyanov86.projecthub.desktop \
	/usr/share/icons/hicolor/256x256/apps/projecthub.png /usr/share/icons/hicolor/scalable/apps/projecthub.svg \
	/usr/share/metainfo/io.github.semelyanov86.projecthub.metainfo.xml /usr/share/man/man1/projecthub.1.gz; do
	[[ -f $file ]] && pass "installed $file" || die "missing $file"
done
if missing=$(ldd /usr/bin/projecthub | grep 'not found'); then die "unresolved libraries: $missing"; fi
pass "all shared libraries resolve"
[[ -z $(find /root /home /etc/skel -path '*projecthub*' 2>/dev/null) ]] && pass "nothing installed into home directories" ||
	die "package created files in home directories"

step "Install GUI test tools"
apt-get install -y -qq --no-install-recommends xvfb xauth x11-utils x11-apps wmctrl openbox dbus dbus-x11 \
	python3-gi gir1.2-glib-2.0 weston procps git >/tmp/apt-tools.log 2>&1 ||
	{
		cat /tmp/apt-tools.log
		die "installing test tools failed"
	}
useradd --create-home --shell /bin/bash tester
install -d -o tester -m 0700 /tmp/runtime-tester
install -d -o tester "$work"

step "GUI tests as an unprivileged user"
runuser -u tester -- env HOME=/home/tester XDG_RUNTIME_DIR=/tmp/runtime-tester OUT="$work" \
	bash /smoke/gui-tests.sh

step "Remove the package"
apt-get remove -y -qq projecthub >/dev/null
[[ ! -e /usr/bin/projecthub && ! -e /usr/share/applications/io.github.semelyanov86.projecthub.desktop ]] &&
	pass "binary and desktop entry removed" || die "files left behind after removal"
[[ -z $(dpkg-query -W -f='${Status}' projecthub 2>/dev/null | grep ' installed') ]] && pass "package no longer installed" ||
	die "package still installed after apt-get remove"
[[ -f /home/tester/.config/projecthub/projects.json ]] && pass "user configuration kept in ~/.config/projecthub" ||
	die "package removal deleted the user configuration"
echo
echo "All container checks passed."

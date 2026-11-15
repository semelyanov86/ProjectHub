#!/usr/bin/env bash
# GUI smoke tests for an installed ProjectHub, run as an unprivileged user by
# in-container.sh. Phases run on private session buses and virtual displays:
#   x11      Xvfb + Openbox: first launch, config creation, single instance,
#            tray menu, close-to-tray, quit-on-close without a tray host
#   wayland  headless Weston: Wayland app id/title, single instance
set -euo pipefail

OUT=${OUT:?}
SMOKE=/smoke
APP_ID=io.github.semelyanov86.projecthub
CONFIG=$HOME/.config/projecthub

pass() { echo "ok    $*"; }
die() {
	echo "FAIL  $*" >&2
	for log in "$OUT"/app-*.log; do
		[[ -f $log ]] && {
			echo "--- tail $log" >&2
			tail -n 40 "$log" >&2
		}
	done
	exit 1
}
wait_for() { # wait_for <seconds> <command...>
	local deadline=$((SECONDS + $1))
	shift
	until "$@" >/dev/null 2>&1; do
		((SECONDS < deadline)) || return 1
		sleep 0.5
	done
}
not_running() { ! kill -0 "$1" 2>/dev/null; }
instances() { pgrep -c -u "$(id -u)" -x projecthub || true; }

x11_window() { xwininfo -root -tree | awk '/"ProjectHub": \(/ {print $1; exit}'; }
x11_has_window() { [[ -n $(x11_window) ]]; }
map_state() { xwininfo -id "$1" | awk '/Map State:/ {print $3}'; }
viewable() { [[ $(map_state "$1") == IsViewable ]]; }
hidden() { [[ $(map_state "$1") != IsViewable ]]; }

start_app() { # start_app <log>
	projecthub >"$1" 2>&1 &
	APP_PID=$!
}

stop_app() {
	kill "$APP_PID" 2>/dev/null || true
	wait_for 10 not_running "$APP_PID" || kill -9 "$APP_PID" 2>/dev/null || true
}

seed_catalog() {
	mkdir -p "$HOME/work/alpha" "$HOME/work/beta"
	git -C "$HOME/work/alpha" init -q -b main
	git -C "$HOME/work/beta" init -q -b develop
	cat >"$CONFIG/projects.json" <<-EOF
		{"schemaVersion": 2, "projects": [
		  {"id": "p-alpha", "name": "alpha_service", "description": "", "path": "$HOME/work/alpha",
		   "favorite": true, "defaultEditor": "", "lastOpened": "2026-11-14T10:00:00Z",
		   "createdAt": "2026-11-01T10:00:00Z", "updatedAt": "2026-11-01T10:00:00Z"},
		  {"id": "p-beta", "name": "beta", "description": "", "path": "$HOME/work/beta",
		   "favorite": false, "defaultEditor": "", "lastOpened": "2026-11-15T09:00:00Z",
		   "createdAt": "2026-11-01T10:00:00Z", "updatedAt": "2026-11-01T10:00:00Z"}
		]}
	EOF
}

set_close_behavior() {
	python3 - "$CONFIG/settings.json" "$1" <<-'EOF'
		import json, sys
		path, value = sys.argv[1], sys.argv[2]
		data = json.load(open(path))
		data["closeBehavior"] = value
		json.dump(data, open(path, "w"), indent=2)
	EOF
}

phase_x11() {
	openbox >/dev/null 2>&1 &
	sleep 1

	echo "-- first launch (no tray host)"
	start_app "$OUT/app-x11.log"
	wait_for 40 x11_has_window || die "no ProjectHub window appeared on X11"
	local wid
	wid=$(x11_window)
	pass "X11 window $wid mapped: $(xprop -id "$wid" WM_CLASS | cut -d= -f2)"
	xprop -id "$wid" WM_CLASS | grep -q '"projecthub"' && pass "WM_CLASS contains projecthub (matches StartupWMClass)" ||
		die "WM_CLASS lacks projecthub"
	wait_for 10 test -f "$CONFIG/projects.json" || die "configuration was not created on first launch"
	python3 -c 'import json,sys; [json.load(open(p)) for p in sys.argv[1:]]' "$CONFIG/settings.json" "$CONFIG/projects.json" &&
		pass "first launch created settings.json and projects.json (mode $(stat -c %a "$CONFIG/settings.json"))" ||
		die "created configuration is not valid JSON"
	sleep 4
	xwd -root -silent >"$OUT/x11-first-launch.xwd"

	timeout 15 projecthub >"$OUT/app-x11-second.log" 2>&1 || die "second instance did not exit cleanly"
	[[ $(instances) == 1 ]] && pass "second launch exited and left one instance" || die "$(instances) instances running"

	wmctrl -c ProjectHub
	wait_for 10 not_running "$APP_PID" && pass "without a tray host, closing the window quits" ||
		die "closing the window without a tray host did not quit"

	echo "-- tray host available"
	python3 "$SMOKE/fake_watcher.py" >"$OUT/watcher.log" 2>&1 &
	local watcher=$!
	wait_for 10 grep -q owned "$OUT/watcher.log" || die "fake StatusNotifierWatcher did not start"
	seed_catalog
	start_app "$OUT/app-x11-tray.log"
	wait_for 40 x11_has_window || die "no window with tray host"
	wid=$(x11_window)
	wait_for 15 grep -q registered "$OUT/watcher.log" || die "tray item never registered"
	local item
	item=$(awk '/registered/ {split($2, a, "/"); print a[1]; exit}' "$OUT/watcher.log")
	pass "tray item registered as $item"
	wait_for 10 sh -c "python3 '$SMOKE/tray_client.py' '$item' labels | grep -q beta" || die "recent projects missing from tray menu"
	python3 "$SMOKE/tray_client.py" "$item" labels | tee "$OUT/tray-menu.txt"
	[[ $(cut -f3 "$OUT/tray-menu.txt" | tr '\n' '|') == "Open ProjectHub|Recent Projects|beta|alpha__service|Settings…|Quit ProjectHub|" ]] &&
		pass "tray menu: Open, recent projects newest first (underscore escaped), Settings, Quit" ||
		die "unexpected tray menu layout"

	wmctrl -c ProjectHub
	wait_for 10 hidden "$wid" || die "window was not hidden on close"
	sleep 1
	kill -0 "$APP_PID" && pass "closing the window hides it; ProjectHub keeps running in the tray" ||
		die "ProjectHub exited instead of hiding to the tray"
	timeout 15 projecthub >/dev/null 2>&1
	wait_for 10 viewable "$wid" && pass "second launch shows the hidden window again" || die "second launch did not show the window"

	wmctrl -c ProjectHub
	wait_for 10 hidden "$wid" || die "window not hidden"
	python3 "$SMOKE/tray_client.py" "$item" click "Open ProjectHub" >/dev/null
	wait_for 10 viewable "$wid" && pass "tray 'Open ProjectHub' shows the window" || die "tray Open did not show the window"

	wmctrl -c ProjectHub
	wait_for 10 hidden "$wid" || die "window not hidden"
	python3 "$SMOKE/tray_client.py" "$item" click "Settings…" >/dev/null
	wait_for 10 viewable "$wid" && pass "tray 'Settings…' shows the window" || die "tray Settings did not show the window"
	sleep 2
	xwd -root -silent >"$OUT/x11-tray-settings.xwd"

	wmctrl -c ProjectHub
	wait_for 10 hidden "$wid" || die "window not hidden"
	python3 "$SMOKE/tray_client.py" "$item" click "beta" >/dev/null
	wait_for 15 viewable "$wid" && pass "tray recent project without an installed editor shows the window with the error" ||
		die "failed tray launch did not surface the window"
	sleep 2
	xwd -root -silent >"$OUT/x11-tray-launch-error.xwd"

	python3 "$SMOKE/tray_client.py" "$item" click "Quit ProjectHub" >/dev/null
	wait_for 10 not_running "$APP_PID" && pass "tray 'Quit ProjectHub' exits" || die "tray Quit did not exit"

	echo "-- closeBehavior=quit with a tray host"
	set_close_behavior quit
	start_app "$OUT/app-x11-quit.log"
	wait_for 40 x11_has_window || die "no window"
	sleep 2
	wmctrl -c ProjectHub
	wait_for 10 not_running "$APP_PID" && pass "closeBehavior=quit quits even with a tray host" || die "closeBehavior=quit did not quit"
	set_close_behavior tray
	kill "$watcher"
}

phase_wayland() {
	unset DISPLAY
	weston --backend=headless --socket=wayland-smoke --idle-time=0 >"$OUT/weston.log" 2>&1 &
	local weston=$!
	wait_for 15 test -S "$XDG_RUNTIME_DIR/wayland-smoke" || die "headless Weston did not start"
	export WAYLAND_DISPLAY=wayland-smoke
	WAYLAND_DEBUG=client GDK_BACKEND=wayland projecthub >"$OUT/app-wayland.log" 2>&1 &
	APP_PID=$!
	wait_for 40 grep -q "set_app_id(\"$APP_ID\")" "$OUT/app-wayland.log" ||
		die "no xdg_toplevel with app id $APP_ID on Wayland"
	pass "Wayland toplevel app_id is $APP_ID (matches the desktop file id)"
	wait_for 10 grep -q 'set_title("ProjectHub")' "$OUT/app-wayland.log" && pass "Wayland toplevel title is ProjectHub" ||
		die "Wayland title not set"
	sleep 5
	kill -0 "$APP_PID" && pass "ProjectHub keeps running under Wayland" || die "ProjectHub exited under Wayland"
	timeout 15 projecthub >/dev/null 2>&1 || die "second Wayland instance did not exit cleanly"
	[[ $(instances) == 1 ]] && pass "single instance under Wayland" || die "$(instances) instances under Wayland"
	stop_app
	kill "$weston" 2>/dev/null || true
	# Keep only the interesting part of the very verbose protocol log.
	grep -E 'set_app_id|set_title|error' "$OUT/app-wayland.log" | head -20 >"$OUT/app-wayland-summary.log" || true
	rm -f "$OUT/app-wayland.log"
}

case ${1:-all} in
x11) phase_x11 ;;
wayland) phase_wayland ;;
all)
	echo "-- X11 (Xvfb + Openbox)"
	dbus-run-session -- xvfb-run -a -s "-screen 0 1280x800x24" bash "$0" x11
	echo "-- Wayland (headless Weston)"
	dbus-run-session -- bash "$0" wayland
	;;
esac

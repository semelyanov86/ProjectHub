package tray

import (
	"bufio"
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// privateBus starts an isolated session bus so tests never touch the
// developer's desktop session. It skips when dbus-daemon is not installed.
func privateBus(t *testing.T) string {
	t.Helper()
	daemon, err := exec.LookPath("dbus-daemon")
	if err != nil {
		t.Skip("dbus-daemon not installed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	//nolint:gosec // The test runs the dbus-daemon found on PATH with fixed arguments.
	cmd := exec.CommandContext(ctx, daemon, "--session", "--nofork", "--nopidfile", "--print-address=1")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		t.Skipf("cannot start dbus-daemon: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
	})
	address, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("read bus address: %v", err)
	}
	return strings.TrimSpace(address)
}

func connect(t *testing.T, address string) *dbus.Conn {
	t.Helper()
	conn, err := dbus.Connect(address)
	if err != nil {
		t.Fatalf("connect to private bus: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// exportWatcher publishes a minimal StatusNotifierWatcher on its own connection.
func exportWatcher(t *testing.T, address string, hostRegistered bool) {
	t.Helper()
	conn := connect(t, address)
	_, err := prop.Export(conn, watcherPath, map[string]map[string]*prop.Prop{
		watcherName: {
			"IsStatusNotifierHostRegistered": {Value: hostRegistered, Emit: prop.EmitFalse},
		},
	})
	if err != nil {
		t.Fatalf("export watcher properties: %v", err)
	}
	reply, err := conn.RequestName(watcherName, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		t.Fatalf("own %s: reply %v, err %v", watcherName, reply, err)
	}
}

func TestHostAvailable(t *testing.T) {
	cases := []struct {
		name    string
		watcher bool
		host    bool
		want    bool
	}{
		{"no watcher (stock GNOME)", false, false, false},
		{"watcher without host", true, false, false},
		{"watcher with host", true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			address := privateBus(t)
			if tc.watcher {
				exportWatcher(t, address, tc.host)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if got := hostAvailable(ctx, connect(t, address)); got != tc.want {
				t.Errorf("hostAvailable() = %v, want %v", got, tc.want)
			}
		})
	}
}

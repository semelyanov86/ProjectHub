package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const homeEntry = "HOME=/home/dev"

func TestSanitizeEnvStripsRuntimeVariables(t *testing.T) {
	env := []string{
		homeEntry,
		"DISPLAY=:0",
		"WAYLAND_DISPLAY=wayland-0",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus",
		"GDK_BACKEND=x11",
		"WEBKIT_DISABLE_DMABUF_RENDERER=1",
		"WAILS_DEV=1",
		"XDG_ACTIVATION_TOKEN=abc",
		"DESKTOP_STARTUP_ID=xyz",
		"FRONTEND_DEVSERVER_URL=http://127.0.0.1:9245",
		"GIO_LAUNCHED_DESKTOP_FILE=/usr/share/applications/projecthub.desktop",
		"malformed",
	}

	got := SanitizeEnv(env)
	want := []string{
		homeEntry,
		"DISPLAY=:0",
		"WAYLAND_DISPLAY=wayland-0",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("SanitizeEnv() = %v, want %v", got, want)
	}
}

func TestSanitizeEnvDropsAppImageTaintedValues(t *testing.T) {
	env := []string{
		"APPDIR=/tmp/.mount_ProjectHub",
		"LD_LIBRARY_PATH=/tmp/.mount_ProjectHub/usr/lib:/usr/lib",
		"GTK_PATH=/tmp/.mount_ProjectHub/usr/lib/gtk-4.0",
		"PATH=/usr/local/bin:/usr/bin",
	}

	got := SanitizeEnv(env)
	if !slices.Equal(got, []string{"PATH=/usr/local/bin:/usr/bin"}) {
		t.Fatalf("SanitizeEnv() = %v", got)
	}
}

func TestWithVariablesOverridesExistingEntries(t *testing.T) {
	got := WithVariables([]string{"LC_ALL=de_DE.UTF-8", homeEntry}, "LC_ALL=C", "GIT_TERMINAL_PROMPT=0")
	want := []string{homeEntry, "LC_ALL=C", "GIT_TERMINAL_PROMPT=0"}
	if !slices.Equal(got, want) {
		t.Fatalf("WithVariables() = %v, want %v", got, want)
	}
}

func TestLimitedBufferTruncates(t *testing.T) {
	buf := &limitedBuffer{limit: 4}
	n, err := buf.Write([]byte("abcdef"))
	if err != nil || n != 6 {
		t.Fatalf("Write() = (%d, %v), want (6, nil)", n, err)
	}
	if _, err := buf.Write([]byte("gh")); err != nil {
		t.Fatalf("second Write() error: %v", err)
	}
	if string(buf.Bytes()) != "abcd" || !buf.truncated {
		t.Fatalf("buffer = %q truncated=%v, want %q truncated=true", buf.Bytes(), buf.truncated, "abcd")
	}
}

func lookTrue(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("true")
	if err != nil {
		t.Skip("true(1) not available")
	}
	return path
}

func TestOSRunnerRunCapturesOutputAndExitCode(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	runner := &OSRunner{OutputLimit: 8}
	dir := t.TempDir()

	// The script is a test fixture constant, not derived from user input.
	res, err := runner.Run(t.Context(), Command{
		Path: sh,
		Args: []string{"-c", `pwd; echo oops >&2; exit 3`},
		Dir:  dir,
		Env:  []string{},
	})
	if err == nil {
		t.Fatal("Run() expected error for non-zero exit")
	}
	if _, ok := errors.AsType[*exec.ExitError](err); !ok {
		t.Fatalf("Run() error %v does not wrap *exec.ExitError", err)
	}
	if res.ExitCode != 3 {
		t.Fatalf("ExitCode = %d, want 3", res.ExitCode)
	}
	if !res.Truncated || len(res.Stdout) != 8 {
		t.Fatalf("stdout = %q truncated=%v, want 8 bytes truncated", res.Stdout, res.Truncated)
	}
	if !strings.HasPrefix(dir+"\n", string(res.Stdout)) {
		t.Fatalf("stdout %q does not start with working dir %q", res.Stdout, dir)
	}
	if strings.TrimSpace(string(res.Stderr)) != "oops" {
		t.Fatalf("stderr = %q, want oops", res.Stderr)
	}
}

func TestOSRunnerRunHonoursContextTimeout(t *testing.T) {
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not available")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err = NewOSRunner().Run(ctx, Command{Path: sleep, Args: []string{"5"}, Env: []string{}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("Run() did not stop at the context deadline")
	}
}

func TestOSRunnerRunReportsMissingExecutable(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := NewOSRunner().Run(t.Context(), Command{Path: missing, Env: []string{}})
	if !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Run() error = %v, want ErrNotStarted", err)
	}
}

func TestOSRunnerStart(t *testing.T) {
	runner := NewOSRunner()
	if err := runner.Start(Command{Path: lookTrue(t), Dir: t.TempDir(), Env: []string{}}); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	if err := runner.Start(Command{Path: missing, Env: []string{}}); !errors.Is(err, ErrNotStarted) {
		t.Fatalf("Start() error = %v, want ErrNotStarted", err)
	}
}

// processGone reports whether pid no longer runs (absent or a reaped-pending zombie).
func processGone(pid int) bool {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return true
	}
	fields := strings.Fields(string(data))
	// Field 3 is the state; the comm field (2) has no spaces for "sleep".
	return len(fields) > 2 && fields[2] == "Z"
}

func waitGone(t *testing.T, pidFile string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Clean(pidFile))
	if err != nil {
		t.Fatalf("read pid file: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("parse pid %q: %v", raw, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !processGone(pid) {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("descendant process %d is still running", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestOSRunnerRunKillsDescendantsOnTimeout(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()

	// Fixed test script: a background child that would outlive its parent.
	_, err = NewOSRunner().Run(ctx, Command{
		Path: sh,
		Args: []string{"-c", `sleep 30 & echo $! > "$1"; wait`, "sh", pidFile},
		Env:  []string{"PATH=/usr/bin:/bin"},
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want context.DeadlineExceeded", err)
	}
	waitGone(t, pidFile)
}

func TestOSRunnerRunKillsDescendantsHoldingOutput(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("sh not available")
	}
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	runner := &OSRunner{WaitDelay: 200 * time.Millisecond}

	// The shell exits successfully while its background child keeps stdout open.
	res, err := runner.Run(t.Context(), Command{
		Path: sh,
		Args: []string{"-c", `sleep 30 & echo $! > "$1"; echo done`, "sh", pidFile},
		Env:  []string{"PATH=/usr/bin:/bin"},
	})
	if err != nil {
		t.Fatalf("Run() error = %v, want success for a zero exit", err)
	}
	if strings.TrimSpace(string(res.Stdout)) != "done" {
		t.Fatalf("stdout = %q, want done", res.Stdout)
	}
	waitGone(t, pidFile)
}

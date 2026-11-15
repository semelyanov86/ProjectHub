package app

import (
	"context"
	"os"
	"runtime"
	"testing"
)

func TestGetInfo(t *testing.T) {
	dir := t.TempDir()
	got := NewService(Options{Version: "1.2.3", ConfigDir: dir}).GetInfo()
	if got.Name != "ProjectHub" || got.Version != "1.2.3" || got.ConfigDir != dir {
		t.Errorf("GetInfo() = %+v, want ProjectHub version 1.2.3 with config dir %q", got, dir)
	}
	if got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
		t.Errorf("platform = %s/%s, want %s/%s", got.OS, got.Arch, runtime.GOOS, runtime.GOARCH)
	}
	if home, err := os.UserHomeDir(); err == nil && got.HomeDir != home {
		t.Errorf("HomeDir = %q, want %q", got.HomeDir, home)
	}
}

func TestTrayAvailable(t *testing.T) {
	if NewService(Options{}).TrayAvailable(context.Background()) {
		t.Error("TrayAvailable() without a probe = true, want false")
	}
	probe := func(context.Context) bool { return true }
	if !NewService(Options{TrayAvailable: probe}).TrayAvailable(context.Background()) {
		t.Error("TrayAvailable() = false, want the probe result true")
	}
}

package app

import (
	"os"
	"runtime"
	"testing"
)

func TestGetInfo(t *testing.T) {
	dir := t.TempDir()
	got := NewService(dir).GetInfo()
	if got.Name != "ProjectHub" || got.Version != Version || got.ConfigDir != dir {
		t.Errorf("GetInfo() = %+v, want ProjectHub version %q with config dir %q", got, Version, dir)
	}
	if got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
		t.Errorf("platform = %s/%s, want %s/%s", got.OS, got.Arch, runtime.GOOS, runtime.GOARCH)
	}
	if home, err := os.UserHomeDir(); err == nil && got.HomeDir != home {
		t.Errorf("HomeDir = %q, want %q", got.HomeDir, home)
	}
}

package launcher

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProjectDir(t *testing.T) {
	tmp := t.TempDir()
	spaceDir := filepath.Join(tmp, "my project (v2)")
	if err := os.Mkdir(spaceDir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	regularFile := filepath.Join(tmp, "README.md")
	if err := os.WriteFile(regularFile, []byte("hi"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := ValidateProjectDir(spaceDir + string(filepath.Separator))
	if err != nil {
		t.Fatalf("expected valid directory, got error: %v", err)
	}
	if got != spaceDir {
		t.Fatalf("got cleaned path %q, want %q", got, spaceDir)
	}

	badPaths := []string{
		"",
		"   ",
		"relative/dir",
		"/tmp/with\x00nul",
		filepath.Join(tmp, "does-not-exist"),
		regularFile,
	}
	for _, p := range badPaths {
		if _, err := ValidateProjectDir(p); !errors.Is(err, ErrInvalidTargetPath) {
			t.Fatalf("ValidateProjectDir(%q): expected ErrInvalidTargetPath, got %v", p, err)
		}
	}
}

func TestValidateProjectDirMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	_, err := ValidateProjectDir(path)
	if !errors.Is(err, ErrInvalidTargetPath) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected validation and filesystem errors, got %v", err)
	}
	if !strings.Contains(err.Error(), "no longer exists") || !strings.Contains(err.Error(), path) {
		t.Fatalf("missing descriptive path error: %v", err)
	}
}

func TestValidateExecutable(t *testing.T) {
	if got, err := ValidateExecutable("  ghostty "); err != nil || got != "ghostty" {
		t.Fatalf("expected ghostty, got %q, %v", got, err)
	}
	if got, err := ValidateExecutable("/opt/My Apps/zed"); err != nil || got != "/opt/My Apps/zed" {
		t.Fatalf("absolute path with spaces rejected: %q, %v", got, err)
	}

	bad := []string{
		"",
		"ghostty; rm -rf /",
		"code\n--help",
		"subl\x00",
		"nvim | cat",
		"subl -n",
	}
	for _, cmd := range bad {
		if _, err := ValidateExecutable(cmd); !errors.Is(err, ErrInvalidExecutable) {
			t.Fatalf("ValidateExecutable(%q): expected ErrInvalidExecutable, got %v", cmd, err)
		}
	}
}

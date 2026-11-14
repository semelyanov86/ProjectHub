package launcher

import (
	"errors"
	"os"
	"path/filepath"
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

func TestValidateExecutable(t *testing.T) {
	if got, err := ValidateExecutable("  ghostty "); err != nil || got != "ghostty" {
		t.Fatalf("expected ghostty, got %q, %v", got, err)
	}

	bad := []string{
		"",
		"ghostty; rm -rf /",
		"code\n--help",
		"subl\x00",
		"nvim | cat",
	}
	for _, cmd := range bad {
		if _, err := ValidateExecutable(cmd); !errors.Is(err, ErrInvalidExecutable) {
			t.Fatalf("ValidateExecutable(%q): expected ErrInvalidExecutable, got %v", cmd, err)
		}
	}
}

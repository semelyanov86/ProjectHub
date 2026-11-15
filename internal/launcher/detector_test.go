package launcher

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const (
	testZedName = string(TargetZed)
	zeditorName = "zeditor"
	missingName = "absent"
)

func fakeResolver(paths map[string]string) *Resolver {
	return &Resolver{
		LookPath: func(name string) (string, error) {
			if path, ok := paths[name]; ok {
				return path, nil
			}
			return "", exec.ErrNotFound
		},
		Stat: os.Stat,
	}
}

func writeExecutable(t *testing.T, path string, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, mode); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePATH(t *testing.T) {
	tests := []struct {
		name       string
		paths      map[string]string
		candidates []string
		expected   string
	}{
		{
			name: "path hit", paths: map[string]string{testZedName: "/usr/bin/zed"},
			candidates: []string{testZedName, zeditorName}, expected: "/usr/bin/zed",
		},
		{
			name: "alternative cli", paths: map[string]string{zeditorName: "/usr/bin/zeditor"},
			candidates: []string{testZedName, zeditorName}, expected: "/usr/bin/zeditor",
		},
		{
			name: "candidate order", paths: map[string]string{testZedName: "/bin/zed", zeditorName: "/bin/zeditor"},
			candidates: []string{testZedName, zeditorName}, expected: "/bin/zed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := fakeResolver(tt.paths).Resolve("", tt.candidates)
			if err != nil || got != tt.expected {
				t.Fatalf("Resolve() = %q, %v; want %q", got, err, tt.expected)
			}
		})
	}
}

func TestResolveRejectsRelativePATH(t *testing.T) {
	tests := []struct {
		name string
		path string
		err  error
	}{
		{name: "errdot", path: "/absolute/zed", err: exec.ErrDot},
		{name: "relative path", path: "bin/zed"},
		{name: "empty path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fakeResolver(nil)
			r.LookPath = func(string) (string, error) { return tt.path, tt.err }
			if _, err := r.Resolve("", []string{testZedName}); !errors.Is(err, ErrToolNotFound) {
				t.Fatalf("want ErrToolNotFound, got %v", err)
			}
		})
	}
}

func TestResolveCommonDirectories(t *testing.T) {
	for _, relative := range []string{".local/bin/zed", ".local/zed.app/bin/zed", "bin/zeditor"} {
		t.Run(relative, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, relative)
			writeExecutable(t, path, 0o755)
			r := fakeResolver(nil)
			r.SearchDirs = DefaultSearchDirs(home)[:3]
			got, err := r.Resolve("", []string{testZedName, zeditorName})
			if err != nil || got != path {
				t.Fatalf("Resolve() = %q, %v; want %q", got, err, path)
			}
		})
	}
}

func TestResolveSearchOrder(t *testing.T) {
	root := t.TempDir()
	first, second := filepath.Join(root, "first"), filepath.Join(root, "second")
	writeExecutable(t, filepath.Join(first, zeditorName), 0o755)
	writeExecutable(t, filepath.Join(second, testZedName), 0o755)
	r := fakeResolver(nil)
	r.SearchDirs = []string{first, second}
	got, err := r.Resolve("", []string{testZedName, zeditorName})
	if err != nil || got != filepath.Join(second, testZedName) {
		t.Fatalf("candidate priority: got %q, %v", got, err)
	}
	r.LookPath = fakeResolver(map[string]string{zeditorName: "/path/zeditor"}).LookPath
	got, err = r.Resolve("", []string{testZedName, zeditorName})
	if err != nil || got != "/path/zeditor" {
		t.Fatalf("PATH priority: got %q, %v", got, err)
	}
}

func TestResolveOverride(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "with spaces", "custom zed")
	nonExecutable := filepath.Join(root, "regular")
	writeExecutable(t, executable, 0o755)
	writeExecutable(t, nonExecutable, 0o600)
	tests := []struct {
		name     string
		override string
		expected string
		wantErr  error
	}{
		{name: "absolute with spaces", override: executable, expected: executable},
		{name: "bare override", override: "  custom  ", expected: "/bin/custom"},
		{name: "missing absolute", override: filepath.Join(root, "missing"), wantErr: ErrToolNotFound},
		{name: "non executable", override: nonExecutable, wantErr: ErrToolNotFound},
		{name: "directory", override: root, wantErr: ErrToolNotFound},
		{name: "missing bare", override: missingName, wantErr: ErrToolNotFound},
		{name: "relative with slash", override: "bin/zed", wantErr: ErrInvalidExecutable},
		{name: "shell metacharacters", override: "zed;echo bad", wantErr: ErrInvalidExecutable},
		{name: "shell variable", override: "/bin/$zed", wantErr: ErrInvalidExecutable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := fakeResolver(map[string]string{testZedName: "/bin/zed", "custom": "/bin/custom"})
			lookup := r.LookPath
			r.LookPath = func(name string) (string, error) {
				if name == testZedName {
					t.Fatal("override fell back to candidate")
				}
				return lookup(name)
			}
			got, err := r.Resolve(tt.override, []string{testZedName})
			if !errors.Is(err, tt.wantErr) || got != tt.expected {
				t.Fatalf("Resolve() = %q, %v; want %q, %v", got, err, tt.expected, tt.wantErr)
			}
		})
	}
}

func TestResolveBareOverrideSearchDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom")
	writeExecutable(t, path, 0o100)
	r := fakeResolver(nil)
	r.SearchDirs = []string{dir}
	got, err := r.Resolve("custom", nil)
	if err != nil || got != path {
		t.Fatalf("Resolve() = %q, %v", got, err)
	}
}

func TestResolveNoCandidates(t *testing.T) {
	if _, err := fakeResolver(nil).Resolve("", nil); !errors.Is(err, ErrToolNotFound) {
		t.Fatalf("want ErrToolNotFound, got %v", err)
	}
	_, err := fakeResolver(nil).Resolve("", []string{testZedName, zeditorName})
	if err == nil || !strings.Contains(err.Error(), testZedName) || !strings.Contains(err.Error(), zeditorName) {
		t.Fatalf("missing candidate names: %v", err)
	}
}

func TestDefaultSearchDirs(t *testing.T) {
	system := []string{"/usr/local/bin", "/usr/bin", "/opt/sublime_text", "/snap/bin"}
	tests := []struct {
		home     string
		expected []string
	}{
		{home: "", expected: system},
		{home: "/home/test user", expected: append([]string{
			"/home/test user/.local/bin",
			"/home/test user/.local/zed.app/bin", "/home/test user/bin",
		}, system...)},
	}
	for _, tt := range tests {
		if got := DefaultSearchDirs(tt.home); !reflect.DeepEqual(got, tt.expected) {
			t.Fatalf("DefaultSearchDirs(%q) = %v; want %v", tt.home, got, tt.expected)
		}
	}
}

func TestNewResolver(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	r := NewResolver()
	if r.LookPath == nil || r.Stat == nil || !reflect.DeepEqual(r.SearchDirs, DefaultSearchDirs(home)) {
		t.Fatalf("unexpected defaults: %+v", r)
	}
	t.Setenv("HOME", "")
	if r = NewResolver(); !reflect.DeepEqual(r.SearchDirs, DefaultSearchDirs("")) {
		t.Fatalf("home error directories: %v", r.SearchDirs)
	}
}

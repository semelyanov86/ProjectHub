package launcher

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Resolver locates executables on PATH and in common install directories.
type Resolver struct {
	LookPath   func(file string) (string, error)
	Stat       func(name string) (fs.FileInfo, error)
	SearchDirs []string
}

// NewResolver returns a resolver using the current user's install directories.
func NewResolver() *Resolver {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return &Resolver{LookPath: exec.LookPath, Stat: os.Stat, SearchDirs: DefaultSearchDirs(home)}
}

// DefaultSearchDirs returns common Linux executable locations in discovery order.
func DefaultSearchDirs(home string) []string {
	dirs := make([]string, 0, 7)
	if home != "" {
		dirs = append(dirs, filepath.Join(home, ".local/bin"), filepath.Join(home, ".local/zed.app/bin"),
			filepath.Join(home, "bin"))
	}
	return append(dirs, "/usr/local/bin", "/usr/bin", "/opt/sublime_text", "/snap/bin")
}

// Resolve finds an executable, treating a configured override as authoritative.
func (r *Resolver) Resolve(override string, candidates []string) (string, error) {
	override = strings.TrimSpace(override)
	if override != "" {
		if _, err := ValidateExecutable(override); err != nil {
			return "", err
		}
		if strings.Contains(override, "/") {
			return r.resolvePath(override)
		}
		candidates = []string{override}
	}
	for _, candidate := range candidates {
		if path, err := r.LookPath(candidate); err == nil && filepath.IsAbs(path) {
			return path, nil
		}
	}
	for _, candidate := range candidates {
		for _, dir := range r.SearchDirs {
			path := filepath.Join(dir, candidate)
			if filepath.IsAbs(path) && r.isExecutable(path) {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("%w: %q was not found in PATH or common install locations",
		ErrToolNotFound, strings.Join(candidates, ", "))
}

func (r *Resolver) resolvePath(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: configured path %q must be an absolute path", ErrInvalidExecutable, path)
	}
	if !r.isExecutable(path) {
		return "", fmt.Errorf("%w: configured path %q is not an executable file", ErrToolNotFound, path)
	}
	return filepath.Clean(path), nil
}

func (r *Resolver) isExecutable(path string) bool {
	info, err := r.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

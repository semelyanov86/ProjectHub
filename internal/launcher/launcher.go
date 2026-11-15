// Package launcher provides safe path validation and target types for launching
// external editors, Ghostty terminal, and the system file manager without shell
// interpolation.
package launcher

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidTargetPath indicates a project directory path failed security or existence checks.
	ErrInvalidTargetPath = errors.New("invalid target directory path")
	// ErrInvalidExecutable indicates an executable name or path is unsafe or invalid.
	ErrInvalidExecutable = errors.New("invalid executable name")
	// ErrToolNotFound indicates an application could not be located.
	ErrToolNotFound = errors.New("application not found")
	// ErrLaunchFailed indicates an application could not be launched.
	ErrLaunchFailed = errors.New("failed to launch application")
	// ErrUnknownTarget indicates an unsupported launch target.
	ErrUnknownTarget = errors.New("unknown launch target")
)

// ValidateProjectDir validates that dirPath is a clean, absolute path pointing to
// an existing directory on the local filesystem. It rejects empty paths and NUL bytes.
func ValidateProjectDir(dirPath string) (string, error) {
	if strings.TrimSpace(dirPath) == "" {
		return "", fmt.Errorf("%w: path is empty", ErrInvalidTargetPath)
	}
	if strings.ContainsRune(dirPath, '\x00') {
		return "", fmt.Errorf("%w: path contains NUL byte", ErrInvalidTargetPath)
	}
	if !filepath.IsAbs(dirPath) {
		return "", fmt.Errorf("%w: path %q must be absolute", ErrInvalidTargetPath, dirPath)
	}

	cleaned := filepath.Clean(dirPath)
	info, err := os.Stat(cleaned)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: directory %q no longer exists: %w", ErrInvalidTargetPath, cleaned, err)
		}
		if errors.Is(err, fs.ErrPermission) {
			return "", fmt.Errorf("%w: permission denied for %q: %w", ErrInvalidTargetPath, cleaned, err)
		}
		return "", fmt.Errorf("%w: %w", ErrInvalidTargetPath, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%w: %q is not a directory", ErrInvalidTargetPath, cleaned)
	}
	return cleaned, nil
}

// ValidateExecutable ensures a configured binary name or path does not contain
// whitespace-separated shell arguments, control characters, or NUL bytes.
func ValidateExecutable(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", fmt.Errorf("%w: executable is empty", ErrInvalidExecutable)
	}
	if strings.ContainsAny(trimmed, "\r\n\t\x00;|&$`") {
		return "", fmt.Errorf("%w: executable %q contains disallowed characters", ErrInvalidExecutable, name)
	}
	if !strings.Contains(trimmed, "/") && strings.Contains(trimmed, " ") {
		return "", fmt.Errorf("%w: %q must be a single command name or an absolute path, without arguments",
			ErrInvalidExecutable, name)
	}
	return trimmed, nil
}

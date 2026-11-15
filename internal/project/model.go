// Package project defines the project domain model, validation, and the
// catalog operations ProjectHub performs on registered projects.
package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/editor"
)

var (
	// ErrInvalidProject indicates a project failed structural validation.
	ErrInvalidProject = errors.New("invalid project")
	// ErrProjectNotFound indicates no registered project has the requested ID.
	ErrProjectNotFound = errors.New("project not found")
	// ErrDuplicateProject indicates the directory is already registered.
	ErrDuplicateProject = errors.New("directory is already registered")
	// ErrProjectChanged indicates a project was removed or moved concurrently.
	ErrProjectChanged = errors.New("project changed while the operation was running")
)

// Project represents a local software project registered in ProjectHub.
type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	Favorite    bool   `json:"favorite"`
	// DefaultEditor is the explicit per-project editor ID; "" inherits.
	DefaultEditor string `json:"defaultEditor"`
	// LastUsedEditor is the editor most recently used to open this project.
	LastUsedEditor string     `json:"lastUsedEditor,omitempty"`
	LastOpened     *time.Time `json:"lastOpened,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// Input carries user-editable project fields for add and update operations.
type Input struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Path          string `json:"path"`
	DefaultEditor string `json:"defaultEditor"`
	Favorite      bool   `json:"favorite"`
}

// Skip describes one directory or record that a batch operation did not add.
type Skip struct {
	Path   string `json:"path"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// BatchReport summarises a multi-project operation such as a drop or import.
type BatchReport struct {
	Added   []Project `json:"added"`
	Skipped []Skip    `json:"skipped"`
}

const (
	maxNameLength        = 200
	maxDescriptionLength = 2000
)

// Validate checks that the project fields satisfy required invariants.
func (p Project) Validate() error {
	id := strings.TrimSpace(p.ID)
	if id == "" || strings.ContainsAny(id, " \t\r\n/\\\x00") {
		return fmt.Errorf("%w: id must be a non-empty identifier without whitespace or path separators", ErrInvalidProject)
	}
	// Stored records are validated structurally only: length and single-line
	// limits apply to user input (see Catalog), so catalogs written by older
	// versions keep loading even if a record predates those limits.
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidProject)
	}
	if strings.ContainsRune(p.Name, '\x00') || strings.ContainsRune(p.Description, '\x00') {
		return fmt.Errorf("%w: name and description must not contain NUL bytes", ErrInvalidProject)
	}
	if p.Path == "" || strings.ContainsRune(p.Path, '\x00') {
		return fmt.Errorf("%w: path is required and must not contain NUL bytes", ErrInvalidProject)
	}
	if !filepath.IsAbs(p.Path) {
		return fmt.Errorf("%w: path %q must be absolute", ErrInvalidProject, p.Path)
	}
	if err := validateEditorRef("defaultEditor", p.DefaultEditor); err != nil {
		return err
	}
	if err := validateEditorRef("lastUsedEditor", p.LastUsedEditor); err != nil {
		return err
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("%w: createdAt is required", ErrInvalidProject)
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updatedAt is required", ErrInvalidProject)
	}
	return nil
}

// validateName applies the limits for names entered by the user.
func validateName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidProject)
	}
	if strings.ContainsAny(name, "\x00\r\n") {
		return fmt.Errorf("%w: name must be a single line without control characters", ErrInvalidProject)
	}
	if len([]rune(trimmed)) > maxNameLength {
		return fmt.Errorf("%w: name must be at most %d characters", ErrInvalidProject, maxNameLength)
	}
	return nil
}

// validateDescription applies the limits for descriptions entered by the user.
func validateDescription(description string) error {
	if strings.ContainsRune(description, '\x00') {
		return fmt.Errorf("%w: description contains NUL byte", ErrInvalidProject)
	}
	if len([]rune(description)) > maxDescriptionLength {
		return fmt.Errorf("%w: description must be at most %d characters", ErrInvalidProject, maxDescriptionLength)
	}
	return nil
}

func validateEditorRef(field, id string) error {
	if id == "" || editor.IsSupported(id) {
		return nil
	}
	return fmt.Errorf("%w: %s %q is not a supported editor", ErrInvalidProject, field, id)
}

// NormalizeEditorRef returns id when it names a supported editor and "" otherwise.
// It is used to migrate references to editors that ProjectHub no longer supports.
func NormalizeEditorRef(id string) string {
	if editor.IsSupported(id) {
		return id
	}
	return ""
}

// SuggestName returns the default project name for a directory: its base
// name, made valid as a project name (control characters such as newlines
// become spaces and the result is limited to the maximum name length).
func SuggestName(dir string) string {
	base := filepath.Base(filepath.Clean(dir))
	if base == "/" || base == "." || base == "" {
		return "Root"
	}
	name := strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, base))
	if runes := []rune(name); len(runes) > maxNameLength {
		name = strings.TrimSpace(string(runes[:maxNameLength]))
	}
	if name == "" {
		return "Project"
	}
	return name
}

// PathKey returns the duplicate-detection key for a directory path. Paths are
// compared lexically after filepath.Clean; symlink aliases stay distinct.
func PathKey(path string) string {
	return filepath.Clean(path)
}

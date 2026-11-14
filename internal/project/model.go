// Package project defines the core project domain model and foundational
// project catalog service for ProjectHub.
package project

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// ErrInvalidProject indicates a project failed structural validation.
var ErrInvalidProject = errors.New("invalid project")

// Project represents a local software project registered in ProjectHub.
type Project struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	Path          string     `json:"path"`
	Favorite      bool       `json:"favorite"`
	DefaultEditor string     `json:"defaultEditor"`
	LastOpened    *time.Time `json:"lastOpened,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// Validate checks that the project fields satisfy required invariants.
func (p Project) Validate() error {
	id := strings.TrimSpace(p.ID)
	if id == "" || strings.ContainsAny(id, " \t\r\n/\\\x00") {
		return fmt.Errorf("%w: id must be a non-empty identifier without whitespace or path separators", ErrInvalidProject)
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidProject)
	}
	if strings.ContainsRune(p.Name, '\x00') {
		return fmt.Errorf("%w: name contains NUL byte", ErrInvalidProject)
	}
	if p.Path == "" || strings.ContainsRune(p.Path, '\x00') {
		return fmt.Errorf("%w: path is required and must not contain NUL bytes", ErrInvalidProject)
	}
	if !filepath.IsAbs(p.Path) {
		return fmt.Errorf("%w: path %q must be absolute", ErrInvalidProject, p.Path)
	}
	if strings.ContainsAny(p.DefaultEditor, "\r\n\x00") {
		return fmt.Errorf("%w: defaultEditor contains invalid control characters", ErrInvalidProject)
	}
	if p.CreatedAt.IsZero() {
		return fmt.Errorf("%w: createdAt is required", ErrInvalidProject)
	}
	if p.UpdatedAt.IsZero() {
		return fmt.Errorf("%w: updatedAt is required", ErrInvalidProject)
	}
	return nil
}

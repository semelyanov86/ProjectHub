package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

const (
	// MaxImportFileSize limits imported projects documents to 10 MiB.
	MaxImportFileSize = 10 << 20
	// MaxImportProjects limits the number of projects in an imported document.
	MaxImportProjects = 10000
)

// ReadProjectsDocument reads, migrates and validates a schema-1 or schema-2 export.
// The absolute path must refer to a regular file within the import size and count limits.
// Unlike the local catalog, an export requires explicit schemaVersion and projects fields.
func (s *Store) ReadProjectsDocument(path string) ([]project.Project, error) {
	data, err := readImportFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		SchemaVersion *int            `json:"schemaVersion"`
		Projects      json.RawMessage `json:"projects"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrCorruptedConfig, path, err)
	}
	if document.SchemaVersion == nil {
		return nil, fmt.Errorf("%w: schemaVersion is required", ErrInvalidConfig)
	}
	version := *document.SchemaVersion
	if version < 1 || version > CurrentProjectsSchemaVersion {
		return nil, fmt.Errorf("%w: projects schemaVersion %d", ErrUnsupportedSchemaVersion, version)
	}
	if !bytes.HasPrefix(bytes.TrimSpace(document.Projects), []byte("[")) {
		return nil, fmt.Errorf("%w: projects must be an array", ErrInvalidConfig)
	}
	pf := ProjectsFile{SchemaVersion: version}
	if err := json.Unmarshal(document.Projects, &pf.Projects); err != nil {
		return nil, fmt.Errorf("%w: projects in %s: %w", ErrInvalidConfig, path, err)
	}
	if len(pf.Projects) > MaxImportProjects {
		return nil, fmt.Errorf("%w: projects must contain at most %d entries", ErrInvalidConfig, MaxImportProjects)
	}
	if err := normalizeLoadedProjects(&pf); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return cloneProjects(pf.Projects), nil
}

func readImportFile(path string) ([]byte, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("%w: import path must be absolute", ErrInvalidConfig)
	}
	info, err := os.Stat(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("stat import %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Size() > MaxImportFileSize {
		return nil, fmt.Errorf("%w: import must be a regular file of at most %d bytes", ErrInvalidConfig, MaxImportFileSize)
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("open import %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	info, err = file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat import %s: %w", path, err)
	}
	if !info.Mode().IsRegular() || info.Size() > MaxImportFileSize {
		return nil, fmt.Errorf("%w: import must be a regular file of at most %d bytes", ErrInvalidConfig, MaxImportFileSize)
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxImportFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read import %s: %w", path, err)
	}
	if len(data) > MaxImportFileSize {
		return nil, fmt.Errorf("%w: import exceeds %d bytes", ErrInvalidConfig, MaxImportFileSize)
	}
	return data, nil
}

// ExportProjects atomically writes the current catalog as a schema-2 document.
// Dest must be absolute and must not refer to either of the store's configuration files.
func (s *Store) ExportProjects(dest string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := s.validateExportDestination(dest); err != nil {
		return 0, err
	}
	pf, err := s.loadProjectsFileUnlocked()
	if err != nil {
		return 0, err
	}
	if err := writeJSONAtomic(filepath.Clean(dest), pf); err != nil {
		return 0, err
	}
	return len(pf.Projects), nil
}

func (s *Store) validateExportDestination(dest string) error {
	if !filepath.IsAbs(dest) {
		return fmt.Errorf("%w: export path must be absolute", ErrInvalidConfig)
	}
	dest = filepath.Clean(dest)
	paths := []string{s.SettingsPath(), s.ProjectsPath()}
	for _, path := range paths {
		if dest == filepath.Clean(path) {
			return fmt.Errorf("%w: export path refers to %s", ErrInvalidConfig, path)
		}
	}
	destInfo, err := os.Stat(dest)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat export %s: %w", dest, err)
	}
	for _, path := range paths {
		info, err := os.Stat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat config %s: %w", path, err)
		}
		if os.SameFile(destInfo, info) {
			return fmt.Errorf("%w: export path refers to %s", ErrInvalidConfig, path)
		}
	}
	return nil
}

// ReplaceProjectsWithBackup validates and atomically replaces the catalog under the store lock.
// An existing readable catalog is backed up byte for byte to a unique, synced 0600 file first.
// A failed backup aborts replacement. The returned path is empty when no catalog existed.
func (s *Store) ReplaceProjectsWithBackup(projects []project.Project) (string, error) {
	pf := ProjectsFile{SchemaVersion: CurrentProjectsSchemaVersion, Projects: cloneProjects(projects)}
	if err := pf.Validate(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.loadProjectsFileUnlocked(); err != nil {
		return "", fmt.Errorf("refusing to replace unreadable existing projects: %w", err)
	}
	backup, err := s.backupProjectsUnlocked()
	if err != nil {
		return "", err
	}
	if err := writeJSONAtomic(s.ProjectsPath(), pf); err != nil {
		return backup, err
	}
	return backup, nil
}

func (s *Store) backupProjectsUnlocked() (string, error) {
	data, err := os.ReadFile(s.ProjectsPath())
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read projects for backup: %w", err)
	}
	base := s.ProjectsPath() + ".bak-" + s.now().UTC().Format("20060102T150405Z")
	for suffix := 0; ; suffix++ {
		path := base
		if suffix > 0 {
			path += "-" + strconv.Itoa(suffix)
		}
		file, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("create projects backup %s: %w", path, err)
		}
		if err := writeBackup(file, data); err != nil {
			_ = os.Remove(path)
			return "", err
		}
		return path, nil
	}
}

func writeBackup(file *os.File, data []byte) error {
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write projects backup: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync projects backup: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close projects backup: %w", err)
	}
	return nil
}

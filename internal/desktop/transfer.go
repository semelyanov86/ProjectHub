package desktop

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

// importPreviewLifetime bounds how long a prepared import can be applied.
const importPreviewLifetime = 10 * time.Minute

// ImportMode selects how imported projects are combined with the catalog.
type ImportMode string

// Import modes.
const (
	// ImportMerge adds new projects and skips already registered directories.
	ImportMerge ImportMode = "merge"
	// ImportReplace replaces the whole catalog after a backup is written.
	ImportReplace ImportMode = "replace"
)

// ImportPreview summarises a selected import file before it is applied.
type ImportPreview struct {
	Cancelled bool   `json:"cancelled"`
	Token     string `json:"token"`
	FileName  string `json:"fileName"`
	// Total is the number of projects in the file.
	Total int `json:"total"`
	// Eligible projects have a directory on this computer.
	Eligible int `json:"eligible"`
	// New is the number of eligible projects whose directory is not registered yet.
	New int `json:"new"`
	// Duplicates is the number of eligible projects already registered (skipped on merge).
	Duplicates int `json:"duplicates"`
	// Current is the size of the current catalog (replaced on replace).
	Current int `json:"current"`
	// Skipped lists projects that will never be imported (missing directory).
	Skipped []project.Skip `json:"skipped"`
}

// ImportRequest applies a prepared import.
type ImportRequest struct {
	Token string     `json:"token"`
	Mode  ImportMode `json:"mode"`
	// ReplaceConfirmed must be true for ImportReplace.
	ReplaceConfirmed bool `json:"replaceConfirmed"`
}

// ImportReport describes the outcome of an applied import.
type ImportReport struct {
	Mode    ImportMode     `json:"mode"`
	Added   int            `json:"added"`
	Skipped []project.Skip `json:"skipped"`
	// BackupPath is the copy of the previous catalog written before a replace.
	BackupPath string `json:"backupPath"`
}

// ExportResult describes a completed or cancelled export.
type ExportResult struct {
	Cancelled bool   `json:"cancelled"`
	Path      string `json:"path"`
	Count     int    `json:"count"`
}

type pendingImport struct {
	token    string
	projects []project.Project
	skipped  []project.Skip
	expires  time.Time
}

// PrepareImport lets the user choose a ProjectHub JSON export, validates it,
// and returns a preview. Nothing is modified until ApplyImport is called with
// the returned token.
func (s *Service) PrepareImport(ctx context.Context) (ImportPreview, error) {
	path, err := s.deps.Dialogs.ChooseImportFile(ctx)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("file picker: %w", err)
	}
	if path == "" {
		return ImportPreview{Cancelled: true, Skipped: []project.Skip{}}, nil
	}
	imported, err := s.deps.Transfer.ReadProjectsDocument(path)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("import %s: %w", baseName(path), err)
	}
	current, err := s.deps.Catalog.List()
	if err != nil {
		return ImportPreview{}, err
	}
	token, err := randomToken()
	if err != nil {
		return ImportPreview{}, err
	}

	registered := make(map[string]struct{}, len(current))
	for _, p := range current {
		registered[project.PathKey(p.Path)] = struct{}{}
	}
	eligible, skipped := splitByDirectory(imported)
	preview := ImportPreview{
		Token:    token,
		FileName: baseName(path),
		Total:    len(imported),
		Eligible: len(eligible),
		Current:  len(current),
		Skipped:  skipped,
	}
	for _, p := range eligible {
		if _, dup := registered[project.PathKey(p.Path)]; dup {
			preview.Duplicates++
		} else {
			preview.New++
		}
	}

	s.mu.Lock()
	s.pending = &pendingImport{token: token, projects: eligible, skipped: skipped, expires: s.now().Add(importPreviewLifetime)}
	s.mu.Unlock()
	return preview, nil
}

// ApplyImport merges or replaces the catalog with a prepared import. Merge is
// the default; replace requires ReplaceConfirmed and writes a backup first.
func (s *Service) ApplyImport(request ImportRequest) (ImportReport, error) {
	pending, err := s.takePending(request)
	if err != nil {
		return ImportReport{}, err
	}
	// Directories may have disappeared since the preview.
	eligible, vanished := splitByDirectory(pending.projects)
	skipped := append(append([]project.Skip{}, pending.skipped...), vanished...)

	if request.Mode == ImportReplace {
		backup, err := s.deps.Transfer.ReplaceProjectsWithBackup(eligible)
		if err != nil {
			s.restorePending(pending)
			return ImportReport{}, fmt.Errorf("replace projects: %w", err)
		}
		return ImportReport{Mode: ImportReplace, Added: len(eligible), Skipped: skipped, BackupPath: backup}, nil
	}

	report, err := s.deps.Catalog.MergeImported(eligible)
	if err != nil {
		s.restorePending(pending)
		return ImportReport{}, fmt.Errorf("merge projects: %w", err)
	}
	return ImportReport{Mode: ImportMerge, Added: len(report.Added), Skipped: append(skipped, report.Skipped...)}, nil
}

// DiscardImport forgets a prepared import that the user cancelled.
func (s *Service) DiscardImport(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending != nil && s.pending.token == token {
		s.pending = nil
	}
}

func (s *Service) takePending(request ImportRequest) (*pendingImport, error) {
	switch request.Mode {
	case ImportMerge:
	case ImportReplace:
		if !request.ReplaceConfirmed {
			return nil, ErrConfirmationRequired
		}
	default:
		return nil, fmt.Errorf("unsupported import mode %q", request.Mode)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pending
	if p == nil || p.token != request.Token || s.now().After(p.expires) {
		return nil, ErrImportExpired
	}
	s.pending = nil
	return p, nil
}

// restorePending keeps a failed import retryable until it expires.
func (s *Service) restorePending(p *pendingImport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		s.pending = p
	}
}

// ExportProjects lets the user choose a destination and writes the catalog as
// a ProjectHub JSON document.
func (s *Service) ExportProjects(ctx context.Context) (ExportResult, error) {
	name := "projecthub-projects-" + s.now().Format("2006-01-02") + ".json"
	dest, err := s.deps.Dialogs.ChooseExportFile(ctx, name)
	if err != nil {
		return ExportResult{}, fmt.Errorf("file picker: %w", err)
	}
	if dest == "" {
		return ExportResult{Cancelled: true}, nil
	}
	count, err := s.deps.Transfer.ExportProjects(dest)
	if err != nil {
		return ExportResult{}, fmt.Errorf("export projects: %w", err)
	}
	return ExportResult{Path: dest, Count: count}, nil
}

// splitByDirectory separates projects whose directory exists on this computer
// from those that are skipped.
func splitByDirectory(projects []project.Project) ([]project.Project, []project.Skip) {
	eligible := make([]project.Project, 0, len(projects))
	skipped := []project.Skip{}
	for _, p := range projects {
		if state := directoryState(p.Path); state != DirectoryPresent {
			skipped = append(skipped, project.Skip{Path: p.Path, Name: p.Name, Reason: skipReason(state)})
			continue
		}
		eligible = append(eligible, p)
	}
	return eligible, skipped
}

func skipReason(state DirectoryState) string {
	switch state {
	case DirectoryMissing:
		return "directory does not exist on this computer"
	case DirectoryNotDirectory:
		return "path is not a directory"
	case DirectoryInaccessible:
		return "directory is not accessible"
	case DirectoryPresent:
		return ""
	default:
		return strings.ReplaceAll(string(state), "_", " ")
	}
}

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate import token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

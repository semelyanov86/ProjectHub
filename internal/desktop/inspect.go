package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/git"
)

// MaxInspectBatch caps the number of projects inspected per call; the
// frontend requests larger catalogs in successive batches.
const MaxInspectBatch = 64

// DirectoryState describes whether a project's directory is usable.
type DirectoryState string

// Directory states.
const (
	DirectoryPresent      DirectoryState = "present"
	DirectoryMissing      DirectoryState = "missing"
	DirectoryInaccessible DirectoryState = "inaccessible"
	DirectoryNotDirectory DirectoryState = "not_directory"
)

// ProjectInspection is the live state of one project's directory.
type ProjectInspection struct {
	ProjectID string         `json:"projectId"`
	Path      string         `json:"path"`
	Directory DirectoryState `json:"directory"`
	// Git is populated only when Directory is DirectoryPresent.
	Git       git.Inspection `json:"git"`
	CheckedAt time.Time      `json:"checkedAt"`
}

// InspectProjects checks directory existence and Git state of the given
// projects with bounded concurrency. Unknown IDs are omitted from the result.
func (s *Service) InspectProjects(ctx context.Context, ids []string) ([]ProjectInspection, error) {
	if len(ids) > MaxInspectBatch {
		return nil, fmt.Errorf("inspect at most %d projects per call (got %d)", MaxInspectBatch, len(ids))
	}
	projects, err := s.deps.Catalog.List()
	if err != nil {
		return nil, err
	}
	byID := make(map[string]int, len(projects))
	for i, p := range projects {
		byID[p.ID] = i
	}

	results := make([]ProjectInspection, 0, len(ids))
	var gitDirs []string
	var gitIndexes []int
	for _, id := range ids {
		i, ok := byID[id]
		if !ok {
			continue
		}
		p := projects[i]
		state := directoryState(p.Path)
		results = append(results, ProjectInspection{ProjectID: p.ID, Path: p.Path, Directory: state})
		if state == DirectoryPresent {
			gitDirs = append(gitDirs, p.Path)
			gitIndexes = append(gitIndexes, len(results)-1)
		}
	}

	inspections := s.deps.Inspector.InspectMany(ctx, gitDirs)
	for k, inspection := range inspections {
		results[gitIndexes[k]].Git = inspection
	}
	checked := s.now().UTC()
	for i := range results {
		results[i].CheckedAt = checked
	}
	return results, nil
}

func directoryState(path string) DirectoryState {
	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		return DirectoryPresent
	case err == nil:
		return DirectoryNotDirectory
	case errors.Is(err, fs.ErrNotExist):
		return DirectoryMissing
	default:
		return DirectoryInaccessible
	}
}

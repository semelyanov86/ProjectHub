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

// directoryCheckTimeout bounds the existence checks of one batch, so a
// directory on an unresponsive network mount (stale NFS or SSHFS) is reported
// as inaccessible instead of blocking the whole refresh.
const directoryCheckTimeout = 2 * time.Second

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
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		i, ok := byID[id]
		if !ok {
			continue
		}
		p := projects[i]
		results = append(results, ProjectInspection{ProjectID: p.ID, Path: p.Path})
		paths = append(paths, p.Path)
	}

	var gitDirs []string
	var gitIndexes []int
	for i, state := range directoryStates(ctx, paths, directoryCheckTimeout, directoryState) {
		results[i].Directory = state
		if state == DirectoryPresent {
			gitDirs = append(gitDirs, results[i].Path)
			gitIndexes = append(gitIndexes, i)
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

// directoryStates checks all paths concurrently. Checks that have not finished
// when timeout expires or ctx ends are reported as DirectoryInaccessible; their
// goroutines finish on their own once the filesystem answers.
func directoryStates(
	ctx context.Context,
	paths []string,
	timeout time.Duration,
	check func(string) DirectoryState,
) []DirectoryState {
	type checked struct {
		index int
		state DirectoryState
	}
	states := make([]DirectoryState, len(paths))
	for i := range states {
		states[i] = DirectoryInaccessible
	}
	done := make(chan checked, len(paths)) // buffered: late checks never block
	for i, path := range paths {
		go func() { done <- checked{index: i, state: check(path)} }()
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for range paths {
		select {
		case result := <-done:
			states[result.index] = result.state
		case <-timer.C:
			return states
		case <-ctx.Done():
			return states
		}
	}
	return states
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

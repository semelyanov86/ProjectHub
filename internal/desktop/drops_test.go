package desktop

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/semelyanov86/ProjectHub/internal/git"
)

func TestDropEmpty(t *testing.T) {
	h := newTestService(t)
	results := []DropResult{}
	handler := NewDropHandler(h.Service, func(result DropResult) { results = append(results, result) })
	handler.Handle(t.Context(), nil)
	handler.Handle(t.Context(), []string{})
	if len(results) != 0 {
		t.Fatalf("empty drop emitted %+v", results)
	}
}

func TestDropSingleDirectory(t *testing.T) {
	h := newTestService(t)
	dir := projectDirectory(t, specialDirectoryName)
	results := []DropResult{}
	handler := NewDropHandler(h.Service, func(result DropResult) { results = append(results, result) })
	handler.Handle(t.Context(), []string{dir})
	if len(results) != 1 {
		t.Fatalf("emitted %d results, want 1", len(results))
	}
	result := results[0]
	if result.Draft == nil || result.Report != nil || result.Error != "" {
		t.Fatalf("single drop = %+v, want only Draft", result)
	}
	draft := result.Draft
	if draft.Path != dir || draft.SuggestedName != specialDirectoryName || draft.Cancelled {
		t.Fatalf("draft = %+v", draft)
	}
	if draft.Git.State != git.StateRepo || draft.Git.Repo.Branch != "main" {
		t.Fatalf("draft Git = %+v", draft.Git)
	}
	if projects := listTestProjects(t, h.Service); len(projects) != 0 {
		t.Fatalf("single drop persisted projects: %+v", projects)
	}
}

func TestDropInvalidPath(t *testing.T) {
	h := newTestService(t)
	results := []DropResult{}
	handler := NewDropHandler(h.Service, func(result DropResult) { results = append(results, result) })
	handler.Handle(t.Context(), []string{filepath.Join(t.TempDir(), "missing")})
	if len(results) != 1 {
		t.Fatalf("emitted %d results, want 1", len(results))
	}
	if results[0].Error == "" || results[0].Draft != nil || results[0].Report != nil {
		t.Fatalf("invalid drop = %+v, want only Error", results[0])
	}
}

func TestDropMultipleDirectories(t *testing.T) {
	h := newTestService(t)
	existing := addTestProject(t, h.Service, projectDirectory(t, "existing"))
	first := projectDirectory(t, specialDirectoryName)
	second := projectDirectory(t, "second directory & more")
	missing := filepath.Join(t.TempDir(), "missing")
	results := []DropResult{}
	handler := NewDropHandler(h.Service, func(result DropResult) { results = append(results, result) })
	handler.Handle(t.Context(), []string{first, missing, second, existing.Path})
	if len(results) != 1 {
		t.Fatalf("emitted %d results, want 1", len(results))
	}
	result := results[0]
	if result.Report == nil || result.Draft != nil || result.Error != "" {
		t.Fatalf("multiple drop = %+v, want only Report", result)
	}
	if len(result.Report.Added) != 2 || len(result.Report.Skipped) != 2 {
		t.Fatalf("drop report = %+v, want 2 added and 2 skipped", result.Report)
	}
	assertSkippedPath(t, result.Report.Skipped, missing, "")
	assertSkippedPath(t, result.Report.Skipped, existing.Path, "already registered")
	projects := listTestProjects(t, h.Service)
	paths := make([]string, len(projects))
	for i, p := range projects {
		paths[i] = p.Path
	}
	if !slices.Equal(paths, []string{existing.Path, first, second}) {
		t.Fatalf("persisted paths = %v", paths)
	}
}

func TestDropRejectedWhileBusy(t *testing.T) {
	h := newTestService(t)
	results := []DropResult{}
	handler := NewDropHandler(h.Service, func(result DropResult) { results = append(results, result) })
	handler.busy.Lock()
	handler.Handle(t.Context(), []string{projectDirectory(t, "first"), projectDirectory(t, "second")})
	handler.busy.Unlock()
	if len(results) != 1 || results[0].Error == "" || results[0].Report != nil {
		t.Fatalf("overlapping drop emitted %+v, want one error", results)
	}
	if projects := listTestProjects(t, h.Service); len(projects) != 0 {
		t.Fatalf("overlapping drop registered %d projects", len(projects))
	}
}

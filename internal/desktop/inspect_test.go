package desktop

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/git"
)

func TestInspectProjects(t *testing.T) {
	h := newTestService(t)
	present := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
	missing := addTestProject(t, h.Service, projectDirectory(t, "deleted"))
	regular := addTestProject(t, h.Service, projectDirectory(t, "became a file"))
	for _, path := range []string{missing.Path, regular.Path} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, regular.Path, []byte("no longer a directory"))
	fixed := time.Date(2026, 11, 15, 15, 30, 0, 123, time.FixedZone("UTC+3", 3*60*60))
	h.now = func() time.Time { return fixed }
	got, err := h.InspectProjects(t.Context(), []string{regular.ID, "unknown", present.ID, missing.ID})
	if err != nil {
		t.Fatal(err)
	}
	want := []ProjectInspection{
		{ProjectID: regular.ID, Path: regular.Path, Directory: DirectoryNotDirectory, CheckedAt: fixed.UTC()},
		{
			ProjectID: present.ID, Path: present.Path, Directory: DirectoryPresent, CheckedAt: fixed.UTC(),
			Git: git.Inspection{State: git.StateRepo, Repo: git.RepoInfo{
				Branch: "main", HeadSHA: "0123456789abcdef0123456789abcdef01234567",
			}},
		},
		{ProjectID: missing.ID, Path: missing.Path, Directory: DirectoryMissing, CheckedAt: fixed.UTC()},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("InspectProjects() = %+v, want %+v", got, want)
	}
	runs, starts := h.runner.commands()
	if len(starts) != 0 || len(runs) == 0 {
		t.Fatalf("Git runs = %d, starts = %d; want git runs only", len(runs), len(starts))
	}
	for _, run := range runs {
		if run.Dir != present.Path {
			t.Fatalf("git ran in %q; want only the present directory %q", run.Dir, present.Path)
		}
	}
}

func TestInspectProjectsBatchSize(t *testing.T) {
	for _, tc := range []struct {
		name    string
		ids     []string
		wantErr bool
	}{
		{name: "nil input"},
		{name: "empty input", ids: []string{}},
		{name: "maximum", ids: make([]string, MaxInspectBatch)},
		{name: "too many", ids: make([]string, MaxInspectBatch+1), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestService(t)
			got, err := h.InspectProjects(t.Context(), tc.ids)
			if (err != nil) != tc.wantErr {
				t.Fatalf("InspectProjects() error = %v, want error %v", err, tc.wantErr)
			}
			if !tc.wantErr && (got == nil || len(got) != 0) {
				t.Fatalf("InspectProjects() = %#v, want non-nil empty slice", got)
			}
		})
	}
}

package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

const testProjectName = "App"

type memoryStorage struct {
	mu           sync.Mutex
	projects     []Project
	writes       int
	beforeUpdate func([]Project) []Project
}

func (s *memoryStorage) LoadProjects() ([]Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneProjects(s.projects), nil
}

func (s *memoryStorage) UpdateProjects(mutate func([]Project) ([]Project, error)) ([]Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.beforeUpdate != nil {
		s.projects = s.beforeUpdate(cloneProjects(s.projects))
		s.beforeUpdate = nil
	}
	next, err := mutate(cloneProjects(s.projects))
	if err != nil {
		return nil, err
	}
	s.projects = cloneProjects(next)
	s.writes++
	return cloneProjects(s.projects), nil
}

func cloneProjects(projects []Project) []Project {
	result := slices.Clone(projects)
	for i := range result {
		if result[i].LastOpened != nil {
			opened := *result[i].LastOpened
			result[i].LastOpened = &opened
		}
	}
	return result
}

func (s *memoryStorage) writeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writes
}

type failingStorage struct{ err error }

func (s failingStorage) LoadProjects() ([]Project, error) { return nil, s.err }
func (s failingStorage) UpdateProjects(_ func([]Project) ([]Project, error)) ([]Project, error) {
	return nil, s.err
}

func validateDir(path string) (string, error) {
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("directory must be absolute: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %q", path)
	}
	return filepath.Clean(path), nil
}

func testCatalog(projects ...Project) (*Catalog, *memoryStorage, time.Time) {
	store := &memoryStorage{projects: cloneProjects(projects)}
	catalog := NewCatalog(store, validateDir)
	fixed := time.Date(2026, 11, 15, 12, 30, 0, 0, time.UTC)
	catalog.now = func() time.Time { return fixed }
	sequence := 0
	catalog.newID = func() (string, error) {
		sequence++
		return fmt.Sprintf("generated-%d", sequence), nil
	}
	return catalog, store, fixed
}

func addProject(t *testing.T, catalog *Catalog, path string) Project {
	t.Helper()
	p, err := catalog.Add(Input{Name: SuggestName(path), Path: path})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func assertWrites(t *testing.T, store *memoryStorage, want int) {
	t.Helper()
	if got := store.writeCount(); got != want {
		t.Fatalf("successful writes = %d, want %d", got, want)
	}
}

func assertStored(t *testing.T, catalog *Catalog, want ...Project) {
	t.Helper()
	got, err := catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stored projects = %+v, want %+v", got, want)
	}
}

func regularFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCatalogAdd(t *testing.T) {
	catalog, store, fixed := testCatalog()
	path := t.TempDir()
	got, err := catalog.Add(Input{
		Name: "  Project  ", Description: "  Description\nsecond line  ", Path: path + "/",
		DefaultEditor: " zed ", Favorite: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := Project{
		ID: "generated-1", Name: "Project", Description: "Description\nsecond line", Path: path,
		DefaultEditor: testEditorZed, Favorite: true, CreatedAt: fixed, UpdatedAt: fixed,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Add = %+v, want %+v", got, want)
	}
	assertStored(t, catalog, want)
	assertWrites(t, store, 1)
	for _, duplicate := range []string{path, path + "/"} {
		if _, err := catalog.Add(Input{Name: "Duplicate", Path: duplicate}); !errors.Is(err, ErrDuplicateProject) {
			t.Fatalf("duplicate error = %v", err)
		}
	}
	assertWrites(t, store, 1)
	assertStored(t, catalog, want)
}

func TestCatalogAddInvalid(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name  string
		input Input
		want  error
	}{
		{name: "empty name", input: Input{Path: dir}, want: ErrInvalidProject},
		{name: "whitespace name", input: Input{Name: " \t ", Path: dir}, want: ErrInvalidProject},
		{name: "multiline name", input: Input{Name: "first\nsecond", Path: dir}, want: ErrInvalidProject},
		{name: "unsupported editor", input: Input{Name: testProjectName, Path: dir, DefaultEditor: testUnsupportedEditor}, want: ErrInvalidProject},
		{name: "empty path", input: Input{Name: testProjectName}, want: ErrInvalidProject},
		{name: "relative path", input: Input{Name: testProjectName, Path: "relative"}},
		{name: "missing path", input: Input{Name: testProjectName, Path: filepath.Join(dir, "missing")}, want: os.ErrNotExist},
		{name: "regular file", input: Input{Name: testProjectName, Path: regularFile(t)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catalog, store, _ := testCatalog()
			_, err := catalog.Add(tc.input)
			if err == nil {
				t.Fatal("expected an error")
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			assertWrites(t, store, 0)
		})
	}
}

func TestCatalogAddSpecialPaths(t *testing.T) {
	for _, name := range []string{"with spaces", `quotes'"`, "$variable", "semi;colon", "проект日本語"} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), name)
			if err := os.Mkdir(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			catalog, store, _ := testCatalog()
			p := addProject(t, catalog, dir)
			if p.Path != dir {
				t.Fatalf("path = %q, want %q", p.Path, dir)
			}
			assertStored(t, catalog, p)
			assertWrites(t, store, 1)
		})
	}
}

func TestCatalogUpdate(t *testing.T) {
	original := validProject()
	original.Path = t.TempDir()
	opened := original.CreatedAt.Add(time.Minute)
	original.LastOpened = &opened
	original.LastUsedEditor = testEditorSublime
	catalog, store, fixed := testCatalog(original)
	got, err := catalog.Update(original.ID, Input{
		Name: " Updated ", Description: " Changed ", Path: original.Path, DefaultEditor: testEditorSublime, Favorite: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := original
	want.Name, want.Description, want.DefaultEditor = "Updated", "Changed", testEditorSublime
	want.Favorite = false
	want.UpdatedAt = fixed
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Update = %+v, want %+v", got, want)
	}
	assertStored(t, catalog, want)
	assertWrites(t, store, 1)
	newPath := t.TempDir()
	got, err = catalog.Update(original.ID, Input{Name: "Moved", Path: newPath + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != newPath {
		t.Fatalf("moved path = %q, want %q", got.Path, newPath)
	}
	assertStored(t, catalog, got)
	assertWrites(t, store, 2)
}

func TestCatalogUpdateFailures(t *testing.T) {
	first, second := validProject(), validProject()
	first.Path = t.TempDir()
	second.ID, second.Path = "second", t.TempDir()
	tests := []struct {
		name, id, path string
		want           error
	}{
		{name: "unknown id", id: "absent", path: first.Path, want: ErrProjectNotFound},
		{name: "missing new directory", id: first.ID, path: filepath.Join(first.Path, "missing"), want: os.ErrNotExist},
		{name: "duplicate path", id: first.ID, path: second.Path + "/", want: ErrDuplicateProject},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catalog, store, _ := testCatalog(first, second)
			_, err := catalog.Update(tc.id, Input{Name: "Changed", Path: tc.path})
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			assertStored(t, catalog, first, second)
			assertWrites(t, store, 0)
		})
	}
}

func TestCatalogUpdateDeletedDirectory(t *testing.T) {
	original := validProject()
	original.Path = t.TempDir()
	if err := os.Remove(original.Path); err != nil {
		t.Fatal(err)
	}
	catalog, store, _ := testCatalog(original)
	catalog.validateDir = func(path string) (string, error) {
		t.Fatalf("unexpected directory validation for %q", path)
		return "", errors.New("unexpected validation")
	}
	got, err := catalog.Update(original.ID, Input{Name: "Metadata only", Path: original.Path})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Metadata only" || got.Path != original.Path {
		t.Fatalf("Update = %+v", got)
	}
	assertStored(t, catalog, got)
	assertWrites(t, store, 1)
}

func TestCatalogUpdateConcurrentMove(t *testing.T) {
	original := validProject()
	original.Path = t.TempDir()
	moved := original
	moved.Path = t.TempDir()
	catalog, store, _ := testCatalog(original)
	store.beforeUpdate = func(projects []Project) []Project { projects[0] = moved; return projects }
	_, err := catalog.Update(original.ID, Input{Name: "Changed", Path: original.Path})
	if !errors.Is(err, ErrProjectChanged) {
		t.Fatalf("error = %v, want ErrProjectChanged", err)
	}
	assertStored(t, catalog, moved)
	assertWrites(t, store, 0)
}

func TestCatalogRemove(t *testing.T) {
	catalog, store, _ := testCatalog()
	first := addProject(t, catalog, t.TempDir())
	second := addProject(t, catalog, t.TempDir())
	if err := catalog.Remove(first.ID); err != nil {
		t.Fatal(err)
	}
	assertStored(t, catalog, second)
	if info, err := os.Stat(first.Path); err != nil || !info.IsDir() {
		t.Fatalf("directory removed: %v", err)
	}
	if err := catalog.Remove("absent"); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("error = %v", err)
	}
	assertWrites(t, store, 3)
}

func TestCatalogSetFavorite(t *testing.T) {
	original := validProject()
	catalog, store, fixed := testCatalog(original)
	for _, favorite := range []bool{false, true} {
		got, err := catalog.SetFavorite(original.ID, favorite)
		if err != nil {
			t.Fatal(err)
		}
		want := original
		want.Favorite, want.UpdatedAt = favorite, fixed
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("SetFavorite = %+v, want %+v", got, want)
		}
		assertStored(t, catalog, want)
	}
	if _, err := catalog.SetFavorite("absent", true); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("error = %v", err)
	}
	assertWrites(t, store, 2)
}

func TestCatalogRecordOpened(t *testing.T) {
	original := validProject()
	original.LastUsedEditor = testEditorSublime
	for _, editorID := range []string{testEditorZed, ""} {
		t.Run("editor="+editorID, func(t *testing.T) {
			catalog, store, fixed := testCatalog(original)
			got, err := catalog.RecordOpened(original.ID, original.Path+"/", editorID)
			if err != nil {
				t.Fatal(err)
			}
			want := original
			want.LastOpened = &fixed
			if editorID != "" {
				want.LastUsedEditor = editorID
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("RecordOpened = %+v, want %+v", got, want)
			}
			assertStored(t, catalog, want)
			assertWrites(t, store, 1)
		})
	}
}

func TestCatalogRecordOpenedFailures(t *testing.T) {
	original := validProject()
	tests := []struct {
		name, id, path, editor string
		want                   error
	}{
		{name: "unsupported editor", id: original.ID, path: original.Path, editor: testUnsupportedEditor, want: ErrInvalidProject},
		{name: "moved project", id: original.ID, path: "/different", want: ErrProjectChanged},
		{name: "unknown id", id: "absent", path: original.Path, want: ErrProjectNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			catalog, store, _ := testCatalog(original)
			_, err := catalog.RecordOpened(tc.id, tc.path, tc.editor)
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			assertStored(t, catalog, original)
			assertWrites(t, store, 0)
		})
	}
}

func TestCatalogAddDirectories(t *testing.T) {
	existing := validProject()
	existing.Path = t.TempDir()
	catalog, store, fixed := testCatalog(existing)
	first, second := t.TempDir(), t.TempDir()
	missing, file := filepath.Join(t.TempDir(), "missing"), regularFile(t)
	report, err := catalog.AddDirectories([]string{first, missing, file, existing.Path, second, first + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 2 || len(report.Skipped) != 4 {
		t.Fatalf("report = %+v", report)
	}
	for i, path := range []string{first, second} {
		p := report.Added[i]
		if p.Path != path || p.Name != SuggestName(path) {
			t.Fatalf("added project = %+v", p)
		}
		if p.ID == "" || !p.CreatedAt.Equal(fixed) || !p.UpdatedAt.Equal(fixed) {
			t.Fatalf("added metadata = %+v", p)
		}
	}
	for _, skipped := range report.Skipped {
		if skipped.Reason == "" {
			t.Fatalf("missing skip reason: %+v", skipped)
		}
	}
	assertStored(t, catalog, existing, report.Added[0], report.Added[1])
	assertWrites(t, store, 1)
}

func TestCatalogAddDirectoriesNoChanges(t *testing.T) {
	for _, paths := range [][]string{nil, {"relative", filepath.Join(t.TempDir(), "missing"), regularFile(t)}} {
		catalog, store, _ := testCatalog()
		report, err := catalog.AddDirectories(paths)
		if err != nil {
			t.Fatal(err)
		}
		if report.Added == nil || len(report.Added) != 0 || len(report.Skipped) != len(paths) {
			t.Fatalf("report = %+v", report)
		}
		for _, skipped := range report.Skipped {
			if skipped.Reason == "" {
				t.Fatalf("missing skip reason: %+v", skipped)
			}
		}
		assertWrites(t, store, 0)
	}
}

func TestCatalogAddDirectoriesOverflow(t *testing.T) {
	catalog, store, _ := testCatalog()
	dir := t.TempDir()
	paths := make([]string, MaxBatchDirectories+2)
	for i := range paths {
		paths[i] = dir
	}
	report, err := catalog.AddDirectories(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 1 || len(report.Skipped) != len(paths)-1 {
		t.Fatalf("batch counts: %+v", report)
	}
	count := 0
	for _, skipped := range report.Skipped {
		if skipped.Reason == "too many directories in one drop" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("overflow skips = %d, want 2", count)
	}
	assertWrites(t, store, 1)
}

func TestCatalogMergeImported(t *testing.T) {
	existing := validProject()
	catalog, store, fixed := testCatalog(existing)
	imported := validProject()
	imported.ID, imported.Path = "imported", t.TempDir()
	opened := imported.CreatedAt.Add(time.Hour)
	imported.LastOpened = &opened
	collision := imported
	collision.ID, collision.Path = existing.ID, t.TempDir()
	invalid := imported
	invalid.ID, invalid.Path = "invalid", "relative/path"
	report, err := catalog.MergeImported([]Project{imported, existing, imported, collision, invalid})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Added) != 2 || len(report.Skipped) != 3 {
		t.Fatalf("report = %+v", report)
	}
	want := imported
	want.UpdatedAt = fixed
	if !reflect.DeepEqual(report.Added[0], want) {
		t.Fatalf("imported project = %+v, want %+v", report.Added[0], want)
	}
	collision.ID, collision.UpdatedAt = "generated-1", fixed
	if !reflect.DeepEqual(report.Added[1], collision) {
		t.Fatalf("collision project = %+v, want %+v", report.Added[1], collision)
	}
	for _, skipped := range report.Skipped {
		if skipped.Reason == "" {
			t.Fatalf("missing skip reason: %+v", skipped)
		}
	}
	assertStored(t, catalog, existing, want, collision)
	assertWrites(t, store, 1)
}

func TestCatalogMergeImportedNoChanges(t *testing.T) {
	existing := validProject()
	invalid := existing
	invalid.Path = "relative"
	for _, imported := range [][]Project{nil, {existing}, {invalid}} {
		catalog, store, _ := testCatalog(existing)
		report, err := catalog.MergeImported(imported)
		if err != nil {
			t.Fatal(err)
		}
		if report.Added == nil || len(report.Added) != 0 || len(report.Skipped) != len(imported) {
			t.Fatalf("report = %+v", report)
		}
		assertStored(t, catalog, existing)
		assertWrites(t, store, 0)
	}
}

func TestCatalogQueries(t *testing.T) {
	catalog, _, _ := testCatalog()
	projects, err := catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	if projects == nil || len(projects) != 0 {
		t.Fatalf("List = %#v, want non-nil empty slice", projects)
	}
	if _, err := catalog.Get("absent"); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("Get error = %v", err)
	}
	if _, found, err := catalog.FindByPath("/absent"); err != nil || found {
		t.Fatalf("FindByPath = %v, %v", found, err)
	}
	original := addProject(t, catalog, t.TempDir())
	got, err := catalog.Get(original.ID)
	if err != nil || !reflect.DeepEqual(got, original) {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	got, found, err := catalog.FindByPath(original.Path + "/")
	if err != nil || !found || !reflect.DeepEqual(got, original) {
		t.Fatalf("FindByPath = %+v, %v, %v", got, found, err)
	}
	projects, err = catalog.List()
	if err != nil {
		t.Fatal(err)
	}
	projects[0].Name = "caller mutation"
	assertStored(t, catalog, original)
}

func TestCatalogStorageErrors(t *testing.T) {
	failure := errors.New("storage failed")
	dir := t.TempDir()
	catalog := NewCatalog(failingStorage{err: failure}, validateDir)
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "List", run: func() error { _, err := catalog.List(); return err }},
		{name: "Get", run: func() error { _, err := catalog.Get("id"); return err }},
		{name: "FindByPath", run: func() error { _, _, err := catalog.FindByPath(dir); return err }},
		{name: "Add", run: func() error { _, err := catalog.Add(Input{Name: testProjectName, Path: dir}); return err }},
		{name: "Update", run: func() error { _, err := catalog.Update("id", Input{Name: testProjectName, Path: dir}); return err }},
		{name: "Remove", run: func() error { return catalog.Remove("id") }},
		{name: "SetFavorite", run: func() error { _, err := catalog.SetFavorite("id", true); return err }},
		{name: "RecordOpened", run: func() error { _, err := catalog.RecordOpened("id", dir, testEditorZed); return err }},
		{name: "AddDirectories", run: func() error { _, err := catalog.AddDirectories([]string{dir}); return err }},
		{name: "MergeImported", run: func() error { _, err := catalog.MergeImported([]Project{validProject()}); return err }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, failure) {
				t.Fatalf("error = %v, want storage error", err)
			}
		})
	}
}

func TestCatalogUpdateTreatsTrailingSpaceAsDifferentDirectory(t *testing.T) {
	root := t.TempDir()
	plain := filepath.Join(root, "project")
	spaced := filepath.Join(root, "project ")
	for _, dir := range []string{plain, spaced} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	catalog, _, _ := testCatalog()
	p := addProject(t, catalog, plain)

	updated, err := catalog.Update(p.ID, Input{Name: p.Name, Path: spaced})
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if updated.Path != spaced {
		t.Fatalf("Update() path = %q, want %q (trailing space is significant)", updated.Path, spaced)
	}
}

func TestCatalogAddDirectoriesSkipsOnlyUnusableNames(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "good")
	newline := filepath.Join(root, "multi\nline")
	long := filepath.Join(root, strings.Repeat("x", 230))
	for _, dir := range []string{good, newline, long} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	catalog, store, _ := testCatalog()
	report, err := catalog.AddDirectories([]string{good, newline, long})
	if err != nil {
		t.Fatalf("AddDirectories() error: %v", err)
	}
	if len(report.Added) != 3 || len(report.Skipped) != 0 {
		t.Fatalf("AddDirectories() = %+v, want all three added with sanitised names", report)
	}
	names := []string{report.Added[0].Name, report.Added[1].Name, report.Added[2].Name}
	if names[0] != "good" || names[1] != "multi line" || len([]rune(names[2])) != maxNameLength {
		t.Fatalf("names = %q, want good, %q and a %d-rune name", names, "multi line", maxNameLength)
	}
	assertWrites(t, store, 1)
}

func TestCatalogInputLimitsApplyOnlyToChangedText(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	legacy := Project{
		ID: "legacy", Name: strings.Repeat("n", maxNameLength+5), Description: strings.Repeat("d", maxDescriptionLength+1),
		Path: dir, CreatedAt: now, UpdatedAt: now,
	}
	catalog, _, _ := testCatalog(legacy)

	updated, err := catalog.Update(legacy.ID, Input{
		Name: legacy.Name, Description: legacy.Description, Path: dir, DefaultEditor: "zed", Favorite: true,
	})
	if err != nil {
		t.Fatalf("Update() with unchanged legacy text error: %v", err)
	}
	if !updated.Favorite || updated.DefaultEditor != "zed" || updated.Name != legacy.Name || updated.Description != legacy.Description {
		t.Fatalf("Update() = %+v, want other fields changed and legacy text preserved", updated)
	}

	if _, err := catalog.Update(legacy.ID, Input{Name: legacy.Name + "x", Description: legacy.Description, Path: dir}); !errors.Is(err, ErrInvalidProject) {
		t.Fatalf("Update() with a new over-limit name error = %v, want ErrInvalidProject", err)
	}
	if _, err := catalog.Update(legacy.ID, Input{Name: "short", Description: legacy.Description + "x", Path: dir}); !errors.Is(err, ErrInvalidProject) {
		t.Fatalf("Update() with a new over-limit description error = %v, want ErrInvalidProject", err)
	}
	if _, err := catalog.Add(Input{Name: "multi\nline", Path: t.TempDir()}); !errors.Is(err, ErrInvalidProject) {
		t.Fatalf("Add() with a multi-line name error = %v, want ErrInvalidProject", err)
	}
}

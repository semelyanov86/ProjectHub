package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

func TestReplaceProjectsWithBackup(t *testing.T) {
	store := newTestStore(t)
	store.now = func() time.Time {
		return time.Date(2026, 11, 15, 15, 4, 5, 0, time.FixedZone("test", 2*60*60))
	}
	// Preserve legacy formatting and unknown fields byte for byte in the backup.
	original := []byte("{ \"schemaVersion\": 1, \"projects\": [], \"legacy\":true }\n")
	writeTestFile(t, store.ProjectsPath(), original)
	projects := []project.Project{testProject("replacement")}
	base := store.ProjectsPath() + ".bak-20261115T130405Z"
	for _, suffix := range []string{"", "-1", "-2"} {
		before := readTestFile(t, store.ProjectsPath())
		backup, err := store.ReplaceProjectsWithBackup(projects)
		if err != nil || backup != base+suffix {
			t.Fatalf("ReplaceProjectsWithBackup() = %q, %v; want %q", backup, err, base+suffix)
		}
		assertFileBytes(t, backup, before)
		info, err := os.Stat(backup)
		if err != nil {
			t.Fatalf("Stat(backup): %v", err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("backup mode = %o; want 0600", info.Mode().Perm())
		}
		loaded, err := store.LoadProjects()
		if err != nil || !reflect.DeepEqual(loaded, projects) {
			t.Fatalf("LoadProjects() = %+v, %v; want %+v", loaded, err, projects)
		}
	}
	assertFileBytes(t, base, original)
}

func TestReplaceProjectsWithoutExistingCatalog(t *testing.T) {
	store := newTestStore(t)
	backup, err := store.ReplaceProjectsWithBackup(nil)
	if err != nil || backup != "" {
		t.Fatalf("ReplaceProjectsWithBackup() = %q, %v", backup, err)
	}
	projects, err := store.LoadProjects()
	if err != nil || projects == nil || len(projects) != 0 {
		t.Fatalf("LoadProjects() = %+v, %v", projects, err)
	}
}

func TestReplaceProjectsRefusesUnreadableCatalog(t *testing.T) {
	for _, tc := range []struct {
		name, content string
		wantErr       error
	}{
		{"corrupt", `{"schemaVersion":2,"projects":[`, ErrCorruptedConfig},
		{"future schema", `{"schemaVersion":3,"projects":[]}`, ErrUnsupportedSchemaVersion},
		{"invalid project", `{"schemaVersion":2,"projects":[{}]}`, ErrInvalidConfig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestStore(t)
			writeTestFile(t, store.ProjectsPath(), []byte(tc.content))
			backup, err := store.ReplaceProjectsWithBackup([]project.Project{testProject("new")})
			if !errors.Is(err, tc.wantErr) || backup != "" {
				t.Fatalf("ReplaceProjectsWithBackup() = %q, %v", backup, err)
			}
			assertFileBytes(t, store.ProjectsPath(), []byte(tc.content))
			assertNoBackups(t, store)
		})
	}
}

func TestReplaceProjectsValidatesBeforeBackup(t *testing.T) {
	store := newTestStore(t)
	original := []byte(`{"schemaVersion":2,"projects":[]}`)
	writeTestFile(t, store.ProjectsPath(), original)
	p := testProject("duplicate")
	_, err := store.ReplaceProjectsWithBackup([]project.Project{p, p})
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("ReplaceProjectsWithBackup() error = %v", err)
	}
	assertFileBytes(t, store.ProjectsPath(), original)
	assertNoBackups(t, store)
}

func TestExportProjectsRoundTrip(t *testing.T) {
	store := newTestStore(t)
	p := testProject("exported")
	p.LastOpened = &p.UpdatedAt
	pf := ProjectsFile{SchemaVersion: 2, Projects: []project.Project{p}}
	if err := store.SaveProjectsFile(pf); err != nil {
		t.Fatalf("SaveProjectsFile(): %v", err)
	}
	dest := filepath.Join(t.TempDir(), "export.json")
	count, err := store.ExportProjects(dest)
	if err != nil || count != 1 {
		t.Fatalf("ExportProjects() = %d, %v; want 1", count, err)
	}
	got, err := store.ReadProjectsDocument(dest)
	if err != nil || !reflect.DeepEqual(got, pf.Projects) {
		t.Fatalf("ReadProjectsDocument() = %+v, %v; want %+v", got, err, pf.Projects)
	}
}

func TestExportProjectsRejectsStoreFiles(t *testing.T) {
	store := newTestStore(t)
	if err := store.SaveSettings(DefaultSettings()); err != nil {
		t.Fatalf("SaveSettings(): %v", err)
	}
	if err := store.SaveProjectsFile(DefaultProjectsFile()); err != nil {
		t.Fatalf("SaveProjectsFile(): %v", err)
	}
	alias := filepath.Join(t.TempDir(), "alias.json")
	if err := os.Symlink(store.ProjectsPath(), alias); err != nil {
		t.Fatalf("Symlink(): %v", err)
	}
	hardlink := filepath.Join(t.TempDir(), "hardlink.json")
	if err := os.Link(store.SettingsPath(), hardlink); err != nil {
		t.Fatalf("Link(): %v", err)
	}
	settingsBefore := readTestFile(t, store.SettingsPath())
	projectsBefore := readTestFile(t, store.ProjectsPath())
	for _, dest := range []string{store.SettingsPath(), store.ProjectsPath(), alias, hardlink, "relative.json"} {
		if _, err := store.ExportProjects(dest); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("ExportProjects(%q) error = %v; want ErrInvalidConfig", dest, err)
		}
	}
	assertFileBytes(t, store.SettingsPath(), settingsBefore)
	assertFileBytes(t, store.ProjectsPath(), projectsBefore)
}

func TestExportProjectsRefusesUnreadableCatalog(t *testing.T) {
	store := newTestStore(t)
	original := []byte(`{"schemaVersion":2,"projects":[`)
	writeTestFile(t, store.ProjectsPath(), original)
	dest := filepath.Join(t.TempDir(), "export.json")
	writeTestFile(t, dest, []byte("keep destination"))
	if _, err := store.ExportProjects(dest); !errors.Is(err, ErrCorruptedConfig) {
		t.Fatalf("ExportProjects() error = %v", err)
	}
	assertFileBytes(t, store.ProjectsPath(), original)
	assertFileBytes(t, dest, []byte("keep destination"))
}

func TestReadProjectsDocument(t *testing.T) {
	store := newTestStore(t)
	p := testProject("imported")
	legacy := p
	legacy.DefaultEditor, legacy.LastUsedEditor = "vscode", "neovim"
	duplicateID := p
	duplicateID.Path += "-other"
	duplicatePath := p
	duplicatePath.ID = "different-id"
	duplicatePath.Path += "/../imported"
	invalid := p
	invalid.Path = "relative"
	tests := []struct {
		name    string
		content []byte
		want    []project.Project
		wantErr error
	}{
		{"v1 normalized", encodeTestJSON(t, ProjectsFile{SchemaVersion: 1, Projects: []project.Project{legacy}}), []project.Project{p}, nil},
		{"v2", encodeTestJSON(t, ProjectsFile{SchemaVersion: 2, Projects: []project.Project{p}}), []project.Project{p}, nil},
		{"empty", []byte(`{"schemaVersion":2,"projects":[]}`), []project.Project{}, nil},
		{"malformed json", []byte(`{"schemaVersion":2,"projects":[`), nil, ErrCorruptedConfig},
		{"missing version", []byte(`{"projects":[]}`), nil, ErrInvalidConfig},
		{"null version", []byte(`{"schemaVersion":null,"projects":[]}`), nil, ErrInvalidConfig},
		{"missing projects", []byte(`{"schemaVersion":2}`), nil, ErrInvalidConfig},
		{"null projects", []byte(`{"schemaVersion":2,"projects":null}`), nil, ErrInvalidConfig},
		{"non-array projects", []byte(`{"schemaVersion":2,"projects":{}}`), nil, ErrInvalidConfig},
		{"future version", []byte(`{"schemaVersion":3,"projects":[]}`), nil, ErrUnsupportedSchemaVersion},
		{"zero version", []byte(`{"schemaVersion":0,"projects":[]}`), nil, ErrUnsupportedSchemaVersion},
		{"oversize", []byte(strings.Repeat(" ", MaxImportFileSize+1)), nil, ErrInvalidConfig},
		{"duplicate ids", encodeTestJSON(t, ProjectsFile{SchemaVersion: 2, Projects: []project.Project{p, duplicateID}}), nil, ErrInvalidConfig},
		{"duplicate paths", encodeTestJSON(t, ProjectsFile{SchemaVersion: 2, Projects: []project.Project{p, duplicatePath}}), nil, ErrInvalidConfig},
		{"invalid timestamp", []byte(`{"schemaVersion":2,"projects":[{"createdAt":"bad"}]}`), nil, ErrInvalidConfig},
		{"invalid project", encodeTestJSON(t, ProjectsFile{SchemaVersion: 2, Projects: []project.Project{invalid}}), nil, ErrInvalidConfig},
		{"too many projects", encodeTestJSON(t, ProjectsFile{SchemaVersion: 2, Projects: make([]project.Project, MaxImportProjects+1)}), nil, ErrInvalidConfig},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "import.json")
			writeTestFile(t, path, tc.content)
			got, err := store.ReadProjectsDocument(path)
			if !errors.Is(err, tc.wantErr) || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ReadProjectsDocument() = %+v, %v; want %+v, %v", got, err, tc.want, tc.wantErr)
			}
			assertFileBytes(t, path, tc.content)
		})
	}
	if _, err := store.ReadProjectsDocument("relative.json"); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("ReadProjectsDocument(relative) error = %v", err)
	}
	if _, err := store.ReadProjectsDocument(t.TempDir()); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("ReadProjectsDocument(directory) error = %v", err)
	}
	if _, err := os.Stat(store.ProjectsPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("import touched catalog: %v", err)
	}
}

func assertNoBackups(t *testing.T, store *Store) {
	t.Helper()
	entries, err := os.ReadDir(store.Dir())
	if err != nil {
		t.Fatalf("ReadDir(): %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ProjectsFileName+".bak-") {
			t.Fatalf("unexpected backup %q", entry.Name())
		}
	}
}

func TestReplaceProjectsBackupFailurePreservesCatalog(t *testing.T) {
	store := newTestStore(t)
	original := []byte(`{"schemaVersion":2,"projects":[]}`)
	writeTestFile(t, store.ProjectsPath(), original)
	//nolint:gosec // Owner-only directory traversal is required to read the catalog while denying backup creation.
	if err := os.Chmod(store.Dir(), 0o500); err != nil {
		t.Fatalf("Chmod(): %v", err)
	}
	t.Cleanup(func() {
		//nolint:gosec // Restore owner-only directory traversal and writes for test cleanup.
		if err := os.Chmod(store.Dir(), 0o700); err != nil {
			t.Errorf("restore directory permissions: %v", err)
		}
	})
	backup, err := store.ReplaceProjectsWithBackup([]project.Project{testProject("replacement")})
	if err == nil || backup != "" {
		t.Fatalf("ReplaceProjectsWithBackup() = %q, %v; want backup failure", backup, err)
	}
	assertFileBytes(t, store.ProjectsPath(), original)
	assertNoBackups(t, store)
}

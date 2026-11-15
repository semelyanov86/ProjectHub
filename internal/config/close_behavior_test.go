package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

func TestCloseBehaviorDefaultsAndValidation(t *testing.T) {
	if got := DefaultSettings().CloseBehavior; got != CloseBehaviorTray {
		t.Fatalf("default closeBehavior = %q, want %q", got, CloseBehaviorTray)
	}

	store := newTestStore(t)
	// Files written before closeBehavior existed (or with an empty value) load with the default.
	writeTestFile(t, store.SettingsPath(), []byte(`{"schemaVersion":2,"defaultEditor":"zed","preferredTerminal":"ghostty",
"preferredFileManager":"auto","theme":"dark","window":{"width":500,"height":700,"rememberSize":true},
"projectSort":"name","closeBehavior":""}`))
	loaded, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error: %v", err)
	}
	if loaded.CloseBehavior != CloseBehaviorTray || loaded.Theme != ThemeDark {
		t.Fatalf("loaded = %+v, want default closeBehavior and the stored theme", loaded)
	}

	loaded.CloseBehavior = CloseBehaviorQuit
	if err := store.SaveSettings(loaded); err != nil {
		t.Fatalf("SaveSettings(quit) error: %v", err)
	}
	if again, _ := store.LoadSettings(); again.CloseBehavior != CloseBehaviorQuit {
		t.Fatalf("closeBehavior after save = %q, want quit", again.CloseBehavior)
	}

	loaded.CloseBehavior = "minimize"
	if err := store.SaveSettings(loaded); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("SaveSettings(minimize) error = %v, want ErrInvalidConfig", err)
	}
}

func TestOnProjectsChangedFiresAfterSuccessfulWrites(t *testing.T) {
	store := newTestStore(t)
	calls := 0
	store.OnProjectsChanged(func() { calls++ })

	if _, err := store.UpdateProjects(func(ps []project.Project) ([]project.Project, error) {
		return append(ps, testProject("a")), nil
	}); err != nil {
		t.Fatalf("UpdateProjects() error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("hook calls after update = %d, want 1", calls)
	}

	if _, err := store.UpdateProjects(func([]project.Project) ([]project.Project, error) {
		return nil, errors.New("abort")
	}); err == nil {
		t.Fatal("UpdateProjects() with failing mutator succeeded")
	}
	if calls != 1 {
		t.Fatalf("hook calls after failed update = %d, want 1", calls)
	}

	if _, err := store.ReplaceProjectsWithBackup([]project.Project{testProject("b")}); err != nil {
		t.Fatalf("ReplaceProjectsWithBackup() error: %v", err)
	}
	if err := store.SaveProjectsFile(DefaultProjectsFile()); err != nil {
		t.Fatalf("SaveProjectsFile() error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("hook calls after replace and save = %d, want 3", calls)
	}

	// The hook may read the catalog: it runs after the store lock is released.
	store.OnProjectsChanged(func() {
		if _, err := store.LoadProjects(); err != nil {
			t.Errorf("LoadProjects() inside hook: %v", err)
		}
		calls++
	})
	if _, err := store.UpdateProjects(func(ps []project.Project) ([]project.Project, error) { return ps, nil }); err != nil {
		t.Fatalf("UpdateProjects() error: %v", err)
	}
	if calls != 4 {
		t.Fatalf("hook calls = %d, want 4", calls)
	}
	backups, err := filepath.Glob(store.ProjectsPath() + ".bak-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("projects backups = %v, %v; want exactly one from the replacement", backups, err)
	}
}

func TestEnsureFilesCreatesDefaultsOnlyWhenMissing(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "nested", "projecthub"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.EnsureFiles(); err != nil {
		t.Fatalf("EnsureFiles() on a fresh directory: %v", err)
	}
	settings, err := store.LoadSettings()
	if err != nil || settings.SchemaVersion != CurrentSettingsSchemaVersion || settings.CloseBehavior != CloseBehaviorTray {
		t.Fatalf("created settings = %+v, %v; want defaults", settings, err)
	}
	if projects, err := store.LoadProjectsFile(); err != nil || len(projects.Projects) != 0 {
		t.Fatalf("created projects = %+v, %v; want an empty catalog", projects, err)
	}
	if info, err := os.Stat(store.SettingsPath()); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("settings.json mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}

	// Corrupted files and user changes are left exactly as they are.
	corrupted := []byte("{not json")
	writeTestFile(t, store.ProjectsPath(), corrupted)
	settings.Theme = ThemeLight
	if err := store.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	before := readTestFile(t, store.SettingsPath())
	if err := store.EnsureFiles(); err != nil {
		t.Fatalf("EnsureFiles() with existing files: %v", err)
	}
	assertFileBytes(t, store.ProjectsPath(), corrupted)
	assertFileBytes(t, store.SettingsPath(), before)
}

package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

func TestDefaultDirUsesXDGConfigHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dir, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir() error: %v", err)
	}
	want := filepath.Join(tmp, AppConfigDirName)
	if dir != want {
		t.Fatalf("got %q, want %q", dir, want)
	}
}

func TestStoreSettingsLifecycle(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "projecthub")
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	// Missing file returns defaults without creating a file.
	got, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() missing file error: %v", err)
	}
	if got != DefaultSettings() {
		t.Fatalf("got %+v, want %+v", got, DefaultSettings())
	}

	// Save custom settings atomically.
	custom := DefaultSettings()
	custom.Theme = ThemeDark
	custom.DefaultEditor = "zed"
	custom.Window.Width = 520

	if err := store.SaveSettings(custom); err != nil {
		t.Fatalf("SaveSettings() error: %v", err)
	}

	reloaded, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() after save error: %v", err)
	}
	if reloaded != custom {
		t.Fatalf("got %+v, want %+v", reloaded, custom)
	}

	// Verify file permissions and absence of leftover temp files.
	info, err := os.Stat(store.SettingsPath())
	if err != nil {
		t.Fatalf("Stat settings.json: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatalf("unexpected leftover temp file %q", entry.Name())
		}
	}
}

func TestStoreHandlesCorruptedJSONWithoutDataLoss(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	corruptContent := `{"schemaVersion": 1, "theme": "dark", BROKEN`
	if err := os.WriteFile(store.SettingsPath(), []byte(corruptContent), 0o600); err != nil {
		t.Fatalf("write corrupt file: %v", err)
	}

	_, err = store.LoadSettings()
	if !errors.Is(err, ErrCorruptedConfig) {
		t.Fatalf("expected ErrCorruptedConfig, got %v", err)
	}

	// Ensure original file was preserved intact.
	raw, err := os.ReadFile(store.SettingsPath())
	if err != nil {
		t.Fatalf("read settings.json after error: %v", err)
	}
	if string(raw) != corruptContent {
		t.Fatalf("corrupted settings.json was modified: got %q, want %q", string(raw), corruptContent)
	}
}

func TestStorePreservesExistingFieldsWhenLoadingPartialConfig(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	// Simulate an older user config that does not include newly added fields like `window`.
	partial := `{"schemaVersion": 1, "defaultEditor": "neovim", "theme": "light"}`
	if err := os.WriteFile(store.SettingsPath(), []byte(partial), 0o600); err != nil {
		t.Fatalf("write partial config: %v", err)
	}

	got, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error: %v", err)
	}
	if got.DefaultEditor != "neovim" || got.Theme != ThemeLight {
		t.Fatalf("user fields were not preserved: %+v", got)
	}
	if got.PreferredTerminal != "ghostty" || got.Window.Width != DefaultWindowWidth {
		t.Fatalf("missing fields did not receive defaults: %+v", got)
	}
}

func TestStoreRejectsUnsupportedFutureSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	future := `{"schemaVersion": 99, "defaultEditor": "vscode", "preferredTerminal": "ghostty", "preferredFileManager": "auto", "theme": "dark", "window": {"width": 440, "height": 680}}`
	if err := os.WriteFile(store.SettingsPath(), []byte(future), 0o600); err != nil {
		t.Fatalf("write future config: %v", err)
	}

	_, err = store.LoadSettings()
	if !errors.Is(err, ErrUnsupportedSchemaVersion) {
		t.Fatalf("expected ErrUnsupportedSchemaVersion, got %v", err)
	}
}

func TestStoreProjectsFileValidationAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	now := time.Date(2026, 11, 14, 15, 30, 0, 0, time.UTC)
	pf := ProjectsFile{
		SchemaVersion: CurrentProjectsSchemaVersion,
		Projects: []project.Project{
			{
				ID:            "proj-a",
				Name:          "Alpha",
				Description:   "First project",
				Path:          "/home/dev/alpha",
				Favorite:      true,
				DefaultEditor: "vscode",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
	}

	if err := store.SaveProjectsFile(pf); err != nil {
		t.Fatalf("SaveProjectsFile() error: %v", err)
	}

	loaded, err := store.LoadProjectsFile()
	if err != nil {
		t.Fatalf("LoadProjectsFile() error: %v", err)
	}
	if len(loaded.Projects) != 1 || loaded.Projects[0].Name != "Alpha" {
		t.Fatalf("unexpected loaded projects: %+v", loaded)
	}

	// Duplicate path must fail validation.
	pf.Projects = append(pf.Projects, project.Project{
		ID:        "proj-b",
		Name:      "Beta",
		Path:      "/home/dev/alpha",
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err := store.SaveProjectsFile(pf); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected ErrInvalidConfig for duplicate path, got %v", err)
	}
}

func TestStoreRefusesToOverwriteUnreadableConfig(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr error
		target  func(*Store) string
		save    func(*Store) error
	}{
		{
			name:    "corrupted settings.json is preserved on SaveSettings",
			content: `{"schemaVersion": 1, "theme": "dark", BROKEN`,
			wantErr: ErrCorruptedConfig,
			target:  func(s *Store) string { return s.SettingsPath() },
			save:    func(s *Store) error { return s.SaveSettings(DefaultSettings()) },
		},
		{
			name:    "unsupported schemaVersion settings.json is preserved on SaveSettings",
			content: `{"schemaVersion": 99, "defaultEditor": "zed", "preferredTerminal": "ghostty", "preferredFileManager": "auto", "theme": "dark", "window": {"width": 500, "height": 700}}`,
			wantErr: ErrUnsupportedSchemaVersion,
			target:  func(s *Store) string { return s.SettingsPath() },
			save:    func(s *Store) error { return s.SaveSettings(DefaultSettings()) },
		},
		{
			name:    "corrupted projects.json is preserved on SaveProjectsFile",
			content: `{"schemaVersion": 1, "projects": [`,
			wantErr: ErrCorruptedConfig,
			target:  func(s *Store) string { return s.ProjectsPath() },
			save:    func(s *Store) error { return s.SaveProjectsFile(DefaultProjectsFile()) },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatalf("NewStore() error: %v", err)
			}

			path := tc.target(store)
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}

			if err := tc.save(store); !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}

			raw, err := os.ReadFile(filepath.Clean(path))
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			if string(raw) != tc.content {
				t.Fatalf("file %s was overwritten: got %q, want %q", path, string(raw), tc.content)
			}
		})
	}
}

func TestResolveInitialWindowSize(t *testing.T) {
	s := DefaultSettings()
	s.Window.Width = 560
	s.Window.Height = 760
	s.Window.RememberSize = true

	w, h := ResolveInitialWindowSize(s)
	if w != 560 || h != 760 {
		t.Fatalf("got (%d, %d), want (560, 760)", w, h)
	}

	s.Window.RememberSize = false
	w, h = ResolveInitialWindowSize(s)
	if w != DefaultWindowWidth || h != DefaultWindowHeight {
		t.Fatalf("got (%d, %d), want defaults (%d, %d)", w, h, DefaultWindowWidth, DefaultWindowHeight)
	}
}

func TestSaveWindowSize(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	if err := store.SaveWindowSize(512, 740); err != nil {
		t.Fatalf("SaveWindowSize() error: %v", err)
	}
	assertStoredWindowSize(t, store, 512, 740)

	// Out-of-bounds GTK minimized sizes are ignored.
	if err := store.SaveWindowSize(0, 0); err != nil {
		t.Fatalf("SaveWindowSize(0, 0) error: %v", err)
	}
	assertStoredWindowSize(t, store, 512, 740)

	// Disabling RememberSize keeps previously saved size untouched.
	s, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error: %v", err)
	}
	s.Window.RememberSize = false
	if err := store.SaveSettings(s); err != nil {
		t.Fatalf("SaveSettings() error: %v", err)
	}
	if err := store.SaveWindowSize(640, 800); err != nil {
		t.Fatalf("SaveWindowSize() error: %v", err)
	}
	assertStoredWindowSize(t, store, 512, 740)
}

func assertStoredWindowSize(t *testing.T, store *Store, wantW, wantH int) {
	t.Helper()
	got, err := store.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() error: %v", err)
	}
	if got.Window.Width != wantW || got.Window.Height != wantH {
		t.Fatalf("got window size (%d, %d), want (%d, %d)", got.Window.Width, got.Window.Height, wantW, wantH)
	}
}

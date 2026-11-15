package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/semelyanov86/ProjectHub/internal/editor"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

func TestSettingsV1LazyMigration(t *testing.T) {
	tests := []struct {
		name, content string
		want          Settings
	}{
		{
			name: "custom legacy tools",
			content: `{"schemaVersion":1,"defaultEditor":"vscode","preferredTerminal":"kitty",
				"preferredFileManager":"nautilus","theme":"dark",
				"window":{"width":510,"height":710,"rememberSize":false,"alwaysOnTop":true}}`,
			want: func() Settings {
				s := DefaultSettings()
				s.PreferredTerminal = TerminalModeCustom
				s.CustomTerminal.Executable = "kitty"
				s.PreferredFileManager = FileManagerModeCustom
				s.Executables.FileManager = "nautilus"
				s.Theme = ThemeDark
				s.Window = WindowPreferences{Width: 510, Height: 710, AlwaysOnTop: true}
				return s
			}(),
		},
		{name: "missing schemaVersion", content: `{"defaultEditor":"neovim"}`, want: DefaultSettings()},
		{name: "empty legacy tools", content: `{"schemaVersion":1,"defaultEditor":"","preferredTerminal":"","preferredFileManager":""}`, want: DefaultSettings()},
		{name: "explicit zero version", content: `{"schemaVersion":0}`, want: DefaultSettings()},
		{
			name: "supported editor and native tools",
			content: `{"schemaVersion":1,"defaultEditor":"sublime","preferredTerminal":"ghostty",
				"preferredFileManager":"auto","rememberLastEditor":false,"showGitDirty":false}`,
			want: func() Settings {
				s := DefaultSettings()
				s.DefaultEditor = editor.IDSublime
				s.RememberLastEditor = false
				s.ShowGitDirty = false
				return s
			}(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestStore(t)
			writeTestFile(t, store.SettingsPath(), []byte(tc.content))
			got, err := store.LoadSettings()
			if err != nil {
				t.Fatalf("LoadSettings(): %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			assertFileBytes(t, store.SettingsPath(), []byte(tc.content))
		})
	}
}

func TestSettingsV2RoundTrip(t *testing.T) {
	store := newTestStore(t)
	settings := DefaultSettings()
	settings.DefaultEditor = editor.IDSublime
	settings.PreferredTerminal = TerminalModeCustom
	settings.CustomTerminal = TerminalCommand{Executable: "/opt/Some Terminal/bin/terminal", Args: []string{"--title", "project with spaces", "{path}"}}
	settings.Executables = ExecutablePaths{Sublime: "subl", Zed: "/usr/bin/zeditor", Ghostty: "ghostty", FileManager: "nautilus"}
	settings.PreferredFileManager = FileManagerModeCustom
	settings.ProjectSort = ProjectSortLastOpened
	settings.DoubleClickOpensEditor = false
	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings(): %v", err)
	}
	got, err := store.LoadSettings()
	if err != nil || !reflect.DeepEqual(got, settings) {
		t.Fatalf("LoadSettings() = %+v, %v; want %+v", got, err, settings)
	}
}

func TestSettingsNormalizesNilArgs(t *testing.T) {
	store := newTestStore(t)
	settings := DefaultSettings()
	settings.CustomTerminal.Args = nil
	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings(): %v", err)
	}
	if !strings.Contains(string(readTestFile(t, store.SettingsPath())), `"args": []`) {
		t.Fatal("saved args must be an empty JSON array")
	}
	writeTestFile(t, store.SettingsPath(), []byte(`{"schemaVersion":2,"customTerminal":{"args":null}}`))
	got, err := store.LoadSettings()
	if err != nil || got.CustomTerminal.Args == nil {
		t.Fatalf("LoadSettings() = %+v, %v; want non-nil args", got, err)
	}
}

func TestSettingsV2Validation(t *testing.T) {
	const firstArgField = "customTerminal.args[0]"
	tests := []struct {
		name, field string
		mutate      func(*Settings)
	}{
		{"unsupported editor", "defaultEditor", func(s *Settings) { s.DefaultEditor = "vscode" }},
		{"empty editor", "defaultEditor", func(s *Settings) { s.DefaultEditor = "" }},
		{"unknown terminal", "preferredTerminal", func(s *Settings) { s.PreferredTerminal = "kitty" }},
		{"empty custom terminal", "customTerminal.executable", func(s *Settings) { s.PreferredTerminal = TerminalModeCustom }},
		{"blank custom terminal", "customTerminal.executable", func(s *Settings) { s.PreferredTerminal = TerminalModeCustom; s.CustomTerminal.Executable = "  " }},
		{"unknown file manager", "preferredFileManager", func(s *Settings) { s.PreferredFileManager = "nautilus" }},
		{"empty custom file manager", "executables.fileManager", func(s *Settings) { s.PreferredFileManager = FileManagerModeCustom }},
		{"relative executable", "executables.zed", func(s *Settings) { s.Executables.Zed = "bin/zed" }},
		{"control in sublime", "executables.sublime", func(s *Settings) { s.Executables.Sublime = "subl\t" }},
		{"control in ghostty", "executables.ghostty", func(s *Settings) { s.Executables.Ghostty = "ghostty\x7f" }},
		{"control in file manager", "executables.fileManager", func(s *Settings) { s.Executables.FileManager = "nautilus\x00" }},
		{"control in terminal", "customTerminal.executable", func(s *Settings) { s.CustomTerminal.Executable = "kitty\x01" }},
		{"long executable", "executables.zed", func(s *Settings) { s.Executables.Zed = strings.Repeat("z", 4097) }},
		{"too many args", "customTerminal.args", func(s *Settings) { s.CustomTerminal.Args = make([]string, 33) }},
		{"long arg", firstArgField, func(s *Settings) { s.CustomTerminal.Args = []string{strings.Repeat("a", 4097)} }},
		{"nul arg", firstArgField, func(s *Settings) { s.CustomTerminal.Args = []string{"a\x00"} }},
		{"cr arg", firstArgField, func(s *Settings) { s.CustomTerminal.Args = []string{"a\r"} }},
		{"lf arg", firstArgField, func(s *Settings) { s.CustomTerminal.Args = []string{"a\n"} }},
		{"bad sort", "projectSort", func(s *Settings) { s.ProjectSort = "recent" }},
		{"bad theme", "theme", func(s *Settings) { s.Theme = "blue" }},
		{"bad width", "window.width", func(s *Settings) { s.Window.Width = 0 }},
		{"bad height", "window.height", func(s *Settings) { s.Window.Height = 9999 }},
		{"old schema payload", "schemaVersion", func(s *Settings) { s.SchemaVersion = 1 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultSettings()
			tc.mutate(&settings)
			err := settings.Validate()
			if !errors.Is(err, ErrInvalidConfig) || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Validate() = %v; want ErrInvalidConfig mentioning %s", err, tc.field)
			}
		})
	}
}

func TestProjectsLazyMigration(t *testing.T) {
	for _, version := range []int{0, 1, 3} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			store := newTestStore(t)
			p := testProject("legacy")
			p.DefaultEditor = "vscode"
			p.LastUsedEditor = "neovim"
			data := encodeTestJSON(t, ProjectsFile{SchemaVersion: version, Projects: []project.Project{p}})
			if version == 0 {
				data = []byte(strings.Replace(string(data), `"schemaVersion":0,`, "", 1))
			}
			writeTestFile(t, store.ProjectsPath(), data)
			got, err := store.LoadProjectsFile()
			if version == 3 {
				if !errors.Is(err, ErrUnsupportedSchemaVersion) {
					t.Fatalf("LoadProjectsFile() error = %v", err)
				}
			} else {
				p.DefaultEditor, p.LastUsedEditor = "", ""
				want := ProjectsFile{SchemaVersion: 2, Projects: []project.Project{p}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("LoadProjectsFile() = %+v, %v; want %+v", got, err, want)
				}
			}
			assertFileBytes(t, store.ProjectsPath(), data)
		})
	}
}

func TestSaveRejectsV1Payloads(t *testing.T) {
	store := newTestStore(t)
	settings := DefaultSettings()
	settings.SchemaVersion = 1
	if err := store.SaveSettings(settings); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("SaveSettings(v1) = %v", err)
	}
	pf := DefaultProjectsFile()
	pf.SchemaVersion = 1
	if err := store.SaveProjectsFile(pf); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("SaveProjectsFile(v1) = %v", err)
	}
	settings.SchemaVersion = 0
	if err := store.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings(version 0): %v", err)
	}
	pf.SchemaVersion = 0
	if err := store.SaveProjectsFile(pf); err != nil {
		t.Fatalf("SaveProjectsFile(version 0): %v", err)
	}
}

func encodeTestJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(): %v", err)
	}
	return data
}

func TestDefaultSettingsV2(t *testing.T) {
	got := DefaultSettings()
	want := Settings{
		SchemaVersion: 2, DefaultEditor: "zed",
		PreferredTerminal: TerminalModeGhostty, PreferredFileManager: FileManagerModeAuto,
		Theme: ThemeSystem, Window: WindowPreferences{Width: 440, Height: 650, RememberSize: true},
		CustomTerminal: TerminalCommand{Args: []string{}}, RememberLastEditor: true,
		DoubleClickOpensEditor: true, ShowGitDirty: true, ProjectSort: ProjectSortName,
		CloseBehavior: CloseBehaviorTray,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultSettings() = %+v; want %+v", got, want)
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("DefaultSettings().Validate(): %v", err)
	}
}

func TestLoadProjectsCleansPaths(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"schemaVersion": 2, "projects": [{"id": "p1", "name": "A", "path": "/home/dev//alpha/./", ` +
		`"createdAt": "2026-11-15T00:00:00Z", "updatedAt": "2026-11-15T00:00:00Z"}]}`
	if err := os.WriteFile(store.ProjectsPath(), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	projects, err := store.LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects() error: %v", err)
	}
	if len(projects) != 1 || projects[0].Path != "/home/dev/alpha" {
		t.Fatalf("LoadProjects() = %+v, want cleaned path /home/dev/alpha", projects)
	}
}

func TestSettingsRejectExecutableCommandLines(t *testing.T) {
	for _, value := range []string{"zed;rm", "/opt/$dir/zed", "subl -n", "zed`x`", "a|b", "a&b"} {
		s := DefaultSettings()
		s.Executables.Zed = value
		if err := s.Validate(); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("Validate(executables.zed=%q) = %v, want ErrInvalidConfig", value, err)
		}
	}
	for _, value := range []string{"zeditor", "/opt/My Apps/zed", ""} {
		s := DefaultSettings()
		s.Executables.Zed = value
		if err := s.Validate(); err != nil {
			t.Errorf("Validate(executables.zed=%q) = %v, want nil", value, err)
		}
	}
}

// legacyEditor is a stage-1 editor ID that is no longer supported.
const (
	legacyEditor      = "vscode"
	nautilusNewWindow = "nautilus --new-window"
)

func TestLoadKeepsLegacyRecordsUsable(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	longDescription := strings.Repeat("d", 2001)
	raw := []byte(`{"schemaVersion":1,"projects":[{"id":"legacy","name":"first\nsecond ` + strings.Repeat("n", 250) +
		`","description":"` + longDescription + `","path":"/code/legacy","favorite":true,"defaultEditor":"` + legacyEditor +
		`","createdAt":"2026-11-01T00:00:00Z","updatedAt":"2026-11-01T00:00:00Z"}]}`)
	if err := os.WriteFile(store.ProjectsPath(), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	projects, err := store.LoadProjects()
	if err != nil {
		t.Fatalf("LoadProjects() rejected a stage-1 catalog: %v", err)
	}
	if len(projects) != 1 || projects[0].Description != longDescription || projects[0].DefaultEditor != "" {
		t.Fatalf("LoadProjects() = %+v, want data preserved and editor inherited", projects)
	}
	export := filepath.Join(t.TempDir(), "export.json")
	if n, err := store.ExportProjects(export); err != nil || n != 1 {
		t.Fatalf("ExportProjects() = %d, %v; want the legacy catalog exported", n, err)
	}
	if _, err := store.UpdateProjects(func(ps []project.Project) ([]project.Project, error) { return ps, nil }); err != nil {
		t.Fatalf("UpdateProjects() on a legacy catalog: %v", err)
	}
}

func TestSettingsV1LegacyCommands(t *testing.T) {
	for _, tc := range []struct {
		name, terminal, fileManager string
		wantTerminal                TerminalMode
		wantCustom                  TerminalCommand
		wantFileManager             FileManagerMode
		wantFileManagerExe          string
	}{
		{
			name: "commands with arguments", terminal: "gnome-terminal --tab", fileManager: nautilusNewWindow,
			wantTerminal: TerminalModeCustom, wantCustom: TerminalCommand{Executable: "gnome-terminal", Args: []string{"--tab"}},
			wantFileManager: FileManagerModeCustom, wantFileManagerExe: strings.Fields(nautilusNewWindow)[0],
		},
		{
			name: "unusable values fall back", terminal: "kitty;rm -rf", fileManager: "$FM",
			wantTerminal: TerminalModeGhostty, wantCustom: TerminalCommand{Executable: "", Args: []string{}},
			wantFileManager: FileManagerModeAuto,
		},
		{
			name: "absolute paths with spaces stay whole", terminal: "/opt/My Tools/bin/launcher",
			fileManager:  " /opt/My Files/bin/files ",
			wantTerminal: TerminalModeCustom, wantCustom: TerminalCommand{Executable: "/opt/My Tools/bin/launcher", Args: []string{}},
			wantFileManager: FileManagerModeCustom, wantFileManagerExe: "/opt/My Files/bin/files",
		},
		{
			name: "defaults with spaces", terminal: " ghostty ", fileManager: " auto ",
			wantTerminal: TerminalModeGhostty, wantCustom: TerminalCommand{Executable: "", Args: []string{}},
			wantFileManager: FileManagerModeAuto,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(map[string]any{
				"schemaVersion": 1, "defaultEditor": legacyEditor, "preferredTerminal": tc.terminal,
				"preferredFileManager": tc.fileManager, "theme": "dark",
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(store.SettingsPath(), raw, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := store.LoadSettings()
			if err != nil {
				t.Fatalf("LoadSettings() rejected stage-1 settings: %v", err)
			}
			if got.PreferredTerminal != tc.wantTerminal || !reflect.DeepEqual(got.CustomTerminal, tc.wantCustom) ||
				got.PreferredFileManager != tc.wantFileManager || got.Executables.FileManager != tc.wantFileManagerExe {
				t.Fatalf("LoadSettings() = %+v", got)
			}
		})
	}
}

func TestLoadNormalisesUnknownEditorsInV2(t *testing.T) {
	store, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw := `{"schemaVersion": 2, "projects": [{"id": "p1", "name": "A", "path": "/code/a", "defaultEditor": "` + legacyEditor + `", ` +
		`"lastUsedEditor": "nvim", "createdAt": "2026-11-15T00:00:00Z", "updatedAt": "2026-11-15T00:00:00Z"}]}`
	if err := os.WriteFile(store.ProjectsPath(), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	projects, err := store.LoadProjects()
	if err != nil || len(projects) != 1 || projects[0].DefaultEditor != "" || projects[0].LastUsedEditor != "" {
		t.Fatalf("LoadProjects() = %+v, %v; want editors normalised to inherit", projects, err)
	}
}

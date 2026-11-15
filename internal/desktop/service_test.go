package desktop

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/git"
	"github.com/semelyanov86/ProjectHub/internal/launcher"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

func TestProjectRoundTrip(t *testing.T) {
	h := newTestService(t)
	dir := projectDirectory(t, specialDirectoryName)
	created := addTestProject(t, h.Service, dir)
	if created.ID == "" {
		t.Fatalf("AddProject() = %+v", created)
	}
	assertProjectInput(t, created, project.Input{Name: specialDirectoryName, Path: dir})
	assertProjects(t, h.Service, []project.Project{created})
	input := project.Input{
		Name: "Renamed project", Description: "Updated description", Path: dir,
		DefaultEditor: "sublime", Favorite: false,
	}
	updated, err := h.UpdateProject(created.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID {
		t.Fatalf("UpdateProject() = %+v", updated)
	}
	assertProjectInput(t, updated, input)
	for _, favorite := range []bool{true, false, true} {
		updated, err = h.SetFavorite(created.ID, favorite)
		if err != nil {
			t.Fatal(err)
		}
		if updated.Favorite != favorite {
			t.Fatalf("SetFavorite(%v) = %+v", favorite, updated)
		}
	}
	reopened, err := config.NewStore(h.store.Dir())
	if err != nil {
		t.Fatal(err)
	}
	fresh := NewService(Deps{Catalog: project.NewCatalog(reopened, launcher.ValidateProjectDir)})
	assertProjects(t, fresh, []project.Project{updated})
	if err := h.RemoveProject(created.ID); err != nil {
		t.Fatal(err)
	}
	assertProjects(t, fresh, []project.Project{})
	assertDirectory(t, dir)
}

func TestChooseDirectory(t *testing.T) {
	for _, tc := range []struct {
		name      string
		duplicate bool
	}{
		{name: "valid"},
		{name: "duplicate", duplicate: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestService(t)
			dir := projectDirectory(t, specialDirectoryName)
			h.dialogs.directory = dir + "/./"
			var existing project.Project
			if tc.duplicate {
				existing = addTestProject(t, h.Service, dir)
			}
			got, err := h.ChooseDirectory(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if got.Cancelled || got.Path != dir || got.SuggestedName != specialDirectoryName {
				t.Fatalf("draft = %+v", got)
			}
			if got.Git.State != git.StateRepo || got.Git.Repo.Branch != "main" {
				t.Fatalf("draft Git = %+v", got.Git)
			}
			if got.DuplicateID != existing.ID || got.DuplicateName != existing.Name {
				t.Fatalf("duplicate = %q/%q, want %q/%q", got.DuplicateID, got.DuplicateName, existing.ID, existing.Name)
			}
		})
	}
}

func TestChooseDirectoryCancelled(t *testing.T) {
	h := newTestService(t)
	got, err := h.ChooseDirectory(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !got.Cancelled {
		t.Fatalf("ChooseDirectory() = %+v, want cancellation", got)
	}
}

func TestChooseDirectoryErrors(t *testing.T) {
	pickerErr := errors.New("picker failed")
	for _, tc := range []struct {
		name               string
		dialogErr, wantErr error
	}{
		{name: "regular file", wantErr: launcher.ErrInvalidTargetPath},
		{name: "dialog error", dialogErr: pickerErr, wantErr: pickerErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestService(t)
			h.dialogs.directory = filepath.Join(t.TempDir(), "file")
			writeTestFile(t, h.dialogs.directory, []byte("file"))
			h.dialogs.directoryErr = tc.dialogErr
			if _, err := h.ChooseDirectory(t.Context()); !errors.Is(err, tc.wantErr) {
				t.Fatalf("ChooseDirectory() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestChooseExecutable(t *testing.T) {
	pickerErr := errors.New("executable picker failed")
	for _, tc := range []struct {
		name, path string
		err        error
	}{
		{name: "cancelled"},
		{name: "selected", path: "/opt/my editor/bin/editor"},
		{name: "error", err: pickerErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestService(t)
			h.dialogs.executable, h.dialogs.executableErr = tc.path, tc.err
			got, err := h.ChooseExecutable(t.Context())
			if got != tc.path || !errors.Is(err, tc.err) {
				t.Fatalf("ChooseExecutable() = %q, %v, want %q, %v", got, err, tc.path, tc.err)
			}
		})
	}
}

func TestListTools(t *testing.T) {
	h := newTestService(t)
	got, err := h.ListTools()
	if err != nil {
		t.Fatal(err)
	}
	targets := []launcher.Target{
		launcher.TargetSublime, launcher.TargetZed, launcher.TargetGhostty,
		launcher.TargetTerminal, launcher.TargetFileManager,
	}
	if len(got) != len(targets) {
		t.Fatalf("ListTools() = %+v, want five tools", got)
	}
	for i, target := range targets {
		if got[i].Target != target {
			t.Errorf("tool[%d] target = %q, want %q", i, got[i].Target, target)
		}
	}
	for i, executable := range []string{sublimeExecutable, "zed", "ghostty"} {
		if !got[i].Available || got[i].Executable != filepath.Join(h.binDir, executable) {
			t.Errorf("tool[%d] = %+v, want available %s", i, got[i], executable)
		}
	}
	if len(h.TerminalPresets()) == 0 {
		t.Error("TerminalPresets() is empty")
	}
}

func TestListToolsCorruptedSettings(t *testing.T) {
	h := newTestService(t)
	writeTestFile(t, h.store.SettingsPath(), []byte("{"))
	if _, err := h.ListTools(); !errors.Is(err, config.ErrCorruptedConfig) {
		t.Fatalf("ListTools() error = %v, want ErrCorruptedConfig", err)
	}
}

func TestPreferences(t *testing.T) {
	settings := config.DefaultSettings()
	settings.PreferredTerminal = config.TerminalModeCustom
	settings.PreferredFileManager = config.FileManagerModeCustom
	settings.Executables = config.ExecutablePaths{
		Sublime: "/opt/subl", Zed: "/opt/zed", Ghostty: "/opt/ghostty", FileManager: "/opt/files",
	}
	settings.CustomTerminal = config.TerminalCommand{
		Executable: "/opt/terminal", Args: []string{"--directory", "{path}"},
	}
	got := preferences(settings)
	wantPaths := launcher.ToolPaths{
		Sublime: "/opt/subl", Zed: "/opt/zed", Ghostty: "/opt/ghostty", FileManager: "/opt/files",
	}
	if got.Paths != wantPaths || got.FileManagerMode != launcher.FileManagerCustom {
		t.Fatalf("preferences = %+v", got)
	}
	if got.CustomTerminal.Executable != settings.CustomTerminal.Executable ||
		!slices.Equal(got.CustomTerminal.Args, settings.CustomTerminal.Args) {
		t.Fatalf("custom terminal = %+v, want %+v", got.CustomTerminal, settings.CustomTerminal)
	}
	got.CustomTerminal.Args[0] = "changed"
	if settings.CustomTerminal.Args[0] != "--directory" {
		t.Error("preferences aliases persisted terminal arguments")
	}
}

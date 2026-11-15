package desktop

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/editor"
	"github.com/semelyanov86/ProjectHub/internal/launcher"
	"github.com/semelyanov86/ProjectHub/internal/process"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

const unsupportedEditorID = "vscode"

func TestOpenProjectEditor(t *testing.T) {
	for _, remember := range []bool{true, false} {
		name := "remember"
		if !remember {
			name = "do not remember"
		}
		t.Run(name, func(t *testing.T) {
			h := newTestService(t)
			p := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
			rememberEditor(t, h, p, editor.IDZed)
			settings := config.DefaultSettings()
			settings.RememberLastEditor = remember
			saveTestSettings(t, h.store, settings)
			got, err := h.OpenProject(t.Context(), p.ID, launcher.TargetSublime)
			if err != nil {
				t.Fatal(err)
			}
			wantEditor := editor.IDZed
			if remember {
				wantEditor = editor.IDSublime
			}
			if got.Warning != "" || got.Project.LastOpened == nil || got.Project.LastUsedEditor != wantEditor {
				t.Fatalf("OpenProject() = %+v", got)
			}
			persisted := getTestProject(t, h.Service, p.ID)
			if persisted.LastOpened == nil || !persisted.LastOpened.Equal(*got.Project.LastOpened) ||
				persisted.LastUsedEditor != wantEditor {
				t.Fatalf("launch metadata not persisted: %+v", persisted)
			}
			runs, starts := h.runner.commands()
			if len(runs) != 0 || len(starts) != 1 {
				t.Fatalf("Run/Start counts = %d/%d, want 0/1", len(runs), len(starts))
			}
			assertCommand(t, starts[0], process.Command{
				Path: filepath.Join(h.binDir, sublimeExecutable), Args: []string{p.Path}, Dir: p.Path,
			})
		})
	}
}

func TestOpenProjectNonEditor(t *testing.T) {
	for _, tc := range []struct {
		target     launcher.Target
		executable string
		args       func(string) []string
	}{
		{
			target: launcher.TargetGhostty, executable: "ghostty",
			args: func(dir string) []string { return []string{"+new-window", "--working-directory=" + dir} },
		},
		{
			target: launcher.TargetFileManager, executable: "gio",
			args: func(dir string) []string { return []string{"open", dir} },
		},
	} {
		t.Run(string(tc.target), func(t *testing.T) {
			h := newTestService(t)
			p := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
			rememberEditor(t, h, p, editor.IDSublime)
			oldOpened := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
			if _, err := h.store.UpdateProjects(func(projects []project.Project) ([]project.Project, error) {
				projects[0].LastOpened = &oldOpened
				return projects, nil
			}); err != nil {
				t.Fatal(err)
			}
			got, err := h.OpenProject(t.Context(), p.ID, tc.target)
			if err != nil {
				t.Fatal(err)
			}
			if got.Project.LastOpened == nil || !got.Project.LastOpened.After(oldOpened) ||
				got.Project.LastUsedEditor != editor.IDSublime {
				t.Fatalf("non-editor launch metadata = %+v", got.Project)
			}
			persisted := getTestProject(t, h.Service, p.ID)
			if persisted.LastOpened == nil || !persisted.LastOpened.Equal(*got.Project.LastOpened) ||
				persisted.LastUsedEditor != editor.IDSublime {
				t.Fatalf("persisted metadata = %+v", persisted)
			}
			runs, starts := h.runner.commands()
			if len(runs) != 1 || len(starts) != 0 {
				t.Fatalf("Run/Start counts = %d/%d, want 1/0", len(runs), len(starts))
			}
			assertCommand(t, runs[0], process.Command{
				Path: filepath.Join(h.binDir, tc.executable), Args: tc.args(p.Path), Dir: p.Path,
			})
		})
	}
}

func TestOpenProjectMissingDirectory(t *testing.T) {
	h := newTestService(t)
	p := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
	if err := os.Remove(p.Path); err != nil {
		t.Fatal(err)
	}
	_, err := h.OpenProject(t.Context(), p.ID, launcher.TargetSublime)
	if !errors.Is(err, launcher.ErrInvalidTargetPath) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("OpenProject() error = %v, want invalid target and missing file", err)
	}
	want := "The directory " + p.Path + " no longer exists. Edit the project to update its location, or remove it."
	if err.Error() != want {
		t.Fatalf("OpenProject() message = %q, want %q", err.Error(), want)
	}
	if got := getTestProject(t, h.Service, p.ID); got.LastOpened != nil {
		t.Fatalf("failed launch recorded LastOpened: %+v", got)
	}
	runs, starts := h.runner.commands()
	if len(runs)+len(starts) != 0 {
		t.Fatal("missing directory invoked a process")
	}
}

func TestOpenProjectUnavailableEditor(t *testing.T) {
	h := newTestService(t)
	p := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
	if err := os.Remove(filepath.Join(h.binDir, "zed")); err != nil {
		t.Fatal(err)
	}
	_, err := h.OpenProject(t.Context(), p.ID, launcher.TargetZed)
	if !errors.Is(err, launcher.ErrToolNotFound) {
		t.Fatalf("OpenProject() error = %v, want ErrToolNotFound", err)
	}
	if msg := err.Error(); !strings.HasPrefix(msg, "Zed was not found (") ||
		!strings.HasSuffix(msg, "). Install it or set its executable in Settings.") ||
		strings.Contains(msg, "application not found") {
		t.Fatalf("OpenProject() message = %q, want a short actionable Zed message", msg)
	}
	if got := getTestProject(t, h.Service, p.ID); got.LastOpened != nil {
		t.Fatalf("unavailable editor recorded launch: %+v", got)
	}
}

func TestOpenUnknownProject(t *testing.T) {
	h := newTestService(t)
	if _, err := h.OpenProject(t.Context(), "unknown", launcher.TargetSublime); !errors.Is(err, project.ErrProjectNotFound) {
		t.Fatalf("OpenProject() error = %v, want ErrProjectNotFound", err)
	}
	if _, err := h.OpenProjectDefault(t.Context(), "unknown"); !errors.Is(err, project.ErrProjectNotFound) {
		t.Fatalf("OpenProjectDefault() error = %v, want ErrProjectNotFound", err)
	}
}

func TestOpenProjectLaunchInProgress(t *testing.T) {
	h := newTestService(t)
	p := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
	entered := make(chan struct{}, 1)
	gate := make(chan struct{})
	h.runner.startEntered, h.runner.startGate = entered, gate
	var once sync.Once
	release := func() { once.Do(func() { close(gate) }) }
	done := make(chan error, 1)
	t.Cleanup(func() {
		release()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("first launch failed: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Error("blocked launch did not finish")
		}
	})
	ctx := t.Context()
	go func() {
		_, err := h.OpenProject(ctx, p.ID, launcher.TargetSublime)
		done <- err
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("first launch did not enter Start")
	}
	if _, err := h.OpenProject(ctx, p.ID, launcher.TargetSublime); !errors.Is(err, ErrLaunchInProgress) {
		t.Fatalf("second launch error = %v, want ErrLaunchInProgress", err)
	}
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
		done <- nil
	case <-time.After(5 * time.Second):
		t.Fatal("first launch did not finish")
	}
	if _, err := h.OpenProject(ctx, p.ID, launcher.TargetSublime); err != nil {
		t.Fatalf("launch guard was not released: %v", err)
	}
	_, starts := h.runner.commands()
	if len(starts) != 2 {
		t.Fatalf("Start count = %d, want 2", len(starts))
	}
}

func TestOpenProjectCorruptedSettings(t *testing.T) {
	h := newTestService(t)
	p := addTestProject(t, h.Service, projectDirectory(t, specialDirectoryName))
	writeTestFile(t, h.store.SettingsPath(), []byte("{"))
	got, err := h.OpenProject(t.Context(), p.ID, launcher.TargetSublime)
	if err != nil {
		t.Fatal(err)
	}
	if got.Warning == "" || got.Project.LastOpened == nil || got.Project.LastUsedEditor != editor.IDSublime {
		t.Fatalf("fallback launch = %+v", got)
	}
	if got := string(readTestFile(t, h.store.SettingsPath())); got != "{" {
		t.Fatalf("corrupted settings overwritten: %q", got)
	}
	_, starts := h.runner.commands()
	if len(starts) != 1 {
		t.Fatalf("Start count = %d, want 1", len(starts))
	}
}

func TestOpenProjectDefault(t *testing.T) {
	for _, tc := range []struct {
		name, explicit, last, global, executable string
		remember                                 bool
	}{
		{
			name: "explicit", explicit: editor.IDSublime, last: editor.IDZed,
			global: editor.IDZed, remember: true, executable: sublimeExecutable,
		},
		{
			name: "remembered", last: editor.IDSublime,
			global: editor.IDZed, remember: true, executable: sublimeExecutable,
		},
		{
			name: "global when remembering disabled", last: editor.IDZed,
			global: editor.IDSublime, remember: false, executable: sublimeExecutable,
		},
		{name: "global without remembered editor", global: editor.IDSublime, remember: true, executable: sublimeExecutable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newTestService(t)
			dir := projectDirectory(t, specialDirectoryName)
			p, err := h.AddProject(project.Input{Name: specialDirectoryName, Path: dir, DefaultEditor: tc.explicit})
			if err != nil {
				t.Fatal(err)
			}
			rememberEditor(t, h, p, tc.last)
			settings := config.DefaultSettings()
			settings.DefaultEditor, settings.RememberLastEditor = tc.global, tc.remember
			saveTestSettings(t, h.store, settings)
			if _, err := h.OpenProjectDefault(t.Context(), p.ID); err != nil {
				t.Fatal(err)
			}
			_, starts := h.runner.commands()
			if len(starts) != 1 {
				t.Fatalf("Start count = %d, want 1", len(starts))
			}
			assertCommand(t, starts[0], process.Command{
				Path: filepath.Join(h.binDir, tc.executable), Args: []string{dir}, Dir: dir,
			})
		})
	}
}

func TestEffectiveEditor(t *testing.T) {
	for _, tc := range []struct {
		name, explicit, last, global, want string
		remember                           bool
	}{
		{
			name: "explicit wins", explicit: editor.IDSublime, last: editor.IDZed,
			global: editor.IDZed, remember: true, want: editor.IDSublime,
		},
		{name: "remembered wins", last: editor.IDSublime, global: editor.IDZed, remember: true, want: editor.IDSublime},
		{name: "remembering disabled", last: editor.IDSublime, global: editor.IDZed, want: editor.IDZed},
		{name: "global", global: editor.IDSublime, remember: true, want: editor.IDSublime},
		{
			name: "unsupported explicit", explicit: unsupportedEditorID, last: editor.IDSublime,
			global: editor.IDZed, remember: true, want: editor.IDSublime,
		},
		{name: "unsupported remembered", last: unsupportedEditorID, global: editor.IDSublime, remember: true, want: editor.IDSublime},
		{
			name: "all unsupported", explicit: unsupportedEditorID, last: unsupportedEditorID, global: unsupportedEditorID,
			remember: true, want: editor.DefaultID,
		},
		{name: "empty values", remember: true, want: editor.DefaultID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := project.Project{DefaultEditor: tc.explicit, LastUsedEditor: tc.last}
			settings := config.Settings{DefaultEditor: tc.global, RememberLastEditor: tc.remember}
			if got := EffectiveEditor(p, settings); got != tc.want {
				t.Errorf("EffectiveEditor() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLaunchDetailStripsLauncherPrefixes(t *testing.T) {
	cases := []struct {
		err      error
		sentinel error
		tool     string
		want     string
	}{
		{
			fmt.Errorf("%w: File Manager: %w", launcher.ErrLaunchFailed, errors.New("gio: exit status 2")),
			launcher.ErrLaunchFailed, "File manager", "gio: exit status 2",
		},
		{
			fmt.Errorf("Zed: %w: \"zed, zeditor\" was not found", launcher.ErrToolNotFound),
			launcher.ErrToolNotFound, "Zed", "\"zed, zeditor\" was not found",
		},
		{
			// Only the launcher's own sentinel prefix is removed, not a later repetition.
			fmt.Errorf("%w: Ghostty: stderr mentions application not found: x", launcher.ErrToolNotFound),
			launcher.ErrToolNotFound, "Ghostty", "stderr mentions application not found: x",
		},
	}
	for _, tc := range cases {
		if got := launchDetail(tc.err, tc.sentinel, tc.tool); got != tc.want {
			t.Errorf("launchDetail(%q) = %q, want %q", tc.err, got, tc.want)
		}
	}
}

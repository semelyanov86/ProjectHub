package config

import (
	"path/filepath"
	"strings"

	"github.com/semelyanov86/ProjectHub/internal/editor"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

func migrateSettingsV1(settings *Settings) {
	if !editor.IsSupported(settings.DefaultEditor) {
		settings.DefaultEditor = editor.DefaultID
	}
	// Stage-1 terminal and file manager values were free-form command names
	// that were never executed. They become custom executables when usable;
	// extra words become terminal arguments (never shell-interpreted), and
	// values that cannot be an executable fall back to the defaults instead of
	// making the whole settings file unreadable.
	legacyTerminal := string(settings.PreferredTerminal)
	legacyFileManager := string(settings.PreferredFileManager)
	settings.PreferredTerminal = TerminalModeGhostty
	if exe, args, ok := legacyCommand(legacyTerminal, string(TerminalModeGhostty)); ok {
		settings.CustomTerminal = TerminalCommand{Executable: exe, Args: args}
		settings.PreferredTerminal = TerminalModeCustom
	}
	settings.PreferredFileManager = FileManagerModeAuto
	if exe, _, ok := legacyCommand(legacyFileManager, string(FileManagerModeAuto)); ok {
		settings.Executables.FileManager = exe
		settings.PreferredFileManager = FileManagerModeCustom
	}
	settings.SchemaVersion = CurrentSettingsSchemaVersion
}

// legacyCommand converts a stage-1 free-form command value into an executable
// and arguments. An absolute path is kept whole (it may contain spaces); a bare
// command name is split on whitespace into the name and its arguments. ok is
// false for empty values, the built-in default, or values whose executable is
// not acceptable under the current rules.
func legacyCommand(value, builtin string) (exe string, args []string, ok bool) {
	if trimmed := strings.TrimSpace(value); filepath.IsAbs(trimmed) {
		if validateExecutable("legacy", trimmed) != nil {
			return "", nil, false
		}
		return trimmed, []string{}, true
	}
	fields := strings.Fields(value)
	if len(fields) == 0 || (len(fields) == 1 && fields[0] == builtin) {
		return "", nil, false
	}
	if validateExecutable("legacy", fields[0]) != nil || validateTerminalArgs(fields[1:]) != nil {
		return "", nil, false
	}
	return fields[0], append([]string{}, fields[1:]...), true
}

func migrateProjectsV1(pf *ProjectsFile) {
	for i := range pf.Projects {
		pf.Projects[i].DefaultEditor = project.NormalizeEditorRef(pf.Projects[i].DefaultEditor)
		pf.Projects[i].LastUsedEditor = project.NormalizeEditorRef(pf.Projects[i].LastUsedEditor)
	}
	pf.SchemaVersion = CurrentProjectsSchemaVersion
}

func normalizeLoadedProjects(pf *ProjectsFile) error {
	if pf.SchemaVersion == 0 {
		pf.SchemaVersion = 1
	}
	if pf.SchemaVersion == 1 {
		migrateProjectsV1(pf)
	}
	if pf.Projects == nil {
		pf.Projects = []project.Project{}
	}
	for i := range pf.Projects {
		p := &pf.Projects[i]
		if p.Path != "" {
			p.Path = project.PathKey(p.Path)
		}
		// Unknown editor references (e.g. hand-edited files) fall back to
		// "inherit" instead of making the whole catalog unreadable.
		p.DefaultEditor = project.NormalizeEditorRef(p.DefaultEditor)
		p.LastUsedEditor = project.NormalizeEditorRef(p.LastUsedEditor)
	}
	return pf.Validate()
}

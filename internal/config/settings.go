// Package config manages XDG-compliant JSON configuration files for ProjectHub.
package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/semelyanov86/ProjectHub/internal/editor"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

const (
	// AppConfigDirName is the subdirectory name under XDG_CONFIG_HOME.
	AppConfigDirName = "projecthub"
	// SettingsFileName is the filename for user preferences.
	SettingsFileName = "settings.json"
	// ProjectsFileName is the filename for registered project metadata.
	ProjectsFileName = "projects.json"

	// CurrentSettingsSchemaVersion is the current schema version for settings.json.
	CurrentSettingsSchemaVersion = 2
	// CurrentProjectsSchemaVersion is the current schema version for projects.json.
	CurrentProjectsSchemaVersion = 2

	// DefaultWindowWidth is the initial compact desktop window width.
	DefaultWindowWidth = 440
	// DefaultWindowHeight is the initial compact desktop window height.
	DefaultWindowHeight = 650

	minWindowDimension = 320
	maxWindowWidth     = 7680
	maxWindowHeight    = 4320
	maxExecutableSize  = 4096
	maxTerminalArgs    = 32
)

var (
	// ErrInvalidConfig indicates configuration failed validation.
	ErrInvalidConfig = errors.New("invalid configuration")
	// ErrCorruptedConfig indicates an existing configuration file could not be parsed as valid JSON.
	ErrCorruptedConfig = errors.New("corrupted configuration file")
	// ErrUnsupportedSchemaVersion indicates a configuration file has an unsupported schema version.
	ErrUnsupportedSchemaVersion = errors.New("unsupported schema version")
)

// ThemeMode defines the UI color theme preference.
type ThemeMode string

// Supported UI color theme modes.
const (
	ThemeSystem ThemeMode = "system"
	ThemeLight  ThemeMode = "light"
	ThemeDark   ThemeMode = "dark"
)

// TerminalMode selects the terminal ProjectHub opens projects in.
type TerminalMode string

// Supported terminal modes.
const (
	TerminalModeGhostty TerminalMode = "ghostty"
	TerminalModeCustom  TerminalMode = "custom"
)

// FileManagerMode selects how project directories are revealed.
type FileManagerMode string

// Supported file manager modes.
const (
	FileManagerModeAuto   FileManagerMode = "auto"
	FileManagerModeCustom FileManagerMode = "custom"
)

// ProjectSort is the list ordering inside the All and Favorites tabs.
type ProjectSort string

// Supported project sort orders.
const (
	ProjectSortName       ProjectSort = "name"
	ProjectSortLastOpened ProjectSort = "lastOpened"
)

// ExecutablePaths stores executable names or absolute paths overriding tool discovery.
type ExecutablePaths struct {
	Sublime     string `json:"sublime"`
	Zed         string `json:"zed"`
	Ghostty     string `json:"ghostty"`
	FileManager string `json:"fileManager"`
}

// TerminalCommand stores the custom terminal executable and discrete arguments.
type TerminalCommand struct {
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

// WindowPreferences stores desktop window geometry and behavior preferences.
type WindowPreferences struct {
	Width        int  `json:"width"`
	Height       int  `json:"height"`
	RememberSize bool `json:"rememberSize"`
	AlwaysOnTop  bool `json:"alwaysOnTop"`
}

// Settings represents the versioned user settings stored in settings.json.
type Settings struct {
	SchemaVersion          int               `json:"schemaVersion"`
	DefaultEditor          string            `json:"defaultEditor"`
	PreferredTerminal      TerminalMode      `json:"preferredTerminal"`
	PreferredFileManager   FileManagerMode   `json:"preferredFileManager"`
	Theme                  ThemeMode         `json:"theme"`
	Window                 WindowPreferences `json:"window"`
	Executables            ExecutablePaths   `json:"executables"`
	CustomTerminal         TerminalCommand   `json:"customTerminal"`
	RememberLastEditor     bool              `json:"rememberLastEditor"`
	DoubleClickOpensEditor bool              `json:"doubleClickOpensEditor"`
	ShowGitDirty           bool              `json:"showGitDirty"`
	ProjectSort            ProjectSort       `json:"projectSort"`
}

// ProjectsFile represents the versioned project catalog stored in projects.json.
type ProjectsFile struct {
	SchemaVersion int               `json:"schemaVersion"`
	Projects      []project.Project `json:"projects"`
}

// DefaultSettings returns the initial default Settings configuration.
func DefaultSettings() Settings {
	return Settings{
		SchemaVersion:        CurrentSettingsSchemaVersion,
		DefaultEditor:        editor.DefaultID,
		PreferredTerminal:    TerminalModeGhostty,
		PreferredFileManager: FileManagerModeAuto,
		Theme:                ThemeSystem,
		Window: WindowPreferences{
			Width:        DefaultWindowWidth,
			Height:       DefaultWindowHeight,
			RememberSize: true,
			AlwaysOnTop:  false,
		},
		CustomTerminal:         TerminalCommand{Args: []string{}},
		RememberLastEditor:     true,
		DoubleClickOpensEditor: true,
		ShowGitDirty:           true,
		ProjectSort:            ProjectSortName,
	}
}

// ResolveInitialWindowSize returns the window dimensions to use on startup,
// honoring saved dimensions when RememberSize is enabled and values are valid.
func ResolveInitialWindowSize(s Settings) (int, int) {
	if !s.Window.RememberSize {
		return DefaultWindowWidth, DefaultWindowHeight
	}
	if s.Window.Width < minWindowDimension || s.Window.Width > maxWindowWidth ||
		s.Window.Height < minWindowDimension || s.Window.Height > maxWindowHeight {
		return DefaultWindowWidth, DefaultWindowHeight
	}
	return s.Window.Width, s.Window.Height
}

// DefaultProjectsFile returns an empty versioned ProjectsFile.
func DefaultProjectsFile() ProjectsFile {
	return ProjectsFile{
		SchemaVersion: CurrentProjectsSchemaVersion,
		Projects:      []project.Project{},
	}
}

// Validate checks Settings invariants and returns a descriptive error if invalid.
func (s Settings) Validate() error {
	if err := validateSchemaVersion("settings", s.SchemaVersion, CurrentSettingsSchemaVersion); err != nil {
		return err
	}
	if !editor.IsSupported(s.DefaultEditor) {
		return fmt.Errorf("%w: defaultEditor %q is not a supported editor", ErrInvalidConfig, s.DefaultEditor)
	}
	if err := s.validateTools(); err != nil {
		return err
	}
	switch s.ProjectSort {
	case ProjectSortName, ProjectSortLastOpened:
	default:
		return fmt.Errorf("%w: projectSort %q must be name or lastOpened", ErrInvalidConfig, s.ProjectSort)
	}
	switch s.Theme {
	case ThemeSystem, ThemeLight, ThemeDark:
	default:
		return fmt.Errorf("%w: theme %q must be one of system, light, dark", ErrInvalidConfig, s.Theme)
	}
	if s.Window.Width < minWindowDimension || s.Window.Width > maxWindowWidth {
		return fmt.Errorf(
			"%w: window.width %d must be between %d and %d",
			ErrInvalidConfig, s.Window.Width, minWindowDimension, maxWindowWidth,
		)
	}
	if s.Window.Height < minWindowDimension || s.Window.Height > maxWindowHeight {
		return fmt.Errorf(
			"%w: window.height %d must be between %d and %d",
			ErrInvalidConfig, s.Window.Height, minWindowDimension, maxWindowHeight,
		)
	}
	return nil
}

func (s Settings) validateTools() error {
	switch s.PreferredTerminal {
	case TerminalModeGhostty:
	case TerminalModeCustom:
		if strings.TrimSpace(s.CustomTerminal.Executable) == "" {
			return fmt.Errorf("%w: customTerminal.executable is required for a custom terminal", ErrInvalidConfig)
		}
	default:
		return fmt.Errorf("%w: preferredTerminal %q must be ghostty or custom", ErrInvalidConfig, s.PreferredTerminal)
	}
	switch s.PreferredFileManager {
	case FileManagerModeAuto:
	case FileManagerModeCustom:
		if strings.TrimSpace(s.Executables.FileManager) == "" {
			return fmt.Errorf("%w: executables.fileManager is required for a custom file manager", ErrInvalidConfig)
		}
	default:
		return fmt.Errorf("%w: preferredFileManager %q must be auto or custom", ErrInvalidConfig, s.PreferredFileManager)
	}
	paths := []struct{ field, value string }{
		{"executables.sublime", s.Executables.Sublime},
		{"executables.zed", s.Executables.Zed},
		{"executables.ghostty", s.Executables.Ghostty},
		{"executables.fileManager", s.Executables.FileManager},
		{"customTerminal.executable", s.CustomTerminal.Executable},
	}
	for _, path := range paths {
		if err := validateExecutable(path.field, path.value); err != nil {
			return err
		}
	}
	return validateTerminalArgs(s.CustomTerminal.Args)
}

func validateExecutable(field, value string) error {
	if len(value) > maxExecutableSize {
		return fmt.Errorf("%w: %s must be at most %d bytes", ErrInvalidConfig, field, maxExecutableSize)
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: %s contains invalid control characters", ErrInvalidConfig, field)
		}
	}
	if strings.ContainsAny(value, shellMetacharacters) {
		return fmt.Errorf("%w: %s must not contain shell characters (%s)", ErrInvalidConfig, field, shellMetacharacters)
	}
	if strings.Contains(value, "/") {
		if !filepath.IsAbs(value) {
			return fmt.Errorf("%w: %s must be an absolute path", ErrInvalidConfig, field)
		}
	} else if strings.ContainsAny(value, " \t") {
		return fmt.Errorf("%w: %s must be a single command name or an absolute path, without arguments", ErrInvalidConfig, field)
	}
	return nil
}

// shellMetacharacters are rejected in executable settings; they are never
// interpreted (no shell is used) but always indicate a pasted command line.
const shellMetacharacters = ";|&$`"

func validateTerminalArgs(args []string) error {
	if len(args) > maxTerminalArgs {
		return fmt.Errorf("%w: customTerminal.args must contain at most %d arguments", ErrInvalidConfig, maxTerminalArgs)
	}
	for i, arg := range args {
		if len(arg) > maxExecutableSize || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("%w: customTerminal.args[%d] must be at most %d bytes without NUL, CR or LF",
				ErrInvalidConfig, i, maxExecutableSize)
		}
	}
	return nil
}

// Validate checks ProjectsFile invariants, each contained project, and uniqueness of IDs and paths.
func (pf ProjectsFile) Validate() error {
	if err := validateSchemaVersion("projects", pf.SchemaVersion, CurrentProjectsSchemaVersion); err != nil {
		return err
	}
	seenIDs := make(map[string]struct{}, len(pf.Projects))
	seenPaths := make(map[string]struct{}, len(pf.Projects))
	for i, p := range pf.Projects {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("%w: projects[%d]: %w", ErrInvalidConfig, i, err)
		}
		if _, exists := seenIDs[p.ID]; exists {
			return fmt.Errorf("%w: duplicate project id %q", ErrInvalidConfig, p.ID)
		}
		seenIDs[p.ID] = struct{}{}

		key := project.PathKey(p.Path)
		if _, exists := seenPaths[key]; exists {
			return fmt.Errorf("%w: duplicate project path %q", ErrInvalidConfig, p.Path)
		}
		seenPaths[key] = struct{}{}
	}
	return nil
}

func validateSchemaVersion(document string, version, current int) error {
	if version < 1 || version > current {
		return fmt.Errorf("%w: %s schemaVersion %d is not supported (supported: 1..%d)",
			ErrUnsupportedSchemaVersion, document, version, current)
	}
	if version != current {
		return fmt.Errorf("%w: %s schemaVersion must be %d", ErrInvalidConfig, document, current)
	}
	return nil
}

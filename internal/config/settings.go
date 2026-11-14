// Package config manages XDG-compliant JSON configuration files for ProjectHub.
package config

import (
	"errors"
	"fmt"
	"strings"

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
	CurrentSettingsSchemaVersion = 1
	// CurrentProjectsSchemaVersion is the current schema version for projects.json.
	CurrentProjectsSchemaVersion = 1

	// DefaultWindowWidth is the initial compact desktop window width.
	DefaultWindowWidth = 440
	// DefaultWindowHeight is the initial compact desktop window height.
	DefaultWindowHeight = 680

	minWindowDimension = 320
	maxWindowWidth     = 7680
	maxWindowHeight    = 4320
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

// WindowPreferences stores desktop window geometry and behavior preferences.
type WindowPreferences struct {
	Width        int  `json:"width"`
	Height       int  `json:"height"`
	RememberSize bool `json:"rememberSize"`
	AlwaysOnTop  bool `json:"alwaysOnTop"`
}

// Settings represents the versioned user settings stored in settings.json.
type Settings struct {
	SchemaVersion        int               `json:"schemaVersion"`
	DefaultEditor        string            `json:"defaultEditor"`
	PreferredTerminal    string            `json:"preferredTerminal"`
	PreferredFileManager string            `json:"preferredFileManager"`
	Theme                ThemeMode         `json:"theme"`
	Window               WindowPreferences `json:"window"`
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
		DefaultEditor:        "vscode",
		PreferredTerminal:    "ghostty",
		PreferredFileManager: "auto",
		Theme:                ThemeSystem,
		Window: WindowPreferences{
			Width:        DefaultWindowWidth,
			Height:       DefaultWindowHeight,
			RememberSize: true,
			AlwaysOnTop:  false,
		},
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
	if s.SchemaVersion < 1 || s.SchemaVersion > CurrentSettingsSchemaVersion {
		return fmt.Errorf(
			"%w: settings schemaVersion %d is not supported (supported: 1..%d)",
			ErrUnsupportedSchemaVersion,
			s.SchemaVersion,
			CurrentSettingsSchemaVersion,
		)
	}
	if err := validateToolIdentifier("defaultEditor", s.DefaultEditor); err != nil {
		return err
	}
	if err := validateToolIdentifier("preferredTerminal", s.PreferredTerminal); err != nil {
		return err
	}
	if err := validateToolIdentifier("preferredFileManager", s.PreferredFileManager); err != nil {
		return err
	}
	switch s.Theme {
	case ThemeSystem, ThemeLight, ThemeDark:
	default:
		return fmt.Errorf("%w: theme %q must be one of system, light, dark", ErrInvalidConfig, s.Theme)
	}
	if s.Window.Width < minWindowDimension || s.Window.Width > maxWindowWidth {
		return fmt.Errorf(
			"%w: window.width %d must be between %d and %d",
			ErrInvalidConfig,
			s.Window.Width,
			minWindowDimension,
			maxWindowWidth,
		)
	}
	if s.Window.Height < minWindowDimension || s.Window.Height > maxWindowHeight {
		return fmt.Errorf(
			"%w: window.height %d must be between %d and %d",
			ErrInvalidConfig,
			s.Window.Height,
			minWindowDimension,
			maxWindowHeight,
		)
	}
	return nil
}

// Validate checks ProjectsFile invariants, each contained project, and uniqueness of IDs and paths.
func (pf ProjectsFile) Validate() error {
	if pf.SchemaVersion < 1 || pf.SchemaVersion > CurrentProjectsSchemaVersion {
		return fmt.Errorf(
			"%w: projects schemaVersion %d is not supported (supported: 1..%d)",
			ErrUnsupportedSchemaVersion,
			pf.SchemaVersion,
			CurrentProjectsSchemaVersion,
		)
	}
	seenIDs := make(map[string]struct{}, len(pf.Projects))
	seenPaths := make(map[string]struct{}, len(pf.Projects))
	for i, p := range pf.Projects {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("projects[%d]: %w", i, err)
		}
		if _, exists := seenIDs[p.ID]; exists {
			return fmt.Errorf("%w: duplicate project id %q", ErrInvalidConfig, p.ID)
		}
		seenIDs[p.ID] = struct{}{}

		if _, exists := seenPaths[p.Path]; exists {
			return fmt.Errorf("%w: duplicate project path %q", ErrInvalidConfig, p.Path)
		}
		seenPaths[p.Path] = struct{}{}
	}
	return nil
}

func validateToolIdentifier(field, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalidConfig, field)
	}
	if strings.ContainsAny(value, "\r\n\x00") {
		return fmt.Errorf("%w: %s contains invalid control characters", ErrInvalidConfig, field)
	}
	return nil
}

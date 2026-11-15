// Package desktop is the Wails-facing application service of ProjectHub. It
// coordinates the project catalog, settings, launcher, Git inspection, native
// dialogs, and import/export, exposing an ID-based API to the frontend so the
// webview never supplies filesystem paths for reading or writing files.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/git"
	"github.com/semelyanov86/ProjectHub/internal/launcher"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

var (
	// ErrLaunchInProgress indicates the project is already being opened.
	ErrLaunchInProgress = errors.New("project is already being opened")
	// ErrImportExpired indicates the prepared import is unknown or expired.
	ErrImportExpired = errors.New("import preview expired — choose the file again")
	// ErrConfirmationRequired indicates replacement was requested without confirmation.
	ErrConfirmationRequired = errors.New("replacing all projects requires explicit confirmation")
)

// SettingsSource loads the current user settings.
type SettingsSource interface {
	LoadSettings() (config.Settings, error)
}

// TransferStore reads import documents, writes exports, and replaces the
// catalog with a backup of the previous one.
type TransferStore interface {
	ReadProjectsDocument(path string) ([]project.Project, error)
	ExportProjects(dest string) (int, error)
	ReplaceProjectsWithBackup(projects []project.Project) (string, error)
}

// Deps are the collaborators of Service.
type Deps struct {
	Catalog   *project.Catalog
	Settings  SettingsSource
	Transfer  TransferStore
	Launcher  *launcher.Launcher
	Inspector *git.Inspector
	Dialogs   Dialogs
	// ValidateDir validates directories chosen in dialogs or dropped onto the
	// window; launcher.ValidateProjectDir in production.
	ValidateDir project.DirValidator
}

// Service is the Wails-bound desktop API. Every exported method is callable
// from the frontend.
type Service struct {
	deps Deps
	now  func() time.Time

	mu        sync.Mutex
	launching map[string]struct{}
	pending   *pendingImport
}

// NewService creates the desktop Service.
func NewService(deps Deps) *Service {
	return &Service{
		deps:      deps,
		now:       time.Now,
		launching: make(map[string]struct{}),
	}
}

// DirectoryDraft describes a directory proposed for registration.
type DirectoryDraft struct {
	// Cancelled reports that the user dismissed the folder picker.
	Cancelled     bool   `json:"cancelled"`
	Path          string `json:"path"`
	SuggestedName string `json:"suggestedName"`
	// DuplicateID and DuplicateName identify an already registered project
	// with the same directory; both are empty when the directory is new.
	DuplicateID   string         `json:"duplicateId"`
	DuplicateName string         `json:"duplicateName"`
	Git           git.Inspection `json:"git"`
}

// ListProjects returns all registered projects.
func (s *Service) ListProjects() ([]project.Project, error) {
	return s.deps.Catalog.List()
}

// AddProject registers an existing directory as a project.
func (s *Service) AddProject(input project.Input) (project.Project, error) {
	return s.deps.Catalog.Add(input)
}

// UpdateProject edits a project's name, description, directory, default
// editor, and favorite flag.
func (s *Service) UpdateProject(id string, input project.Input) (project.Project, error) {
	return s.deps.Catalog.Update(id, input)
}

// RemoveProject unregisters a project without touching its directory.
func (s *Service) RemoveProject(id string) error {
	return s.deps.Catalog.Remove(id)
}

// SetFavorite marks or unmarks a project as favorite.
func (s *Service) SetFavorite(id string, favorite bool) (project.Project, error) {
	return s.deps.Catalog.SetFavorite(id, favorite)
}

// ChooseDirectory opens the native folder picker and describes the selection.
func (s *Service) ChooseDirectory(ctx context.Context) (DirectoryDraft, error) {
	path, err := s.deps.Dialogs.ChooseDirectory(ctx, "Choose Project Directory")
	if err != nil {
		return DirectoryDraft{}, fmt.Errorf("folder picker: %w", err)
	}
	if path == "" {
		return DirectoryDraft{Cancelled: true}, nil
	}
	return s.describeDirectory(ctx, path)
}

// ChooseExecutable opens the native file picker for selecting an application
// executable in Settings. It returns "" when cancelled.
func (s *Service) ChooseExecutable(ctx context.Context) (string, error) {
	path, err := s.deps.Dialogs.ChooseExecutable(ctx)
	if err != nil {
		return "", fmt.Errorf("file picker: %w", err)
	}
	return path, nil
}

// ListTools reports which applications are available for opening projects.
func (s *Service) ListTools() ([]launcher.ToolStatus, error) {
	settings, err := s.deps.Settings.LoadSettings()
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	return s.deps.Launcher.Tools(preferences(settings)), nil
}

// TerminalPresets returns ready-made custom terminal configurations.
func (s *Service) TerminalPresets() []launcher.TerminalPreset {
	return launcher.TerminalPresets()
}

// describeDirectory validates a directory and gathers what the Add Project
// dialog needs: suggested name, duplicate registration, and Git state.
func (s *Service) describeDirectory(ctx context.Context, raw string) (DirectoryDraft, error) {
	path, err := s.deps.ValidateDir(raw)
	if err != nil {
		return DirectoryDraft{}, err
	}
	draft := DirectoryDraft{Path: path, SuggestedName: project.SuggestName(path)}
	existing, found, err := s.deps.Catalog.FindByPath(path)
	if err != nil {
		return DirectoryDraft{}, err
	}
	if found {
		draft.DuplicateID = existing.ID
		draft.DuplicateName = existing.Name
	}
	draft.Git = s.deps.Inspector.Inspect(ctx, path)
	return draft, nil
}

// preferences maps persisted settings to launcher preferences.
func preferences(s config.Settings) launcher.Preferences {
	fileManager := launcher.FileManagerAuto
	if s.PreferredFileManager == config.FileManagerModeCustom {
		fileManager = launcher.FileManagerCustom
	}
	return launcher.Preferences{
		Paths: launcher.ToolPaths{
			Sublime:     s.Executables.Sublime,
			Zed:         s.Executables.Zed,
			Ghostty:     s.Executables.Ghostty,
			FileManager: s.Executables.FileManager,
		},
		CustomTerminal: launcher.TerminalCommand{
			Executable: s.CustomTerminal.Executable,
			Args:       append([]string(nil), s.CustomTerminal.Args...),
		},
		FileManagerMode: fileManager,
	}
}

func baseName(path string) string {
	return filepath.Base(filepath.Clean(path))
}

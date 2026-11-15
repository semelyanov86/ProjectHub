package desktop

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/editor"
	"github.com/semelyanov86/ProjectHub/internal/launcher"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

// OpenResult is returned after a project was launched.
type OpenResult struct {
	// Project is the project after its lastOpened timestamp was recorded.
	Project project.Project `json:"project"`
	// Warning is non-empty when the launch succeeded but a follow-up step
	// (recording the timestamp, loading settings) failed.
	Warning string `json:"warning"`
}

// OpenProject opens project id with the given target application and records
// the launch. Errors (missing directory, application not found, launch
// failure) are returned for display as notifications.
func (s *Service) OpenProject(ctx context.Context, id string, target launcher.Target) (OpenResult, error) {
	p, err := s.deps.Catalog.Get(id)
	if err != nil {
		return OpenResult{}, err
	}
	settings, warning := s.launchSettings()
	return s.open(ctx, p, target, settings, warning)
}

// OpenProjectDefault opens project id with its effective editor: the
// project's explicit default editor, else its last used editor (when
// "remember last editor" is enabled), else the global default editor.
func (s *Service) OpenProjectDefault(ctx context.Context, id string) (OpenResult, error) {
	p, err := s.deps.Catalog.Get(id)
	if err != nil {
		return OpenResult{}, err
	}
	settings, warning := s.launchSettings()
	target := launcher.Target(EffectiveEditor(p, settings))
	return s.open(ctx, p, target, settings, warning)
}

// EffectiveEditor resolves the editor used when a project is opened without
// choosing an application explicitly.
func EffectiveEditor(p project.Project, s config.Settings) string {
	if editor.IsSupported(p.DefaultEditor) {
		return p.DefaultEditor
	}
	if s.RememberLastEditor && editor.IsSupported(p.LastUsedEditor) {
		return p.LastUsedEditor
	}
	if editor.IsSupported(s.DefaultEditor) {
		return s.DefaultEditor
	}
	return editor.DefaultID
}

// launchSettings loads settings for a launch, falling back to defaults with a
// warning so an unreadable settings file never prevents opening a project.
func (s *Service) launchSettings() (config.Settings, string) {
	settings, err := s.deps.Settings.LoadSettings()
	if err != nil {
		return config.DefaultSettings(), fmt.Sprintf("Settings could not be loaded, defaults were used: %v", err)
	}
	return settings, ""
}

func (s *Service) open(
	ctx context.Context,
	p project.Project,
	target launcher.Target,
	settings config.Settings,
	warning string,
) (OpenResult, error) {
	if !s.beginLaunch(p.ID) {
		return OpenResult{}, fmt.Errorf("%w: %s", ErrLaunchInProgress, p.Name)
	}
	defer s.endLaunch(p.ID)

	if err := s.deps.Launcher.Open(ctx, target, p.Path, preferences(settings)); err != nil {
		return OpenResult{}, launchError(p, target, err)
	}

	editorID := ""
	if settings.RememberLastEditor && editor.IsSupported(string(target)) {
		editorID = string(target)
	}
	updated, err := s.deps.Catalog.RecordOpened(p.ID, p.Path, editorID)
	if err != nil {
		return OpenResult{
			Project: p,
			Warning: joinWarnings(warning, fmt.Sprintf("Opened %s, but the launch could not be recorded: %v", p.Name, err)),
		}, nil
	}
	return OpenResult{Project: updated, Warning: warning}, nil
}

func (s *Service) beginLaunch(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, busy := s.launching[id]; busy {
		return false
	}
	s.launching[id] = struct{}{}
	return true
}

func (s *Service) endLaunch(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.launching, id)
}

func joinWarnings(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	default:
		return a + " " + b
	}
}

// LaunchError is a failed launch with a short, actionable message for the
// notification shown to the user (whose title already names the project).
// The launcher error stays in the chain for errors.Is and errors.As.
type LaunchError struct {
	Message string
	Err     error
}

func (e *LaunchError) Error() string { return e.Message }

func (e *LaunchError) Unwrap() error { return e.Err }

// launchError turns a launcher failure into a LaunchError that says what went
// wrong and what to do about it.
func launchError(p project.Project, target launcher.Target, err error) error {
	tool := targetName(target)
	var message string
	switch {
	case errors.Is(err, launcher.ErrInvalidTargetPath) && errors.Is(err, fs.ErrNotExist):
		message = fmt.Sprintf("The directory %s no longer exists. Edit the project to update its location, or remove it.", p.Path)
	case errors.Is(err, launcher.ErrInvalidTargetPath) && errors.Is(err, fs.ErrPermission):
		message = fmt.Sprintf("You do not have permission to open %s.", p.Path)
	case errors.Is(err, launcher.ErrInvalidTargetPath):
		message = fmt.Sprintf("%s cannot be opened: %s", p.Path, launchDetail(err, launcher.ErrInvalidTargetPath, tool))
	case errors.Is(err, launcher.ErrToolNotFound):
		message = fmt.Sprintf("%s was not found (%s). Install it or set its executable in Settings.",
			tool, launchDetail(err, launcher.ErrToolNotFound, tool))
	case errors.Is(err, launcher.ErrInvalidExecutable):
		message = fmt.Sprintf("The %s executable configured in Settings is invalid: %s",
			tool, launchDetail(err, launcher.ErrInvalidExecutable, tool))
	case errors.Is(err, launcher.ErrLaunchFailed):
		message = fmt.Sprintf("%s could not be started: %s", tool, launchDetail(err, launcher.ErrLaunchFailed, tool))
	default:
		message = err.Error()
	}
	return &LaunchError{Message: message, Err: err}
}

// launchDetail strips the sentinel text and the tool-name prefix the launcher
// adds ("Zed: ", "File Manager: "), leaving the specific reason, for example a
// missing path or the application's stderr output.
func launchDetail(err, sentinel error, tool string) string {
	detail := strings.Replace(err.Error(), sentinel.Error()+": ", "", 1)
	if prefix := tool + ": "; len(detail) >= len(prefix) && strings.EqualFold(detail[:len(prefix)], prefix) {
		detail = detail[len(prefix):]
	}
	return detail
}

func targetName(target launcher.Target) string {
	switch target {
	case launcher.TargetSublime, launcher.TargetZed:
		if ed, ok := editor.FindByID(string(target)); ok {
			return ed.Name
		}
		return string(target)
	case launcher.TargetGhostty:
		return "Ghostty"
	case launcher.TargetTerminal:
		return "Custom terminal"
	case launcher.TargetFileManager:
		return "File manager"
	default:
		return string(target)
	}
}

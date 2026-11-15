package desktop

import (
	"context"
	"fmt"

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
		return OpenResult{}, fmt.Errorf("open %s: %w", p.Name, err)
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

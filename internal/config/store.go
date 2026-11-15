package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

// Store manages atomic persistence of ProjectHub JSON configuration files.
type Store struct {
	mu  sync.RWMutex
	dir string
	now func() time.Time

	hookMu           sync.Mutex
	onProjectsChange func()
}

// DefaultDir resolves the ProjectHub configuration directory following the XDG Base Directory Specification.
func DefaultDir() (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(baseDir, AppConfigDirName), nil
}

// NewStore creates a Store rooted at dir. If dir is empty, DefaultDir is used.
func NewStore(dir string) (*Store, error) {
	if dir == "" {
		resolved, err := DefaultDir()
		if err != nil {
			return nil, err
		}
		dir = resolved
	}
	cleanDir := filepath.Clean(dir)
	if !filepath.IsAbs(cleanDir) {
		return nil, fmt.Errorf("%w: config directory %q must be absolute", ErrInvalidConfig, dir)
	}
	return &Store{dir: cleanDir, now: time.Now}, nil
}

// OnProjectsChanged registers fn to be called after every successful write of
// projects.json (catalog mutations and replacement imports). fn runs after the
// store lock is released and must not block; it replaces any earlier hook.
func (s *Store) OnProjectsChanged(fn func()) {
	s.hookMu.Lock()
	defer s.hookMu.Unlock()
	s.onProjectsChange = fn
}

func (s *Store) notifyProjectsChanged() {
	s.hookMu.Lock()
	fn := s.onProjectsChange
	s.hookMu.Unlock()
	if fn != nil {
		fn()
	}
}

// Dir returns the resolved configuration directory path.
func (s *Store) Dir() string {
	return s.dir
}

// EnsureFiles creates settings.json and projects.json with defaults when they
// do not exist yet, so a fresh installation has its configuration on disk after
// the first launch. Existing files, including unreadable or corrupted ones, are
// never modified.
func (s *Store) EnsureFiles() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := createIfMissing(s.SettingsPath(), DefaultSettings()); err != nil {
		return err
	}
	return createIfMissing(s.ProjectsPath(), DefaultProjectsFile())
}

func createIfMissing(path string, payload any) error {
	_, err := os.Lstat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check %s: %w", path, err)
	}
	return writeJSONAtomic(path, payload)
}

// SettingsPath returns the full path to settings.json.
func (s *Store) SettingsPath() string {
	return filepath.Join(s.dir, SettingsFileName)
}

// ProjectsPath returns the full path to projects.json.
func (s *Store) ProjectsPath() string {
	return filepath.Join(s.dir, ProjectsFileName)
}

// LoadSettings reads settings.json from disk, returning DefaultSettings if the file does not exist yet.
// Corrupted JSON or invalid configuration files are never overwritten and return a descriptive error.
func (s *Store) LoadSettings() (Settings, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadSettingsUnlocked()
}

func (s *Store) loadSettingsUnlocked() (Settings, error) {
	settings := DefaultSettings()
	settings.SchemaVersion = 0
	path := s.SettingsPath()
	found, err := readJSONFile(path, &settings)
	if err != nil {
		return Settings{}, err
	}
	if !found {
		return DefaultSettings(), nil
	}
	if settings.SchemaVersion == 0 {
		settings.SchemaVersion = 1
	}
	if settings.SchemaVersion == 1 {
		migrateSettingsV1(&settings)
	}
	if settings.CustomTerminal.Args == nil {
		settings.CustomTerminal.Args = []string{}
	}
	if settings.CloseBehavior == "" {
		settings.CloseBehavior = CloseBehaviorTray
	}
	if err := settings.Validate(); err != nil {
		return Settings{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return settings, nil
}

// SaveSettings validates and atomically writes settings.json.
// If an existing settings.json on disk is unreadable, corrupted, or uses an unsupported
// schema version, SaveSettings refuses to overwrite it so user data is never lost.
func (s *Store) SaveSettings(settings Settings) error {
	if settings.CustomTerminal.Args == nil {
		settings.CustomTerminal.Args = []string{}
	}
	if settings.SchemaVersion == 0 {
		settings.SchemaVersion = CurrentSettingsSchemaVersion
	}
	if settings.CloseBehavior == "" {
		settings.CloseBehavior = CloseBehaviorTray
	}
	if err := settings.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.loadSettingsUnlocked(); err != nil {
		return fmt.Errorf("refusing to overwrite unreadable existing settings: %w", err)
	}
	return writeJSONAtomic(s.SettingsPath(), settings)
}

// SaveWindowSize persists updated window dimensions when RememberSize is enabled.
// Invalid sizes (such as minimized 0x0 bounds) are ignored, and unreadable existing
// settings files are never overwritten.
func (s *Store) SaveWindowSize(width, height int) error {
	if width < minWindowDimension || width > maxWindowWidth ||
		height < minWindowDimension || height > maxWindowHeight {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.loadSettingsUnlocked()
	if err != nil {
		return fmt.Errorf("refusing to persist window size over unreadable settings: %w", err)
	}
	if !current.Window.RememberSize {
		return nil
	}
	if current.Window.Width == width && current.Window.Height == height {
		return nil
	}
	current.Window.Width = width
	current.Window.Height = height
	return writeJSONAtomic(s.SettingsPath(), current)
}

// LoadProjectsFile reads projects.json from disk, returning DefaultProjectsFile if it does not exist yet.
func (s *Store) LoadProjectsFile() (ProjectsFile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.loadProjectsFileUnlocked()
}

func (s *Store) loadProjectsFileUnlocked() (ProjectsFile, error) {
	pf := DefaultProjectsFile()
	pf.SchemaVersion = 0
	path := s.ProjectsPath()
	found, err := readJSONFile(path, &pf)
	if err != nil {
		return ProjectsFile{}, err
	}
	if !found {
		return DefaultProjectsFile(), nil
	}
	if err := normalizeLoadedProjects(&pf); err != nil {
		return ProjectsFile{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return pf, nil
}

// LoadProjects satisfies project.Loader by returning the validated list of projects.
func (s *Store) LoadProjects() ([]project.Project, error) {
	pf, err := s.LoadProjectsFile()
	if err != nil {
		return nil, err
	}
	return cloneProjects(pf.Projects), nil
}

// SaveProjectsFile validates and atomically writes projects.json.
// If an existing projects.json on disk is unreadable, corrupted, or uses an unsupported
// schema version, SaveProjectsFile refuses to overwrite it.
func (s *Store) SaveProjectsFile(pf ProjectsFile) error {
	if pf.SchemaVersion == 0 {
		pf.SchemaVersion = CurrentProjectsSchemaVersion
	}
	if pf.Projects == nil {
		pf.Projects = []project.Project{}
	}
	if err := pf.Validate(); err != nil {
		return err
	}
	if err := s.saveProjectsFile(pf); err != nil {
		return err
	}
	s.notifyProjectsChanged()
	return nil
}

func (s *Store) saveProjectsFile(pf ProjectsFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.loadProjectsFileUnlocked(); err != nil {
		return fmt.Errorf("refusing to overwrite unreadable existing projects: %w", err)
	}
	return writeJSONAtomic(s.ProjectsPath(), pf)
}

// UpdateProjects applies mutate to a deep copy of the catalog under the store lock.
// Mutator errors are returned unchanged, and invalid or unreadable catalogs are never overwritten.
// The returned committed projects are a deep copy and are never nil.
func (s *Store) UpdateProjects(mutate func([]project.Project) ([]project.Project, error)) ([]project.Project, error) {
	committed, err := s.updateProjects(mutate)
	if err != nil {
		return nil, err
	}
	s.notifyProjectsChanged()
	return committed, nil
}

func (s *Store) updateProjects(mutate func([]project.Project) ([]project.Project, error)) ([]project.Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, err := s.loadProjectsFileUnlocked()
	if err != nil {
		return nil, err
	}
	updated, err := mutate(cloneProjects(current.Projects))
	if err != nil {
		return nil, err
	}
	pf := ProjectsFile{SchemaVersion: CurrentProjectsSchemaVersion, Projects: cloneProjects(updated)}
	if err := pf.Validate(); err != nil {
		return nil, err
	}
	if err := writeJSONAtomic(s.ProjectsPath(), pf); err != nil {
		return nil, err
	}
	return cloneProjects(pf.Projects), nil
}

func cloneProjects(projects []project.Project) []project.Project {
	out := make([]project.Project, len(projects))
	copy(out, projects)
	for i := range out {
		if out[i].LastOpened != nil {
			opened := *out[i].LastOpened
			out[i].LastOpened = &opened
		}
	}
	return out
}

func readJSONFile(path string, target any) (bool, error) {
	cleanPath := filepath.Clean(path)
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("read %s: %w", cleanPath, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return true, fmt.Errorf("%w: %s: %w", ErrCorruptedConfig, cleanPath, err)
	}
	return true, nil
}

func writeJSONAtomic(path string, payload any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", dir, err)
	}

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode JSON for %s: %w", path, err)
	}
	encoded = append(encoded, '\n')

	tmpFile, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("set permissions on temp file %s: %w", tmpPath, err)
	}
	if _, err := tmpFile.Write(encoded); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("write temp file %s: %w", tmpPath, err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("sync temp file %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}
	cleanup = false
	return syncDir(dir)
}

// syncDir flushes a directory entry change (a rename or a newly created file)
// to disk, so a crash right after a save cannot resurrect the old file.
func syncDir(dir string) error {
	d, err := os.Open(filepath.Clean(dir))
	if err != nil {
		return fmt.Errorf("open config directory %s: %w", dir, err)
	}
	if err := d.Sync(); err != nil {
		_ = d.Close()
		return fmt.Errorf("sync config directory %s: %w", dir, err)
	}
	if err := d.Close(); err != nil {
		return fmt.Errorf("close config directory %s: %w", dir, err)
	}
	return nil
}

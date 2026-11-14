package config

// Service exposes validated configuration operations to the frontend via Wails bindings.
type Service struct {
	store             *Store
	onSettingsUpdated func(Settings)
}

// NewService creates a config Service backed by the provided Store.
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// NewServiceWithHook creates a config Service that invokes onUpdated after each successful save.
func NewServiceWithHook(store *Store, onUpdated func(Settings)) *Service {
	return &Service{
		store:             store,
		onSettingsUpdated: onUpdated,
	}
}

// GetSettings returns the current user settings, falling back to defaults if not yet saved.
func (s *Service) GetSettings() (Settings, error) {
	return s.store.LoadSettings()
}

// UpdateSettings validates and atomically persists user settings.
func (s *Service) UpdateSettings(settings Settings) (Settings, error) {
	if err := s.store.SaveSettings(settings); err != nil {
		return Settings{}, err
	}
	saved, err := s.store.LoadSettings()
	if err != nil {
		return Settings{}, err
	}
	if s.onSettingsUpdated != nil {
		s.onSettingsUpdated(saved)
	}
	return saved, nil
}

// GetConfigDir returns the XDG configuration directory used by ProjectHub.
func (s *Service) GetConfigDir() string {
	return s.store.Dir()
}

package project

import (
	"errors"
	"fmt"
)

// Loader loads persisted projects from configuration storage.
type Loader interface {
	LoadProjects() ([]Project, error)
}

// Service provides project catalog operations exposed to the frontend via Wails bindings.
type Service struct {
	loader Loader
}

// NewService constructs a project Service backed by the given Loader.
func NewService(loader Loader) *Service {
	return &Service{loader: loader}
}

// ListProjects returns all registered projects from configuration storage.
// When no projects file exists yet, it returns an empty non-nil slice.
func (s *Service) ListProjects() ([]Project, error) {
	if s.loader == nil {
		return nil, errors.New("project service loader is not configured")
	}
	projects, err := s.loader.LoadProjects()
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	if projects == nil {
		return []Project{}, nil
	}
	return projects, nil
}

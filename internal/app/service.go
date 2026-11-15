// Package app provides top-level application metadata and lifecycle helpers
// exposed to the ProjectHub frontend.
package app

import (
	"os"
	"runtime"
)

// Version is the semantic version of ProjectHub. It can be overridden via ldflags at build time.
var Version = "0.1.0"

// Info summarizes runtime metadata about the running ProjectHub instance.
type Info struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	ConfigDir string `json:"configDir"`
	// HomeDir is the user's home directory, used to abbreviate paths as "~".
	HomeDir string `json:"homeDir"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// Service exposes application metadata to the frontend via Wails bindings.
type Service struct {
	configDir string
}

// NewService creates an app Service with the resolved configuration directory.
func NewService(configDir string) *Service {
	return &Service{configDir: configDir}
}

// GetInfo returns runtime application metadata.
func (s *Service) GetInfo() Info {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return Info{
		Name:      "ProjectHub",
		Version:   Version,
		ConfigDir: s.configDir,
		HomeDir:   home,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

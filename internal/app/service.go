// Package app provides top-level application metadata and lifecycle helpers
// exposed to the ProjectHub frontend.
package app

import (
	"context"
	"os"
	"runtime"
)

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

// Options configure the app Service.
type Options struct {
	// Version is the application version (the repository VERSION file).
	Version string
	// ConfigDir is the resolved configuration directory.
	ConfigDir string
	// TrayAvailable reports whether a system tray host is currently
	// available. A nil func means the tray is never available.
	TrayAvailable func(context.Context) bool
}

// Service exposes application metadata to the frontend via Wails bindings.
type Service struct {
	opts Options
}

// NewService creates an app Service.
func NewService(opts Options) *Service {
	return &Service{opts: opts}
}

// GetInfo returns runtime application metadata.
func (s *Service) GetInfo() Info {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return Info{
		Name:      "ProjectHub",
		Version:   s.opts.Version,
		ConfigDir: s.opts.ConfigDir,
		HomeDir:   home,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}

// TrayAvailable reports whether the desktop session currently shows tray
// icons. Without a tray, closing the window always quits ProjectHub.
func (s *Service) TrayAvailable(ctx context.Context) bool {
	if s.opts.TrayAvailable == nil {
		return false
	}
	return s.opts.TrayAvailable(ctx)
}

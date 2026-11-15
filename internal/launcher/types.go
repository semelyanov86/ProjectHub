package launcher

// Target identifies an application a project can be opened with.
type Target string

// Supported launch targets shown in the "Open with" menu.
const (
	TargetSublime     Target = "sublime"
	TargetZed         Target = "zed"
	TargetGhostty     Target = "ghostty"
	TargetTerminal    Target = "terminal" // user-configured custom terminal
	TargetFileManager Target = "file_manager"
)

// FileManagerMode selects how project directories are revealed.
type FileManagerMode string

// Supported file manager modes.
const (
	// FileManagerAuto uses the desktop default handler via gio open, then xdg-open.
	FileManagerAuto FileManagerMode = "auto"
	// FileManagerCustom runs ToolPaths.FileManager with the directory as its only argument.
	FileManagerCustom FileManagerMode = "custom"
)

// ToolPaths holds user-configured executable overrides; "" means auto-detect.
// A value containing "/" must be an absolute path; a bare name is looked up.
type ToolPaths struct {
	Sublime     string
	Zed         string
	Ghostty     string
	FileManager string
}

// TerminalCommand is a custom terminal invocation. Every literal "{path}" inside
// an argument is replaced by the project directory; arguments are never split
// or interpreted by a shell.
type TerminalCommand struct {
	Executable string
	Args       []string
}

// Preferences is the launch-relevant subset of user settings.
type Preferences struct {
	Paths           ToolPaths
	CustomTerminal  TerminalCommand
	FileManagerMode FileManagerMode
}

// ToolStatus reports whether a launch target can currently be used.
type ToolStatus struct {
	Target Target `json:"target"`
	// Name is the display label, e.g. "Sublime Text" or "Terminal (kitty)".
	Name      string `json:"name"`
	Available bool   `json:"available"`
	// Executable is the resolved executable path when Available.
	Executable string `json:"executable"`
	// Reason explains why the tool is unavailable; empty when Available.
	Reason string `json:"reason"`
}

// TerminalPreset is a ready-made custom terminal configuration.
type TerminalPreset struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Executable string   `json:"executable"`
	Args       []string `json:"args"`
}

package process

import "strings"

// strippedVariables are runtime variables of ProjectHub's own process that
// must not leak into launched desktop applications.
var strippedVariables = map[string]struct{}{
	// Wails/GTK may force a backend that is wrong for GTK4 children on Wayland.
	"GDK_BACKEND": {},
	// Activation tokens belong to ProjectHub's own launch and are single-use.
	"XDG_ACTIVATION_TOKEN": {},
	"DESKTOP_STARTUP_ID":   {},
	// Describe how ProjectHub itself was launched, not the child.
	"GIO_LAUNCHED_DESKTOP_FILE":     {},
	"GIO_LAUNCHED_DESKTOP_FILE_PID": {},
	// Wails development server wiring.
	"FRONTEND_DEVSERVER_URL": {},
	// AppImage runtime metadata.
	"APPIMAGE": {},
	"APPDIR":   {},
	"ARGV0":    {},
	"OWD":      {},
}

// strippedPrefixes are variable-name prefixes owned by the embedded webview or Wails.
var strippedPrefixes = []string{"WEBKIT_", "WAILS_"}

// SanitizeEnv returns a copy of env without ProjectHub/WebKit/Wails runtime
// variables. When running from an AppImage (APPDIR set), variables whose value
// references the AppImage mount (for example LD_LIBRARY_PATH or GTK_PATH) are
// removed as well, so children use the host's libraries. Display, D-Bus
// session, locale, and user variables are preserved.
func SanitizeEnv(env []string) []string {
	appDir := lookup(env, "APPDIR")
	out := make([]string, 0, len(env))
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if !ok || name == "" {
			continue
		}
		if shouldStrip(name, value, appDir) {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func shouldStrip(name, value, appDir string) bool {
	if _, ok := strippedVariables[name]; ok {
		return true
	}
	for _, prefix := range strippedPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return appDir != "" && strings.Contains(value, appDir)
}

func lookup(env []string, key string) string {
	for _, entry := range env {
		if name, value, ok := strings.Cut(entry, "="); ok && name == key {
			return value
		}
	}
	return ""
}

// WithVariables returns a copy of env with each "KEY=value" in vars replacing
// any existing entry for the same KEY.
func WithVariables(env []string, vars ...string) []string {
	override := make(map[string]struct{}, len(vars))
	for _, v := range vars {
		if name, _, ok := strings.Cut(v, "="); ok {
			override[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(env)+len(vars))
	for _, entry := range env {
		name, _, _ := strings.Cut(entry, "=")
		if _, replaced := override[name]; replaced {
			continue
		}
		out = append(out, entry)
	}
	return append(out, vars...)
}

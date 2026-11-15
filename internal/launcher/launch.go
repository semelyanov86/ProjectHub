package launcher

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/editor"
	"github.com/semelyanov86/ProjectHub/internal/process"
)

// PathPlaceholder is replaced by the project directory inside custom terminal arguments.
const PathPlaceholder = "{path}"

// Launcher discovers applications and opens validated project directories.
type Launcher struct {
	runner             process.Runner
	resolver           *Resolver
	ghosttyTimeout     time.Duration
	fileManagerTimeout time.Duration
}

// New creates a launcher with bounded IPC attempts and detached GUI launches.
// Nil dependencies use the production runner and resolver.
func New(runner process.Runner, resolver *Resolver) *Launcher {
	if runner == nil {
		runner = process.NewOSRunner()
	}
	if resolver == nil {
		resolver = NewResolver()
	}
	return &Launcher{
		runner: runner, resolver: resolver,
		ghosttyTimeout: 4 * time.Second, fileManagerTimeout: 5 * time.Second,
	}
}

// Tools reports application availability in menu order.
func (l *Launcher) Tools(prefs Preferences) []ToolStatus {
	sublime, _ := editor.FindByID(editor.IDSublime)
	zed, _ := editor.FindByID(editor.IDZed)
	return []ToolStatus{
		l.toolStatus(TargetSublime, sublime.Name, prefs.Paths.Sublime, sublime.Candidates),
		l.toolStatus(TargetZed, zed.Name, prefs.Paths.Zed, zed.Candidates),
		l.toolStatus(TargetGhostty, "Ghostty", prefs.Paths.Ghostty, []string{"ghostty"}),
		l.terminalStatus(prefs.CustomTerminal),
		l.fileManagerStatus(prefs),
	}
}

func (l *Launcher) toolStatus(target Target, name, override string, candidates []string) ToolStatus {
	status := ToolStatus{Target: target, Name: name}
	exe, err := l.resolver.Resolve(override, candidates)
	if err != nil {
		status.Reason = unavailableReason(override, err)
		return status
	}
	status.Available = true
	status.Executable = exe
	return status
}

// unavailableReason turns a resolver error into a short sentence for the UI.
func unavailableReason(override string, err error) string {
	override = strings.TrimSpace(override)
	switch {
	case override == "":
		return "Not found in PATH or common install locations. Set its path in Settings."
	case errors.Is(err, ErrInvalidExecutable):
		return fmt.Sprintf("Configured path %q is invalid: use an absolute path without shell characters.", override)
	default:
		return fmt.Sprintf("Configured path %q was not found or is not executable.", override)
	}
}

func (l *Launcher) terminalStatus(cmd TerminalCommand) ToolStatus {
	if strings.TrimSpace(cmd.Executable) == "" {
		return ToolStatus{
			Target: TargetTerminal, Name: "Custom terminal",
			Reason: "No custom terminal configured — set one in Settings.",
		}
	}
	name := "Terminal (" + filepath.Base(strings.TrimSpace(cmd.Executable)) + ")"
	return l.toolStatus(TargetTerminal, name, cmd.Executable, nil)
}

func (l *Launcher) fileManagerStatus(prefs Preferences) ToolStatus {
	if prefs.FileManagerMode == FileManagerCustom {
		if strings.TrimSpace(prefs.Paths.FileManager) == "" {
			return ToolStatus{
				Target: TargetFileManager, Name: "File Manager",
				Reason: "No file manager configured — set one in Settings.",
			}
		}
		return l.toolStatus(TargetFileManager, "File Manager", prefs.Paths.FileManager, nil)
	}
	status := l.toolStatus(TargetFileManager, "File Manager", "", []string{"gio"})
	if status.Available {
		return status
	}
	status = l.toolStatus(TargetFileManager, "File Manager", "", []string{"xdg-open"})
	if !status.Available {
		status.Reason = "Neither gio nor xdg-open was found. Set its path in Settings."
	}
	return status
}

// Open opens dir with the selected application after validating the directory.
func (l *Launcher) Open(ctx context.Context, target Target, dir string, prefs Preferences) error {
	dir, err := ValidateProjectDir(dir)
	if err != nil {
		return err
	}
	switch target {
	case TargetSublime, TargetZed:
		return l.openEditor(target, dir, prefs)
	case TargetGhostty:
		return l.openGhostty(ctx, dir, prefs.Paths.Ghostty)
	case TargetTerminal:
		return l.openTerminal(dir, prefs.CustomTerminal)
	case TargetFileManager:
		return l.openFileManager(ctx, dir, prefs)
	default:
		return fmt.Errorf("%w: %q", ErrUnknownTarget, target)
	}
}

func (l *Launcher) openEditor(target Target, dir string, prefs Preferences) error {
	ed, _ := editor.FindByID(string(target))
	override := prefs.Paths.Sublime
	if target == TargetZed {
		override = prefs.Paths.Zed
	}
	return l.startResolved(ed.Name, override, ed.Candidates, []string{dir}, dir)
}

func (l *Launcher) startResolved(name, override string, candidates, args []string, dir string) error {
	exe, err := l.resolve(name, override, candidates)
	if err != nil {
		return err
	}
	return l.start(name, process.Command{Path: exe, Args: args, Dir: dir})
}

func (l *Launcher) resolve(name, override string, candidates []string) (string, error) {
	exe, err := l.resolver.Resolve(override, candidates)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return exe, nil
}

func (l *Launcher) start(name string, cmd process.Command) error {
	if err := l.runner.Start(cmd); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrLaunchFailed, name, err)
	}
	return nil
}

func (l *Launcher) openGhostty(ctx context.Context, dir, override string) error {
	exe, err := l.resolve("Ghostty", override, []string{"ghostty"})
	if err != nil {
		return err
	}
	child, cancel := context.WithTimeout(ctx, l.ghosttyTimeout)
	defer cancel()
	result, primaryErr := l.runner.Run(child, process.Command{
		Path: exe, Args: []string{"+new-window", "--working-directory=" + dir}, Dir: dir,
	})
	if primaryErr == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Errorf("%w: Ghostty: %w", ErrLaunchFailed, ctxErr)
	}
	fallbackErr := l.runner.Start(process.Command{
		Path: exe, Args: []string{"--gtk-single-instance=false", "--working-directory=" + dir}, Dir: dir,
	})
	if fallbackErr != nil {
		return fmt.Errorf("%w: Ghostty primary: %w%s; fallback: %w",
			ErrLaunchFailed, primaryErr, stderrSummary(result.Stderr), fallbackErr)
	}
	return nil
}

func (l *Launcher) openTerminal(dir string, cmd TerminalCommand) error {
	if strings.TrimSpace(cmd.Executable) == "" {
		return fmt.Errorf("%w: no custom terminal configured", ErrToolNotFound)
	}
	exe, err := l.resolve("Custom terminal", cmd.Executable, nil)
	if err != nil {
		return err
	}
	for _, arg := range cmd.Args {
		if strings.ContainsRune(arg, '\x00') {
			return fmt.Errorf("%w: terminal argument contains NUL byte", ErrInvalidExecutable)
		}
	}
	return l.start("Custom terminal", process.Command{Path: exe, Args: ExpandArgs(cmd.Args, dir), Dir: dir})
}

func (l *Launcher) openFileManager(ctx context.Context, dir string, prefs Preferences) error {
	if prefs.FileManagerMode == FileManagerCustom {
		if strings.TrimSpace(prefs.Paths.FileManager) == "" {
			return fmt.Errorf("%w: no file manager configured", ErrToolNotFound)
		}
		return l.startResolved("File Manager", prefs.Paths.FileManager, nil, []string{dir}, dir)
	}
	return l.openDefaultFileManager(ctx, dir)
}

func (l *Launcher) openDefaultFileManager(ctx context.Context, dir string) error {
	gio, gioErr := l.resolver.Resolve("", []string{"gio"})
	if gioErr == nil {
		child, cancel := context.WithTimeout(ctx, l.fileManagerTimeout)
		defer cancel()
		result, err := l.runner.Run(child, process.Command{Path: gio, Args: []string{"open", dir}, Dir: dir})
		if err == nil {
			return nil
		}
		gioErr = fmt.Errorf("%w%s", err, stderrSummary(result.Stderr))
	}
	xdg, xdgErr := l.resolver.Resolve("", []string{"xdg-open"})
	if xdgErr == nil {
		xdgErr = l.runner.Start(process.Command{Path: xdg, Args: []string{dir}, Dir: dir})
		if xdgErr == nil {
			return nil
		}
	}
	kind := ErrLaunchFailed
	if gio == "" && xdg == "" {
		kind = ErrToolNotFound
	}
	return fmt.Errorf("%w: File Manager: %w", kind, errors.Join(
		fmt.Errorf("gio: %w", gioErr), fmt.Errorf("xdg-open: %w", xdgErr)))
}

func stderrSummary(stderr []byte) string {
	line, _, _ := strings.Cut(string(stderr), "\n")
	runes := []rune(strings.TrimSpace(line))
	if len(runes) == 0 {
		return ""
	}
	return ": " + string(runes[:min(len(runes), 200)])
}

// ExpandArgs replaces every path placeholder without splitting arguments.
func ExpandArgs(args []string, dir string) []string {
	expanded := make([]string, len(args))
	for i, arg := range args {
		expanded[i] = strings.ReplaceAll(arg, PathPlaceholder, dir)
	}
	return expanded
}

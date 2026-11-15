package desktop

import (
	"context"
	"errors"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// errDialogsUnavailable indicates native dialogs were requested before the
// Wails application started.
var errDialogsUnavailable = errors.New("native dialogs are not available")

// Wails discovers typed events only from static RegisterEvent calls, so the
// drop event is registered at package initialisation.
func init() {
	application.RegisterEvent[DropResult](EventProjectsDropped)
}

// Dialogs opens native file and folder pickers. Each method returns "" when
// the user cancels.
type Dialogs interface {
	ChooseDirectory(ctx context.Context, title string) (string, error)
	ChooseImportFile(ctx context.Context) (string, error)
	ChooseExportFile(ctx context.Context, defaultName string) (string, error)
	ChooseExecutable(ctx context.Context) (string, error)
}

// WailsDialogs implements Dialogs with Wails' native GTK file dialogs,
// attached to the window that issued the call.
type WailsDialogs struct{}

var jsonFilter = application.FileFilter{DisplayName: "JSON files (*.json)", Pattern: "*.json"}

// ChooseDirectory implements Dialogs.
func (WailsDialogs) ChooseDirectory(ctx context.Context, title string) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errDialogsUnavailable
	}
	dialog := app.Dialog.OpenFile().
		SetTitle(title).
		CanChooseDirectories(true).
		CanChooseFiles(false)
	if home, err := os.UserHomeDir(); err == nil {
		dialog.SetDirectory(home)
	}
	if window := callerWindow(ctx, app); window != nil {
		dialog.AttachToWindow(window)
	}
	return dialog.PromptForSingleSelection()
}

// ChooseImportFile implements Dialogs.
func (WailsDialogs) ChooseImportFile(ctx context.Context) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errDialogsUnavailable
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Import Projects").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		AddFilter(jsonFilter.DisplayName, jsonFilter.Pattern)
	if window := callerWindow(ctx, app); window != nil {
		dialog.AttachToWindow(window)
	}
	return dialog.PromptForSingleSelection()
}

// ChooseExportFile implements Dialogs.
func (WailsDialogs) ChooseExportFile(ctx context.Context, defaultName string) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errDialogsUnavailable
	}
	options := &application.SaveFileDialogOptions{
		Title:                "Export Projects",
		Filename:             defaultName,
		CanCreateDirectories: true,
		Filters:              []application.FileFilter{jsonFilter},
		Window:               callerWindow(ctx, app),
	}
	if home, err := os.UserHomeDir(); err == nil {
		options.Directory = home
	}
	return app.Dialog.SaveFileWithOptions(options).PromptForSingleSelection()
}

// ChooseExecutable implements Dialogs.
func (WailsDialogs) ChooseExecutable(ctx context.Context) (string, error) {
	app := application.Get()
	if app == nil {
		return "", errDialogsUnavailable
	}
	dialog := app.Dialog.OpenFile().
		SetTitle("Choose Application Executable").
		CanChooseFiles(true).
		CanChooseDirectories(false)
	if window := callerWindow(ctx, app); window != nil {
		dialog.AttachToWindow(window)
	}
	return dialog.PromptForSingleSelection()
}

// callerWindow returns the window that issued the bound call, falling back to
// the currently focused window.
func callerWindow(ctx context.Context, app *application.App) application.Window {
	if window, ok := ctx.Value(application.WindowKey).(application.Window); ok && window != nil {
		return window
	}
	return app.Window.Current()
}

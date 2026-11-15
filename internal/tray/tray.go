package tray

import (
	"context"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

// Actions are the callbacks behind the tray menu entries. Wails runs menu
// callbacks in their own goroutine; they must not assume the GTK main thread.
type Actions struct {
	ShowWindow   func()
	OpenSettings func()
	OpenProject  func(id string)
	Quit         func()
}

// Tray owns the StatusNotifierItem and rebuilds its menu whenever Refresh is
// called (after every catalog change). When the session has no tray host the
// item is simply not shown; the application keeps working without it.
type Tray struct {
	tray    *application.SystemTray
	menu    *application.Menu
	load    func() ([]project.Project, error)
	actions Actions
	refresh chan struct{}
}

// New creates the tray icon. load returns the current project catalog; icon
// is a PNG image. Call Start once the application context exists.
func New(app *application.App, icon []byte, load func() ([]project.Project, error), actions Actions) *Tray {
	t := &Tray{
		menu:    application.NewMenu(),
		load:    load,
		actions: actions,
		refresh: make(chan struct{}, 1),
	}
	t.apply(Items(nil, nil))
	t.tray = app.SystemTray.New()
	t.tray.SetLabel("ProjectHub")
	t.tray.SetTooltip("ProjectHub")
	t.tray.SetIcon(icon)
	// A primary click (Activate) opens the window; the menu is on the
	// secondary button (or on primary, depending on the tray host).
	t.tray.OnClick(actions.ShowWindow)
	t.tray.SetMenu(t.menu)
	return t
}

// Start rebuilds the menu from the catalog now and after every Refresh call
// until ctx is cancelled.
func (t *Tray) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.refresh:
				t.rebuild()
			}
		}
	}()
	t.Refresh()
}

// Refresh schedules a menu rebuild. It never blocks; bursts of catalog
// changes coalesce into one rebuild.
func (t *Tray) Refresh() {
	select {
	case t.refresh <- struct{}{}:
	default:
	}
}

func (t *Tray) rebuild() {
	projects, err := t.load()
	items := Items(RecentProjects(projects, MaxRecent), err)
	application.InvokeSync(func() {
		t.menu.Clear()
		t.apply(items)
		t.tray.SetMenu(t.menu)
	})
}

func (t *Tray) apply(items []Item) {
	for _, item := range items {
		if item.Separator {
			t.menu.AddSeparator()
			continue
		}
		entry := t.menu.Add(item.Label).SetEnabled(item.Enabled)
		if item.Tooltip != "" {
			entry.SetTooltip(item.Tooltip)
		}
		if callback := t.callback(item); callback != nil {
			entry.OnClick(func(*application.Context) { callback() })
		}
	}
}

func (t *Tray) callback(item Item) func() {
	switch item.Action {
	case ActionShowWindow:
		return t.actions.ShowWindow
	case ActionOpenSettings:
		return t.actions.OpenSettings
	case ActionQuit:
		return t.actions.Quit
	case ActionOpenProject:
		id := item.ProjectID
		return func() { t.actions.OpenProject(id) }
	case ActionNone:
		return nil
	default:
		return nil
	}
}

// HideOnClose reports whether closing the main window should hide it to the
// tray instead of quitting. Hiding requires both the user's preference and a
// tray host that can bring the window back; otherwise the window quits.
func HideOnClose(behavior config.CloseBehavior, hostAvailable bool) bool {
	return behavior == config.CloseBehaviorTray && hostAvailable
}

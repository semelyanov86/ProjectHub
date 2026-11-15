// Package main boots the ProjectHub Wails 3 desktop application and registers
// backend services exposed to the Vue frontend.
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/semelyanov86/ProjectHub/internal/app"
	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/desktop"
	"github.com/semelyanov86/ProjectHub/internal/git"
	"github.com/semelyanov86/ProjectHub/internal/launcher"
	"github.com/semelyanov86/ProjectHub/internal/process"
	"github.com/semelyanov86/ProjectHub/internal/project"
	"github.com/semelyanov86/ProjectHub/internal/tray"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

//go:embed build/icons/tray.png
var trayIcon []byte

// versionFile is the single source of the application version.
//
//go:embed VERSION
var versionFile string

const (
	applicationID       = "io.github.semelyanov86.projecthub"
	gitConcurrency      = 4
	gitInspectTimeout   = 3 * time.Second
	mainWindowName      = "main"
	mainWindowMinWidth  = 380
	mainWindowMinHeight = 480
)

func main() {
	version := strings.TrimSpace(versionFile)
	if handled := handleCommandLine(os.Args[1:], version); handled {
		return
	}
	run(version)
}

// handleCommandLine answers --version and --help without starting the GUI,
// so both work without a display (for example in package checks).
func handleCommandLine(args []string, version string) bool {
	for _, arg := range args {
		switch arg {
		case "--version", "-v":
			fmt.Printf("ProjectHub %s\n", version)
			return true
		case "--help", "-h":
			fmt.Printf(`ProjectHub %s — manage and open development projects

Usage: projecthub [--version] [--help]

Starting ProjectHub while it is already running focuses the existing window.
Settings and the project catalog are stored in $XDG_CONFIG_HOME/projecthub
(default ~/.config/projecthub).
`, version)
			return true
		}
	}
	return false
}

func run(version string) {
	configStore, err := config.NewStore("")
	if err != nil {
		log.Fatalf("failed to initialize configuration store: %v", err)
	}

	initialSettings := loadInitialSettings(configStore)
	catalog := project.NewCatalog(configStore, launcher.ValidateProjectDir)
	desktopService := newDesktopService(configStore, catalog)

	var mainWindow *application.WebviewWindow
	// quitting is set once ProjectHub is shutting down, so the close hook no
	// longer turns window closes into "hide to tray".
	var quitting atomic.Bool

	showMainWindow := func() {
		if mainWindow == nil {
			return
		}
		if mainWindow.IsMinimised() {
			mainWindow.UnMinimise()
		}
		mainWindow.Show()
		mainWindow.Focus()
	}

	configService := config.NewServiceWithHook(configStore, func(saved config.Settings) {
		if mainWindow != nil {
			mainWindow.SetAlwaysOnTop(saved.Window.AlwaysOnTop)
		}
	})

	wailsApp := application.New(application.Options{
		Name:        "ProjectHub",
		Description: "Manage and open development projects",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(app.NewService(app.Options{
				Version:       version,
				ConfigDir:     configStore.Dir(),
				TrayAvailable: tray.HostAvailable,
			})),
			application.NewService(configService),
			application.NewService(desktopService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ProgramName:   "projecthub",
			ApplicationID: applicationID,
		},
		// A second launch focuses the running window (also when it is hidden
		// in the tray) instead of starting another process that could race on
		// projects.json; the running instance keeps its state.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: applicationID,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				showMainWindow()
			},
		},
	})
	wailsApp.OnShutdown(func() { quitting.Store(true) })

	windowWidth, windowHeight := config.ResolveInitialWindowSize(initialSettings)

	mainWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:           mainWindowName,
		Title:          "ProjectHub",
		Width:          windowWidth,
		Height:         windowHeight,
		MinWidth:       mainWindowMinWidth,
		MinHeight:      mainWindowMinHeight,
		AlwaysOnTop:    initialSettings.Window.AlwaysOnTop,
		EnableFileDrop: true,
		BackgroundColour: application.NewRGB(
			15,
			17,
			23,
		),
		Linux: application.LinuxWindow{
			Icon:             appIcon,
			WebviewGpuPolicy: application.WebviewGpuPolicyOnDemand,
		},
		URL: "/",
	})

	quit := func() {
		quitting.Store(true)
		wailsApp.Quit()
	}
	systemTray := tray.New(wailsApp, trayIcon, catalog.List, tray.Actions{
		ShowWindow: showMainWindow,
		OpenSettings: func() {
			showMainWindow()
			wailsApp.Event.Emit(desktop.EventOpenSettings)
		},
		OpenProject: func(id string) {
			openFromTray(wailsApp, desktopService, catalog, showMainWindow, id)
		},
		Quit: quit,
	})
	configStore.OnProjectsChanged(systemTray.Refresh)
	systemTray.Start(wailsApp.Context())

	// Native drops are validated in Go; the frontend only receives the result.
	drops := desktop.NewDropHandler(desktopService, func(result desktop.DropResult) {
		wailsApp.Event.Emit(desktop.EventProjectsDropped, result)
	})
	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		go drops.Handle(wailsApp.Context(), files)
	})

	installCloseHook(wailsApp, mainWindow, configStore, &quitting)

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// loadInitialSettings creates the configuration on first launch and loads the
// settings needed before the window exists. Existing files are never touched:
// unreadable settings are reported and the window starts with defaults.
func loadInitialSettings(store *config.Store) config.Settings {
	if err := store.EnsureFiles(); err != nil {
		log.Printf("warning: failed to create default configuration: %v", err)
	}
	settings, err := store.LoadSettings()
	if err != nil {
		log.Printf("warning: failed to load settings (%v); using default window geometry", err)
		return config.DefaultSettings()
	}
	return settings
}

func newDesktopService(store *config.Store, catalog *project.Catalog) *desktop.Service {
	runner := process.NewOSRunner()
	return desktop.NewService(desktop.Deps{
		Catalog:     catalog,
		Settings:    store,
		Transfer:    store,
		Launcher:    launcher.New(runner, launcher.NewResolver()),
		Inspector:   git.NewInspector(runner, nil, gitConcurrency, gitInspectTimeout),
		Dialogs:     desktop.WailsDialogs{},
		ValidateDir: launcher.ValidateProjectDir,
	})
}

// installCloseHook saves the window size and decides what closing the window
// does. RegisterHook runs before Wails's internal WindowClosing listener
// destroys the window, so Size() still returns the live dimensions and the
// close can be cancelled to hide the window to the tray. While ProjectHub is
// quitting (tray Quit, session logout) every close goes through.
func installCloseHook(
	wailsApp *application.App,
	window *application.WebviewWindow,
	store *config.Store,
	quitting *atomic.Bool,
) {
	window.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if window.IsVisible() {
			width, height := window.Size()
			if err := store.SaveWindowSize(width, height); err != nil {
				log.Printf("warning: failed to save window size: %v", err)
			}
		}
		if quitting.Load() || wailsApp.Context().Err() != nil {
			return
		}
		behavior := config.DefaultSettings().CloseBehavior
		if settings, err := store.LoadSettings(); err == nil {
			behavior = settings.CloseBehavior
		}
		// Only "tray" needs the D-Bus probe (at most 500 ms); "quit" closes at once.
		if behavior == config.CloseBehaviorTray && tray.HideOnClose(behavior, tray.HostAvailable(wailsApp.Context())) {
			event.Cancel()
			window.Hide()
		}
	})
}

// openFromTray opens a recent project in its effective editor. Failures bring
// the window forward with an error notification, because the tray menu has
// no other way to report them.
func openFromTray(
	wailsApp *application.App,
	service *desktop.Service,
	catalog *project.Catalog,
	showWindow func(),
	id string,
) {
	ctx, cancel := context.WithTimeout(wailsApp.Context(), 30*time.Second)
	defer cancel()
	result, err := service.OpenProjectDefault(ctx, id)
	if err != nil {
		title := "Could not open project"
		if p, getErr := catalog.Get(id); getErr == nil {
			title = "Could not open " + p.Name
		}
		showWindow()
		wailsApp.Event.Emit(desktop.EventNotice, desktop.Notice{
			Level:  desktop.NoticeError,
			Title:  title,
			Detail: err.Error(),
		})
		return
	}
	if result.Warning != "" {
		// A hidden window would let the warning toast expire unseen.
		showWindow()
		wailsApp.Event.Emit(desktop.EventNotice, desktop.Notice{
			Level:  desktop.NoticeWarning,
			Title:  "Opened " + result.Project.Name,
			Detail: result.Warning,
		})
	}
	wailsApp.Event.Emit(desktop.EventProjectsChanged)
}

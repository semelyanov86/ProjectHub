// Package main boots the ProjectHub Wails 3 desktop application and registers
// backend services exposed to the Vue frontend.
package main

import (
	"embed"
	"log"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/app"
	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/desktop"
	"github.com/semelyanov86/ProjectHub/internal/git"
	"github.com/semelyanov86/ProjectHub/internal/launcher"
	"github.com/semelyanov86/ProjectHub/internal/process"
	"github.com/semelyanov86/ProjectHub/internal/project"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

const (
	applicationID       = "io.github.semelyanov86.projecthub"
	gitConcurrency      = 4
	gitInspectTimeout   = 3 * time.Second
	mainWindowName      = "main"
	mainWindowMinWidth  = 380
	mainWindowMinHeight = 480
)

func main() {
	configStore, err := config.NewStore("")
	if err != nil {
		log.Fatalf("failed to initialize configuration store: %v", err)
	}

	initialSettings, err := configStore.LoadSettings()
	if err != nil {
		// Keep corrupted or invalid user config untouched on disk and boot with safe window defaults.
		log.Printf("warning: failed to load settings (%v); using default window geometry", err)
		initialSettings = config.DefaultSettings()
	}

	runner := process.NewOSRunner()
	desktopService := desktop.NewService(desktop.Deps{
		Catalog:     project.NewCatalog(configStore, launcher.ValidateProjectDir),
		Settings:    configStore,
		Transfer:    configStore,
		Launcher:    launcher.New(runner, launcher.NewResolver()),
		Inspector:   git.NewInspector(runner, nil, gitConcurrency, gitInspectTimeout),
		Dialogs:     desktop.WailsDialogs{},
		ValidateDir: launcher.ValidateProjectDir,
	})

	var mainWindow *application.WebviewWindow

	configService := config.NewServiceWithHook(configStore, func(saved config.Settings) {
		if mainWindow != nil {
			mainWindow.SetAlwaysOnTop(saved.Window.AlwaysOnTop)
		}
	})

	wailsApp := application.New(application.Options{
		Name:        "ProjectHub",
		Description: "Lightweight desktop project manager for Linux developers",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(app.NewService(configStore.Dir())),
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
		// A second launch focuses the running window instead of starting another
		// process that could race on projects.json.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: applicationID,
			OnSecondInstanceLaunch: func(_ application.SecondInstanceData) {
				if mainWindow == nil {
					return
				}
				if mainWindow.IsMinimised() {
					mainWindow.UnMinimise()
				}
				mainWindow.Show()
				mainWindow.Focus()
			},
		},
	})

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

	// Native drops are validated in Go; the frontend only receives the result.
	drops := desktop.NewDropHandler(desktopService, func(result desktop.DropResult) {
		wailsApp.Event.Emit(desktop.EventProjectsDropped, result)
	})
	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		files := event.Context().DroppedFiles()
		go drops.Handle(wailsApp.Context(), files)
	})

	// RegisterHook runs before Wails's internal WindowClosing listener destroys the window,
	// so mainWindow.Size() still returns the live window dimensions.
	mainWindow.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		width, height := mainWindow.Size()
		if err := configStore.SaveWindowSize(width, height); err != nil {
			log.Printf("warning: failed to save window size: %v", err)
		}
	})

	if err := wailsApp.Run(); err != nil {
		log.Fatal(err)
	}
}

// Package main boots the ProjectHub Wails 3 desktop application and registers
// backend services exposed to the Vue frontend.
package main

import (
	"embed"
	"log"

	"github.com/semelyanov86/ProjectHub/internal/app"
	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/project"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

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

	var mainWindow *application.WebviewWindow

	appService := app.NewService(configStore.Dir())
	configService := config.NewServiceWithHook(configStore, func(saved config.Settings) {
		if mainWindow != nil {
			mainWindow.SetAlwaysOnTop(saved.Window.AlwaysOnTop)
		}
	})
	projectService := project.NewService(configStore)

	wailsApp := application.New(application.Options{
		Name:        "ProjectHub",
		Description: "Lightweight desktop project manager for Linux developers",
		Icon:        appIcon,
		Services: []application.Service{
			application.NewService(appService),
			application.NewService(configService),
			application.NewService(projectService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Linux: application.LinuxOptions{
			ProgramName:   "projecthub",
			ApplicationID: "io.github.semelyanov86.projecthub",
		},
	})

	windowWidth, windowHeight := config.ResolveInitialWindowSize(initialSettings)

	mainWindow = wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:        "main",
		Title:       "ProjectHub",
		Width:       windowWidth,
		Height:      windowHeight,
		MinWidth:    380,
		MinHeight:   480,
		AlwaysOnTop: initialSettings.Window.AlwaysOnTop,
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

package desktop

import "github.com/wailsapp/wails/v3/pkg/application"

// Events emitted by the Go side outside a frontend call, for example from the
// system tray or a second application launch.
const (
	// EventOpenSettings asks the frontend to show the Preferences dialog.
	EventOpenSettings = "app:open-settings"
	// EventNotice delivers a notification (Notice) for the frontend to show.
	EventNotice = "app:notice"
	// EventProjectsChanged tells the frontend that the catalog changed
	// outside its own calls, so it reloads the project list.
	EventProjectsChanged = "projects:changed"
)

// NoticeLevel is the severity of a Notice.
type NoticeLevel string

// Notice severities, matching the frontend toast variants.
const (
	NoticeSuccess NoticeLevel = "success"
	NoticeWarning NoticeLevel = "warning"
	NoticeError   NoticeLevel = "error"
)

// Notice is a user-facing notification produced by a background action.
type Notice struct {
	Level  NoticeLevel `json:"level"`
	Title  string      `json:"title"`
	Detail string      `json:"detail"`
}

// Wails discovers typed events only from static RegisterEvent calls.
func init() {
	application.RegisterEvent[application.Void](EventOpenSettings)
	application.RegisterEvent[Notice](EventNotice)
	application.RegisterEvent[application.Void](EventProjectsChanged)
}

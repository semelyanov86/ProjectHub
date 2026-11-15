// Package tray implements ProjectHub's system tray integration: a
// StatusNotifierItem icon whose menu opens the main window, recent projects,
// Settings, and quits the application. The menu model is built by pure
// functions so it can be tested without a desktop session.
package tray

import (
	"cmp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/semelyanov86/ProjectHub/internal/project"
)

// MaxRecent is the number of recently opened projects listed in the menu.
const MaxRecent = 8

// maxLabelRunes bounds menu labels so long project names cannot make the
// menu wider than the screen.
const maxLabelRunes = 48

// Action identifies what a menu item does when activated.
type Action int

// Menu actions.
const (
	ActionNone Action = iota
	ActionShowWindow
	ActionOpenProject
	ActionOpenSettings
	ActionQuit
)

// Item is one entry of the tray menu. Separator items carry no label.
type Item struct {
	Label     string
	Tooltip   string
	Enabled   bool
	Separator bool
	Action    Action
	// ProjectID is set for ActionOpenProject items.
	ProjectID string
}

// RecentProjects returns up to limit projects that have been opened, most
// recently opened first; ties are ordered by name. The input is not modified.
func RecentProjects(projects []project.Project, limit int) []project.Project {
	recent := make([]project.Project, 0, len(projects))
	for _, p := range projects {
		if p.LastOpened != nil {
			recent = append(recent, p)
		}
	}
	slices.SortStableFunc(recent, func(a, b project.Project) int {
		if c := b.LastOpened.Compare(*a.LastOpened); c != 0 {
			return c
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	if limit >= 0 && len(recent) > limit {
		recent = recent[:limit]
	}
	return recent
}

// Items builds the tray menu from the recent projects. loadErr is non-nil
// when the catalog could not be read; the menu then explains that instead of
// claiming there are no recent projects.
func Items(recent []project.Project, loadErr error) []Item {
	items := []Item{
		{Label: "Open ProjectHub", Enabled: true, Action: ActionShowWindow},
		{Separator: true},
		{Label: "Recent Projects", Enabled: false},
	}
	switch {
	case loadErr != nil:
		items = append(items, Item{Label: "Projects could not be loaded", Tooltip: loadErr.Error()})
	case len(recent) == 0:
		items = append(items, Item{Label: "No recently opened projects"})
	default:
		for _, p := range recent {
			items = append(items, Item{
				Label:     MenuLabel(p.Name),
				Tooltip:   p.Path,
				Enabled:   true,
				Action:    ActionOpenProject,
				ProjectID: p.ID,
			})
		}
	}
	return append(items,
		Item{Separator: true},
		Item{Label: "Settings…", Enabled: true, Action: ActionOpenSettings},
		Item{Separator: true},
		Item{Label: "Quit ProjectHub", Enabled: true, Action: ActionQuit},
	)
}

// MenuLabel turns a project name into a single-line dbusmenu label: control
// characters and runs of whitespace (including Unicode line separators)
// become single spaces, long names are shortened with an ellipsis, and
// underscores are doubled because dbusmenu treats a single underscore as a
// mnemonic marker (my_project would otherwise render as "myproject").
func MenuLabel(name string) string {
	clean := strings.Join(strings.FieldsFunc(name, func(r rune) bool {
		return unicode.IsControl(r) || unicode.IsSpace(r)
	}), " ")
	if clean == "" {
		clean = "Untitled project"
	}
	if utf8.RuneCountInString(clean) > maxLabelRunes {
		runes := []rune(clean)
		clean = strings.TrimSpace(string(runes[:maxLabelRunes-1])) + "…"
	}
	return strings.ReplaceAll(clean, "_", "__")
}

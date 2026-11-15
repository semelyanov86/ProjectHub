package tray

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

func opened(id, name string, at time.Time) project.Project {
	return project.Project{ID: id, Name: name, Path: "/work/" + id, LastOpened: &at}
}

func TestRecentProjectsOrdersByLastOpenedAndLimits(t *testing.T) {
	base := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	projects := []project.Project{
		{ID: "never", Name: "Never opened", Path: "/work/never"},
		opened("old", "Old", base),
		opened("new", "New", base.Add(2*time.Hour)),
		opened("b", "beta", base.Add(time.Hour)),
		opened("a", "Alpha", base.Add(time.Hour)),
	}
	got := RecentProjects(projects, 3)
	want := []string{"new", "a", "b"}
	if len(got) != len(want) {
		t.Fatalf("RecentProjects() returned %d projects, want %d", len(got), len(want))
	}
	for i, id := range want {
		if got[i].ID != id {
			t.Errorf("RecentProjects()[%d] = %q, want %q", i, got[i].ID, id)
		}
	}
	if projects[1].ID != "old" {
		t.Error("RecentProjects() modified its input")
	}
	if all := RecentProjects(projects, -1); len(all) != 4 {
		t.Errorf("RecentProjects(limit -1) = %d projects, want all 4 opened ones", len(all))
	}
}

func TestItemsListsRecentProjects(t *testing.T) {
	recent := []project.Project{opened("p1", "my_project", time.Now())}
	items := Items(recent, nil)

	var found *Item
	actions := map[Action]bool{}
	for i := range items {
		actions[items[i].Action] = true
		if items[i].Action == ActionOpenProject {
			found = &items[i]
		}
	}
	if found == nil {
		t.Fatal("Items() has no project entry")
	}
	if found.ProjectID != "p1" || found.Label != "my__project" || found.Tooltip != "/work/p1" || !found.Enabled {
		t.Errorf("project item = %+v, want enabled p1 labelled my__project with its path as tooltip", *found)
	}
	for _, action := range []Action{ActionShowWindow, ActionOpenSettings, ActionQuit} {
		if !actions[action] {
			t.Errorf("Items() is missing action %d", action)
		}
	}
	if items[len(items)-1].Action != ActionQuit {
		t.Error("Quit must be the last menu item")
	}
}

func TestItemsExplainEmptyAndFailedCatalogs(t *testing.T) {
	cases := map[string]struct {
		err  error
		want string
	}{
		"empty":  {nil, "No recently opened projects"},
		"failed": {errors.New("corrupted"), "Projects could not be loaded"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			items := Items(nil, tc.err)
			for _, item := range items {
				if item.Action == ActionOpenProject {
					t.Fatalf("Items() contains a project entry: %+v", item)
				}
				if item.Label == tc.want {
					if item.Enabled {
						t.Errorf("%q must be disabled", tc.want)
					}
					return
				}
			}
			t.Errorf("Items() lacks %q", tc.want)
		})
	}
}

func TestMenuLabel(t *testing.T) {
	long := strings.Repeat("x", 80)
	cases := map[string]string{
		"plain":                   "plain",
		"snake_case_name":         "snake__case__name",
		"two\nlines\there":        "two lines here",
		"next\u0085line\u2028sep": "next line sep",
		"non\u00a0breaking":       "non breaking",
		"   ":                     "Untitled project",
	}
	for in, want := range cases {
		if got := MenuLabel(in); got != want {
			t.Errorf("MenuLabel(%q) = %q, want %q", in, got, want)
		}
	}
	got := MenuLabel(long)
	if runes := []rune(got); len(runes) != maxLabelRunes || !strings.HasSuffix(got, "…") {
		t.Errorf("MenuLabel(long) = %q (%d runes), want %d runes ending in an ellipsis", got, len(runes), maxLabelRunes)
	}
}

func TestHideOnClose(t *testing.T) {
	cases := []struct {
		behavior config.CloseBehavior
		host     bool
		want     bool
	}{
		{config.CloseBehaviorTray, true, true},
		{config.CloseBehaviorTray, false, false},
		{config.CloseBehaviorQuit, true, false},
		{config.CloseBehaviorQuit, false, false},
	}
	for _, tc := range cases {
		if got := HideOnClose(tc.behavior, tc.host); got != tc.want {
			t.Errorf("HideOnClose(%q, host=%v) = %v, want %v", tc.behavior, tc.host, got, tc.want)
		}
	}
}

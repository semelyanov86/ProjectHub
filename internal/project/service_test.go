package project

import (
	"errors"
	"testing"
)

type stubLoader struct {
	projects []Project
	err      error
}

func (s stubLoader) LoadProjects() ([]Project, error) {
	return s.projects, s.err
}

func TestServiceListProjects(t *testing.T) {
	t.Run("returns empty slice instead of nil", func(t *testing.T) {
		svc := NewService(stubLoader{})
		got, err := svc.ListProjects()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || len(got) != 0 {
			t.Fatalf("expected non-nil empty slice, got %#v", got)
		}
	})

	t.Run("returns loaded projects", func(t *testing.T) {
		want := validProject()
		svc := NewService(stubLoader{projects: []Project{want}})
		got, err := svc.ListProjects()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].ID != want.ID {
			t.Fatalf("unexpected projects: %#v", got)
		}
	})

	t.Run("propagates loader error", func(t *testing.T) {
		boom := errors.New("disk read failed")
		svc := NewService(stubLoader{err: boom})
		_, err := svc.ListProjects()
		if !errors.Is(err, boom) {
			t.Fatalf("expected wrapped boom error, got %v", err)
		}
	})
}

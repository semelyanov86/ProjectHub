package project

import (
	"errors"
	"testing"
	"time"
)

func validProject() Project {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	return Project{
		ID:            "proj-1",
		Name:          "ProjectHub",
		Description:   "Desktop project manager",
		Path:          "/home/dev/code/ProjectHub",
		Favorite:      true,
		DefaultEditor: "vscode",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func TestProjectValidate(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		mutate  func(*Project)
		wantErr bool
	}{
		{
			name:    "valid project",
			mutate:  func(_ *Project) {},
			wantErr: false,
		},
		{
			name:    "valid project with spaces in path",
			mutate:  func(p *Project) { p.Path = "/home/dev/My Projects/App (v2)" },
			wantErr: false,
		},
		{
			name:    "empty id",
			mutate:  func(p *Project) { p.ID = "   " },
			wantErr: true,
		},
		{
			name:    "id with slash",
			mutate:  func(p *Project) { p.ID = "a/b" },
			wantErr: true,
		},
		{
			name:    "empty name",
			mutate:  func(p *Project) { p.Name = "" },
			wantErr: true,
		},
		{
			name:    "relative path",
			mutate:  func(p *Project) { p.Path = "relative/path" },
			wantErr: true,
		},
		{
			name:    "nul byte in path",
			mutate:  func(p *Project) { p.Path = "/tmp/bad\x00path" },
			wantErr: true,
		},
		{
			name:    "zero createdAt",
			mutate:  func(p *Project) { p.CreatedAt = time.Time{}; p.UpdatedAt = now },
			wantErr: true,
		},
		{
			name:    "zero updatedAt",
			mutate:  func(p *Project) { p.UpdatedAt = time.Time{} },
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validProject()
			tc.mutate(&p)
			err := p.Validate()
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidProject) {
					t.Fatalf("expected ErrInvalidProject, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

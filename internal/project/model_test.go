package project

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	testEditorZed         = "zed"
	testEditorSublime     = "sublime"
	testUnsupportedEditor = "vscode"
	testCleanPath         = "/a/b"
)

func validProject() Project {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	return Project{
		ID:            "proj-1",
		Name:          "ProjectHub",
		Description:   "Desktop project manager",
		Path:          "/home/dev/code/ProjectHub",
		Favorite:      true,
		DefaultEditor: testEditorZed,
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

func TestProjectValidateFieldConstraints(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Project)
		wantErr bool
	}{
		{name: "inherited editor", mutate: func(p *Project) { p.DefaultEditor = "" }},
		{name: "sublime default", mutate: func(p *Project) { p.DefaultEditor = testEditorSublime }},
		{name: "zed default", mutate: func(p *Project) { p.DefaultEditor = testEditorZed }},
		{name: "unsupported default", mutate: func(p *Project) { p.DefaultEditor = testUnsupportedEditor }, wantErr: true},
		{name: "empty last used editor", mutate: func(p *Project) { p.LastUsedEditor = "" }},
		{name: "sublime last used", mutate: func(p *Project) { p.LastUsedEditor = testEditorSublime }},
		{name: "zed last used", mutate: func(p *Project) { p.LastUsedEditor = testEditorZed }},
		{name: "unsupported last used", mutate: func(p *Project) { p.LastUsedEditor = testUnsupportedEditor }, wantErr: true},
		{name: "name at rune limit", mutate: func(p *Project) { p.Name = strings.Repeat("界", 200) }},
		// Stored records predating the input limits must keep loading.
		{name: "stored name exceeds input limit", mutate: func(p *Project) { p.Name = strings.Repeat("界", 201) }},
		{name: "stored newline name", mutate: func(p *Project) { p.Name = "first\nsecond" }},
		{name: "stored carriage return name", mutate: func(p *Project) { p.Name = "first\rsecond" }},
		{name: "NUL name", mutate: func(p *Project) { p.Name = "bad\x00name" }, wantErr: true},
		{name: "description at rune limit", mutate: func(p *Project) { p.Description = strings.Repeat("界", 2000) }},
		{name: "stored description exceeds input limit", mutate: func(p *Project) { p.Description = strings.Repeat("界", 2001) }},
		{name: "NUL description", mutate: func(p *Project) { p.Description = "bad\x00description" }, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := validProject()
			tc.mutate(&p)
			err := p.Validate()
			if tc.wantErr {
				if !errors.Is(err, ErrInvalidProject) {
					t.Fatalf("error = %v, want ErrInvalidProject", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNormalizeEditorRef(t *testing.T) {
	for _, tc := range []struct{ id, want string }{
		{id: testEditorSublime, want: testEditorSublime},
		{id: testEditorZed, want: testEditorZed},
		{id: ""},
		{id: testUnsupportedEditor},
		{id: "unknown"},
		{id: " zed "},
	} {
		t.Run("editor="+tc.id, func(t *testing.T) {
			if got := NormalizeEditorRef(tc.id); got != tc.want {
				t.Fatalf("NormalizeEditorRef(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}

func TestSuggestName(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{path: "/", want: "Root"},
		{path: "/a/b/", want: "b"},
		{path: "/a/My Projects", want: "My Projects"},
		{path: "/a/проект", want: "проект"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := SuggestName(tc.path); got != tc.want {
				t.Fatalf("SuggestName(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestPathKey(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{path: "/a/b/", want: testCleanPath},
		{path: "/a/./b", want: testCleanPath},
		{path: "/a/c/../b", want: testCleanPath},
		{path: "//a///b", want: testCleanPath},
		{path: "/", want: "/"},
		{path: "/My Projects/проект", want: "/My Projects/проект"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			if got := PathKey(tc.path); got != tc.want {
				t.Fatalf("PathKey(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestSuggestNameSanitises(t *testing.T) {
	for _, tc := range []struct{ dir, want string }{
		{"/code/multi\nline", "multi line"},
		{"/code/tab\tname", "tab name"},
		{"/code/\n", "Project"},
		{"/code/" + strings.Repeat("я", 250), strings.Repeat("я", maxNameLength)},
	} {
		if got := SuggestName(tc.dir); got != tc.want {
			t.Errorf("SuggestName(%q) = %q, want %q", tc.dir, got, tc.want)
		}
		if err := validateName(SuggestName(tc.dir)); err != nil {
			t.Errorf("SuggestName(%q) produced an invalid name: %v", tc.dir, err)
		}
	}
}

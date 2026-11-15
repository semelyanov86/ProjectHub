package editor

import (
	"slices"
	"testing"
)

func TestKnown(t *testing.T) {
	want := []Editor{
		{ID: "sublime", Name: "Sublime Text", Candidates: []string{"subl"}},
		{ID: "zed", Name: "Zed", Candidates: []string{"zed", "zeditor"}},
	}
	got := Known()
	if len(got) != len(want) {
		t.Fatalf("Known() length = %d, want %d", len(got), len(want))
	}
	for i, ed := range got {
		if ed.ID != want[i].ID || ed.Name != want[i].Name || !slices.Equal(ed.Candidates, want[i].Candidates) {
			t.Errorf("Known()[%d] = %+v, want %+v", i, ed, want[i])
		}
	}
}

func TestCatalogReturnsCopies(t *testing.T) {
	for _, source := range []string{"Known", "FindByID"} {
		t.Run(source, func(t *testing.T) {
			for _, expected := range Known() {
				var ed Editor
				if source == "Known" {
					catalog := Known()
					for _, candidate := range catalog {
						if candidate.ID == expected.ID {
							ed = candidate
						}
					}
					catalog[0] = Editor{ID: "changed"}
				} else {
					var ok bool
					ed, ok = FindByID(expected.ID)
					if !ok {
						t.Fatalf("FindByID(%q) failed", expected.ID)
					}
				}
				ed.Candidates[0] = "changed"
				fresh, ok := FindByID(expected.ID)
				if !ok || !slices.Equal(fresh.Candidates, expected.Candidates) {
					t.Errorf("mutation leaked into catalog: %+v", fresh)
				}
				for _, fresh := range Known() {
					if fresh.ID == expected.ID && !slices.Equal(fresh.Candidates, expected.Candidates) {
						t.Errorf("mutation leaked into Known(): %+v", fresh)
					}
				}
			}
		})
	}
}

func TestFindByIDUnknown(t *testing.T) {
	if got, ok := FindByID("unknown"); ok || got.ID != "" {
		t.Fatalf("FindByID(unknown) = %+v, %v", got, ok)
	}
}

func TestIsSupported(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want bool
	}{
		{id: "", want: false},
		{id: "vscode", want: false},
		{id: IDZed, want: true},
		{id: IDSublime, want: true},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if got := IsSupported(tc.id); got != tc.want {
				t.Errorf("IsSupported(%q) = %v, want %v", tc.id, got, tc.want)
			}
		})
	}
}

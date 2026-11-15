// Package editor defines the catalog of code editors ProjectHub can open
// projects with. Executable discovery lives in internal/launcher.
package editor

// Supported editor identifiers persisted in settings.json and projects.json.
const (
	IDSublime = "sublime"
	IDZed     = "zed"

	// DefaultID is the global default editor for fresh installations and the
	// replacement for editor identifiers that are no longer supported.
	DefaultID = IDZed
)

// Editor describes a supported code editor integration target.
type Editor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Candidates lists CLI executable names probed (in order) during detection.
	Candidates []string `json:"candidates"`
}

var knownEditors = []Editor{
	{ID: IDSublime, Name: "Sublime Text", Candidates: []string{"subl"}},
	{ID: IDZed, Name: "Zed", Candidates: []string{"zed", "zeditor"}},
}

// Known returns a copy of the supported editor catalog in display order.
func Known() []Editor {
	out := make([]Editor, len(knownEditors))
	for i, ed := range knownEditors {
		out[i] = clone(ed)
	}
	return out
}

// FindByID returns the Editor definition matching id, if supported.
func FindByID(id string) (Editor, bool) {
	for _, ed := range knownEditors {
		if ed.ID == id {
			return clone(ed), true
		}
	}
	return Editor{}, false
}

// IsSupported reports whether id names a supported editor.
func IsSupported(id string) bool {
	_, ok := FindByID(id)
	return ok
}

func clone(ed Editor) Editor {
	ed.Candidates = append([]string(nil), ed.Candidates...)
	return ed
}

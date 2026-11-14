// Package editor defines supported code editor metadata and lookup helpers.
// Discovery of installed editor binaries on the host system will be implemented
// in a subsequent feature stage.
package editor

// Editor describes a supported code editor or IDE integration target.
type Editor struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Binary     string   `json:"binary"`
	Candidates []string `json:"candidates"`
}

var knownEditors = []Editor{
	{ID: "vscode", Name: "Visual Studio Code", Binary: "code", Candidates: []string{"code", "code-insiders"}},
	{ID: "cursor", Name: "Cursor", Binary: "cursor", Candidates: []string{"cursor"}},
	{ID: "zed", Name: "Zed", Binary: "zed", Candidates: []string{"zed", "zeditor"}},
	{ID: "goland", Name: "GoLand", Binary: "goland", Candidates: []string{"goland"}},
	{ID: "idea", Name: "IntelliJ IDEA", Binary: "idea", Candidates: []string{"idea"}},
	{ID: "webstorm", Name: "WebStorm", Binary: "webstorm", Candidates: []string{"webstorm"}},
	{ID: "pycharm", Name: "PyCharm", Binary: "pycharm", Candidates: []string{"pycharm"}},
	{ID: "sublime", Name: "Sublime Text", Binary: "subl", Candidates: []string{"subl"}},
	{ID: "neovim", Name: "Neovim", Binary: "nvim", Candidates: []string{"nvim"}},
}

// Known returns a copy of the built-in supported editor catalog.
func Known() []Editor {
	out := make([]Editor, len(knownEditors))
	for i, ed := range knownEditors {
		candidates := make([]string, len(ed.Candidates))
		copy(candidates, ed.Candidates)
		ed.Candidates = candidates
		out[i] = ed
	}
	return out
}

// FindByID returns the built-in Editor definition matching id, if any.
func FindByID(id string) (Editor, bool) {
	for _, ed := range knownEditors {
		if ed.ID == id {
			candidates := make([]string, len(ed.Candidates))
			copy(candidates, ed.Candidates)
			ed.Candidates = candidates
			return ed, true
		}
	}
	return Editor{}, false
}

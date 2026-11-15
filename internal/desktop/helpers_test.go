package desktop

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/config"
	"github.com/semelyanov86/ProjectHub/internal/git"
	"github.com/semelyanov86/ProjectHub/internal/launcher"
	"github.com/semelyanov86/ProjectHub/internal/process"
	"github.com/semelyanov86/ProjectHub/internal/project"
)

const (
	specialDirectoryName = "project with spaces 'quotes' $cash; [demo]"
	sublimeExecutable    = "subl"
)

type fakeResponse struct {
	result process.Result
	err    error
}

type fakeRunner struct {
	mu           sync.Mutex
	runs         []process.Command
	starts       []process.Command
	responses    map[string]fakeResponse
	startErrors  map[string]error
	startEntered chan struct{}
	startGate    <-chan struct{}
}

func (r *fakeRunner) Run(_ context.Context, cmd process.Command) (process.Result, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, cloneCommand(cmd))
	if response, ok := r.responses[cmd.Path]; ok {
		return response.result, response.err
	}
	if filepath.Base(cmd.Path) == "git" {
		return process.Result{
			Stdout: []byte("# branch.oid 0123456789abcdef0123456789abcdef01234567\n# branch.head main\n"),
		}, nil
	}
	return process.Result{}, nil
}

func (r *fakeRunner) Start(cmd process.Command) error {
	r.mu.Lock()
	r.starts = append(r.starts, cloneCommand(cmd))
	entered, gate, err := r.startEntered, r.startGate, r.startErrors[cmd.Path]
	r.mu.Unlock()
	if entered != nil {
		select {
		case entered <- struct{}{}:
		default:
		}
	}
	if gate != nil {
		<-gate
	}
	return err
}

func cloneCommand(cmd process.Command) process.Command {
	cmd.Args = slices.Clone(cmd.Args)
	cmd.Env = slices.Clone(cmd.Env)
	return cmd
}

func (r *fakeRunner) commands() (runs, starts []process.Command) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.runs), slices.Clone(r.starts)
}

type fakeDialogs struct {
	directory, executable, importFile, exportFile     string
	directoryErr, executableErr, importErr, exportErr error
	exportDefaultName                                 string
}

func (d *fakeDialogs) ChooseDirectory(context.Context, string) (string, error) {
	return d.directory, d.directoryErr
}

func (d *fakeDialogs) ChooseExecutable(context.Context) (string, error) {
	return d.executable, d.executableErr
}

func (d *fakeDialogs) ChooseImportFile(context.Context) (string, error) {
	return d.importFile, d.importErr
}

func (d *fakeDialogs) ChooseExportFile(_ context.Context, defaultName string) (string, error) {
	d.exportDefaultName = defaultName
	return d.exportFile, d.exportErr
}

type testService struct {
	*Service
	store   *config.Store
	runner  *fakeRunner
	dialogs *fakeDialogs
	binDir  string
}

func newTestService(t *testing.T) *testService {
	t.Helper()
	store, err := config.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	for _, name := range []string{sublimeExecutable, "zed", "ghostty", "gio"} {
		path := filepath.Join(binDir, name)
		writeTestFile(t, path, []byte("fake executable; never run\n"))
		//nolint:gosec // Resolver fixtures need executable bits; the fake runner never executes these temporary files.
		if err := os.Chmod(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	runner := &fakeRunner{
		responses: make(map[string]fakeResponse), startErrors: make(map[string]error),
	}
	dialogs := &fakeDialogs{}
	svc := NewService(Deps{
		Catalog:  project.NewCatalog(store, launcher.ValidateProjectDir),
		Settings: store,
		Transfer: store,
		Launcher: launcher.New(runner, &launcher.Resolver{
			LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			Stat:     os.Stat, SearchDirs: []string{binDir},
		}),
		Inspector: git.NewInspector(
			runner,
			func(string) (string, error) { return "/usr/bin/git", nil },
			2,
			time.Second,
		),
		Dialogs: dialogs, ValidateDir: launcher.ValidateProjectDir,
	})
	svc.now = func() time.Time { return time.Date(2026, 11, 15, 12, 30, 0, 0, time.UTC) }
	return &testService{Service: svc, store: store, runner: runner, dialogs: dialogs, binDir: binDir}
}

func projectDirectory(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func addTestProject(t *testing.T, svc *Service, path string) project.Project {
	t.Helper()
	p, err := svc.AddProject(project.Input{Name: filepath.Base(path), Path: path})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func listTestProjects(t *testing.T, svc *Service) []project.Project {
	t.Helper()
	projects, err := svc.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	return projects
}

func getTestProject(t *testing.T, svc *Service, id string) project.Project {
	t.Helper()
	p, err := svc.deps.Catalog.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func writeTestFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func saveTestSettings(t *testing.T, store *config.Store, settings config.Settings) {
	t.Helper()
	if err := store.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
}

func rememberEditor(t *testing.T, h *testService, p project.Project, id string) {
	t.Helper()
	if _, err := h.store.UpdateProjects(func(projects []project.Project) ([]project.Project, error) {
		for i := range projects {
			if projects[i].ID == p.ID {
				projects[i].LastUsedEditor = id
			}
		}
		return projects, nil
	}); err != nil {
		t.Fatal(err)
	}
}

func assertCommand(t *testing.T, got, want process.Command) {
	t.Helper()
	if got.Path != want.Path || got.Dir != want.Dir || !slices.Equal(got.Args, want.Args) {
		t.Fatalf("command = %+v, want %+v", got, want)
	}
}

func assertProjects(t *testing.T, svc *Service, want []project.Project) {
	t.Helper()
	if got := listTestProjects(t, svc); !reflect.DeepEqual(got, want) {
		t.Fatalf("projects = %+v, want %+v", got, want)
	}
}

func assertProjectInput(t *testing.T, got project.Project, want project.Input) {
	t.Helper()
	fields := project.Input{
		Name: got.Name, Description: got.Description, Path: got.Path,
		DefaultEditor: got.DefaultEditor, Favorite: got.Favorite,
	}
	if fields != want {
		t.Fatalf("project fields = %+v, want %+v", fields, want)
	}
}

func assertDirectory(t *testing.T, path string) {
	t.Helper()
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("directory %q: info=%v, err=%v", path, info, err)
	}
}

type importFixture struct {
	*testService
	imported []project.Project
}

func newImportFixture(t *testing.T) *importFixture {
	t.Helper()
	h := newTestService(t)
	duplicate := addTestProject(t, h.Service, projectDirectory(t, "existing"))
	addTestProject(t, h.Service, projectDirectory(t, "not in import"))
	duplicate.Name = "Imported existing"
	newProject := project.Project{
		ID: "import-new", Name: "Imported new", Path: projectDirectory(t, specialDirectoryName),
		CreatedAt: h.now(), UpdatedAt: h.now(),
	}
	missing := project.Project{
		ID: "import-missing", Name: "Imported missing", Path: filepath.Join(t.TempDir(), "missing"),
		CreatedAt: h.now(), UpdatedAt: h.now(),
	}
	imported := []project.Project{duplicate, newProject, missing}
	document, err := json.Marshal(config.ProjectsFile{SchemaVersion: 2, Projects: imported})
	if err != nil {
		t.Fatal(err)
	}
	h.dialogs.importFile = filepath.Join(t.TempDir(), "imported projects.json")
	writeTestFile(t, h.dialogs.importFile, document)
	return &importFixture{testService: h, imported: imported}
}

func prepareTestImport(t *testing.T, svc *Service) ImportPreview {
	t.Helper()
	preview, err := svc.PrepareImport(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if preview.Cancelled || preview.Token == "" {
		t.Fatalf("PrepareImport() = %+v, want usable token", preview)
	}
	return preview
}

func assertSkippedPath(t *testing.T, skipped []project.Skip, path, reason string) {
	t.Helper()
	i := slices.IndexFunc(skipped, func(skip project.Skip) bool { return skip.Path == path })
	if i < 0 {
		t.Fatalf("%q not found in skipped projects: %+v", path, skipped)
	}
	if skipped[i].Reason == "" || !strings.Contains(skipped[i].Reason, reason) {
		t.Errorf("skip reason = %q, want containing %q", skipped[i].Reason, reason)
	}
}

package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/process"
)

const testStatus = "# branch.oid (initial)\n# branch.head main\n"

type fakeRunner struct {
	mu       sync.Mutex
	commands []process.Command
	result   process.Result
	err      error
	delay    time.Duration
	started  chan struct{}
	run      func(context.Context, process.Command) (process.Result, error)
	inFlight atomic.Int32
	maximum  atomic.Int32
}

func (r *fakeRunner) Run(ctx context.Context, cmd process.Command) (process.Result, error) {
	r.mu.Lock()
	r.commands = append(r.commands, cmd)
	r.mu.Unlock()
	current := r.inFlight.Add(1)
	defer r.inFlight.Add(-1)
	for {
		previous := r.maximum.Load()
		if current <= previous || r.maximum.CompareAndSwap(previous, current) {
			break
		}
	}
	if r.started != nil {
		select {
		case r.started <- struct{}{}:
		default:
		}
	}
	if r.delay > 0 {
		timer := time.NewTimer(r.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return process.Result{}, fmt.Errorf("fake git: %w", ctx.Err())
		}
	}
	if r.run != nil {
		return r.run(ctx, cmd)
	}
	return r.result, r.err
}

func (*fakeRunner) Start(process.Command) error {
	return errors.New("unexpected Start call")
}

func (r *fakeRunner) recorded() []process.Command {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.commands)
}

func fakeLookPath(name string) (string, error) {
	if name != "git" {
		return "", exec.ErrNotFound
	}
	return "/usr/bin/git", nil
}

func TestInspectorCommand(t *testing.T) {
	t.Setenv("GIT_DIR", "/inherited/git")
	runner := &fakeRunner{result: process.Result{Stdout: []byte(testStatus)}}
	i := NewInspector(runner, fakeLookPath, 2, time.Second)
	i.home = "/home/test"
	dir := "/projects/path with spaces;$(touch nope)"
	got := i.Inspect(t.Context(), dir)
	if got.State != StateRepo || got.Repo.Branch != testBranch || !got.Repo.Unborn || got.Repo.IsDirty {
		t.Fatalf("Inspect() = %+v, want clean unborn main repository", got)
	}
	commands := runner.recorded()
	if len(commands) != 2 {
		t.Fatalf("runner calls = %d, want config lookup and status", len(commands))
	}
	assertConfigLookup(t, commands[0], dir)
	cmd := commands[1]
	wantArgs := []string{
		"--no-optional-locks", "-c", "core.fsmonitor=false", "-c", "safe.bareRepository=explicit",
		"-c", "color.ui=false", "-C", dir, "status", "--porcelain=v2", "--branch",
		"--untracked-files=no", "--no-renames", "--ignore-submodules=all",
	}
	if cmd.Path != "/usr/bin/git" || cmd.Dir != dir || !slices.Equal(cmd.Args, wantArgs) {
		t.Fatalf("command = %+v, want resolved git with dir %q and argv %q", cmd, dir, wantArgs)
	}
	for _, entry := range []string{
		"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_CEILING_DIRECTORIES=/home/test",
		"GIT_ALLOW_PROTOCOL=", "GIT_NO_LAZY_FETCH=1",
	} {
		if !slices.Contains(cmd.Env, entry) {
			t.Errorf("command environment is missing %q", entry)
		}
	}
	if slices.ContainsFunc(cmd.Env, func(entry string) bool { return strings.HasPrefix(entry, "GIT_DIR=") }) {
		t.Error("command inherited GIT_DIR")
	}
}

func assertConfigLookup(t *testing.T, lookup process.Command, dir string) {
	t.Helper()
	want := []string{
		"--no-optional-locks", "-c", "safe.bareRepository=explicit", "-C", dir,
		"config", "--show-scope", "--name-only", "--get-regexp", `^filter\.`,
	}
	if lookup.Path != "/usr/bin/git" || lookup.Dir != dir || !slices.Equal(lookup.Args, want) {
		t.Fatalf("config lookup = %+v, want argv %q", lookup, want)
	}
}

func TestInspectionEnv(t *testing.T) {
	for _, name := range []string{
		"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_COUNT", "GIT_UNKNOWN",
	} {
		t.Setenv(name, "inherited")
	}
	t.Setenv("GIT_OPTIONAL_LOCKS", "1")
	t.Setenv("GIT_TERMINAL_PROMPT", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", "/inherited")
	t.Setenv("LC_ALL", "de_DE.UTF-8")
	t.Setenv("WEBKIT_TEST", "inherited")
	t.Setenv("PROJECTHUB_GIT_TEST", "preserved")
	for _, home := range []string{"", "/home/test"} {
		t.Run(home, func(t *testing.T) {
			env := inspectionEnv(home)
			allowed := []string{
				"GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_ALLOW_PROTOCOL=", "GIT_NO_LAZY_FETCH=1",
			}
			if home != "" {
				allowed = append(allowed, "GIT_CEILING_DIRECTORIES="+home)
			}
			for _, entry := range allowed {
				if !slices.Contains(env, entry) {
					t.Errorf("environment is missing %q", entry)
				}
			}
			for _, entry := range env {
				if strings.HasPrefix(entry, "GIT_") && !slices.Contains(allowed, entry) {
					t.Errorf("unexpected inherited git variable %q", entry)
				}
			}
			if slices.Contains(env, "WEBKIT_TEST=inherited") || !slices.Contains(env, "PROJECTHUB_GIT_TEST=preserved") {
				t.Error("environment did not preserve process sanitisation and unrelated variables")
			}
		})
	}
}

func TestInspectorGitMissing(t *testing.T) {
	runner := &fakeRunner{}
	var lookups atomic.Int32
	i := NewInspector(runner, func(name string) (string, error) {
		lookups.Add(1)
		if name != "git" {
			t.Errorf("lookPath(%q), want git", name)
		}
		return "", exec.ErrNotFound
	}, 1, time.Second)
	want := Inspection{State: StateUnavailable, Message: unavailableMessage}
	if got := i.Inspect(t.Context(), "/project"); got != want {
		t.Fatalf("Inspect() = %+v, want %+v", got, want)
	}
	for _, got := range i.InspectMany(t.Context(), []string{"/one", "/two", "/three"}) {
		if got != want {
			t.Errorf("InspectMany() entry = %+v, want %+v", got, want)
		}
	}
	if len(runner.recorded()) != 0 || lookups.Load() != 2 {
		t.Fatalf("runner calls = %d, lookups = %d, want 0 and 2", len(runner.recorded()), lookups.Load())
	}
}

func TestInspectorFailures(t *testing.T) {
	tests := []struct {
		name   string
		result process.Result
		err    error
		want   Inspection
	}{
		{
			name: "not repository", result: process.Result{ExitCode: 128, Stderr: []byte("fatal: NOT A GIT REPOSITORY")},
			err: errors.New("exit 128"), want: Inspection{State: StateNotRepo},
		},
		{
			name: "dubious ownership", result: process.Result{ExitCode: 128, Stderr: []byte("fatal: detected dubious ownership\nhelp")},
			err: errors.New("exit 128"), want: Inspection{State: StateError, Message: "fatal: detected dubious ownership"},
		},
		{
			name: "first nonempty line", result: process.Result{ExitCode: 1, Stderr: []byte("\n \r\n  permission denied \r\nextra")},
			err: errors.New("exit 1"), want: Inspection{State: StateError, Message: "permission denied"},
		},
		{
			name: "bounded unicode message", result: process.Result{ExitCode: 1, Stderr: []byte(strings.Repeat("é", 220))},
			err: errors.New("exit 1"), want: Inspection{State: StateError, Message: strings.Repeat("é", 200)},
		},
		{
			name: "error fallback", err: errors.New("cannot start git"),
			want: Inspection{State: StateError, Message: "cannot start git"},
		},
		{
			name: "blank stderr fallback", result: process.Result{Stderr: []byte("\n \n")}, err: errors.New("cannot start git"),
			want: Inspection{State: StateError, Message: "cannot start git"},
		},
		{
			name: "malformed status", result: process.Result{Stdout: []byte("not porcelain")},
			want: Inspection{State: StateError, Message: "Could not parse git status output."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{result: tt.result, err: tt.err}
			i := NewInspector(runner, fakeLookPath, 1, time.Second)
			if got := i.Inspect(t.Context(), "/project"); got != tt.want {
				t.Fatalf("Inspect() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestInspectorTimeout(t *testing.T) {
	runner := &fakeRunner{delay: time.Second}
	i := NewInspector(runner, fakeLookPath, 1, 10*time.Millisecond)
	want := Inspection{State: StateError, Message: "git status timed out after 10ms"}
	if got := i.Inspect(t.Context(), "/project"); got != want {
		t.Fatalf("Inspect() = %+v, want %+v", got, want)
	}
}

func TestInspectorParentCancellation(t *testing.T) {
	t.Run("before starting", func(t *testing.T) {
		runner := &fakeRunner{}
		i := NewInspector(runner, fakeLookPath, 1, time.Second)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		assertCancelled(t, i.Inspect(ctx, "/project"))
		if len(runner.recorded()) != 0 {
			t.Fatal("cancelled inspection started git")
		}
	})
	t.Run("during command", func(t *testing.T) {
		runner := &fakeRunner{delay: time.Second, started: make(chan struct{}, 1)}
		i := NewInspector(runner, fakeLookPath, 1, 2*time.Second)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		done := make(chan Inspection, 1)
		go func() { done <- i.Inspect(ctx, "/project") }()
		select {
		case <-runner.started:
		case <-time.After(time.Second):
			t.Fatal("git did not start")
		}
		cancel()
		assertCancelled(t, <-done)
	})
	t.Run("waiting for semaphore", func(t *testing.T) {
		runner := &fakeRunner{}
		i := NewInspector(runner, fakeLookPath, 1, time.Second)
		i.sem <- struct{}{}
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		defer cancel()
		assertCancelled(t, i.Inspect(ctx, "/project"))
		<-i.sem
		if len(runner.recorded()) != 0 {
			t.Fatal("waiting inspection started git")
		}
	})
}

func assertCancelled(t *testing.T, got Inspection) {
	t.Helper()
	want := Inspection{State: StateError, Message: cancelledMessage}
	if got != want {
		t.Fatalf("Inspect() = %+v, want %+v", got, want)
	}
}

func TestInspectorInspectMany(t *testing.T) {
	runner := &fakeRunner{
		delay: 5 * time.Millisecond,
		run: func(_ context.Context, cmd process.Command) (process.Result, error) {
			return process.Result{Stdout: []byte("# branch.head " + cmd.Dir + "\n")}, nil
		},
	}
	var lookups atomic.Int32
	i := NewInspector(runner, func(name string) (string, error) {
		lookups.Add(1)
		return fakeLookPath(name)
	}, 2, time.Second)
	dirs := make([]string, 10)
	for n := range dirs {
		dirs[n] = fmt.Sprintf("/project-%d", n)
	}
	ctx := t.Context()
	start := make(chan struct{})
	results := make([][]Inspection, 2)
	var wg sync.WaitGroup
	for batch := range results {
		wg.Go(func() {
			<-start
			results[batch] = i.InspectMany(ctx, dirs)
		})
	}
	close(start)
	wg.Wait()
	for _, batch := range results {
		if len(batch) != len(dirs) {
			t.Fatalf("InspectMany() length = %d, want %d", len(batch), len(dirs))
		}
		for n, got := range batch {
			if got.State != StateRepo || got.Repo.Branch != dirs[n] {
				t.Errorf("InspectMany()[%d] = %+v, want repository %q", n, got, dirs[n])
			}
		}
	}
	if got := runner.maximum.Load(); got != 2 {
		t.Errorf("maximum concurrency = %d, want 2", got)
	}
	if calls := len(runner.recorded()); calls != 40 || lookups.Load() != 2 {
		t.Errorf("runner calls = %d, lookups = %d, want 40 (config + status per dir) and 2", calls, lookups.Load())
	}
}

func TestInspectorDefaultsAndEmptyInput(t *testing.T) {
	for _, value := range []int{0, -1} {
		i := NewInspector(&fakeRunner{}, nil, value, time.Duration(value))
		if cap(i.sem) != 4 || i.timeout != 3*time.Second || i.lookPath == nil {
			t.Fatalf("NewInspector() defaults: capacity=%d timeout=%s", cap(i.sem), i.timeout)
		}
		i.lookPath = func(string) (string, error) {
			t.Fatal("empty batch resolved git")
			return "", exec.ErrNotFound
		}
		got := i.InspectMany(t.Context(), nil)
		if got == nil || len(got) != 0 {
			t.Fatalf("InspectMany(nil) = %#v, want non-nil empty slice", got)
		}
	}
}

func TestInspectorIntegration(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	gitCommand(t, gitPath, "-C", dir, "init", "-b", testBranch)
	i := NewInspector(process.NewOSRunner(), nil, 1, 3*time.Second)
	i.home = ""
	got := i.Inspect(t.Context(), dir)
	if got.State != StateRepo || got.Repo.Branch != testBranch || !got.Repo.Unborn || got.Repo.IsDirty {
		t.Fatalf("Inspect(empty repo) = %+v, want clean unborn main", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("tracked\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := i.Inspect(t.Context(), dir); got.State != StateRepo || got.Repo.IsDirty {
		t.Fatalf("Inspect(untracked file) = %+v, want clean repository", got)
	}
	gitCommand(t, gitPath, "-C", dir, "add", "--", "file.txt")
	if got := i.Inspect(t.Context(), dir); got.State != StateRepo || !got.Repo.IsDirty {
		t.Fatalf("Inspect(staged file) = %+v, want dirty repository", got)
	}
	if got := i.Inspect(t.Context(), t.TempDir()); got.State != StateNotRepo || got.Message != "" {
		t.Fatalf("Inspect(non-repo) = %+v, want not_repo without a message", got)
	}
}

func TestInspectorDisablesConfiguredFilters(t *testing.T) {
	runner := &fakeRunner{
		run: func(_ context.Context, cmd process.Command) (process.Result, error) {
			if slices.Contains(cmd.Args, "config") {
				return process.Result{Stdout: []byte(
					"local\tfilter.evil.clean\nworktree\tfilter.evil.process\nlocal\tfilter.a=b.c.clean\n" +
						"global\tfilter.lfs.clean\nsystem\tfilter.sys.process\n",
				)}, nil
			}
			return process.Result{Stdout: []byte(testStatus)}, nil
		},
	}
	i := NewInspector(runner, fakeLookPath, 1, time.Second)
	if got := i.Inspect(t.Context(), "/repo"); got.State != StateRepo {
		t.Fatalf("Inspect() = %+v, want repo", got)
	}
	status := runner.recorded()[1]
	if slices.ContainsFunc(status.Env, func(e string) bool { return strings.Contains(e, "filter.lfs.") || strings.Contains(e, "filter.sys.") }) {
		t.Error("filters from trusted global/system config were disabled")
	}
	for _, entry := range []string{
		"GIT_CONFIG_COUNT=8",
		"GIT_CONFIG_KEY_0=filter.evil.clean", "GIT_CONFIG_VALUE_0=",
		"GIT_CONFIG_KEY_2=filter.evil.process", "GIT_CONFIG_VALUE_2=",
		"GIT_CONFIG_KEY_3=filter.evil.required", "GIT_CONFIG_VALUE_3=false",
		"GIT_CONFIG_KEY_4=filter.a=b.c.clean", "GIT_CONFIG_VALUE_4=",
	} {
		if !slices.Contains(status.Env, entry) {
			t.Errorf("status environment is missing %q", entry)
		}
	}
}

func TestInspectorWithoutFilterDrivers(t *testing.T) {
	runner := &fakeRunner{
		run: func(_ context.Context, cmd process.Command) (process.Result, error) {
			if slices.Contains(cmd.Args, "config") {
				return process.Result{ExitCode: 1}, errors.New("exit status 1")
			}
			return process.Result{Stdout: []byte(testStatus)}, nil
		},
	}
	i := NewInspector(runner, fakeLookPath, 1, time.Second)
	if got := i.Inspect(t.Context(), "/repo"); got.State != StateRepo {
		t.Fatalf("Inspect() = %+v, want repo when no filters are configured", got)
	}
	if slices.ContainsFunc(runner.recorded()[1].Env, func(e string) bool { return strings.HasPrefix(e, "GIT_CONFIG_COUNT=") }) {
		t.Error("status environment overrides config although no filters are configured")
	}
}

func TestFilterDriverNames(t *testing.T) {
	out := []byte("local\tfilter.lfs.clean\nlocal\tfilter.lfs.smudge\r\nworktree\tfilter.my.driver.process\n" +
		"local\tfilter..clean\nlocal\tfilter.bad\nlocal\tother.key\nglobal\tfilter.trusted.clean\n" +
		"system\tfilter.admin.clean\nfilter.noscope.clean\n\n")
	if got, want := untrustedFilterDrivers(out), []string{"lfs", "my.driver", "", "noscope"}; !slices.Equal(got, want) {
		t.Fatalf("untrustedFilterDrivers() = %q, want %q", got, want)
	}
	if env := filterOverrideEnv(nil); env != nil {
		t.Fatalf("filterOverrideEnv(nil) = %q, want nil", env)
	}
}

// TestInspectorNeverRunsRepositoryFilters is a regression test: a repository
// may configure a clean filter in its own .git/config and assign it through
// .gitattributes or .git/info/attributes; `git status` would run it when it
// re-reads a stat-dirty tracked file.
func TestInspectorNeverRunsRepositoryFilters(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	for _, tc := range []struct{ name, config, attribute string }{
		{name: "named driver", config: "[filter \"evil\"]\n", attribute: "*.txt filter=evil\n"},
		{name: "empty driver name", config: "[filter \"\"]\n", attribute: "*.txt filter=\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(t.TempDir(), "filter-ran")
			tracked := filepath.Join(dir, "file.txt")
			if err := os.WriteFile(tracked, []byte("data\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			gitCommand(t, gitPath, "-C", dir, "init", "-b", testBranch)
			gitCommand(t, gitPath, "-C", dir, "add", "--", "file.txt")
			gitCommand(t, gitPath, "-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "init")
			appendFile(t, filepath.Join(dir, ".git", "config"), tc.config+"\tclean = touch "+marker+"; cat\n")
			appendFile(t, filepath.Join(dir, ".git", "info", "attributes"), tc.attribute)
			future := time.Now().Add(2 * time.Second)
			if err := os.Chtimes(tracked, future, future); err != nil {
				t.Fatal(err)
			}

			i := NewInspector(process.NewOSRunner(), nil, 1, 3*time.Second)
			i.home = ""
			if got := i.Inspect(t.Context(), dir); got.State != StateRepo {
				t.Fatalf("Inspect() = %+v, want repo", got)
			}
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("repository-configured filter was executed (marker stat error: %v)", err)
			}
		})
	}
}

// TestInspectorDirtyStateWithFilters checks that a clean tree with a
// content-transforming filter is not reported as plainly dirty: drivers from
// the user's global config stay enabled (accurate result), while drivers from
// the repository's own config are disabled and the result is marked uncertain.
func TestInspectorDirtyStateWithFilters(t *testing.T) {
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not available")
	}
	const upper = "[filter \"upper\"]\n\tclean = tr a-z A-Z\n\tsmudge = cat\n"
	for _, tc := range []struct {
		name          string
		global, local string
		wantDirty     bool
		wantUncertain bool
	}{
		{name: "global filter stays enabled", global: upper, wantDirty: false},
		{name: "repository filter disabled", local: upper, wantDirty: true, wantUncertain: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			appendFile(t, filepath.Join(home, ".gitconfig"), tc.global)
			dir := t.TempDir()
			tracked := filepath.Join(dir, "file.txt")
			if err := os.WriteFile(tracked, []byte("Hello\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			appendFile(t, filepath.Join(dir, ".gitattributes"), "*.txt filter=upper\n")
			userGit(t, gitPath, "-C", dir, "init", "-q", "-b", testBranch)
			appendFile(t, filepath.Join(dir, ".git", "config"), tc.local)
			userGit(t, gitPath, "-C", dir, "add", "--", "file.txt", ".gitattributes")
			userGit(t, gitPath, "-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "init")
			future := time.Now().Add(2 * time.Second)
			if err := os.Chtimes(tracked, future, future); err != nil {
				t.Fatal(err)
			}

			i := NewInspector(process.NewOSRunner(), nil, 1, 3*time.Second)
			i.home = ""
			got := i.Inspect(t.Context(), dir)
			if got.State != StateRepo || got.Repo.IsDirty != tc.wantDirty || got.Repo.DirtyUncertain != tc.wantUncertain {
				t.Fatalf("Inspect() = %+v, want dirty=%v uncertain=%v", got, tc.wantDirty, tc.wantUncertain)
			}
		})
	}
}

// userGit runs git with the test's HOME (including its global config).
func userGit(t *testing.T, gitPath string, args ...string) {
	t.Helper()
	//nolint:gosec // The resolved Git binary receives separate arguments for a temporary test repository.
	cmd := exec.CommandContext(t.Context(), gitPath, args...)
	cmd.Env = inspectionEnv("")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %q: %v\n%s", args, err, out)
	}
}

func appendFile(t *testing.T, path, content string) {
	t.Helper()
	file, err := os.OpenFile(filepath.Clean(path), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if _, err := file.WriteString(content); err != nil {
		t.Fatal(err)
	}
}

func gitCommand(t *testing.T, gitPath string, args ...string) {
	t.Helper()
	//nolint:gosec // The resolved Git binary receives separate arguments for a temporary test repository.
	cmd := exec.CommandContext(t.Context(), gitPath, args...)
	cmd.Env = process.WithVariables(inspectionEnv(""), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %q: %v\n%s", args, err, out)
	}
}

package launcher

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/process"
)

const (
	xdgName     = "xdg-open"
	kittyName   = "kitty"
	gioName     = "gio"
	runMethod   = "run"
	startMethod = "start"
	filesName   = "files"
	fakeSublime = "/fake/subl"
	fakeZed     = "/fake/zed"
	fakeGhostty = "/fake/ghostty"
	fakeKitty   = "/fake/kitty"
	fakeGio     = "/fake/gio"
	fakeXdg     = "/fake/xdg-open"
	fakeFiles   = "/fake/files"
)

type recordedCommand struct {
	method  string
	command process.Command
}

type fakeRunner struct {
	calls    []recordedCommand
	result   process.Result
	runErr   error
	startErr error
	onRun    func(context.Context)
}

func (r *fakeRunner) Run(ctx context.Context, cmd process.Command) (process.Result, error) {
	r.calls = append(r.calls, recordedCommand{method: runMethod, command: cmd})
	if r.onRun != nil {
		r.onRun(ctx)
	}
	return r.result, r.runErr
}

func (r *fakeRunner) Start(cmd process.Command) error {
	r.calls = append(r.calls, recordedCommand{method: startMethod, command: cmd})
	return r.startErr
}

func projectDirectory(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "project 'quoted' \"double\" $value; путь")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	return dir
}

func allToolsResolver() *Resolver {
	return fakeResolver(map[string]string{
		"subl": fakeSublime, testZedName: fakeZed, "ghostty": fakeGhostty,
		kittyName: fakeKitty, gioName: fakeGio, xdgName: fakeXdg, filesName: fakeFiles,
	})
}

func assertCalls(t *testing.T, runner *fakeRunner, expected ...recordedCommand) {
	t.Helper()
	if !reflect.DeepEqual(runner.calls, expected) {
		t.Fatalf("calls = %#v; want %#v", runner.calls, expected)
	}
}

func TestOpenCommands(t *testing.T) {
	dir := projectDirectory(t)
	tests := []struct {
		name   string
		target Target
		prefs  Preferences
		method string
		exe    string
		args   []string
	}{
		{name: "sublime", target: TargetSublime, method: startMethod, exe: fakeSublime, args: []string{dir}},
		{name: testZedName, target: TargetZed, method: startMethod, exe: fakeZed, args: []string{dir}},
		{
			name: "ghostty", target: TargetGhostty, method: runMethod, exe: fakeGhostty,
			args: []string{"+new-window", "--working-directory=" + dir},
		},
		{
			name: "terminal", target: TargetTerminal, method: startMethod, exe: fakeKitty,
			prefs: Preferences{CustomTerminal: TerminalCommand{
				Executable: kittyName,
				Args:       []string{"--working-directory={path}", PathPlaceholder, "{path}:{path}", "literal ;$' ü"},
			}},
			args: []string{"--working-directory=" + dir, dir, dir + ":" + dir, "literal ;$' ü"},
		},
		{
			name: "auto files", target: TargetFileManager, method: runMethod, exe: fakeGio,
			args: []string{"open", dir},
		},
		{
			name: "custom files", target: TargetFileManager, method: startMethod, exe: fakeFiles,
			prefs: Preferences{FileManagerMode: FileManagerCustom, Paths: ToolPaths{FileManager: filesName}},
			args:  []string{dir},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{}
			l := New(runner, allToolsResolver())
			if err := l.Open(t.Context(), tt.target, dir, tt.prefs); err != nil {
				t.Fatal(err)
			}
			assertCalls(t, runner, recordedCommand{
				method:  tt.method,
				command: process.Command{Path: tt.exe, Args: tt.args, Dir: dir},
			})
		})
	}
}

func TestOpenGhosttyFallback(t *testing.T) {
	dir := projectDirectory(t)
	primaryErr, fallbackErr := errors.New("IPC failed"), errors.New("spawn failed")
	tests := []struct {
		name     string
		startErr error
		wantErr  error
	}{
		{name: "fallback succeeds"},
		{name: "both fail", startErr: fallbackErr, wantErr: ErrLaunchFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{
				runErr: primaryErr, startErr: tt.startErr,
				result: process.Result{Stderr: []byte("  activation file missing  \nignored line")},
			}
			err := New(runner, allToolsResolver()).Open(t.Context(), TargetGhostty, dir, Preferences{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v; want %v", err, tt.wantErr)
			}
			if err != nil {
				for _, text := range []string{"IPC failed", "activation file missing", "spawn failed"} {
					if !strings.Contains(err.Error(), text) {
						t.Errorf("error %q lacks %q", err, text)
					}
				}
				wrapsBoth := errors.Is(err, primaryErr) && errors.Is(err, fallbackErr)
				if !wrapsBoth || strings.Contains(err.Error(), "ignored line") {
					t.Fatalf("invalid combined error: %v", err)
				}
			}
			assertCalls(t, runner,
				recordedCommand{method: runMethod, command: process.Command{
					Path: fakeGhostty,
					Args: []string{"+new-window", "--working-directory=" + dir}, Dir: dir,
				}},
				recordedCommand{method: startMethod, command: process.Command{
					Path: fakeGhostty,
					Args: []string{"--gtk-single-instance=false", "--working-directory=" + dir}, Dir: dir,
				}})
		})
	}
}

func TestOpenGhosttyContext(t *testing.T) {
	dir := projectDirectory(t)
	tests := []struct {
		name            string
		parentCancelled bool
		wantCalls       int
	}{
		{name: "cancelled parent", parentCancelled: true, wantCalls: 1},
		{name: "child timeout", wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			runner := &fakeRunner{runErr: context.DeadlineExceeded}
			l := New(runner, allToolsResolver())
			l.ghosttyTimeout = time.Nanosecond
			runner.onRun = func(child context.Context) {
				if _, ok := child.Deadline(); !ok {
					t.Error("no IPC timeout")
				}
				if tt.parentCancelled {
					cancel()
				} else {
					<-child.Done()
				}
			}
			err := l.Open(ctx, TargetGhostty, dir, Preferences{})
			if tt.parentCancelled && !errors.Is(err, context.Canceled) {
				t.Fatalf("want cancellation: %v", err)
			}
			if !tt.parentCancelled && err != nil {
				t.Fatal(err)
			}
			if len(runner.calls) != tt.wantCalls {
				t.Fatalf("calls = %v", runner.calls)
			}
		})
	}
}

func TestOpenFileManagerFallback(t *testing.T) {
	dir := projectDirectory(t)
	tests := []struct {
		name     string
		paths    map[string]string
		runErr   error
		startErr error
		wantErr  error
		methods  []string
	}{
		{
			name: "gio fails", paths: map[string]string{gioName: fakeGio, xdgName: fakeXdg},
			runErr: errors.New("gio rejected"), methods: []string{runMethod, startMethod},
		},
		{name: "gio missing", paths: map[string]string{xdgName: fakeXdg}, methods: []string{startMethod}},
		{name: "both missing", wantErr: ErrToolNotFound},
		{
			name: "both fail", paths: map[string]string{gioName: fakeGio, xdgName: fakeXdg},
			runErr: errors.New("gio rejected"), startErr: errors.New("xdg rejected"),
			wantErr: ErrLaunchFailed, methods: []string{runMethod, startMethod},
		},
		{
			name: "only gio fails", paths: map[string]string{gioName: fakeGio}, runErr: errors.New("gio rejected"),
			wantErr: ErrLaunchFailed, methods: []string{runMethod},
		},
		{
			name: "only xdg fails", paths: map[string]string{xdgName: fakeXdg},
			startErr: errors.New("xdg rejected"), wantErr: ErrLaunchFailed, methods: []string{startMethod},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{runErr: tt.runErr, startErr: tt.startErr}
			runner.onRun = func(ctx context.Context) {
				if _, ok := ctx.Deadline(); !ok {
					t.Error("no file manager timeout")
				}
			}
			err := New(runner, fakeResolver(tt.paths)).Open(t.Context(), TargetFileManager, dir, Preferences{})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v; want %v", err, tt.wantErr)
			}
			if err != nil && (!strings.Contains(err.Error(), gioName) || !strings.Contains(err.Error(), xdgName)) {
				t.Fatalf("missing attempt descriptions: %v", err)
			}
			if len(runner.calls) != len(tt.methods) {
				t.Fatalf("calls = %v", runner.calls)
			}
			for i, method := range tt.methods {
				expected := recordedCommand{
					method:  startMethod,
					command: process.Command{Path: fakeXdg, Args: []string{dir}, Dir: dir},
				}
				if method == runMethod {
					expected = recordedCommand{
						method:  runMethod,
						command: process.Command{Path: fakeGio, Args: []string{"open", dir}, Dir: dir},
					}
				}
				if !reflect.DeepEqual(runner.calls[i], expected) {
					t.Fatalf("call = %#v; want %#v", runner.calls[i], expected)
				}
			}
		})
	}
}

func TestOpenErrors(t *testing.T) {
	dir := projectDirectory(t)
	tests := []struct {
		name    string
		target  Target
		prefs   Preferences
		wantErr error
	}{
		{name: "unknown", target: Target("other"), wantErr: ErrUnknownTarget},
		{name: "terminal not configured", target: TargetTerminal, wantErr: ErrToolNotFound},
		{
			name: "blank terminal", target: TargetTerminal,
			prefs: Preferences{CustomTerminal: TerminalCommand{Executable: "  "}}, wantErr: ErrToolNotFound,
		},
		{
			name: "terminal nul arg", target: TargetTerminal, wantErr: ErrInvalidExecutable,
			prefs: Preferences{CustomTerminal: TerminalCommand{Executable: kittyName, Args: []string{"a\x00b"}}},
		},
		{
			name: "custom files not configured", target: TargetFileManager, wantErr: ErrToolNotFound,
			prefs: Preferences{FileManagerMode: FileManagerCustom},
		},
		{
			name: "invalid override", target: TargetZed, wantErr: ErrInvalidExecutable,
			prefs: Preferences{Paths: ToolPaths{Zed: "bin/zed"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &fakeRunner{}
			err := New(runner, allToolsResolver()).Open(t.Context(), tt.target, dir, tt.prefs)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("got %v; want %v", err, tt.wantErr)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("unexpected calls: %v", runner.calls)
			}
		})
	}
}

func TestOpenMissingTools(t *testing.T) {
	dir := projectDirectory(t)
	for _, target := range []Target{TargetSublime, TargetZed, TargetGhostty, TargetTerminal, TargetFileManager} {
		t.Run(string(target), func(t *testing.T) {
			runner := &fakeRunner{}
			prefs := Preferences{
				CustomTerminal:  TerminalCommand{Executable: "missing"},
				FileManagerMode: FileManagerCustom, Paths: ToolPaths{FileManager: "missing"},
			}
			err := New(runner, fakeResolver(nil)).Open(t.Context(), target, dir, prefs)
			if !errors.Is(err, ErrToolNotFound) || len(runner.calls) != 0 {
				t.Fatalf("got %v, calls %v", err, runner.calls)
			}
		})
	}
}

func TestOpenStartErrors(t *testing.T) {
	dir := projectDirectory(t)
	for _, target := range []Target{TargetSublime, TargetZed, TargetTerminal, TargetFileManager} {
		t.Run(string(target), func(t *testing.T) {
			cause := errors.New("cannot spawn")
			runner := &fakeRunner{startErr: cause}
			prefs := Preferences{
				CustomTerminal:  TerminalCommand{Executable: kittyName},
				FileManagerMode: FileManagerCustom, Paths: ToolPaths{FileManager: filesName},
			}
			err := New(runner, allToolsResolver()).Open(t.Context(), target, dir, prefs)
			if !errors.Is(err, ErrLaunchFailed) || !errors.Is(err, cause) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestOpenMissingProject(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "missing")
	runner := &fakeRunner{}
	err := New(runner, allToolsResolver()).Open(t.Context(), TargetZed, dir, Preferences{})
	if !errors.Is(err, ErrInvalidTargetPath) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected both path and os errors: %v", err)
	}
	if !strings.Contains(err.Error(), "no longer exists") || len(runner.calls) != 0 {
		t.Fatalf("got %v", err)
	}
}

func TestTools(t *testing.T) {
	prefs := Preferences{CustomTerminal: TerminalCommand{Executable: kittyName}}
	got := New(&fakeRunner{}, allToolsResolver()).Tools(prefs)
	expected := []ToolStatus{
		{Target: TargetSublime, Name: "Sublime Text", Available: true, Executable: fakeSublime},
		{Target: TargetZed, Name: "Zed", Available: true, Executable: fakeZed},
		{Target: TargetGhostty, Name: "Ghostty", Available: true, Executable: fakeGhostty},
		{Target: TargetTerminal, Name: "Terminal (kitty)", Available: true, Executable: fakeKitty},
		{Target: TargetFileManager, Name: "File Manager", Available: true, Executable: fakeGio},
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("Tools = %#v; want %#v", got, expected)
	}
}

func TestToolsUnavailable(t *testing.T) {
	got := New(&fakeRunner{}, fakeResolver(nil)).Tools(Preferences{})
	for _, tool := range got {
		if tool.Available || tool.Executable != "" || tool.Reason == "" {
			t.Errorf("invalid status: %+v", tool)
		}
		if tool.Target != TargetTerminal && !strings.HasSuffix(tool.Reason, "Set its path in Settings.") {
			t.Errorf("missing settings hint: %+v", tool)
		}
	}
	if got[3].Name != "Custom terminal" || got[3].Reason != "No custom terminal configured — set one in Settings." {
		t.Errorf("unconfigured terminal: %+v", got[3])
	}
	custom := New(&fakeRunner{}, allToolsResolver()).Tools(Preferences{FileManagerMode: FileManagerCustom})[4]
	if custom.Available || custom.Reason != "No file manager configured — set one in Settings." {
		t.Errorf("unconfigured file manager: %+v", custom)
	}
}

func TestToolsOverrides(t *testing.T) {
	tests := []struct {
		name   string
		prefs  Preferences
		index  int
		exe    string
		reason string
	}{
		{name: "custom files", prefs: Preferences{
			FileManagerMode: FileManagerCustom,
			Paths:           ToolPaths{FileManager: filesName},
		}, index: 4, exe: fakeFiles},
		{
			name: "invalid zed override", prefs: Preferences{Paths: ToolPaths{Zed: "./zed"}},
			index: 1, reason: "invalid",
		},
		{
			name: "missing sublime override", prefs: Preferences{Paths: ToolPaths{Sublime: missingName}},
			index: 0, reason: missingName,
		},
		{
			name: "missing terminal", prefs: Preferences{CustomTerminal: TerminalCommand{Executable: missingName}},
			index: 3, reason: missingName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := New(&fakeRunner{}, allToolsResolver()).Tools(tt.prefs)[tt.index]
			if got.Executable != tt.exe || got.Available != (tt.exe != "") || !strings.Contains(got.Reason, tt.reason) {
				t.Fatalf("unexpected status: %+v", got)
			}
		})
	}
}

func TestToolsFileManagerPreference(t *testing.T) {
	dir := t.TempDir()
	gio := filepath.Join(dir, gioName)
	writeExecutable(t, gio, 0o755)
	r := fakeResolver(map[string]string{xdgName: fakeXdg})
	r.SearchDirs = []string{dir}
	if got := New(&fakeRunner{}, r).Tools(Preferences{})[4]; got.Executable != gio {
		t.Fatalf("gio in install dir must precede xdg in PATH: %+v", got)
	}
	r.SearchDirs = nil
	if got := New(&fakeRunner{}, r).Tools(Preferences{FileManagerMode: FileManagerAuto})[4]; got.Executable != fakeXdg {
		t.Fatalf("missing gio must use xdg: %+v", got)
	}
}

func TestExpandArgs(t *testing.T) {
	args := []string{"--cwd={path}", PathPlaceholder, "{path}{path}", "no change"}
	original := slices.Clone(args)
	got := ExpandArgs(args, "a b;$ü")
	expected := []string{"--cwd=a b;$ü", "a b;$ü", "a b;$üa b;$ü", "no change"}
	if !slices.Equal(got, expected) || !slices.Equal(args, original) {
		t.Fatalf("got %v, original %v", got, args)
	}
	got[0] = "mutated"
	if !slices.Equal(args, original) {
		t.Fatal("ExpandArgs shares storage")
	}
}

func TestStderrSummary(t *testing.T) {
	if got := stderrSummary([]byte("  first line \nsecond line")); got != ": first line" {
		t.Fatalf("got %q", got)
	}
	if got := stderrSummary(nil); got != "" {
		t.Fatalf("got %q", got)
	}
	long := strings.Repeat("ü", 220)
	if got := stderrSummary([]byte(long)); got != ": "+strings.Repeat("ü", 200) {
		t.Fatalf("got %q", got)
	}
}

func TestTerminalPresets(t *testing.T) {
	tests := []struct {
		id   string
		name string
		args []string
	}{
		{id: "ptyxis", name: "Ptyxis", args: []string{"--new-window", "--working-directory={path}"}},
		{id: "gnome-terminal", name: "GNOME Terminal", args: []string{"--window", "--working-directory={path}"}},
		{id: kittyName, name: kittyName, args: []string{"--directory", PathPlaceholder}},
		{id: "alacritty", name: "Alacritty", args: []string{"--working-directory", PathPlaceholder}},
		{id: "wezterm", name: "WezTerm", args: []string{"start", "--cwd", PathPlaceholder}},
		{id: "konsole", name: "Konsole", args: []string{"--workdir", PathPlaceholder}},
	}
	got := TerminalPresets()
	if len(got) != len(tests) {
		t.Fatalf("got %d presets; want %d", len(got), len(tests))
	}
	for i, tt := range tests {
		preset := got[i]
		matchesIdentity := preset.ID == tt.id && preset.Name == tt.name && preset.Executable == tt.id
		if !matchesIdentity || !slices.Equal(preset.Args, tt.args) {
			t.Fatalf("preset %d = %+v; want %+v", i, preset, tt)
		}
		got[i].ID = "mutated"
		got[i].Args[0] = "mutated"
	}
	fresh := TerminalPresets()
	for i, tt := range tests {
		if fresh[i].ID != tt.id || !slices.Equal(fresh[i].Args, tt.args) {
			t.Fatal("presets share mutable storage")
		}
	}
}

func TestNew(t *testing.T) {
	l := New(nil, nil)
	hasDependencies := l.runner != nil && l.resolver != nil
	hasTimeouts := l.ghosttyTimeout == 4*time.Second && l.fileManagerTimeout == 5*time.Second
	if !hasDependencies || !hasTimeouts {
		t.Fatalf("unexpected defaults: %+v", l)
	}
}

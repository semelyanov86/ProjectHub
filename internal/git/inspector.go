package git

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/semelyanov86/ProjectHub/internal/process"
)

const (
	unavailableMessage = "Git is not installed or not in PATH."
	cancelledMessage   = "Git inspection cancelled."
)

// Inspector bounds concurrent read-only Git status commands.
type Inspector struct {
	runner   process.Runner
	lookPath func(string) (string, error)
	home     string
	timeout  time.Duration
	sem      chan struct{}
}

// NewInspector defaults to four concurrent commands and a three-second timeout.
// A nil lookPath uses exec.LookPath.
func NewInspector(
	runner process.Runner,
	lookPath func(string) (string, error),
	concurrency int,
	timeout time.Duration,
) *Inspector {
	if concurrency <= 0 {
		concurrency = 4
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return &Inspector{
		runner: runner, lookPath: lookPath, home: home,
		timeout: timeout, sem: make(chan struct{}, concurrency),
	}
}

// Inspect returns the Git working-tree state of dir.
func (i *Inspector) Inspect(ctx context.Context, dir string) Inspection {
	gitPath, err := i.lookPath("git")
	if err != nil {
		return Inspection{State: StateUnavailable, Message: unavailableMessage}
	}
	return i.inspect(ctx, gitPath, dir)
}

// InspectMany inspects directories concurrently and returns results in input order.
// All calls on an Inspector share the same process concurrency limit.
func (i *Inspector) InspectMany(ctx context.Context, dirs []string) []Inspection {
	results := make([]Inspection, len(dirs))
	if len(dirs) == 0 {
		return results
	}
	gitPath, err := i.lookPath("git")
	if err != nil {
		for n := range results {
			results[n] = Inspection{State: StateUnavailable, Message: unavailableMessage}
		}
		return results
	}
	var wg sync.WaitGroup
	for n, dir := range dirs {
		wg.Go(func() {
			results[n] = i.inspect(ctx, gitPath, dir)
		})
	}
	wg.Wait()
	return results
}

func (i *Inspector) inspect(ctx context.Context, gitPath, dir string) Inspection {
	if ctx.Err() != nil {
		return Inspection{State: StateError, Message: cancelledMessage}
	}
	select {
	case i.sem <- struct{}{}:
		defer func() { <-i.sem }()
	case <-ctx.Done():
		return Inspection{State: StateError, Message: cancelledMessage}
	}
	if ctx.Err() != nil {
		return Inspection{State: StateError, Message: cancelledMessage}
	}
	child, cancel := context.WithTimeout(ctx, i.timeout)
	defer cancel()
	env, filtersDisabled, failure, ok := i.statusEnv(ctx, child, gitPath, dir)
	if !ok {
		return failure
	}
	result, err := i.runner.Run(child, process.Command{
		Path: gitPath,
		Dir:  dir,
		Env:  env,
		Args: []string{
			"--no-optional-locks", "-c", "core.fsmonitor=false",
			"-c", "safe.bareRepository=explicit", "-c", "color.ui=false",
			"-C", dir, "status", "--porcelain=v2", "--branch",
			"--untracked-files=no", "--no-renames", "--ignore-submodules=all",
		},
	})
	if err != nil {
		return i.failedInspection(ctx, child, result, err)
	}
	repo, err := ParseStatus(result.Stdout)
	if err != nil {
		return Inspection{State: StateError, Message: "Could not parse git status output."}
	}
	repo.DirtyUncertain = repo.IsDirty && filtersDisabled
	return Inspection{State: StateRepo, Repo: repo}
}

// statusEnv builds the environment for `git status`. Besides the sanitised
// base environment it disables every filter driver defined by the repository
// itself: its config could otherwise make `git status` execute a clean/process
// filter command when it re-reads a modified tracked file. Reading config never
// runs repository commands. filtersDisabled reports whether any driver was
// disabled (the dirty state may then be a false positive); ok is false when the
// config lookup itself failed.
func (i *Inspector) statusEnv(
	parent, child context.Context,
	gitPath, dir string,
) (env []string, filtersDisabled bool, failure Inspection, ok bool) {
	env = inspectionEnv(i.home)
	result, err := i.runner.Run(child, process.Command{
		Path: gitPath,
		Dir:  dir,
		Env:  env,
		Args: []string{
			"--no-optional-locks", "-c", "safe.bareRepository=explicit",
			"-C", dir, "config", "--show-scope", "--name-only", "--get-regexp", filterConfigPattern,
		},
	})
	// Exit code 1 means no filter drivers are configured.
	if err != nil && result.ExitCode != 1 {
		return nil, false, i.failedInspection(parent, child, result, err), false
	}
	if result.Truncated {
		return nil, false, Inspection{State: StateError, Message: "Too many Git filter drivers configured."}, false
	}
	drivers := untrustedFilterDrivers(result.Stdout)
	return append(env, filterOverrideEnv(drivers)...), len(drivers) > 0, Inspection{}, true
}

func (i *Inspector) failedInspection(
	parent, child context.Context,
	result process.Result,
	err error,
) Inspection {
	if parent.Err() != nil {
		return Inspection{State: StateError, Message: cancelledMessage}
	}
	if errors.Is(child.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return Inspection{State: StateError, Message: fmt.Sprintf("git status timed out after %s", i.timeout)}
	}
	if result.ExitCode == 128 && strings.Contains(strings.ToLower(string(result.Stderr)), "not a git repository") {
		return Inspection{State: StateNotRepo}
	}
	message := firstMessageLine(string(result.Stderr))
	if message == "" {
		message = firstMessageLine(err.Error())
	}
	return Inspection{State: StateError, Message: message}
}

func inspectionEnv(home string) []string {
	base := process.SanitizedEnviron()
	env := make([]string, 0, len(base))
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if !strings.HasPrefix(name, "GIT_") {
			env = append(env, entry)
		}
	}
	// GIT_ALLOW_PROTOCOL="" forbids every transport, so a partial clone's lazy
	// fetch of a missing object can neither reach the network nor run
	// repository-configured transport commands (core.sshCommand, remote helpers);
	// GIT_NO_LAZY_FETCH disables lazy fetching outright on Git 2.44+.
	env = process.WithVariables(env, "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "LC_ALL=C",
		"GIT_ALLOW_PROTOCOL=", "GIT_NO_LAZY_FETCH=1")
	if home != "" {
		env = process.WithVariables(env, "GIT_CEILING_DIRECTORIES="+home)
	}
	return env
}

func firstMessageLine(message string) string {
	for line := range strings.SplitSeq(message, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			chars := []rune(trimmed)
			return string(chars[:min(len(chars), 200)])
		}
	}
	return ""
}

// Package process runs external programs for ProjectHub without a shell.
//
// It provides one neutral command description shared by the launcher and Git
// packages, a production Runner backed by os/exec, and environment sanitising
// so child desktop applications do not inherit ProjectHub's WebKit/Wails
// runtime configuration.
package process

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// DefaultOutputLimit caps captured stdout/stderr per stream for bounded runs.
const DefaultOutputLimit = 64 * 1024

// defaultWaitDelay bounds how long Wait blocks after the context is done or the
// process exits while a grandchild still holds an inherited output pipe.
const defaultWaitDelay = 500 * time.Millisecond

// ErrNotStarted indicates the command could not be started at all.
var ErrNotStarted = errors.New("process could not be started")

// Command describes one program invocation. Args never pass through a shell.
type Command struct {
	// Path is the executable path (absolute after resolution).
	Path string
	// Args are the discrete arguments, excluding the program name.
	Args []string
	// Dir is the working directory for the child process.
	Dir string
	// Env is the complete child environment; nil means SanitizedEnviron().
	Env []string
}

// Result holds the captured output of a bounded run.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	// Truncated reports that stdout or stderr exceeded the output limit.
	Truncated bool
}

// Runner executes external commands. Production code uses OSRunner; tests
// substitute fakes so no real editors, terminals, or Git processes start.
type Runner interface {
	// Run executes a short-lived command bounded by ctx and captures output.
	// A non-zero exit returns the Result (with ExitCode) and a non-nil error
	// wrapping *exec.ExitError. Context expiry returns an error wrapping ctx.Err().
	Run(ctx context.Context, cmd Command) (Result, error)
	// Start launches a long-lived program detached in its own session with
	// /dev/null stdio and reaps it in the background. It returns once the
	// process has started.
	Start(cmd Command) error
}

// OSRunner is the production Runner backed by os/exec.
type OSRunner struct {
	// OutputLimit caps captured bytes per stream; <= 0 uses DefaultOutputLimit.
	OutputLimit int
	// WaitDelay bounds pipe draining after exit or cancellation; <= 0 uses 500ms.
	WaitDelay time.Duration
}

// NewOSRunner returns an OSRunner with default limits.
func NewOSRunner() *OSRunner {
	return &OSRunner{}
}

// Run implements Runner.
func (r *OSRunner) Run(ctx context.Context, c Command) (Result, error) {
	limit := r.OutputLimit
	if limit <= 0 {
		limit = DefaultOutputLimit
	}
	stdout := &limitedBuffer{limit: limit}
	stderr := &limitedBuffer{limit: limit}

	//nolint:gosec // Path is a resolved executable and Args are discrete argv elements; no shell is involved.
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = environment(c.Env)
	cmd.Stdin = nil
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.WaitDelay = r.waitDelay()
	// Run the command in its own process group so cancellation also stops its
	// descendants (e.g. a Git filter driver that hangs) instead of orphaning them.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killProcessGroup(cmd) }

	err := cmd.Run()
	result := Result{
		Stdout:    stdout.Bytes(),
		Stderr:    stderr.Bytes(),
		ExitCode:  exitCode(cmd),
		Truncated: stdout.truncated || stderr.truncated,
	}
	waitDelayExpired := errors.Is(err, exec.ErrWaitDelay)
	if ctx.Err() != nil || waitDelayExpired {
		// Descendants may outlive the group leader (after a timeout, or while
		// holding the output pipes open); never leave them running.
		_ = killProcessGroup(cmd)
	}
	if err == nil || (waitDelayExpired && ctx.Err() == nil && result.ExitCode == 0) {
		return result, nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return result, fmt.Errorf("%s: %w", c.Path, ctxErr)
	}
	if _, ok := errors.AsType[*exec.ExitError](err); ok {
		return result, fmt.Errorf("%s exited with code %d: %w", c.Path, result.ExitCode, err)
	}
	return result, fmt.Errorf("%w: %s: %w", ErrNotStarted, c.Path, err)
}

// Start implements Runner.
func (r *OSRunner) Start(c Command) error {
	//nolint:gosec,noctx // Detached GUI child must outlive the request context; argv is discrete and validated.
	cmd := exec.Command(c.Path, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = environment(c.Env)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %s: %w", ErrNotStarted, c.Path, err)
	}
	// Reap the child so it never lingers as a zombie; Process.Release would not.
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

// killProcessGroup sends SIGKILL to the process group led by cmd's process.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
		if errors.Is(err, syscall.ESRCH) {
			return os.ErrProcessDone
		}
		return fmt.Errorf("kill process group %d: %w", cmd.Process.Pid, err)
	}
	return nil
}

func (r *OSRunner) waitDelay() time.Duration {
	if r.WaitDelay > 0 {
		return r.WaitDelay
	}
	return defaultWaitDelay
}

func environment(env []string) []string {
	if env != nil {
		return env
	}
	return SanitizedEnviron()
}

func exitCode(cmd *exec.Cmd) int {
	if cmd.ProcessState == nil {
		return -1
	}
	return cmd.ProcessState.ExitCode()
}

// limitedBuffer keeps at most limit bytes and records whether more arrived.
type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = b.truncated || len(p) > 0
		return len(p), nil
	}
	if len(p) > remaining {
		b.buf.Write(p[:remaining])
		b.truncated = true
		return len(p), nil
	}
	b.buf.Write(p)
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte {
	return append([]byte(nil), b.buf.Bytes()...)
}

// SanitizedEnviron returns os.Environ() filtered by SanitizeEnv.
func SanitizedEnviron() []string {
	return SanitizeEnv(os.Environ())
}

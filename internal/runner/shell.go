// Package runner implements the persistent-shell marker protocol for executing
// scenario steps inside the dapi-client container.
package runner

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
)

// envSetup is pre-exported into every shell on Open. These are the env vars
// available to all scenario steps. Changing them requires updating every
// scenario file in the same commit.
const envSetup = `export API=http://haproxy:5555/v3
export HAP=$API/services/haproxy
export AUTH=admin:haproxypwd
`

// Shell represents one persistent sh session inside the dapi-client container.
type Shell struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	stdout  *bufio.Reader
	cancel  context.CancelFunc
	counter atomic.Uint64
}

// Open starts a persistent shell in the dapi-client container and pre-exports
// the standard environment variables. The caller must call Close when done.
func Open(ctx context.Context) (*Shell, error) {
	ctx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(ctx, "podman", "exec", "-i", "dapi-client", "sh")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	// Discard stderr so it doesn't contaminate the output region.
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("starting shell in dapi-client: %w", err)
	}

	s := &Shell{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdoutPipe),
		cancel: cancel,
	}

	// Write env setup and drain via a marker so we know the shell is ready.
	if _, err := fmt.Fprint(s.stdin, envSetup); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("writing env setup: %w", err)
	}

	marker := s.nextMarker()
	if _, err := fmt.Fprintf(s.stdin, "echo %s$?\n", marker); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("writing init marker: %w", err)
	}

	if _, _, err := s.readUntilMarker(ctx, marker); err != nil {
		_ = s.Close()
		return nil, fmt.Errorf("waiting for shell init: %w", err)
	}

	return s, nil
}

// Run executes cmd in the persistent shell and returns stdout, exit code, and
// any transport error. The command string is written byte-identical to what
// the user sees in the TUI. If ctx is cancelled before the command completes,
// the shell process is killed to unblock the read and Run returns ctx.Err();
// the Shell must not be reused afterward.
func (s *Shell) Run(ctx context.Context, cmd string) (stdout string, exitCode int, err error) {
	marker := s.nextMarker()

	// Write: command, then our sentinel that prints the exit code.
	script := fmt.Sprintf("%s\necho %s$?\n", strings.TrimRight(cmd, "\n"), marker)
	if _, err := fmt.Fprint(s.stdin, script); err != nil {
		return "", -1, fmt.Errorf("writing command to shell: %w", err)
	}

	return s.readUntilMarker(ctx, marker)
}

// Close sends exit to the shell and waits for the process to terminate.
func (s *Shell) Close() error {
	_, _ = fmt.Fprintln(s.stdin, "exit")
	_ = s.stdin.Close()
	s.cancel()
	return s.cmd.Wait()
}

// nextMarker returns a unique sentinel string for this shell instance.
func (s *Shell) nextMarker() string {
	n := s.counter.Add(1)
	return fmt.Sprintf("__DAPI_DONE_%d__", n)
}

// readResult carries the outcome of the background read in readUntilMarker.
type readResult struct {
	out  string
	code int
	err  error
}

// readUntilMarker reads stdout lines until one starts with marker.
// It returns accumulated output (excluding the marker line) and the parsed
// exit code embedded in the marker line as __DAPI_DONE_N__<code>.
//
// The read happens in a goroutine so ctx cancellation can interrupt it: the
// underlying pipe read has no cancellation hook of its own, so on ctx.Done
// we kill the shell process to unblock it, then drain the goroutine's result
// before returning so it never leaks.
func (s *Shell) readUntilMarker(ctx context.Context, marker string) (string, int, error) {
	done := make(chan readResult, 1)

	go func() {
		var buf strings.Builder

		for {
			line, err := s.stdout.ReadString('\n')

			trimmed := strings.TrimRight(line, "\r\n")

			if strings.HasPrefix(trimmed, marker) {
				rest := strings.TrimPrefix(trimmed, marker)
				code, scanErr := strconv.Atoi(rest)
				if scanErr != nil {
					done <- readResult{buf.String(), -1, fmt.Errorf("parsing exit code from marker %s (got %q): %w", marker, rest, scanErr)}
					return
				}
				done <- readResult{buf.String(), code, nil}
				return
			}

			buf.WriteString(line)

			if err != nil {
				if err == io.EOF {
					done <- readResult{buf.String(), -1, fmt.Errorf("shell exited before marker %s", marker)}
					return
				}
				done <- readResult{buf.String(), -1, fmt.Errorf("reading shell output: %w", err)}
				return
			}
		}
	}()

	select {
	case r := <-done:
		return r.out, r.code, r.err
	case <-ctx.Done():
		s.cancel()
		<-done // drain so the goroutine never leaks
		return "", -1, fmt.Errorf("running command: %w", ctx.Err())
	}
}

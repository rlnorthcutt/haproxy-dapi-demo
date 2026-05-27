// Package logs provides live log tailing from Podman containers.
// It has no internal dependencies.
package logs

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
)

// Line is a single log line from a named container.
type Line struct {
	Container string
	Text      string
}

// Tail starts tailing logs for container, sending each new line to out.
// It reads the last tailLines historical lines before following new output.
// stdout and stderr are both captured (HAProxy logs to stdout; DPA may use
// either). The provided wg is decremented once when both scanners have exited,
// allowing the caller to know when no more lines will be sent.
//
// Tail returns immediately after launching background goroutines; use the
// returned error only for startup failures (e.g. subprocess launch errors).
// Callers must cancel ctx to stop the tail; the wg signals completion.
func Tail(ctx context.Context, container string, tailLines int, out chan<- Line, wg *sync.WaitGroup) error {
	cmd := exec.CommandContext(ctx, "podman", "logs", "-f",
		"--tail", fmt.Sprintf("%d", tailLines), container)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe for %s logs: %w", container, err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe for %s logs: %w", container, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting log tail for %s: %w", container, err)
	}

	// scan reads all lines from r and sends them to out until r closes or ctx
	// is cancelled.
	scan := func(r io.Reader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			select {
			case out <- Line{Container: container, Text: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
	}

	// Track when both pipes are fully drained.
	var innerWg sync.WaitGroup
	innerWg.Add(2)
	go func() { defer innerWg.Done(); scan(stdout) }()
	go func() { defer innerWg.Done(); scan(stderr) }()

	// Signal the Manager WaitGroup once both scanners are done.
	go func() {
		defer wg.Done()
		innerWg.Wait()
	}()

	go func() { _ = cmd.Wait() }()

	return nil
}

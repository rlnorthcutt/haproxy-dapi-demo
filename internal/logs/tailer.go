// Package logs provides live log tailing from Podman containers.
// It has no internal dependencies.
package logs

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
)

// Line is a single log line from a named container.
type Line struct {
	Container string
	Text      string
}

// Tail starts tailing logs for container, sending each new line to out.
// It reads the last tailLines historical lines before following new output.
// The tail stops when ctx is cancelled. Tail returns immediately after
// starting the background goroutines; use the returned error only for
// startup failures (e.g. subprocess launch errors).
//
// Callers must ensure ctx is cancelled to prevent goroutine leaks.
func Tail(ctx context.Context, container string, tailLines int, out chan<- Line) error {
	tail := fmt.Sprintf("%d", tailLines)

	cmd := exec.CommandContext(ctx, "podman", "logs", "-f",
		"--tail", tail, container)

	// HAProxy and DPA write to both stdout and stderr; merge them.
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe for %s logs: %w", container, err)
	}
	cmd.Stderr = cmd.Stdout // merge

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting log tail for %s: %w", container, err)
	}

	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case out <- Line{Container: container, Text: scanner.Text()}:
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

// Package compose wraps podman-compose operations and host-side HAProxy
// management (reset, health checks). It has no internal dependencies.
package compose

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Up brings up the compose stack with --wait (blocks until all healthy).
func Up(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "podman", "compose", "up", "-d", "--wait")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman compose up: %w", err)
	}
	return nil
}

// Down tears down the compose stack.
func Down(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "podman", "compose", "down")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("podman compose down: %w", err)
	}
	return nil
}

// Reset copies haproxy/baseline.cfg over haproxy/haproxy.cfg and sends
// SIGUSR2 to the haproxy container to trigger a live reload.
func Reset(ctx context.Context) error {
	baselinePath, err := findInWorkdir("haproxy/baseline.cfg")
	if err != nil {
		return fmt.Errorf("finding baseline.cfg: %w", err)
	}

	configPath := filepath.Join(filepath.Dir(baselinePath), "haproxy.cfg")

	if err := copyFile(baselinePath, configPath); err != nil {
		return fmt.Errorf("copying baseline.cfg to haproxy.cfg: %w", err)
	}

	// Reload via s6: send SIGUSR2 to the supervised haproxy service.
	// We must target the haproxy service specifically via s6-svc, not the
	// container PID 1 (which is s6 itself — USR2 to s6 stops everything).
	cmd := exec.CommandContext(ctx, "podman", "exec", "haproxy",
		"/package/admin/s6/command/s6-svc", "-2",
		"/run/s6-rc/servicedirs/haproxy",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("reloading haproxy via s6: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}

	return nil
}

// IsStackHealthy returns true if the DPA /info endpoint responds.
func IsStackHealthy(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx,
		"podman", "exec", "dapi-client",
		"curl", "-su", "admin:haproxypwd", "--max-time", "3",
		"http://haproxy:5555/v3/info",
	)
	out, err := cmd.Output()
	return err == nil && len(out) > 2
}

// CheckPodman verifies that podman is installed and the daemon is reachable.
// Returns a descriptive error suitable for display in the TUI error screen.
func CheckPodman(ctx context.Context) error {
	podmanPath, err := exec.LookPath("podman")
	if err != nil {
		return &PodmanError{
			Detected: "podman binary not found in PATH",
			Tried:    "exec.LookPath(\"podman\")",
			Got:      "not found",
			Hint:     "Install Podman: https://podman.io/getting-started/installation",
		}
	}

	cmd := exec.CommandContext(ctx, "podman", "info")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &PodmanError{
			Detected: "podman binary at " + podmanPath,
			Tried:    "podman info",
			Got:      strings.TrimSpace(string(out)),
			Hint:     "podman machine start",
		}
	}
	return nil
}

// PodmanError carries structured detail for the TUI error screen.
type PodmanError struct {
	Detected string
	Tried    string
	Got      string
	Hint     string
}

func (e *PodmanError) Error() string {
	return fmt.Sprintf("podman not running: %s", e.Got)
}

// copyFile copies src to dst, creating dst if needed.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// findInWorkdir finds a file relative to the current working directory.
func findInWorkdir(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		abs, err := filepath.Abs(name)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	return "", fmt.Errorf("file not found in working directory: %s", name)
}

package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Open an interactive shell in dapi-client",
	Long: `Drops into the dapi-client container with $API, $HAP, and $AUTH pre-set.
This is the same environment that scenarios run in.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		// Build the shell command that pre-exports the env vars.
		script := `export API=http://haproxy:5555/v3; export HAP=$API/services/haproxy; export AUTH=admin:haproxypwd; exec sh`
		cmd := exec.Command("podman", "exec", "-it", "dapi-client", "sh", "-c", script)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			// Exit code from the shell is not an error we report.
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == 0 {
					return nil
				}
				fmt.Fprintf(os.Stderr, "shell exited with code %d\n", exitErr.ExitCode())
				return nil
			}
			return fmt.Errorf("exec dapi-client: %w", err)
		}
		return nil
	},
}

package main

import (
	"context"
	"fmt"

	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/compose"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Restore baseline HAProxy config and reload",
	Long: `Copies haproxy/baseline.cfg over haproxy/haproxy.cfg and sends SIGUSR2 to
the haproxy container. This is intentionally destructive — there are no prompts
or backups. Use git to preserve changes you care about.`,
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()
		fmt.Println("Resetting to baseline config…")
		if err := compose.Reset(ctx); err != nil {
			return err
		}
		fmt.Println("Reset complete. HAProxy reloaded.")
		return nil
	},
}

package main

import (
	"context"
	"fmt"

	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/compose"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Tear down the compose stack",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()
		fmt.Println("Stopping compose stack…")
		if err := compose.Down(ctx); err != nil {
			return err
		}
		fmt.Println("Stack stopped.")
		return nil
	},
}

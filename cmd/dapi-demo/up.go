package main

import (
	"context"
	"fmt"

	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/compose"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Bring up the compose stack",
	Long:  "Starts HAProxy + DPA + backends + client container and waits until healthy.",
	RunE: func(_ *cobra.Command, _ []string) error {
		ctx := context.Background()
		fmt.Println("Starting compose stack…")
		if err := compose.Up(ctx); err != nil {
			return err
		}
		fmt.Println("Stack is up and healthy.")
		return nil
	},
}

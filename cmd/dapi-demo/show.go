package main

import (
	"fmt"
	"os"

	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Print scenario YAML",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		scenarios, err := loadScenarios()
		if err != nil {
			return err
		}

		s, ok := scenario.Find(scenarios, args[0])
		if !ok {
			return fmt.Errorf("scenario %q not found", args[0])
		}

		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		if err := enc.Encode(s); err != nil {
			return fmt.Errorf("encoding scenario: %w", err)
		}
		return enc.Close()
	},
}

package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available scenarios",
	RunE: func(_ *cobra.Command, _ []string) error {
		scenarios, err := loadScenarios()
		if err != nil {
			return err
		}

		fmt.Printf("%-30s  %-10s  %-6s  %s\n", "ID", "CATEGORY", "RELOAD", "TITLE")
		fmt.Println(strings.Repeat("─", 80))
		for _, s := range scenarios {
			reload := "no"
			if s.Reload {
				reload = "yes"
			}
			fmt.Printf("%-30s  %-10s  %-6s  %s\n", s.ID, s.Category, reload, s.Title)
		}
		fmt.Printf("\n%d scenarios total\n", len(scenarios))
		return nil
	},
}

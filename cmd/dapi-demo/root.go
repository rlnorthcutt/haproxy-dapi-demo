package main

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/compose"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/tui"
	"github.com/spf13/cobra"
)

// scenariosDir is the default location for scenario YAML files, relative to
// the working directory (which must be the repo root).
const scenariosDir = "scenarios"

var rootCmd = &cobra.Command{
	Use:   "dapi-demo",
	Short: "HAProxy Data Plane API demo runner",
	Long: `dapi-demo is a TUI for walking through HAProxy Data Plane API v3 demos.

Run without arguments to open the interactive TUI (starts the compose stack
if needed). Use subcommands for specific operations.`,
	RunE: runTUI,
}

func init() {
	rootCmd.AddCommand(
		upCmd,
		downCmd,
		resetCmd,
		listCmd,
		showCmd,
		runCmd,
		shellCmd,
		versionCmd,
	)
}

// runTUI is the default action: check podman, ensure stack is up, open TUI.
func runTUI(_ *cobra.Command, _ []string) error {
	ctx := context.Background()

	scenarios, err := loadScenarios()
	if err != nil {
		return err
	}

	var m tui.Model
	if err := compose.CheckPodman(ctx); err != nil {
		// Show the designed error screen rather than a raw stderr dump.
		m = tui.InitialErrorModel(err)
	} else {
		m = tui.InitialModel(scenarios, "")
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	fmt.Println("Stack is still running.  Run  dapi-demo down  when you're done.")
	return nil
}

// loadScenarios loads all scenarios from scenariosDir (+ extras subdir).
func loadScenarios() ([]scenario.Scenario, error) {
	main, err := scenario.LoadDir(scenariosDir)
	if err != nil {
		return nil, fmt.Errorf("loading scenarios: %w", err)
	}

	extras, _ := scenario.LoadDir(scenariosDir + "/extras")
	return append(main, extras...), nil
}

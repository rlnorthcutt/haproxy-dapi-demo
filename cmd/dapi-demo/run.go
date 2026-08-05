package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/runner"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/tui"
)

var autoMode bool

var runCmd = &cobra.Command{
	Use:   "run <id>",
	Short: "Run a scenario interactively (or headless with --auto)",
	Args:  cobra.ExactArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		scenarios, err := loadScenarios()
		if err != nil {
			return err
		}

		sc, ok := scenario.Find(scenarios, args[0])
		if !ok {
			return fmt.Errorf("scenario %q not found — try: dapi-demo list", args[0])
		}

		if autoMode {
			return runHeadless(sc, scenarios)
		}
		return runInteractive(sc, scenarios)
	},
}

func init() {
	runCmd.Flags().BoolVar(&autoMode, "auto", false, "Run headless (no TUI), exit when done")
}

// runHeadless runs all steps sequentially and prints results to stdout.
// Used for CI and self-serve testing.
func runHeadless(sc scenario.Scenario, all []scenario.Scenario) error {
	ctx := context.Background()

	fmt.Printf("=== %s ===\n", sc.Title)

	sh, err := runner.Open(ctx)
	if err != nil {
		return fmt.Errorf("opening shell: %w", err)
	}
	defer func() { _ = sh.Close() }()

	// Prereqs
	if len(sc.Prereqs) > 0 {
		fmt.Println("--- prereqs ---")
		if err := runner.RunPrereqs(ctx, sh, sc.Prereqs, all); err != nil {
			return fmt.Errorf("prereqs: %w", err)
		}
	}

	// Steps
	failed := false
	for i, step := range sc.Steps {
		fmt.Printf("\n[step %d/%d] %s\n", i+1, len(sc.Steps), step.Title)
		if step.Explain != "" {
			fmt.Printf("  %s\n", strings.TrimSpace(step.Explain))
		}
		fmt.Printf("  $ %s\n", strings.TrimSpace(step.Run))

		result := runner.Run(ctx, sh, step)
		if result.Output != "" {
			fmt.Println(result.Output)
		}
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "  ERROR: %v\n", result.Err)
			failed = true
			continue
		}
		if result.ExitCode != 0 {
			fmt.Fprintf(os.Stderr, "  exit %d\n", result.ExitCode)
			failed = true
		}

		// Small pause between steps so output is readable.
		time.Sleep(500 * time.Millisecond)
	}

	// Verify
	if len(sc.Verify) > 0 {
		fmt.Println("\n--- verify ---")
		results := runner.RunVerify(ctx, sh, sc.Verify)
		for _, vr := range results {
			if vr.Pass {
				fmt.Printf("  ✓ %s\n", vr.Run)
			} else {
				fmt.Fprintf(os.Stderr, "  ✗ %s\n    expected to contain: %q\n    got: %s\n",
					vr.Run, vr.Contains, vr.Output)
				failed = true
			}
		}
	}

	// Cleanup
	if len(sc.Cleanup) > 0 {
		fmt.Println("\n--- cleanup ---")
		if err := runner.RunCleanup(ctx, sh, sc.Cleanup, all); err != nil {
			fmt.Fprintf(os.Stderr, "  cleanup error: %v\n", err)
		}
	}

	if failed {
		return fmt.Errorf("scenario %s had failures", sc.ID)
	}
	fmt.Printf("\n=== %s PASSED ===\n", sc.ID)
	return nil
}

// runInteractive launches the full TUI with the given scenario pre-selected.
func runInteractive(sc scenario.Scenario, all []scenario.Scenario) error {
	m := tui.InitialModel(all, sc.ID)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}
	fmt.Println("Stack is still running.  Run  dapi-demo down  when you're done.")
	return nil
}

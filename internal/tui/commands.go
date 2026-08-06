package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/compose"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/logs"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/runner"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
)

// ── message types ─────────────────────────────────────────────────────────────

// stackReadyMsg signals that the compose stack is healthy.
type stackReadyMsg struct{}

// stackErrorMsg signals that the stack could not be started.
type stackErrorMsg struct{ err error }

// shellOpenedMsg carries the newly opened shell.
type shellOpenedMsg struct{ shell *runner.Shell }

// shellErrorMsg signals that opening a shell failed.
type shellErrorMsg struct{ err error }

// stepFinishedMsg carries the result of running one step (stay on step).
type stepFinishedMsg struct {
	stepIdx int
	result  StepResult
}

// verboseFinishedMsg carries the result of a verbose re-run.
type verboseFinishedMsg struct {
	title  string
	result runner.StepResult
}

// resetDoneMsg signals that a reset completed.
type resetDoneMsg struct{ err error }

// logLineMsg delivers one log line from a tailed container.
type logLineMsg struct {
	container string
	text      string
}

// logManagerReadyMsg delivers a newly created log manager.
type logManagerReadyMsg struct{ mgr *logs.Manager }

// ── helpers ───────────────────────────────────────────────────────────────────

// toStepResult converts a runner.StepResult to a tui.StepResult, deriving
// Status from whether the step errored or returned a non-zero exit code.
func toStepResult(r runner.StepResult) StepResult {
	status := StatusDone
	if r.Err != nil || r.ExitCode != 0 {
		status = StatusError
	}
	return StepResult{
		Status:   status,
		Output:   r.Output,
		ExitCode: r.ExitCode,
		Err:      r.Err,
	}
}

// ── commands ──────────────────────────────────────────────────────────────────

// startStackCmd brings the compose stack up (`podman compose up --wait`,
// creating containers if needed) and then polls the Data Plane API through
// dapi-client until it responds or times out. The extra poll matters because
// dapi-client has no healthcheck of its own — compose --wait only confirms
// the container is running, not that curl is installed yet.
func startStackCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		if err := compose.UpQuiet(ctx); err != nil {
			return stackErrorMsg{err: fmt.Errorf("starting stack: %w", err)}
		}
		return waitForStack(ctx)()
	}
}

// waitForStack polls the compose stack health until it responds or times out.
// Assumes the stack has already been started (see startStackCmd); it does
// not create containers itself.
func waitForStack(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			select {
			case <-ctx.Done():
				return stackErrorMsg{err: ctx.Err()}
			default:
			}
			if compose.IsStackHealthy(ctx) {
				return stackReadyMsg{}
			}
			time.Sleep(2 * time.Second)
		}
		return stackErrorMsg{err: fmt.Errorf("stack did not become healthy within 90 s — try: dapi-demo up")}
	}
}

// startLogsCmd launches log tailers and returns a logManagerReadyMsg.
func startLogsCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		// Both HAProxy and Data Plane API log to the same container ("haproxy").
		// Lines are routed to separate buffers in appendLog by content.
		mgr, err := logs.NewManager(ctx, []string{"haproxy"})
		if err != nil {
			// Non-fatal: log tailing is supplementary to the demo.
			return nil
		}
		return logManagerReadyMsg{mgr: mgr}
	}
}

// listenLogsCmd blocks until one line arrives on ch, then delivers it.
// Update reschedules this after each logLineMsg to keep the stream running.
func listenLogsCmd(ch <-chan logs.Line) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return nil
		}
		return logLineMsg{container: line.Container, text: line.Text}
	}
}

// openShellCmd opens a persistent shell in the dapi-client container.
func openShellCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		sh, err := runner.Open(ctx)
		if err != nil {
			return shellErrorMsg{err: err}
		}
		return shellOpenedMsg{shell: sh}
	}
}

// closeShellCmd closes sh in the background so Update never blocks on the
// subprocess Wait() inside Shell.Close.
func closeShellCmd(sh *runner.Shell) tea.Cmd {
	return func() tea.Msg {
		_ = sh.Close()
		return nil
	}
}

// runStepCmd executes one step and returns stepFinishedMsg (stay on step).
func runStepCmd(ctx context.Context, sh *runner.Shell, step scenario.Step, idx int) tea.Cmd {
	return func() tea.Msg {
		return stepFinishedMsg{stepIdx: idx, result: toStepResult(runner.Run(ctx, sh, step))}
	}
}

// runVerboseCmd re-runs a step with curl -v and returns verboseFinishedMsg.
func runVerboseCmd(ctx context.Context, sh *runner.Shell, step scenario.Step) tea.Cmd {
	return func() tea.Msg {
		r := runner.RunVerbose(ctx, sh, step)
		return verboseFinishedMsg{title: step.Title, result: r}
	}
}

// resetCmd performs a stack reset and returns resetDoneMsg.
func resetCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		return resetDoneMsg{err: compose.Reset(ctx)}
	}
}

package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/runner"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
)

// stepFinishedAdvanceMsg is returned by runStepAndAdvanceCmd (space key).
// It carries the same payload as stepFinishedMsg but signals Update to also
// advance the step pointer.
type stepFinishedAdvanceMsg struct {
	stepIdx int
	result  StepResult
}

// Update is the Bubble Tea update function. Pure — no IO.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── System ────────────────────────────────────────────────────────────

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	// ── Spinner ───────────────────────────────────────────────────────────

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// ── Stack lifecycle ───────────────────────────────────────────────────

	case stackReadyMsg:
		m.stackReady = true
		m.mode = ModePicker
		ctx := context.Background()
		return m, startLogsCmd(ctx)

	case stackErrorMsg:
		m.mode = ModeError
		m.errTitle = "Cannot start"
		m.errBody = "The compose stack failed to become healthy."
		m.errGot = msg.err.Error()
		m.errHint = "dapi-demo up"
		return m, nil

	// ── Log manager ───────────────────────────────────────────────────────

	case logManagerReadyMsg:
		m.logMgr = msg.mgr
		m.logChan = msg.mgr.Lines
		return m, listenLogsCmd(m.logChan)

	case logLineMsg:
		m.appendLog(msg.container, msg.text)
		return m, listenLogsCmd(m.logChan)

	// ── Shell ─────────────────────────────────────────────────────────────

	case shellOpenedMsg:
		m.shell = msg.shell
		return m, nil

	case shellErrorMsg:
		m.mode = ModeError
		m.errTitle = "Shell error"
		m.errBody = "Could not open shell in dapi-client."
		m.errGot = msg.err.Error()
		m.errHint = "dapi-demo up"
		return m, nil

	// ── Step execution ────────────────────────────────────────────────────

	case stepFinishedMsg:
		m.running = false
		if msg.stepIdx < len(m.results) {
			m.results[msg.stepIdx] = msg.result
		}
		return m, m.spinner.Tick

	case stepFinishedAdvanceMsg:
		// space key: run + advance.
		m.running = false
		if msg.stepIdx < len(m.results) {
			m.results[msg.stepIdx] = msg.result
		}
		// Advance only if the step didn't error.
		if msg.result.Status != StatusError && msg.stepIdx < len(m.results)-1 {
			m.stepIdx = msg.stepIdx + 1
		}
		return m, m.spinner.Tick

	// ── Verbose overlay ───────────────────────────────────────────────────

	case verboseFinishedMsg:
		m.running = false
		m.verboseTitle = msg.title
		m.verboseLines = splitLines(msg.result.Output)
		m.verboseScroll = 0
		m.mode = ModeVerbose
		return m, nil

	// ── Reset ─────────────────────────────────────────────────────────────

	case resetDoneMsg:
		m.running = false
		if msg.err != nil {
			m.mode = ModeError
			m.errTitle = "Reset failed"
			m.errBody = "Could not restore baseline config."
			m.errGot = msg.err.Error()
			m.errHint = "podman kill -s USR2 haproxy"
		}
		return m, nil
	}

	return m, nil
}

// handleKey dispatches key events by mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case ModeError:
		return m.handleKeyError(msg)
	case ModeSplash:
		return m.handleKeySplash(msg)
	case ModePicker:
		return m.handleKeyPicker(msg)
	case ModeScenario:
		return m.handleKeyScenario(msg)
	case ModeVerbose:
		return m.handleKeyVerbose(msg)
	}
	return m, nil
}

func (m Model) handleKeyError(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r":
		m.mode = ModeSplash
		ctx := context.Background()
		return m, tea.Batch(m.spinner.Tick, waitForStack(ctx))
	case "q", "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleKeySplash(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "q" || msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleKeyPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.pickerIdx > 0 {
			m.pickerIdx--
		}
	case "down", "j":
		if m.pickerIdx < len(m.scenarios)-1 {
			m.pickerIdx++
		}
	case "enter", " ":
		if len(m.scenarios) == 0 {
			return m, nil
		}
		m.scenarioIdx = m.pickerIdx
		m.mode = ModeScenario
		m.initResults()
		if m.shell != nil {
			_ = m.shell.Close()
			m.shell = nil
		}
		ctx := context.Background()
		return m, tea.Batch(m.spinner.Tick, openShellCmd(ctx))
	}
	return m, nil
}

func (m Model) handleKeyScenario(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.running {
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	}

	steps := m.current().Steps
	ctx := context.Background()

	switch msg.String() {

	case "q", "ctrl+c":
		return m, tea.Quit

	case "n":
		if m.stepIdx < len(steps)-1 {
			m.stepIdx++
		}

	case "p":
		if m.stepIdx > 0 {
			m.stepIdx--
		}

	case " ":
		// Run current step, then advance.
		if m.shell == nil {
			return m, nil
		}
		m.running = true
		m.results[m.stepIdx] = StepResult{Status: StatusRunning}
		step := steps[m.stepIdx]
		idx := m.stepIdx
		return m, tea.Batch(m.spinner.Tick, runStepAndAdvanceCmd(ctx, m.shell, step, idx))

	case "r":
		// Run current step, stay on it.
		if m.shell == nil {
			return m, nil
		}
		m.running = true
		m.results[m.stepIdx] = StepResult{Status: StatusRunning}
		step := steps[m.stepIdx]
		idx := m.stepIdx
		return m, tea.Batch(m.spinner.Tick, runStepCmd(ctx, m.shell, step, idx))

	case "v":
		// Verbose re-run. Only if step has been executed at least once.
		if m.shell == nil || m.results[m.stepIdx].Status == StatusPending {
			return m, nil
		}
		m.running = true
		step := steps[m.stepIdx]
		return m, tea.Batch(m.spinner.Tick, runVerboseCmd(ctx, m.shell, step))

	case "R":
		m.running = true
		return m, tea.Batch(m.spinner.Tick, resetCmd(ctx))

	case "l":
		m.mode = ModePicker
		m.pickerIdx = m.scenarioIdx

	case "j":
		m.logScroll++

	case "k":
		if m.logScroll > 0 {
			m.logScroll--
		}
	}

	return m, nil
}

func (m Model) handleKeyVerbose(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c", "esc", "v":
		m.mode = ModeScenario
	case "j", "down":
		m.verboseScroll++
	case "k", "up":
		if m.verboseScroll > 0 {
			m.verboseScroll--
		}
	}
	return m, nil
}

// runStepAndAdvanceCmd runs a step and returns stepFinishedAdvanceMsg so
// Update knows to also advance the step pointer.
func runStepAndAdvanceCmd(ctx context.Context, sh *runner.Shell, step scenario.Step, idx int) tea.Cmd {
	return func() tea.Msg {
		r := runner.Run(ctx, sh, step)
		status := StatusDone
		if r.Err != nil || r.ExitCode != 0 {
			status = StatusError
		}
		return stepFinishedAdvanceMsg{
			stepIdx: idx,
			result: StepResult{
				Status:   status,
				Output:   r.Output,
				ExitCode: r.ExitCode,
				Err:      r.Err,
			},
		}
	}
}

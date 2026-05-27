package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

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

	case tea.MouseMsg:
		return m.handleMouse(msg)

	// ── Spinner ───────────────────────────────────────────────────────────

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	// ── Stack lifecycle ───────────────────────────────────────────────────

	case stackReadyMsg:
		// Only advance to picker from splash; don't clobber an already-selected scenario.
		if m.mode == ModeSplash {
			m.mode = ModePicker
		}
		ctx := context.Background()
		return m, startLogsCmd(ctx)

	case stackErrorMsg:
		m.mode = ModeError
		m.errState = errorState{
			title: "Cannot start",
			body:  "The compose stack failed to become healthy.",
			got:   msg.err.Error(),
			hint:  "dapi-demo up",
		}
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
		m.errState = errorState{
			title: "Shell error",
			body:  "Could not open shell in dapi-client.",
			got:   msg.err.Error(),
			hint:  "dapi-demo up",
		}
		return m, nil

	// ── Step execution ────────────────────────────────────────────────────

	case stepFinishedMsg:
		m.running = false
		if msg.stepIdx < len(m.results) {
			m.results[msg.stepIdx] = msg.result
		}
		m.outputScroll = 0 // reset to tail on new output
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
			m.errState = errorState{
				title: "Reset failed",
				body:  "Could not restore baseline config.",
				got:   msg.err.Error(),
				hint:  "podman kill -s USR2 haproxy",
			}
		}
		return m, nil
	}

	return m, nil
}

// quit stops background workers and signals Bubble Tea to exit.
func (m Model) quit() (tea.Model, tea.Cmd) {
	if m.logMgr != nil {
		m.logMgr.Stop()
	}
	return m, tea.Quit
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
		return m.quit()
	}
	return m, nil
}

func (m Model) handleKeySplash(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "q" || msg.String() == "ctrl+c" {
		return m.quit()
	}
	return m, nil
}

func (m Model) handleKeyPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m.quit()
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
			return m.quit()
		}
		return m, nil
	}

	steps := m.current().Steps
	ctx := context.Background()

	switch msg.String() {

	case "q", "ctrl+c":
		return m.quit()

	case "n":
		if m.stepIdx < len(steps)-1 {
			m.stepIdx++
			m.outputScroll = 0
		} else {
			// Last step — return to picker so the presenter can choose what's next.
			m.mode = ModePicker
			m.pickerIdx = m.scenarioIdx
		}

	case "p":
		if m.stepIdx > 0 {
			m.stepIdx--
			m.outputScroll = 0
		}

	case " ":
		// Run current step, stay to show output. Press n to advance.
		if m.shell == nil {
			return m, nil
		}
		m.running = true
		m.results[m.stepIdx] = StepResult{Status: StatusRunning}
		step := steps[m.stepIdx]
		idx := m.stepIdx
		return m, tea.Batch(m.spinner.Tick, runStepCmd(ctx, m.shell, step, idx))

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
	case "q", "ctrl+c":
		return m.quit()
	case "esc", "v":
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

// handleMouse dispatches mouse wheel events to the appropriate scroll offset.
// Left pane (X < leftW): controls outputScroll. Right pane: controls logScroll.
// Verbose overlay: controls verboseScroll.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Action != tea.MouseActionPress {
		return m, nil
	}

	if m.mode == ModeVerbose {
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m.verboseScroll++
		case tea.MouseButtonWheelDown:
			if m.verboseScroll > 0 {
				m.verboseScroll--
			}
		}
		return m, nil
	}

	if m.mode != ModeScenario {
		return m, nil
	}

	leftW := (m.width * 6) / 10
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		if msg.X < leftW {
			m.outputScroll++
		} else {
			m.logScroll++
		}
	case tea.MouseButtonWheelDown:
		if msg.X < leftW {
			if m.outputScroll > 0 {
				m.outputScroll--
			}
		} else {
			if m.logScroll > 0 {
				m.logScroll--
			}
		}
	}
	return m, nil
}

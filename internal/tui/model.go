package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/logs"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/runner"
	"github.com/rlnorthcutt/haproxy-dapi-demo/internal/scenario"
)

// Mode is the current top-level display mode of the TUI.
type Mode int

const (
	ModeError    Mode = iota // Podman not running, stack not up, etc.
	ModeSplash               // Bringing the stack up (boot screen)
	ModePicker               // Scenario selection list
	ModeScenario             // Main step-through view
	ModeVerbose              // Verbose curl overlay
)

// StepStatus tracks execution state for one step.
type StepStatus int

const (
	StatusPending StepStatus = iota
	StatusRunning
	StatusDone
	StatusError
)

// StepResult holds the outcome of running one scenario step.
type StepResult struct {
	Status   StepStatus
	Output   string
	ExitCode int
	Err      error
}

// logBuffer holds per-container log lines, capped at logs.MaxBuffered.
type logBuffer struct {
	lines []string
	total int // how many lines ever received (for "N more above" display)
}

func (b *logBuffer) append(line string) {
	b.total++
	b.lines = append(b.lines, line)
	if len(b.lines) > logs.MaxBuffered {
		b.lines = b.lines[len(b.lines)-logs.MaxBuffered:]
	}
}

// Model is the Bubble Tea model for the dapi-demo TUI.
type Model struct {
	// Terminal dimensions — populated by tea.WindowSizeMsg.
	width  int
	height int

	// Current display mode.
	mode Mode

	// All loaded scenarios.
	scenarios []scenario.Scenario

	// Currently active scenario index (into scenarios).
	scenarioIdx int

	// Current step index (0-based).
	stepIdx int

	// Per-step results, len == len(currentScenario().Steps).
	results []StepResult

	// Log buffers keyed by container name.
	haproxyLog logBuffer
	dapiLog    logBuffer

	// Log scroll offset (j/k keys). 0 = tail (newest at bottom).
	logScroll int

	// Active log manager (non-nil when a scenario is running).
	logMgr *logs.Manager

	// Channel we read log lines from; set when logMgr is active.
	logChan <-chan logs.Line

	// Active shell (non-nil during a scenario run).
	shell *runner.Shell

	// Spinner for the running-step indicator.
	spinner spinner.Model

	// Whether the current step is being executed.
	running bool

	// Picker cursor.
	pickerIdx int

	// Error screen fields.
	errTitle    string
	errBody     string
	errDetected string
	errTried    string
	errGot      string
	errHint     string

	// Verbose overlay content.
	verboseTitle string
	verboseLines []string
	verboseScroll int

	// Whether the stack is confirmed healthy (set by healthCheckCmd).
	stackReady bool
}

// InitialModel returns a Model in splash/startup state.
func InitialModel(scenarios []scenario.Scenario, startID string) Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = styleStepIconActive

	m := Model{
		scenarios: scenarios,
		spinner:   sp,
		mode:      ModeSplash,
	}

	// If a specific scenario was requested, find and pre-select it.
	if startID != "" {
		for i, s := range scenarios {
			if s.ID == startID {
				m.scenarioIdx = i
				m.mode = ModeScenario
				m.initResults()
				break
			}
		}
	}

	return m
}

// current returns the currently selected scenario. Panics if scenarios is empty
// (caller must guard).
func (m Model) current() scenario.Scenario {
	return m.scenarios[m.scenarioIdx]
}

// initResults resets step results for the current scenario.
func (m *Model) initResults() {
	steps := m.current().Steps
	m.results = make([]StepResult, len(steps))
	m.stepIdx = 0
}

// Init implements tea.Model. It kicks off the spinner and the stack health check.
func (m Model) Init() tea.Cmd {
	ctx := context.Background()
	return tea.Batch(
		m.spinner.Tick,
		waitForStack(ctx),
	)
}

// appendLog appends a line to the appropriate container log buffer.
func (m *Model) appendLog(container, text string) {
	switch container {
	case "haproxy":
		m.haproxyLog.append(text)
	case "dataplaneapi":
		m.dapiLog.append(text)
	}
}

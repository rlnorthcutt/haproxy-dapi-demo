// Package tui implements the Bubble Tea TUI for dapi-demo.
// All colors, borders, and spacing are defined here. No magic numbers or
// hex codes appear anywhere else in the tui package.
package tui

import "github.com/charmbracelet/lipgloss"

// ── palette ──────────────────────────────────────────────────────────────────

var (
	colorBlue   = lipgloss.Color("#4A9EFF")
	colorGreen  = lipgloss.Color("#50FA7B")
	colorAmber  = lipgloss.Color("#FFB347")
	colorPurple = lipgloss.Color("#BD93F9")
	colorRed    = lipgloss.Color("#FF5555")
	colorCyan   = lipgloss.Color("#8BE9FD")
	colorWhite  = lipgloss.Color("#F8F8F2")
	colorDim    = lipgloss.Color("#6272A4")
	colorSubtle = lipgloss.Color("#44475A")
	colorBg     = lipgloss.Color("#282A36")
)

// categoryColor returns the accent color for a scenario category.
func categoryColor(cat string) lipgloss.Color {
	switch cat {
	case "runtime":
		return colorGreen
	case "storage":
		return colorAmber
	case "client":
		return colorPurple
	default: // "config"
		return colorBlue
	}
}

// ── header ───────────────────────────────────────────────────────────────────

var (
	styleHeaderDemoLabel = lipgloss.NewStyle().Foreground(colorDim)
	styleHeaderDemoNum   = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	styleHeaderTitle     = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)
	styleHeaderStep      = lipgloss.NewStyle().Foreground(colorDim)
)

// badgeStyle returns a styled badge for the given category.
func badgeStyle(cat string) lipgloss.Style {
	col := categoryColor(cat)
	return lipgloss.NewStyle().
		Background(col).
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Padding(0, 1)
}

// ── left pane ────────────────────────────────────────────────────────────────

var (
	styleStepsHeader = lipgloss.NewStyle().
				Foreground(colorDim).
				Bold(true)

	styleStepNumDone    = lipgloss.NewStyle().Foreground(colorDim)
	styleStepIconDone   = lipgloss.NewStyle().Foreground(colorGreen)
	styleStepNumActive  = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleStepIconActive = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleStepNumError   = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleStepIconError  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleStepNumPending = lipgloss.NewStyle().Foreground(colorDim)
	styleStepIconPend   = lipgloss.NewStyle().Foreground(colorDim)
	styleStepTitleDone  = lipgloss.NewStyle().Foreground(colorDim)
	styleStepTitleAct   = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleStepTitlePend  = lipgloss.NewStyle().Foreground(colorSubtle)

	styleCurrentStepLabel = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleCurrentStepTitle = lipgloss.NewStyle().Foreground(colorWhite).Bold(true)

	styleExplain = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))

	stylePrompt  = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	styleCommand = lipgloss.NewStyle().Foreground(colorWhite)
	styleCmdCont = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))

	styleDivider  = lipgloss.NewStyle().Foreground(colorSubtle)
	styleOutput   = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	styleNotYet   = lipgloss.NewStyle().Foreground(colorDim).Italic(true)
	styleExitOK   = lipgloss.NewStyle().Foreground(colorGreen)
	styleExitFail = lipgloss.NewStyle().Foreground(colorRed)
)

// ── right pane (logs) ────────────────────────────────────────────────────────

var (
	styleLogLabel  = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleLogMeta   = lipgloss.NewStyle().Foreground(colorDim)
	styleLogNotice = lipgloss.NewStyle().Foreground(colorBlue)
	styleLogWarn   = lipgloss.NewStyle().Foreground(colorAmber)
	styleLogErr    = lipgloss.NewStyle().Foreground(colorRed)
	styleLogPlain  = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	styleLogInfo   = lipgloss.NewStyle().Foreground(colorCyan)
)

// ── footer ───────────────────────────────────────────────────────────────────

var (
	styleFooterKey  = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	styleFooterDesc = lipgloss.NewStyle().Foreground(colorDim)
	styleFooterSep  = lipgloss.NewStyle().Foreground(colorSubtle)
)

// ── picker ───────────────────────────────────────────────────────────────────

var (
	stylePickerHeader   = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
	stylePickerSelected = lipgloss.NewStyle().
				Background(colorSubtle).
				Foreground(colorWhite).
				Bold(true)
	stylePickerNormal = lipgloss.NewStyle().Foreground(lipgloss.Color("#CCCCCC"))
	stylePickerID     = lipgloss.NewStyle().Foreground(colorDim)
	stylePickerHint   = lipgloss.NewStyle().Foreground(colorDim).Italic(true)
)

// ── verbose overlay ──────────────────────────────────────────────────────────

var (
	styleVerboseTitle  = lipgloss.NewStyle().Foreground(colorAmber).Bold(true)
	styleVerboseBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorAmber).
				Padding(0, 1)
	styleVerboseCmd  = lipgloss.NewStyle().Foreground(colorWhite)
	styleVerboseOut  = lipgloss.NewStyle().Foreground(lipgloss.Color("#AAAAAA"))
	styleVerboseSend = lipgloss.NewStyle().Foreground(colorCyan)
	styleVerboseRecv = lipgloss.NewStyle().Foreground(colorGreen)
	styleVerboseMeta = lipgloss.NewStyle().Foreground(colorAmber)
)

// ── error screen ─────────────────────────────────────────────────────────────

var (
	styleErrorBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorRed).
				Padding(1, 2)
	styleErrorTitle  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	styleErrorBody   = lipgloss.NewStyle().Foreground(colorWhite)
	styleErrorLabel  = lipgloss.NewStyle().Foreground(colorDim)
	styleErrorValue  = lipgloss.NewStyle().Foreground(colorWhite)
	styleErrorHint   = lipgloss.NewStyle().Foreground(colorAmber)
	styleErrorButton = lipgloss.NewStyle().Foreground(colorCyan).Bold(true)
)

// ── pane borders/dividers ────────────────────────────────────────────────────

var (
	// Divider between header row and content area.
	styleHRule = lipgloss.NewStyle().Foreground(colorSubtle)

	// Vertical divider between left and right panes.
	stylePaneDivider = lipgloss.NewStyle().Foreground(colorSubtle)
)

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the full TUI frame. Pure — same model → same output.
func (m Model) View() string {
	if m.width == 0 {
		return ""
	}

	switch m.mode {
	case ModeError:
		return m.viewError()
	case ModeSplash:
		return m.viewSplash()
	case ModePicker:
		return m.viewPicker()
	case ModeScenario:
		return m.viewScenario()
	case ModeVerbose:
		return m.viewVerboseOverlay()
	}
	return ""
}

// ── helper ────────────────────────────────────────────────────────────────────

// hRule returns a horizontal rule of the given width using box-drawing chars.
func hRule(width int, style lipgloss.Style) string {
	if width <= 0 {
		return ""
	}
	return style.Render(strings.Repeat("─", width))
}

// padLines pads lines to exactly n entries (appending empty strings) and
// then trims to n entries, ensuring exact height.
func padLines(lines []string, n int) []string {
	for len(lines) < n {
		lines = append(lines, "")
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

// trunc truncates s to maxLen runes, appending … if needed.
func trunc(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-1]) + "…"
}

// splitLines splits s on newlines, trimming trailing empty lines.
func splitLines(s string) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines
}

// wordWrap wraps text at maxWidth, returning individual lines.
func wordWrap(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return []string{text}
	}
	var result []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		line := ""
		for _, w := range words {
			if line == "" {
				line = w
			} else if len(line)+1+len(w) <= maxWidth {
				line += " " + w
			} else {
				result = append(result, line)
				line = w
			}
		}
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// colorLog applies severity-based color to a log line.
func colorLog(line string) string {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "[WARNING]") || strings.Contains(line, "level=warning"):
		return styleLogWarn.Render(line)
	case strings.Contains(upper, "[ERROR]") || strings.Contains(upper, "[ALERT]") ||
		strings.Contains(upper, "[CRIT]") || strings.Contains(upper, "[EMERG]") ||
		strings.Contains(line, "level=error"):
		return styleLogErr.Render(line)
	case strings.Contains(upper, "[NOTICE]") || strings.Contains(line, "level=info"):
		return styleLogNotice.Render(line)
	default:
		return styleLogPlain.Render(line)
	}
}

// ── splash ────────────────────────────────────────────────────────────────────

func (m Model) viewSplash() string {
	lines := make([]string, m.height)
	mid := m.height / 2
	lines[mid-1] = lipgloss.PlaceHorizontal(m.width, lipgloss.Center,
		styleHeaderTitle.Render("dapi-demo"))
	lines[mid] = lipgloss.PlaceHorizontal(m.width, lipgloss.Center,
		styleLogMeta.Render(m.spinner.View()+" Starting compose stack…"))
	lines[mid+1] = lipgloss.PlaceHorizontal(m.width, lipgloss.Center,
		styleFooterDesc.Render("q quit"))
	return strings.Join(lines, "\n")
}

// ── error screen ──────────────────────────────────────────────────────────────

func (m Model) viewError() string {
	boxW := min(80, m.width-4)

	var b strings.Builder
	b.WriteString(styleErrorTitle.Render("✗  " + m.errTitle))
	b.WriteString("\n\n")
	b.WriteString(styleErrorBody.Render(m.errBody))
	b.WriteString("\n\n")

	if m.errDetected != "" {
		b.WriteString(styleErrorLabel.Render("Detected: "))
		b.WriteString(styleErrorValue.Render(m.errDetected))
		b.WriteString("\n")
	}
	if m.errTried != "" {
		b.WriteString(styleErrorLabel.Render("Tried:    "))
		b.WriteString(styleErrorValue.Render(m.errTried))
		b.WriteString("\n")
	}
	if m.errGot != "" {
		b.WriteString(styleErrorLabel.Render("Got:      "))
		b.WriteString(styleErrorValue.Render(trunc(m.errGot, boxW-12)))
		b.WriteString("\n")
	}

	if m.errHint != "" {
		b.WriteString("\n")
		b.WriteString(styleErrorHint.Render("Try:"))
		b.WriteString("\n")
		b.WriteString(styleErrorHint.Render("  $ " + m.errHint))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString("  ")
	b.WriteString(styleErrorButton.Render("[ r retry ]"))
	b.WriteString("   ")
	b.WriteString(styleErrorButton.Render("[ q quit ]"))

	box := styleErrorBorder.
		Width(boxW - 4).
		Render(b.String())

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// ── picker ────────────────────────────────────────────────────────────────────

func (m Model) viewPicker() string {
	return renderPicker(m)
}

// ── scenario (main view) ──────────────────────────────────────────────────────

func (m Model) viewScenario() string {
	if len(m.scenarios) == 0 {
		return "No scenarios loaded."
	}

	// Layout constants
	const sepHeight = 1 // one line for the horizontal rule below header / above footer
	headerH := 1
	footerH := 1
	contentH := m.height - headerH - sepHeight - footerH - sepHeight
	if contentH < 4 {
		return "Terminal too small."
	}

	leftW := (m.width * 6) / 10
	rightW := m.width - leftW - 1 // -1 for the vertical divider

	// Render sections.
	header := m.renderHeader()
	footer := m.renderFooter()
	leftPane := m.renderLeftPane(leftW, contentH)
	rightPane := m.renderRightPane(rightW, contentH)

	// Vertical divider between panes.
	divider := stylePaneDivider.Render(strings.Repeat("│\n", contentH-1)+"│")

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, divider, rightPane)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		hRule(m.width, styleHRule),
		content,
		hRule(m.width, styleHRule),
		footer,
	)
}

// ── header ────────────────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	sc := m.current()

	// Left side: "DEMO N · Title  [CATEGORY · RELOAD]"
	demoNum := parseDemoNumber(sc.ID)
	var demoStr string
	if demoNum > 0 {
		demoStr = styleHeaderDemoLabel.Render("DEMO ") +
			styleHeaderDemoNum.Render(fmt.Sprintf("%d", demoNum)) +
			styleHeaderDemoLabel.Render("  ·  ")
	}
	title := styleHeaderTitle.Render(sc.Title)

	badge := buildBadge(sc.Category, sc.Reload)

	left := demoStr + title + "  " + badge

	// Right side: "step N / N"
	total := len(sc.Steps)
	right := styleHeaderStep.Render(fmt.Sprintf("step %d / %d", m.stepIdx+1, total))

	// Pad between left and right.
	leftPlain := lipgloss.Width(left)
	rightPlain := lipgloss.Width(right)
	gap := m.width - leftPlain - rightPlain
	if gap < 1 {
		gap = 1
	}

	return " " + left + strings.Repeat(" ", gap-1) + right
}

func buildBadge(category string, reload bool) string {
	cat := strings.ToUpper(category)
	text := cat
	if reload {
		text += " · RELOAD"
	}
	return badgeStyle(category).Render(text)
}

// parseDemoNumber extracts a leading integer from a scenario ID like "02-add-server".
func parseDemoNumber(id string) int {
	n := 0
	for _, c := range id {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ── left pane ─────────────────────────────────────────────────────────────────

func (m Model) renderLeftPane(width, height int) string {
	sc := m.current()
	steps := sc.Steps
	inner := width // no borders on left pane, just content

	var lines []string

	// STEPS header
	sep := hRule(inner-8, styleDivider)
	lines = append(lines, styleStepsHeader.Render("STEPS ")+" "+sep)

	// Step list
	for i, step := range steps {
		num := fmt.Sprintf("%d", i+1)
		status := StatusPending
		if i < len(m.results) {
			status = m.results[i].Status
		}

		var icon, numStr, titleStr string
		switch status {
		case StatusDone:
			numStr = styleStepNumDone.Render(num)
			icon = styleStepIconDone.Render("✓")
			titleStr = styleStepTitleDone.Render(trunc(step.Title, inner-6))
		case StatusRunning:
			numStr = styleStepNumActive.Render(num)
			icon = m.spinner.View()
			titleStr = styleStepTitleAct.Render(trunc(step.Title, inner-6))
		case StatusError:
			numStr = styleStepNumError.Render(num)
			icon = styleStepIconError.Render("✗")
			titleStr = styleExitFail.Render(trunc(step.Title, inner-6))
		default: // Pending
			active := i == m.stepIdx
			if active {
				numStr = styleStepNumActive.Render(num)
				icon = styleStepIconActive.Render("›")
				titleStr = styleStepTitleAct.Render(trunc(step.Title, inner-6))
			} else {
				numStr = styleStepNumPending.Render(num)
				icon = styleStepIconPend.Render("○")
				titleStr = styleStepTitlePend.Render(trunc(step.Title, inner-6))
			}
		}

		lines = append(lines, numStr+" "+icon+"  "+titleStr)
	}
	lines = append(lines, "")

	// Current step detail
	if m.stepIdx < len(steps) {
		step := steps[m.stepIdx]
		result := StepResult{}
		if m.stepIdx < len(m.results) {
			result = m.results[m.stepIdx]
		}

		// "STEP N  Title"
		stepLabel := styleCurrentStepLabel.Render(fmt.Sprintf("STEP %d", m.stepIdx+1))
		stepTitle := styleCurrentStepTitle.Render("  "+step.Title)
		lines = append(lines, stepLabel+stepTitle)
		lines = append(lines, "")

		// Explain (word-wrapped)
		if step.Explain != "" {
			for _, wl := range wordWrap(step.Explain, inner-2) {
				lines = append(lines, styleExplain.Render(trunc(wl, inner)))
			}
			lines = append(lines, "")
		}

		// Command with $ prompt
		cmdLines := splitLines(strings.TrimRight(step.Run, "\n"))
		for i, cl := range cmdLines {
			cl = strings.TrimRight(cl, " ")
			if i == 0 {
				lines = append(lines, stylePrompt.Render("$ ")+styleCommand.Render(trunc(cl, inner-2)))
			} else {
				lines = append(lines, styleCmdCont.Render("    "+trunc(cl, inner-4)))
			}
		}
		lines = append(lines, "")

		// Output or not-yet-run message
		switch result.Status {
		case StatusPending:
			lines = append(lines, styleNotYet.Render("(not yet run — press space to execute)"))
		case StatusRunning:
			lines = append(lines, styleStepIconActive.Render(m.spinner.View()+" running…"))
		default:
			// stdout divider
			divLabel := "── stdout "
			divLine := divLabel + strings.Repeat("─", max(0, inner-len(divLabel)-1))
			lines = append(lines, styleDivider.Render(divLine))

			// Exit code hint
			if result.ExitCode != 0 {
				lines = append(lines, styleExitFail.Render(fmt.Sprintf("exit %d", result.ExitCode)))
			}

			// Output lines
			for _, ol := range splitLines(result.Output) {
				lines = append(lines, styleOutput.Render(trunc(ol, inner)))
			}

			// Error
			if result.Err != nil {
				lines = append(lines, styleExitFail.Render("error: "+result.Err.Error()))
			}
		}
	}

	// Pad/trim to exact height and join.
	lines = padLines(lines, height)
	return strings.Join(lines, "\n")
}

// ── right pane ────────────────────────────────────────────────────────────────

func (m Model) renderRightPane(width, height int) string {
	topH := height / 2
	bottomH := height - topH - 1 // -1 for the horizontal separator

	top := m.renderLogSection("haproxy", m.haproxyLog, width, topH)
	sep := hRule(width, styleHRule)
	bottom := m.renderLogSection("dataplaneapi", m.dapiLog, width, bottomH)

	return lipgloss.JoinVertical(lipgloss.Left, top, sep, bottom)
}

func (m Model) renderLogSection(container string, buf logBuffer, width, height int) string {
	if height <= 1 {
		return ""
	}

	// Header: "• haproxy logs   tail · N more above"
	above := max(0, buf.total-height+1)
	var metaStr string
	if above > 0 {
		metaStr = styleLogMeta.Render(fmt.Sprintf("  tail · %d more above", above))
	}
	header := styleLogLabel.Render("• "+container+" logs") + metaStr
	lines := []string{trunc(header, width)}

	// Show last (height-1) lines from the buffer.
	visH := height - 1
	src := buf.lines

	// Apply scroll (j/k moves scroll; 0 = tail).
	if m.logScroll > 0 && m.logScroll < len(src) {
		end := len(src) - m.logScroll
		if end < 0 {
			end = 0
		}
		src = src[:end]
	}

	start := 0
	if len(src) > visH {
		start = len(src) - visH
	}
	visible := src[start:]

	for _, l := range visible {
		lines = append(lines, trunc(colorLog(l), width))
	}

	lines = padLines(lines, height)
	return strings.Join(lines, "\n")
}

// ── footer ────────────────────────────────────────────────────────────────────

func (m Model) renderFooter() string {
	type binding struct{ key, desc string }
	bindings := []binding{
		{"n", "next"},
		{"space", "run+next"},
		{"r", "run"},
		{"p", "prev"},
		{"v", "verbose"},
		{"R", "reset"},
		{"s", "shell"},
		{"l", "list"},
		{"?", "help"},
		{"q", "quit"},
	}

	var parts []string
	for _, b := range bindings {
		parts = append(parts,
			styleFooterKey.Render(b.key)+
				styleFooterDesc.Render(" "+b.desc),
		)
	}
	line := " " + strings.Join(parts, styleFooterSep.Render("  "))
	return trunc(line, m.width)
}

// ── verbose overlay ───────────────────────────────────────────────────────────

func (m Model) viewVerboseOverlay() string {
	// Render the scenario view dimmed behind the overlay.
	base := m.viewScenario()

	overlayW := min(m.width-4, 100)
	overlayH := m.height - 4

	var b strings.Builder
	b.WriteString(styleVerboseTitle.Render("◆ VERBOSE") + "  ")
	b.WriteString(styleCurrentStepTitle.Render(m.verboseTitle))
	b.WriteString(styleLogMeta.Render("  (re-run with -v)"))
	b.WriteString("\n\n")

	// Visible lines with scroll.
	visLines := overlayH - 4
	src := m.verboseLines
	start := m.verboseScroll
	if start >= len(src) {
		start = max(0, len(src)-1)
	}
	end := start + visLines
	if end > len(src) {
		end = len(src)
	}
	for _, l := range src[start:end] {
		l = trunc(l, overlayW-4)
		// Color verbose curl output.
		switch {
		case strings.HasPrefix(l, ">"):
			b.WriteString(styleVerboseSend.Render(l))
		case strings.HasPrefix(l, "<"):
			b.WriteString(styleVerboseRecv.Render(l))
		case strings.HasPrefix(l, "*"):
			b.WriteString(styleVerboseMeta.Render(l))
		default:
			b.WriteString(styleVerboseOut.Render(l))
		}
		b.WriteString("\n")
	}

	overlay := styleVerboseBorder.
		Width(overlayW - 4).
		Render(b.String())

	_ = base // the base view; in a real impl we'd composite. For now just show overlay.
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

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

// ── helpers ───────────────────────────────────────────────────────────────────

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

// logStyle returns the lipgloss style for a log line based on severity keywords.
func logStyle(line string) lipgloss.Style {
	upper := strings.ToUpper(line)
	switch {
	case strings.Contains(upper, "[WARNING]") || strings.Contains(line, "level=warning"):
		return styleLogWarn
	case strings.Contains(upper, "[ERROR]") || strings.Contains(upper, "[ALERT]") ||
		strings.Contains(upper, "[CRIT]") || strings.Contains(upper, "[EMERG]") ||
		strings.Contains(line, "level=error"):
		return styleLogErr
	case strings.Contains(upper, "[NOTICE]") || strings.Contains(line, "level=info"):
		return styleLogNotice
	default:
		return styleLogPlain
	}
}

// colorLog applies severity-based color to a log line.
func colorLog(line string) string { return logStyle(line).Render(line) }

// wrapLog wraps line at width characters, indenting continuation lines by 4
// spaces. Character-level (not word) wrapping preserves structured log content.
func wrapLog(line string, width int) []string {
	const indent = "    "
	r := []rune(line)
	if width <= 0 || len(r) <= width {
		return []string{line}
	}
	contW := width - len([]rune(indent))
	if contW <= 0 {
		return []string{line}
	}
	out := []string{string(r[:width])}
	r = r[width:]
	for len(r) > 0 {
		if len(r) <= contW {
			out = append(out, indent+string(r))
			break
		}
		out = append(out, indent+string(r[:contW]))
		r = r[contW:]
	}
	return out
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
	es := m.errState

	var b strings.Builder
	b.WriteString(styleErrorTitle.Render("✗  " + es.title))
	b.WriteString("\n\n")
	b.WriteString(styleErrorBody.Render(es.body))
	b.WriteString("\n\n")

	if es.detected != "" {
		b.WriteString(styleErrorLabel.Render("Detected: "))
		b.WriteString(styleErrorValue.Render(es.detected))
		b.WriteString("\n")
	}
	if es.tried != "" {
		b.WriteString(styleErrorLabel.Render("Tried:    "))
		b.WriteString(styleErrorValue.Render(es.tried))
		b.WriteString("\n")
	}
	if es.got != "" {
		b.WriteString(styleErrorLabel.Render("Got:      "))
		b.WriteString(styleErrorValue.Render(trunc(es.got, boxW-12)))
		b.WriteString("\n")
	}

	if es.hint != "" {
		b.WriteString("\n")
		b.WriteString(styleErrorHint.Render("Try:"))
		b.WriteString("\n")
		b.WriteString(styleErrorHint.Render("  $ " + es.hint))
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

// renderLeftPane splits the left column into a fixed upper section (steps +
// detail + command) and a scrollable stdout panel below it.
func (m Model) renderLeftPane(width, height int) string {
	sc := m.current()
	steps := sc.Steps
	inner := width

	// --- Upper section: steps list + current step detail + command ---
	var upper []string

	// STEPS header
	sep := hRule(inner-8, styleDivider)
	upper = append(upper, styleStepsHeader.Render("STEPS ")+" "+sep)

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
		upper = append(upper, numStr+" "+icon+"  "+titleStr)
	}
	upper = append(upper, "")

	// Current step detail (label, explain, command) — no output here
	if m.stepIdx < len(steps) {
		step := steps[m.stepIdx]

		stepLabel := styleCurrentStepLabel.Render(fmt.Sprintf("STEP %d", m.stepIdx+1))
		stepTitle := styleCurrentStepTitle.Render("  " + step.Title)
		upper = append(upper, stepLabel+stepTitle)
		upper = append(upper, "")

		if step.Explain != "" {
			for _, wl := range wordWrap(step.Explain, inner-2) {
				upper = append(upper, styleExplain.Render(trunc(wl, inner)))
			}
			upper = append(upper, "")
		}

		cmdLines := splitLines(strings.TrimRight(step.Run, "\n"))
		for i, cl := range cmdLines {
			cl = strings.TrimRight(cl, " ")
			if i == 0 {
				upper = append(upper, stylePrompt.Render("$ ")+styleCommand.Render(trunc(cl, inner-2)))
			} else {
				upper = append(upper, styleCmdCont.Render("    "+trunc(cl, inner-4)))
			}
		}
	}

	// --- Allocate heights ---
	// Reserve at least minOutputH rows for the stdout panel.
	const minOutputH = 6
	upperH := len(upper)
	outputH := height - upperH
	if outputH < minOutputH {
		upperH = height - minOutputH
		if upperH < 0 {
			upperH = 0
		}
		outputH = height - upperH
	}

	// Trim upper from the bottom (cut off trailing empty lines first) if needed.
	if len(upper) > upperH {
		upper = upper[:upperH]
	}
	// Pad upper with empty lines to fill its reserved height.
	for len(upper) < upperH {
		upper = append(upper, "")
	}

	// --- Stdout panel (scrollable) ---
	outputPanel := m.renderOutputPanel(inner, outputH)

	all := append(upper, outputPanel...)
	return strings.Join(all, "\n")
}

// renderOutputPanel renders the stdout output section as a fixed-height
// scrollable panel. outputScroll=0 shows the tail (newest output at bottom);
// increasing scroll moves the view toward older output.
func (m Model) renderOutputPanel(width, height int) []string {
	if height <= 0 {
		return nil
	}

	lines := make([]string, height)

	result := StepResult{}
	if m.stepIdx < len(m.results) {
		result = m.results[m.stepIdx]
	}

	// Build the divider header line, optionally with a scroll indicator suffix.
	buildHeader := func(suffix string) string {
		label := "── stdout "
		availW := width - len(label) - lipgloss.Width(suffix) - 1
		if availW < 0 {
			availW = 0
		}
		return styleDivider.Render(label+strings.Repeat("─", availW)) + suffix
	}

	switch result.Status {
	case StatusPending:
		lines[0] = buildHeader("")
		if height > 1 {
			lines[1] = styleNotYet.Render("(not yet run — press space to execute)")
		}

	case StatusRunning:
		lines[0] = buildHeader("")
		if height > 1 {
			lines[1] = styleStepIconActive.Render(m.spinner.View() + " running…")
		}

	default:
		// Lines reserved for non-output content (exit code, error).
		extraH := 0
		if result.ExitCode != 0 {
			extraH++
		}
		if result.Err != nil {
			extraH++
		}

		contentH := height - 1 - extraH // rows available for stdout text
		if contentH < 0 {
			contentH = 0
		}

		// Gather output lines (drop the single empty line from empty output).
		outputSrc := splitLines(result.Output)
		if len(outputSrc) == 1 && outputSrc[0] == "" {
			outputSrc = nil
		}
		total := len(outputSrc)

		// Apply scroll: 0 = tail; N = scrolled N lines toward older output.
		scroll := m.outputScroll
		if scroll > total {
			scroll = total
		}
		end := total - scroll
		if end < 0 {
			end = 0
		}
		start := end - contentH
		if start < 0 {
			start = 0
		}
		above := start // lines above the visible window

		var suffix string
		if above > 0 {
			suffix = styleLogMeta.Render(fmt.Sprintf("  ↑ %d above", above))
		}
		lines[0] = buildHeader(suffix)

		row := 1
		if result.ExitCode != 0 && row < height {
			lines[row] = styleExitFail.Render(fmt.Sprintf("exit %d", result.ExitCode))
			row++
		}

		// Tail-anchor: pad empty rows above content so output sits at the bottom
		// when there are fewer lines than the panel height.
		visible := outputSrc[start:end]
		topPad := contentH - len(visible)
		for i := 0; i < topPad && row < height; i++ {
			// lines[row] is already "" from make
			row++
		}

		for _, ol := range visible {
			if row >= height {
				break
			}
			lines[row] = styleOutput.Render(trunc(ol, width))
			row++
		}

		if result.Err != nil && row < height {
			lines[row] = styleExitFail.Render("error: " + result.Err.Error())
		}
	}

	return lines
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

	visH := height - 1
	src := buf.lines

	// Clamp scroll to the buffer length so overshoot can't cause a loopback to
	// tail and doesn't require many down-scrolls to recover.
	scroll := m.logScroll
	if scroll > len(src) {
		scroll = len(src)
	}

	// Slice src: scroll=0 → full buffer (tail); scroll=N → drop last N source lines.
	if scroll > 0 {
		src = src[:len(src)-scroll]
	}

	// Expand source lines to physical (wrapped) lines. All wrapped pieces of one
	// source line share the same severity color so the block looks uniform.
	var physical []string
	for _, l := range src {
		style := logStyle(l)
		for _, piece := range wrapLog(l, width) {
			physical = append(physical, style.Render(piece))
		}
	}

	start := 0
	if len(physical) > visH {
		start = len(physical) - visH
	}

	// Header: reflect tail vs. scrolled state.
	var metaStr string
	if scroll > 0 {
		// Show how far above tail the user has scrolled.
		metaStr = styleLogMeta.Render(fmt.Sprintf("  ↑ %d from tail", scroll))
	} else {
		above := max(0, buf.total-visH)
		if above > 0 {
			metaStr = styleLogMeta.Render(fmt.Sprintf("  tail · %d more above", above))
		}
	}
	header := styleLogLabel.Render("• "+container+" logs") + metaStr
	lines := []string{trunc(header, width)}

	for _, l := range physical[start:] {
		lines = append(lines, l)
	}

	lines = padLines(lines, height)
	return strings.Join(lines, "\n")
}

// ── footer ────────────────────────────────────────────────────────────────────

func (m Model) renderFooter() string {
	type binding struct{ key, desc string }
	bindings := []binding{
		{"space", "run"},
		{"n", "next"},
		{"p", "prev"},
		{"v", "verbose"},
		{"R", "reset"},
		{"l", "list"},
		{"q", "quit"},
	}

	// Build left-to-right using lipgloss.Width for visible-character measurement
	// (trunc counts ANSI escape bytes as runes and cuts off too early).
	sep := styleFooterSep.Render("  ")
	sepW := lipgloss.Width(sep)

	var parts []string
	used := 1 // leading space
	for _, b := range bindings {
		part := styleFooterKey.Render(b.key) + styleFooterDesc.Render(" "+b.desc)
		gap := lipgloss.Width(part)
		if len(parts) > 0 {
			gap += sepW
		}
		if used+gap > m.width {
			break
		}
		parts = append(parts, part)
		used += gap
	}
	return " " + strings.Join(parts, sep)
}

// ── verbose overlay ───────────────────────────────────────────────────────────

func (m Model) viewVerboseOverlay() string {
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

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

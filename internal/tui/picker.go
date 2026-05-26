package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderPicker renders the full-screen scenario selection list.
func renderPicker(m Model) string {
	if m.width == 0 || m.height == 0 {
		return ""
	}

	var lines []string

	// Header
	title := stylePickerHeader.Render("  Select a scenario")
	hint := stylePickerHint.Render("↑↓ / j k  navigate    enter / space  run    q  quit")
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(hint)
	if gap < 2 {
		gap = 2
	}
	lines = append(lines, title+strings.Repeat(" ", gap)+hint)
	lines = append(lines, hRule(m.width, styleHRule))

	// Scenario list
	listH := m.height - 4 // header(1) + sep(1) + sep(1) + footer(1)

	// Scroll window around the cursor
	start := 0
	if m.pickerIdx >= listH {
		start = m.pickerIdx - listH + 1
	}
	end := start + listH
	if end > len(m.scenarios) {
		end = len(m.scenarios)
	}

	for i := start; i < end; i++ {
		sc := m.scenarios[i]
		col := categoryColor(sc.Category)
		catLabel := lipgloss.NewStyle().Foreground(col).Bold(true).Render(
			fmt.Sprintf("%-8s", strings.ToUpper(sc.Category)),
		)
		reload := "      "
		if sc.Reload {
			reload = lipgloss.NewStyle().Foreground(colorAmber).Render("RELOAD")
		}
		idStr := stylePickerID.Render(fmt.Sprintf("%-28s", sc.ID))
		titleStr := sc.Title

		row := fmt.Sprintf("  %s  %s  %s  %s", catLabel, reload, idStr, titleStr)
		row = trunc(row, m.width)

		if i == m.pickerIdx {
			// Pad to full width for the highlight bar.
			rowW := lipgloss.Width(row)
			pad := m.width - rowW
			if pad > 0 {
				row += strings.Repeat(" ", pad)
			}
			lines = append(lines, stylePickerSelected.Render(row))
		} else {
			lines = append(lines, stylePickerNormal.Render(row))
		}
	}

	// Pad remaining height
	for len(lines) < m.height-2 {
		lines = append(lines, "")
	}

	lines = append(lines, hRule(m.width, styleHRule))
	// Footer
	lines = append(lines, stylePickerHint.Render(
		"  "+fmt.Sprintf("%d scenarios", len(m.scenarios))+
			"  ·  core scenarios 01-05  extras in scenarios/extras/",
	))

	return strings.Join(lines, "\n")
}

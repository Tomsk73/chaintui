package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// overlayCenter draws box over the centre of bg, keeping bg's dimensions so the
// result still fits the window.
//
// The backdrop is stripped of its own styling and re-rendered dim: cutting a
// styled line to slot the box in would leave the fragment on the right without
// the escape sequence that coloured it, so a highlighted row behind the box
// would come out half-coloured. Dimming the whole backdrop is both correct and
// the usual way a modal shows it has focus.
func overlayCenter(bg, box string) string {
	bgLines := strings.Split(bg, "\n")
	boxLines := strings.Split(box, "\n")
	bgW, boxW := lipgloss.Width(bg), lipgloss.Width(box)
	if len(boxLines) > len(bgLines) || boxW > bgW {
		return box // window too small to frame the dialog
	}
	x := (bgW - boxW) / 2
	y := (len(bgLines) - len(boxLines)) / 2

	out := make([]string, len(bgLines))
	for i, line := range bgLines {
		plain := ansi.Strip(line)
		if w := ansi.StringWidth(plain); w < bgW {
			plain += strings.Repeat(" ", bgW-w)
		}
		if i < y || i >= y+len(boxLines) {
			out[i] = dimStyle.Render(plain)
			continue
		}
		// plain carries no escape sequences, so these are plain width-aware cuts.
		left := ansi.Truncate(plain, x, "")
		right := ansi.TruncateLeft(plain, x+boxW, "")
		out[i] = dimStyle.Render(left) + boxLines[i-y] + dimStyle.Render(right)
	}
	return strings.Join(out, "\n")
}

// quitDialog is the "are you sure" pop-up shown before the app exits.
func quitDialog() string {
	hint := func(key, desc string) string {
		return dialogKeyStyle.Render("<"+key+">") + dialogDescStyle.Render(" "+desc)
	}
	lines := []string{
		dialogTitleStyle.Render("Quit chaintui?"),
		"",
		hint("y", "quit") + dialogDescStyle.Render("    ") + hint("n", "cancel"),
	}
	width := 0
	for _, l := range lines {
		width = max(width, lipgloss.Width(l))
	}
	// Centre the lines here rather than with JoinVertical, which would pad them
	// out with unstyled spaces and leave gaps in the panel.
	row := lipgloss.NewStyle().Background(navy).Width(width).Align(lipgloss.Center)
	for i, l := range lines {
		lines[i] = row.Render(l)
	}
	return dialogStyle.Render(strings.Join(lines, "\n"))
}

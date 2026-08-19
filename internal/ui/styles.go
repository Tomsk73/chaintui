package ui

import "github.com/charmbracelet/lipgloss"

var (
	cyan   = lipgloss.Color("#00D7FF")
	purple = lipgloss.Color("#875FFF")
	green  = lipgloss.Color("#00FF87")
	yellow = lipgloss.Color("#FFD700")
	red    = lipgloss.Color("#FF5F5F")
	gray   = lipgloss.Color("#626262")
	white  = lipgloss.Color("#EEEEEE")
	navy   = lipgloss.Color("#1C1C2E")
	mid    = lipgloss.Color("#2A2A4A")

	appNameStyle = lipgloss.NewStyle().Bold(true).Foreground(cyan)
	resTypeStyle = lipgloss.NewStyle().Bold(true).Foreground(green)
	ctxStyle     = lipgloss.NewStyle().Foreground(purple)
	sepStyle     = lipgloss.NewStyle().Foreground(gray)
	dimStyle     = lipgloss.NewStyle().Foreground(gray)
	errStyle     = lipgloss.NewStyle().Foreground(red).Bold(true)

	headerStyle = lipgloss.NewStyle().
			Background(navy).
			Width(0) // set per render

	footerStyle = lipgloss.NewStyle().
			Background(navy).
			Foreground(gray)

	cmdBarStyle = lipgloss.NewStyle().
			Background(mid).
			Foreground(cyan).
			Padding(0, 1)

	keyStyle  = lipgloss.NewStyle().Foreground(cyan).Bold(true)
	descStyle = lipgloss.NewStyle().Foreground(gray)

	// Header and cell padding must match: the bubbles table sizes a cell as
	// column width + the style's horizontal frame, so a header without the
	// cells' padding leaves the two grids misaligned.
	tableCellStyle = lipgloss.NewStyle().Padding(0, 1)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(white).
				Background(mid).
				Padding(0, 1)

	selectedRowStyle = lipgloss.NewStyle().
				Foreground(cyan).
				Background(navy).
				Bold(true)

	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cyan).
			Background(navy).
			Padding(1, 4)

	// Every span inside the dialog carries the panel background: lipgloss does
	// not re-open an outer background after a nested style resets, so text left
	// unstyled here would punch a hole in the panel.
	dialogTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(white).Background(navy)
	dialogKeyStyle   = lipgloss.NewStyle().Bold(true).Foreground(cyan).Background(navy)
	dialogDescStyle  = lipgloss.NewStyle().Foreground(gray).Background(navy)

	borderColor = gray
)

func keyHint(key, desc string) string {
	return keyStyle.Render("<"+key+">") + " " + descStyle.Render(desc)
}

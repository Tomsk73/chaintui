package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tomsk73/chaintui/internal/api"
	"github.com/Tomsk73/chaintui/internal/ui"
)

func main() {
	client, err := api.NewClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "auth error:", err)
		os.Exit(1)
	}

	m := ui.New(client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

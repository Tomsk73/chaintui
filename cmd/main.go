package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tomsk73/chaintui/internal/api"
	"github.com/Tomsk73/chaintui/internal/ui"
)

func main() {
	debug := flag.Bool("debug", false, "write debug log to /tmp/chaintui-debug.log")
	flag.BoolVar(debug, "d", false, "write debug log to /tmp/chaintui-debug.log (shorthand)")
	flag.Parse()

	if *debug {
		if err := ui.InitDebugLog(); err != nil {
			fmt.Fprintln(os.Stderr, "debug log:", err)
		}
	}

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

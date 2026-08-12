package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

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

	client, err := resolveClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "auth:", err)
		os.Exit(1)
	}

	m := ui.New(client)
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// resolveClient uses a cached token when available. If the user is not logged
// in and stdin is a TTY, it offers to run chainctl auth login before starting
// the TUI.
func resolveClient() (*api.Client, error) {
	client, err := api.NewClient()
	if err == nil {
		return client, nil
	}
	if !api.IsNotLoggedIn(err) {
		return nil, err
	}

	fmt.Fprintln(os.Stderr, err.Error())
	fmt.Fprintln(os.Stderr, "chaintui needs a Chainguard token from chainctl or CHAINGUARD_TOKEN.")

	if !stdinIsTTY() {
		fmt.Fprintln(os.Stderr, "Run: chainctl auth login")
		fmt.Fprintln(os.Stderr, "Or set: export CHAINGUARD_TOKEN=$(chainctl auth token)")
		return nil, err
	}

	if !confirm(os.Stderr, "Run chainctl auth login now? [Y/n] ") {
		fmt.Fprintln(os.Stderr, "Aborted. Run chainctl auth login, then retry.")
		return nil, err
	}

	return api.Login()
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func confirm(out *os.File, prompt string) bool {
	fmt.Fprint(out, prompt)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(strings.TrimSpace(line)) == 0 {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true
	default:
		return false
	}
}

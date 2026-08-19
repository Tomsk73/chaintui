package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestOverlayCenterKeepsBackdropDimensions(t *testing.T) {
	t.Parallel()
	bg := strings.TrimSuffix(strings.Repeat(strings.Repeat("x", 60)+"\n", 20), "\n")
	box := "┌────┐\n│ hi │\n└────┘"

	out := overlayCenter(bg, box)
	lines := strings.Split(out, "\n")
	if len(lines) != 20 {
		t.Fatalf("height=%d, want the backdrop's 20", len(lines))
	}
	for i, l := range lines {
		if got := lipgloss.Width(l); got != 60 {
			t.Fatalf("line %d width=%d, want 60", i, got)
		}
	}
	// The box lands centred, with backdrop either side of it.
	var mid string
	for _, l := range lines {
		if strings.Contains(l, "│ hi │") {
			mid = l
		}
	}
	if mid == "" {
		t.Fatalf("box not drawn:\n%s", out)
	}
	if !strings.HasPrefix(mid, "x") || !strings.HasSuffix(mid, "x") {
		t.Fatalf("backdrop should survive either side: %q", mid)
	}
}

func TestOverlayCenterWindowTooSmall(t *testing.T) {
	t.Parallel()
	box := "┌────┐\n│ hi │\n└────┘"
	if got := overlayCenter("tiny", box); got != box {
		t.Fatalf("a window smaller than the dialog should show the dialog alone, got %q", got)
	}
}

func TestQuitDialogIsRectangular(t *testing.T) {
	t.Parallel()
	lines := strings.Split(quitDialog(), "\n")
	if len(lines) < 3 {
		t.Fatalf("dialog has %d lines", len(lines))
	}
	want := lipgloss.Width(lines[0])
	for i, l := range lines {
		if got := lipgloss.Width(l); got != want {
			t.Errorf("line %d width=%d, want %d", i, got, want)
		}
	}
	if !strings.Contains(quitDialog(), "Quit chaintui?") {
		t.Error("dialog is missing its question")
	}
}

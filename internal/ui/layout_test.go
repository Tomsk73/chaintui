package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// appShowing returns an app of the given size with a repos page on top, loaded
// with more rows than fit and a status line, which is the busiest a list gets.
func appShowing(t *testing.T, width, height int) App {
	t.Helper()
	a := New(nil)
	m, _ := a.Update(tea.WindowSizeMsg{Width: width, Height: height})
	a = m.(App)
	a.orgName = "chainguard"
	page := NewReposPage(nil, "org/1").WithLabel("chainguard repos")
	m, _ = a.Update(PushMsg{P: page})
	a = m.(App)
	names := make([]string, 60)
	for i := range names {
		names[i] = fmt.Sprintf("repo-with-a-fairly-long-name-%02d", i)
	}
	pm, _ := page.Update(LoadedMsg{PageResult: PageResult{
		Rows:          rows(names...),
		NextPageToken: "n",
		TotalCount:    1300,
		Status:        "Chainguard advisory catalogue — not scoped to this org",
	}})
	a.stack[len(a.stack)-1] = pm.(*ListPage)
	return a
}

// The view is the header, the page and the footer stacked to exactly the
// window's height, with no line wider than the window. Anything more and
// bubbletea cannot repaint the rows it believes are off-screen, so the previous
// frame's footer stays visible under the new one — the key hints shown twice.
func TestViewFitsTheWindow(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{60, 20}, {80, 24}, {100, 30}, {120, 40}, {200, 50}} {
		width, height := size[0], size[1]
		a := appShowing(t, width, height)
		for _, mode := range []string{"hints", "cmd", "notice"} {
			b := a
			switch mode {
			case "cmd":
				b.cmdMode = true
			case "notice":
				b.notice = strings.Repeat("something happened and this is a long notice ", 4)
			}
			view := b.View()
			if got := lipgloss.Height(view); got != height {
				t.Errorf("%dx%d %s: view is %d rows", width, height, mode, got)
			}
			for i, line := range strings.Split(view, "\n") {
				if w := lipgloss.Width(line); w > width {
					t.Errorf("%dx%d %s: line %d is %d cells wide", width, height, mode, i, w)
				}
			}
			if mode == "hints" {
				if n := strings.Count(view, "quit"); n != 1 {
					t.Errorf("%dx%d: key hints appear %d times", width, height, n)
				}
			}
		}
	}
}

// Narrow windows lose the generic hints first; the page's own keys and the way
// out stay.
func TestFooterHintsFitOneRow(t *testing.T) {
	t.Parallel()
	for _, width := range []int{60, 80, 100, 160} {
		f := renderFooter(width, "repos", true)
		if got := lipgloss.Height(f); got != footerH {
			t.Errorf("width %d: footer is %d rows, want %d", width, got, footerH)
		}
		for _, want := range []string{"delete repo", "cves", "back", "quit"} {
			if !strings.Contains(f, want) {
				t.Errorf("width %d: footer lost %q: %s", width, want, f)
			}
		}
	}
	wide := renderFooter(200, "repos", true)
	for _, want := range []string{"cmd", "search", "next page", "quit"} {
		if !strings.Contains(wide, want) {
			t.Errorf("a wide footer should show everything, missing %q", want)
		}
	}
}

// The declared column widths add up to more than a narrow window, so columns
// have to give width up rather than let every row wrap.
func TestFitColumnsShrinksToTheWindow(t *testing.T) {
	t.Parallel()
	cols := []table.Column{{Width: 35}, {Width: 30}, {Width: 14}}
	pad := 2
	for _, width := range []int{60, 80, 110, 200} {
		c := append([]table.Column(nil), cols...)
		fitColumns(c, width, pad)
		grid := 0
		for _, col := range c {
			grid += col.Width + pad
			if col.Width < 4 {
				t.Errorf("width %d: column shrunk to %d", width, col.Width)
			}
		}
		if grid != width {
			t.Errorf("width %d: grid is %d", width, grid)
		}
	}
	// The widest column pays first, so the narrow ones stay legible.
	c := append([]table.Column(nil), cols...)
	fitColumns(c, 60, pad)
	if c[2].Width != 14 || c[0].Width >= 35 {
		t.Errorf("shrink order wrong: %+v", c)
	}
}

func TestFitLine(t *testing.T) {
	t.Parallel()
	long := dimStyle.Render(strings.Repeat("abcdefghij", 10))
	if got := fitLine(long, 20); lipgloss.Width(got) != 20 || !strings.HasSuffix(got, "…") && !strings.Contains(got, "…") {
		t.Errorf("width=%d %q", lipgloss.Width(got), got)
	}
	if got := fitLine("short", 20); got != "short" {
		t.Errorf("a fitting line should be untouched, got %q", got)
	}
}

// A long breadcrumb is cut from its start, so the page you are on stays visible
// and the header stays on its two rows.
func TestHeaderKeepsCurrentPageVisible(t *testing.T) {
	t.Parallel()
	crumb := "organizations > chainguard > chainguard repos > nginx > tags > nginx:latest cves"
	h := renderHeader(70, "cves", "chainguard / folder", crumb, "as deploy-bot")
	if got := lipgloss.Height(h); got != headerH {
		t.Fatalf("header is %d rows", got)
	}
	first := strings.Split(h, "\n")[0]
	if w := lipgloss.Width(first); w > 70 {
		t.Errorf("header line is %d cells wide", w)
	}
	if !strings.Contains(first, "cves") || !strings.Contains(first, "…") {
		t.Errorf("breadcrumb should be cut from the left and still end on the page: %q", first)
	}
}

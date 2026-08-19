package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRootPageIsOrgPicker(t *testing.T) {
	t.Parallel()
	a := New(nil)
	if len(a.stack) != 1 {
		t.Fatalf("stack=%d", len(a.stack))
	}
	if got := a.top().ResourceType(); got != orgListResource {
		t.Fatalf("root resource=%q, want the org picker", got)
	}
}

func TestSelectOrgPushesOrgMenu(t *testing.T) {
	t.Parallel()
	a := New(nil)
	m, _ := a.Update(SelectOrgMsg{UID: "org/1", Name: "acme"})
	a = m.(App)
	if a.orgCtx != "org/1" || a.orgName != "acme" {
		t.Fatalf("org context: %q %q", a.orgCtx, a.orgName)
	}
	if len(a.stack) != 2 || a.top().ResourceType() != "org" {
		t.Fatalf("stack=%d top=%q", len(a.stack), a.top().ResourceType())
	}
	// The org menu carries the org UIDP so `:` commands scope to it.
	if a.top().GroupContext() != "org/1" {
		t.Fatalf("org menu groupCtx=%q", a.top().GroupContext())
	}
	if !strings.Contains(a.groupPath(), "acme") {
		t.Fatalf("header path=%q", a.groupPath())
	}

	// Going back to the picker drops the org context rather than keeping a stale one.
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if len(a.stack) != 1 || a.orgCtx != "" || a.orgName != "" {
		t.Fatalf("after esc: stack=%d org=%q/%q", len(a.stack), a.orgCtx, a.orgName)
	}
}

func TestSwitchKeepsOrgPickerBelow(t *testing.T) {
	t.Parallel()
	a := New(nil)
	m, _ := a.Update(SelectOrgMsg{UID: "org/1", Name: "acme"})
	a = m.(App)
	m, _ = a.Update(SwitchMsg{Resource: "charts", GroupCtx: "org/1"})
	a = m.(App)
	if len(a.stack) != 2 {
		t.Fatalf("stack=%d, want picker + charts", len(a.stack))
	}
	if a.stack[0].ResourceType() != orgListResource || a.top().ResourceType() != "charts" {
		t.Fatalf("stack=%q/%q", a.stack[0].ResourceType(), a.top().ResourceType())
	}
	// Switching back to the picker resets to a single page and clears the org.
	m, _ = a.Update(SwitchMsg{Resource: "orgs"})
	a = m.(App)
	if len(a.stack) != 1 || a.orgCtx != "" {
		t.Fatalf("stack=%d org=%q", len(a.stack), a.orgCtx)
	}
}

func TestResolveResourcePageNames(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"charts":    "charts",
		"helm":      "charts",
		"libraries": "libraries",
		"libpolicy": "librariespolicy",
		"policy":    "librariespolicy",
		"ents":      "entitlements",
		"policies":  "libraries-policies",
		"bindings":  "policybindings",
		"blocked":   "blocked",
		"repos":     "repos",
		"folders":   "groups",
		"orgs":      orgListResource,
		"python":    "artifacts",
	}
	for cmd, want := range cases {
		page := resolveResourcePage(nil, cmd, "org/1", "acme")
		if page == nil {
			t.Errorf("%q resolved to nil", cmd)
			continue
		}
		if got := page.ResourceType(); got != want {
			t.Errorf("%q -> %q, want %q", cmd, got, want)
		}
	}
	if resolveResourcePage(nil, "nonsense", "org/1", "acme") != nil {
		t.Error("unknown resource should resolve to nil")
	}
}

func TestFooterHintsPerResource(t *testing.T) {
	t.Parallel()
	artifacts := renderFooter(200, "artifacts", true)
	for _, want := range []string{"remediated", "export json"} {
		if !strings.Contains(artifacts, want) {
			t.Fatalf("artifacts footer missing %q: %s", want, artifacts)
		}
	}
	if got := renderFooter(200, "roles", true); !strings.Contains(got, "custom only") {
		t.Fatalf("roles footer missing the built-in toggle: %s", got)
	}
	if got := renderFooter(200, "blocked", true); !strings.Contains(got, "log mode") {
		t.Fatalf("blocked footer missing the mode toggle: %s", got)
	}

	// The export sweep is libraries-only; other pages should not advertise it.
	if got := renderFooter(200, "repos", true); strings.Contains(got, "export json") {
		t.Fatalf("repos footer should not offer export: %s", got)
	}
}

// sized returns an app that has been told the window size, so View renders.
func sized(a App) App {
	m, _ := a.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m.(App)
}

func TestQuitAsksForConfirmation(t *testing.T) {
	t.Parallel()
	a := sized(New(nil))
	m, cmd := a.Update(keyPress('q'))
	a = m.(App)
	if cmd != nil {
		t.Fatal("q must not quit on its own")
	}
	if !a.quitting {
		t.Fatal("q should open the confirmation")
	}
	if !strings.Contains(a.View(), "Quit chaintui?") {
		t.Fatalf("dialog not rendered:\n%s", a.View())
	}

	// Cancelling leaves the session untouched.
	m, cmd = a.Update(keyPress('n'))
	a = m.(App)
	if a.quitting || cmd != nil {
		t.Fatal("n should dismiss the dialog without quitting")
	}
	if strings.Contains(a.View(), "Quit chaintui?") {
		t.Fatal("dialog still rendered after cancel")
	}
}

func TestQuitConfirmKeys(t *testing.T) {
	t.Parallel()
	quits := map[string]tea.KeyMsg{
		"y":     keyPress('y'),
		"Y":     keyPress('Y'),
		"enter": {Type: tea.KeyEnter},
	}
	for name, key := range quits {
		a := sized(New(nil))
		a.quitting = true
		_, cmd := a.Update(key)
		if cmd == nil {
			t.Fatalf("%s: expected a quit cmd", name)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%s: cmd is not tea.Quit", name)
		}
	}

	// esc cancels; an unrecognised key is neither answer.
	for name, key := range map[string]tea.KeyMsg{"esc": {Type: tea.KeyEsc}, "n": keyPress('n')} {
		a := sized(New(nil))
		a.quitting = true
		m, cmd := a.Update(key)
		if m.(App).quitting || cmd != nil {
			t.Fatalf("%s should cancel", name)
		}
	}
	a := sized(New(nil))
	a.quitting = true
	m, cmd := a.Update(keyPress('z'))
	if !m.(App).quitting || cmd != nil {
		t.Fatal("an unrecognised key should leave the dialog up")
	}
}

func TestQuitFromNestedPageDoesNotPop(t *testing.T) {
	t.Parallel()
	a := sized(New(nil))
	m, _ := a.Update(SelectOrgMsg{UID: "org/1", Name: "acme"})
	a = m.(App)
	if len(a.stack) != 2 {
		t.Fatalf("stack=%d", len(a.stack))
	}
	// q used to mean "back" on a nested page; it now offers to quit, and esc is
	// the way back.
	m, _ = a.Update(keyPress('q'))
	a = m.(App)
	if !a.quitting || len(a.stack) != 2 {
		t.Fatalf("quitting=%v stack=%d", a.quitting, len(a.stack))
	}
	m, _ = a.Update(keyPress('n'))
	a = m.(App)
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if len(m.(App).stack) != 1 {
		t.Fatalf("esc should still pop, stack=%d", len(m.(App).stack))
	}
}

func TestPagePromptKeepsItsKeystrokes(t *testing.T) {
	t.Parallel()
	a := sized(New(nil))
	page := testListPage(nil)
	m, _ := a.Update(PushMsg{P: page})
	a = m.(App)

	m, _ = a.Update(keyPress('/'))
	a = m.(App)
	if !a.inputActive() {
		t.Fatal("filter prompt should be capturing keys")
	}
	// q, esc and : belong to the filter box while it is open.
	for _, r := range []rune{'q', 'u', 'x'} {
		m, _ = a.Update(keyPress(r))
		a = m.(App)
	}
	if a.quitting {
		t.Fatal("typing q into a filter must not offer to quit")
	}
	if got := page.filterIn.Value(); got != "qux" {
		t.Fatalf("filter value=%q, want the typed text", got)
	}
	m, _ = a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if len(a.stack) != 2 {
		t.Fatal("esc should close the filter, not pop the page")
	}
	if a.inputActive() {
		t.Fatal("filter should be closed")
	}
}

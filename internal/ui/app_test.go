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
	// The export sweep is libraries-only; other pages should not advertise it.
	if got := renderFooter(200, "repos", true); strings.Contains(got, "export json") {
		t.Fatalf("repos footer should not offer export: %s", got)
	}
}

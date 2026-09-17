package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tomsk73/chaintui/internal/api"
)

func bindingRow() RowData {
	b := api.RoleBinding{
		UID:         "org/1/rb-1",
		IdentityUID: "org/1/id-1",
		Identity: &api.RoleBindingIdentity{
			UID: "org/1/id-1", Name: "tom", Email: "tom@example.com",
			Issuer: "https://accounts.google.com",
		},
		Role:  &api.RoleBindingRole{UID: "role-owner", Name: "owner"},
		Group: &api.RoleBindingGroup{UID: "org/1", Name: "acme"},
	}
	return RowData{UID: b.UID, Raw: b}
}

func TestUsersPageRows(t *testing.T) {
	t.Parallel()
	p := NewUsersPage(nil, "org/1", "acme")
	if got := p.Label(); got != "acme users" {
		t.Errorf("label=%q", got)
	}
	if p.rowActionFor("a") == nil || p.rowActionFor("D") == nil {
		t.Fatal("users page should bind a and D")
	}
	// The page must not offer server sorts: which order_by fields this RPC
	// accepts is unverified.
	if len(p.serverSortFields) != 0 {
		t.Errorf("serverSortFields=%v", p.serverSortFields)
	}

	b := bindingRow().Raw.(api.RoleBinding)
	if got := b.Holder(); got != "tom@example.com" {
		t.Errorf("holder=%q", got)
	}
	if got := bindingScope(b); got != "acme" {
		t.Errorf("scope=%q", got)
	}
	if got := bindingIssuer(b); got != "accounts.google.com" {
		t.Errorf("issuer=%q", got)
	}

	// A workload identity is labelled as one rather than shown with a blank IdP.
	workload := api.RoleBinding{Identity: &api.RoleBindingIdentity{Name: "ci-deployer"}}
	if got := bindingIssuer(workload); got != "(workload)" {
		t.Errorf("issuer=%q", got)
	}
	if got := bindingIssuer(api.RoleBinding{}); got != "-" {
		t.Errorf("unhydrated issuer=%q", got)
	}
}

func TestUserSummaryCountsPeopleAndWorkloads(t *testing.T) {
	t.Parallel()
	items := []api.RoleBinding{
		{Identity: &api.RoleBindingIdentity{Email: "a@x.com", Issuer: "https://accounts.google.com"}},
		{Identity: &api.RoleBindingIdentity{Name: "ci"}},
		{Identity: &api.RoleBindingIdentity{Name: "syncer"}},
	}
	got := userSummary(items)
	for _, want := range []string{"3 bindings", "1 people", "2 workloads"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
	if got := userSummary(nil); got != "" {
		t.Errorf("empty summary=%q", got)
	}
}

// Revoking must be confirmed, and must say that the identity survives so it is
// not mistaken for deleting the user.
func TestRevokeBindingAsksFirst(t *testing.T) {
	t.Parallel()
	msg := confirmFrom(t, revokeBindingAction(nil)(bindingRow()))
	if msg.Prompt != "Are you sure you want to revoke this access?" {
		t.Errorf("prompt=%q", msg.Prompt)
	}
	for _, want := range []string{"tom@example.com", "owner", "acme"} {
		if !strings.Contains(msg.Detail, want) {
			t.Errorf("detail %q missing %q", msg.Detail, want)
		}
	}
	if !strings.Contains(msg.Warning, "identity itself is kept") {
		t.Errorf("warning=%q", msg.Warning)
	}
	if msg.Action == nil {
		t.Fatal("no action to run on yes")
	}
	if revokeBindingAction(nil)(RowData{UID: "x"}) != nil {
		t.Error("unexpected command for a malformed row")
	}
}

func TestAssumeAsksFirstAndNamesTheIdentity(t *testing.T) {
	t.Parallel()
	msg := confirmFrom(t, assumeBindingAction(bindingRow()))
	if msg.Prompt != "Switch to this identity?" {
		t.Errorf("prompt=%q", msg.Prompt)
	}
	if !strings.Contains(msg.Detail, "tom@example.com") {
		t.Errorf("detail=%q", msg.Detail)
	}
	// Switching replaces the cached token, so a running export cannot survive it.
	if !strings.Contains(msg.Warning, "export") {
		t.Errorf("warning=%q", msg.Warning)
	}
	// Answering yes only asks the App to switch; it does not itself log in.
	assume, ok := msg.Action().(AssumeIdentityMsg)
	if !ok {
		t.Fatalf("action returned %T, want AssumeIdentityMsg", msg.Action())
	}
	if assume.UID != "org/1/id-1" {
		t.Errorf("uid=%q", assume.UID)
	}

	// A binding with no identity to assume offers nothing.
	if assumeBindingAction(RowData{Raw: api.RoleBinding{UID: "org/1/rb"}}) != nil {
		t.Error("a binding with no identity should not offer to assume one")
	}
	if assumeIdentityCmd("", "x") != nil {
		t.Error("an empty identity should not offer to assume one")
	}
}

func TestIdentitiesPageActionsAndColumns(t *testing.T) {
	t.Parallel()
	p := NewIdentitiesPage(nil, "org/1")
	if p.rowActionFor("a") == nil || p.rowActionFor("D") == nil {
		t.Fatal("identities page should bind a and D")
	}
	var titles []string
	for _, c := range p.cols {
		titles = append(titles, c.Title)
	}
	want := []string{"NAME", "EMAIL", "TYPE", "LAST SEEN", "DESCRIPTION"}
	if fmt.Sprint(titles) != fmt.Sprint(want) {
		t.Fatalf("columns=%v, want %v", titles, want)
	}

	id := api.Identity{UID: "org/1/id-1", Name: "tom", Email: "tom@example.com"}
	msg := confirmFrom(t, assumeIdentityAction(RowData{Raw: id}))
	if !strings.Contains(msg.Detail, "tom") {
		t.Errorf("detail=%q", msg.Detail)
	}

	// An unverified address is shown, but marked as such.
	if got := identityEmail(api.Identity{EmailUnverified: "x@y.com"}); got != "x@y.com (unverified)" {
		t.Errorf("email=%q", got)
	}
	if got := identityEmail(api.Identity{}); got != "" {
		t.Errorf("email=%q, want empty", got)
	}
}

func TestDescribeBindings(t *testing.T) {
	t.Parallel()
	// The count is the blast radius, so the dialog has to state it.
	one := describeBindings([]api.RoleBinding{{Role: &api.RoleBindingRole{Name: "owner"}}})
	if one != "1 binding: owner" {
		t.Errorf("got %q", one)
	}
	two := describeBindings([]api.RoleBinding{
		{Role: &api.RoleBindingRole{Name: "owner"}},
		{Role: &api.RoleBindingRole{Name: "viewer"}},
	})
	if two != "2 bindings: owner, viewer" {
		t.Errorf("got %q", two)
	}
}

func TestShortIssuer(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"https://accounts.google.com":        "accounts.google.com",
		"https://token.actions.github.com/x": "token.actions.github.com",
		"":                                   "-",
	} {
		if got := shortIssuer(in); got != want {
			t.Errorf("shortIssuer(%q)=%q, want %q", in, got, want)
		}
	}
}

func TestActionDoneMsgVerbs(t *testing.T) {
	t.Parallel()
	p := testListPage(func(string, int, string, string) (PageResult, error) {
		return PageResult{Rows: rows("a")}, nil
	})
	m, cmd := p.Update(actionDoneMsg{done: revokedAccess, what: "tom@example.com"})
	p = m.(*ListPage)
	if !strings.Contains(p.saveMsg, "revoked access for tom@example.com") {
		t.Errorf("saveMsg=%q", p.saveMsg)
	}
	if cmd == nil || !p.loading {
		t.Error("a successful revoke should reload the list")
	}

	// Failures are reported in the same voice as the action.
	m, _ = p.Update(actionDoneMsg{done: revokedAccess, what: "tom", err: fmt.Errorf("denied")})
	if got := m.(*ListPage).saveMsg; !strings.Contains(got, "revoke failed") {
		t.Errorf("saveMsg=%q", got)
	}
	m, _ = p.Update(actionDoneMsg{done: "deleted", what: "nginx", err: fmt.Errorf("denied")})
	if got := m.(*ListPage).saveMsg; !strings.Contains(got, "delete failed") {
		t.Errorf("saveMsg=%q", got)
	}
}

func TestUsersCommandRouting(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{"users", "user", "members", "member", "u"} {
		page := resolveResourcePage(nil, cmd, "org/1", "acme")
		if page == nil {
			t.Errorf("%q resolved to nil", cmd)
			continue
		}
		if got := page.ResourceType(); got != "users" {
			t.Errorf("%q -> %q", cmd, got)
		}
	}
	// The footers have to advertise the new keys.
	for _, resource := range []string{"users", "identities"} {
		got := renderFooter(220, resource, true)
		for _, want := range []string{"assume", "revoke access"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s footer missing %q: %s", resource, want, got)
			}
		}
	}
}

// :login switches session rather than navigating, so it must not be routed to a
// page lookup.
func TestLoginCommandIsNotAPage(t *testing.T) {
	t.Parallel()
	for _, cmd := range []string{"login", "relogin", "whoami"} {
		if page := resolveResourcePage(nil, cmd, "org/1", "acme"); page != nil {
			t.Errorf("%q resolved to page %q, want session handling", cmd, page.ResourceType())
		}
	}
	a := sized(New(nil))
	a.cmdMode = true
	a.cmd.SetValue("login")
	m, cmd := a.handleCmdKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.(App).cmdMode {
		t.Error("command bar should close")
	}
	if cmd == nil {
		t.Fatal("no command returned")
	}
	if _, ok := cmd().(ReloginMsg); !ok {
		t.Fatalf("msg type %T, want ReloginMsg", cmd())
	}
}

// Declared column widths have to leave the last column room to stretch,
// otherwise the grid overruns the window and the selected row's highlight wraps
// onto the next line. See TestTableGridFillsWindowWidth for the mechanism.
func TestUserPageColumnsFitTheWindow(t *testing.T) {
	t.Parallel()
	pages := map[string]*ListPage{
		"users":      NewUsersPage(nil, "org/1", "acme"),
		"identities": NewIdentitiesPage(nil, "org/1"),
	}
	for name, p := range pages {
		declared := 0
		for _, c := range p.cols {
			declared += c.Width + tableCellStyle.GetHorizontalFrameSize()
		}
		if declared > 120 {
			t.Errorf("%s: declared grid is %d cells, too wide for a 120-column window", name, declared)
		}
		for _, width := range []int{120, 200} {
			p.SetSize(width, 24)
			grid := 0
			for _, c := range p.table.Columns() {
				grid += c.Width + tableCellStyle.GetHorizontalFrameSize()
			}
			if grid != width {
				t.Errorf("%s at %d: grid is %d cells wide", name, width, grid)
			}
		}
	}
}

package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tomsk73/chaintui/internal/api"
)

// NewUsersPage lists who has access to an org and with what role.
//
// Chainguard has no users resource: access is a role binding between an identity
// and a role, so that is what this lists. `a` assumes the selected identity and
// `D` revokes the binding.
func NewUsersPage(client *api.Client, orgUID, orgName string) *ListPage {
	// Widths are kept inside an 80+40 terminal: the grid is the sum of these
	// plus two per column, and only the last one stretches to fill. ISSUER goes
	// last because it is the column that benefits from spare width.
	cols := []table.Column{
		{Title: "USER", Width: 32},
		{Title: "ROLE", Width: 20},
		{Title: "SCOPE", Width: 16},
		{Title: "GRANTED", Width: 12},
		{Title: "ISSUER", Width: 22},
	}
	load := func(token string, pageSize int, _, orderBy string) (PageResult, error) {
		page, err := client.ListRoleBindings(orgUID, pageOpts(token, pageSize, "", orderBy))
		if err != nil {
			return PageResult{}, err
		}
		res := toPageResult(page, func(v api.RoleBinding) RowData {
			return RowData{
				UID: v.UID,
				Columns: []string{
					dash(v.Holder()),
					dash(v.RoleName()),
					dash(bindingScope(v)),
					relativeTime(v.CreateTime),
					bindingIssuer(v),
				},
				Raw: v,
			}
		})
		res.Status = userSummary(page.Items)
		return res, nil
	}
	label := "users"
	if orgName != "" {
		label = orgName + " users"
	}
	// No server sort mappings: which order_by fields this RPC accepts is
	// unverified, and guessing cost us InvalidArgument errors on advisories.
	return newListPage("users", orgUID, cols, load, nil).
		WithLabel(label).
		WithRowAction("a", assumeBindingAction).
		WithRowAction("D", revokeBindingAction(client))
}

// bindingScope names the group the binding applies to. Bindings inherit down the
// hierarchy, so a binding on a parent group is the reason someone has access to
// a folder they were never granted directly.
func bindingScope(b api.RoleBinding) string {
	if b.Group != nil {
		if b.Group.Name != "" {
			return b.Group.Name
		}
		return shortUID(b.Group.UID)
	}
	return ""
}

// bindingIssuer shows where a person authenticates from, and marks the rows that
// are workloads rather than people.
func bindingIssuer(b api.RoleBinding) string {
	if b.Identity == nil {
		return "-"
	}
	if !b.Identity.IsHuman() {
		return "(workload)"
	}
	return shortIssuer(b.Identity.Issuer)
}

// shortIssuer trims an issuer URL to its host, which is the part that says which
// IdP a person came from.
func shortIssuer(issuer string) string {
	s := strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://")
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "-"
	}
	return s
}

// userSummary counts people against workloads on the page, since a role binding
// list mixes the two.
func userSummary(bindings []api.RoleBinding) string {
	if len(bindings) == 0 {
		return ""
	}
	people := 0
	for _, b := range bindings {
		if b.Identity != nil && b.Identity.IsHuman() {
			people++
		}
	}
	return fmt.Sprintf("%d bindings  │  %d people  │  %d workloads",
		len(bindings), people, len(bindings)-people)
}

// revokeBindingAction asks for confirmation, then deletes one role binding.
func revokeBindingAction(client *api.Client) func(RowData) tea.Cmd {
	return func(row RowData) tea.Cmd {
		binding, ok := row.Raw.(api.RoleBinding)
		if !ok {
			return nil
		}
		who := binding.Holder()
		return func() tea.Msg {
			return ConfirmMsg{
				Prompt: "Are you sure you want to revoke this access?",
				Detail: who + "  —  " + binding.RoleName() + " on " + dash(bindingScope(binding)),
				// Say what survives, so this is not mistaken for deleting the user.
				Warning: "Removes this role binding. The identity itself is kept.",
				Action: func() tea.Msg {
					if err := client.DeleteRoleBinding(binding.UID); err != nil {
						return actionDoneMsg{done: revokedAccess, what: who, err: err}
					}
					return actionDoneMsg{done: revokedAccess, what: who}
				},
			}
		}
	}
}

// revokedAccess is the past tense used in the status line after a revoke.
const revokedAccess = "revoked access for"

// assumeBindingAction offers to log in as the identity holding a binding.
func assumeBindingAction(row RowData) tea.Cmd {
	binding, ok := row.Raw.(api.RoleBinding)
	if !ok {
		return nil
	}
	uid := binding.IdentityUID
	if uid == "" && binding.Identity != nil {
		uid = binding.Identity.UID
	}
	return assumeIdentityCmd(uid, binding.Holder())
}

// assumeIdentityCmd raises the confirmation for switching the logged-in session
// to another identity. The switch itself is the App's job: it has to give the
// terminal back to chainctl and rebuild the client afterwards.
func assumeIdentityCmd(identityUID, name string) tea.Cmd {
	if strings.TrimSpace(identityUID) == "" {
		return nil
	}
	if name == "" {
		name = shortUID(identityUID)
	}
	return func() tea.Msg {
		return ConfirmMsg{
			Prompt: "Switch to this identity?",
			Detail: name,
			// Everything about the current session goes: chainctl replaces the
			// cached token, so the old credentials stop working.
			Warning: "Logs in again as this identity. Any running export is cancelled.",
			Action: func() tea.Msg {
				return AssumeIdentityMsg{UID: identityUID, Name: name}
			},
		}
	}
}

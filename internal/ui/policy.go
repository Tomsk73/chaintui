package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/Tomsk73/chaintui/internal/api"
)

// cooldownDays formats an entitlement's cooldown window. 0 means the gate is off.
func cooldownDays(n int32) string {
	if n <= 0 {
		return "none"
	}
	return fmt.Sprintf("%dd", n)
}

// policyCooldown formats a policy's cooldown window: nil inherits the platform
// default of 7 days, 0 disables the gate.
func policyCooldown(n *int32) string {
	if n == nil {
		return "default (7d)"
	}
	if *n == 0 {
		return "off"
	}
	return fmt.Sprintf("%dd", *n)
}

// bindingPolicyNames joins the policies bound to an ecosystem, falling back to
// the policy UIDP when the policy itself is not visible to the caller.
func bindingPolicyNames(bindings []api.LibraryPolicyBinding) string {
	names := make([]string, 0, len(bindings))
	for _, b := range bindings {
		if b.PolicyName != "" {
			names = append(names, b.PolicyName)
			continue
		}
		names = append(names, shortUID(b.PolicyUID))
	}
	return strings.Join(names, ", ")
}

func bindingModes(bindings []api.LibraryPolicyBinding) string {
	modes := make([]string, 0, len(bindings))
	for _, b := range bindings {
		modes = append(modes, b.Mode)
	}
	return strings.Join(modes, ", ")
}

// NewLibraryPolicyMenuPage lists the org's Libraries policy views.
func NewLibraryPolicyMenuPage(client *api.Client, orgUID, orgName string) *ListPage {
	entries := []menuEntry{
		{"entitlements", "Ecosystems this org may pull, and from where", func() Page {
			return NewLibraryEntitlementsPage(client, orgUID)
		}},
		{"policies", "Gate configuration (cooldown, licences, block/allow)", func() Page {
			return NewLibraryPoliciesPage(client, orgUID)
		}},
		{"bindings", "Which policy is active per ecosystem", func() Page {
			return NewLibraryPolicyBindingsPage(client, orgUID)
		}},
		{"blocked", "Packages policy has withheld", func() Page {
			return NewLibraryBlockEventsPage(client, orgUID)
		}},
	}
	label := "libraries policy"
	if orgName != "" {
		label = orgName + " libraries policy"
	}
	return newMenuPage("librariespolicy", orgUID, label, entries)
}

// NewLibraryEntitlementsPage shows which library ecosystems an org may pull and
// whether upstream packages are included.
func NewLibraryEntitlementsPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "ECOSYSTEM", Width: 20},
		{Title: "ACCESS", Width: 22},
		{Title: "COOLDOWN", Width: 10},
		{Title: "SOURCE", Width: 10},
		{Title: "UID", Width: 20},
	}
	load := func(string, int, string, string) (PageResult, error) {
		ents, err := client.ListLibraryEntitlements(orgUID)
		if err != nil {
			return PageResult{}, err
		}
		rows := make([]RowData, len(ents))
		for i, e := range ents {
			rows[i] = RowData{
				UID: e.UID,
				Columns: []string{
					e.Ecosystem,
					e.Access,
					cooldownDays(e.CooldownDays),
					dash(e.Source),
					shortUID(e.UID),
				},
				Raw: e,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	return newListPage("entitlements", orgUID, cols, load, nil).WithLabel("entitlements")
}

// NewLibraryPoliciesPage lists the policies available to an org: Chainguard
// system policies plus the org's custom ones. Press d for the block/allow lists
// and, for system policies, the Rego expression.
func NewLibraryPoliciesPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 26},
		{Title: "TYPE", Width: 8},
		{Title: "COOLDOWN", Width: 12},
		{Title: "BLOCKED LICENCES", Width: 26},
		{Title: "BLOCK", Width: 6},
		{Title: "ALLOW", Width: 6},
		{Title: "UPDATED", Width: 12},
	}
	load := func(string, int, string, string) (PageResult, error) {
		policies, err := client.ListLibraryPolicies(orgUID)
		if err != nil {
			return PageResult{}, err
		}
		rows := make([]RowData, len(policies))
		for i, p := range policies {
			rows[i] = RowData{
				UID: p.UID,
				Columns: []string{
					p.Name,
					dash(p.Type),
					policyCooldown(p.CooldownDays),
					dash(truncate(strings.Join(p.BlockedLicenses, ", "), 60)),
					fmt.Sprintf("%d", len(p.BlockList)),
					fmt.Sprintf("%d", len(p.AllowList)),
					relativeTime(p.UpdateTime),
				},
				Raw: p,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	return newListPage("libraries-policies", orgUID, cols, load, nil).WithLabel("policies")
}

// NewLibraryPolicyBindingsPage shows which policy is active for each ecosystem
// and whether it blocks pulls (enforced) or only records them (log).
func NewLibraryPolicyBindingsPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "ECOSYSTEM", Width: 20},
		{Title: "MODE", Width: 10},
		{Title: "POLICY", Width: 28},
		{Title: "POLICY UID", Width: 20},
		{Title: "UPDATED", Width: 12},
	}
	load := func(string, int, string, string) (PageResult, error) {
		policy, err := client.GetLibraryOrgPolicy(orgUID)
		if err != nil {
			return PageResult{}, err
		}
		rows := make([]RowData, len(policy.Bindings))
		for i, b := range policy.Bindings {
			rows[i] = RowData{
				UID: b.UID,
				Columns: []string{
					b.Ecosystem,
					dash(b.Mode),
					dash(b.PolicyName),
					shortUID(b.PolicyUID),
					relativeTime(b.UpdateTime),
				},
				Raw: b,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	return newListPage("policybindings", orgUID, cols, load, nil).WithLabel("bindings")
}

// NewLibraryBlockEventsPage lists packages that policy withheld from the org.
// The API returns enforced blocks from the last 30 days by default; `l` switches
// to log-mode (shadow) violations and `/` filters by exact package name.
func NewLibraryBlockEventsPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "PACKAGE", Width: 30},
		{Title: "VERSION", Width: 16},
		{Title: "ECOSYSTEM", Width: 12},
		{Title: "REASON", Width: 10},
		{Title: "TRIES", Width: 6},
		{Title: "LAST BLOCKED", Width: 14},
		{Title: "UNBLOCKS", Width: 14},
	}
	logMode := false
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListLibraryBlockEvents(orgUID, pageOpts(token, pageSize, query, orderBy), logMode)
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(e api.LibraryBlockEvent) RowData {
			// A cooldown block carries a future unblock time; other reasons have none.
			unblocks := untilTime(e.UnblocksAt)
			return RowData{
				UID: e.UID,
				Columns: []string{
					e.Package,
					dash(e.Version),
					dash(e.Ecosystem),
					dash(e.Reason),
					fmt.Sprintf("%d", e.AttemptCount),
					relativeTime(e.LastBlockedAt),
					unblocks,
				},
				Raw: e,
			}
		}), nil
	}
	return newListPage("blocked", orgUID, cols, load, nil).
		WithLabel("blocked packages").
		WithServerNameFilter().
		WithBoolToggle("l", "log mode", &logMode).
		WithPageSize(25)
}

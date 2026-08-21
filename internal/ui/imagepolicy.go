package ui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/table"

	"github.com/Tomsk73/chaintui/internal/api"
)

// NewImagePolicyMenuPage lists the org's container policy views. Image policies
// govern what may be pulled from a repo, so this is the container counterpart of
// the libraries policy menu.
func NewImagePolicyMenuPage(client *api.Client, orgUID, orgName string) *ListPage {
	entries := []menuEntry{
		{"policies", "Rules images are evaluated against", func() Page {
			return NewImagePoliciesPage(client, orgUID)
		}},
		{"bindings", "Which policy is active where, and in what mode", func() Page {
			return NewImagePolicyBindingsPage(client, orgUID)
		}},
		{"decisions", "Allow/deny outcomes from image pulls", func() Page {
			return NewImagePolicyDecisionsPage(client, orgUID, orgUID, "")
		}},
		{"overrides", "Waivers granted for specific digests", func() Page {
			return NewImagePolicyOverridesPage(client, orgUID)
		}},
	}
	label := "container policy"
	if orgName != "" {
		label = orgName + " container policy"
	}
	return newMenuPage("containerpolicy", orgUID, label, entries)
}

// NewImagePoliciesPage lists the policies an org can bind: its own custom ones
// first, then Chainguard's system policies.
func NewImagePoliciesPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "TYPE", Width: 8},
		{Title: "APPLIES TO", Width: 14},
		{Title: "PARAMETERS", Width: 30},
		{Title: "DESCRIPTION", Width: 40},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListImagePolicies(orgUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.ImagePolicy) RowData {
			return RowData{
				UID: v.UID,
				Columns: []string{
					v.Name,
					dash(v.Type),
					dash(shortResourceType(v.ResourceType)),
					dash(policyParameterNames(v.Parameters)),
					truncate(dash(v.Description), 120),
				},
				Raw: v,
			}
		}), nil
	}
	return newListPage("imagepolicies", orgUID, cols, load, nil).
		WithLabel("image policies").
		WithServerNameFilter()
}

// NewImagePolicyBindingsPage shows which policies are switched on, over what
// scope, and whether they block or only log.
func NewImagePolicyBindingsPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "POLICY", Width: 30},
		{Title: "MODE", Width: 10},
		{Title: "SCOPE", Width: 20},
		{Title: "APPLIES TO", Width: 14},
		{Title: "PARAMETERS", Width: 40},
	}
	names := newPolicyNameCache(client, orgUID)
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListImagePolicyBindings(orgUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		// A binding names its policy by id only; the policy list supplies the name.
		byUID := names.lookup()
		return toPageResult(page, func(v api.ImagePolicyBinding) RowData {
			return RowData{
				UID: v.UID,
				Columns: []string{
					policyDisplayName(v.PolicyUID, v.PolicyName, byUID),
					dash(v.Mode),
					shortUID(v.Scope()),
					dash(shortResourceTypes(v.ResourceTypes)),
					dash(policyParameterPairs(v.Parameters)),
				},
				Raw: v,
			}
		}), nil
	}
	return newListPage("imagepolicybindings", orgUID, cols, load, nil).
		WithLabel("policy bindings")
}

// NewImagePolicyDecisionsPage lists pull-time policy outcomes under scopeUID.
// orgUID scopes the repo-name lookup; repoName, when set, names the single repo
// in scope and is used for the page label.
//
// `x` narrows to denials, which is the view that usually matters.
func NewImagePolicyDecisionsPage(client *api.Client, orgUID, scopeUID, repoName string) *ListPage {
	cols := []table.Column{
		{Title: "RESULT", Width: 8},
		{Title: "REPO", Width: 24},
		{Title: "DIGEST", Width: 20},
		{Title: "POLICY", Width: 26},
		{Title: "MODE", Width: 9},
		{Title: "PULLED", Width: 12},
		{Title: "REASON", Width: 40},
	}
	repos := newRepoNameCache(client, orgUID)
	deniedOnly := false
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListImagePolicyDecisions(scopeUID,
			api.ImagePolicyDecisionFilter{DeniedOnly: deniedOnly},
			pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		byUID := repos.lookup()
		res := toPageResult(page, func(v api.ImagePolicyDecision) RowData {
			repo := repoName
			if repo == "" {
				repo = repoDisplayName(v.RepoUID, byUID)
			}
			return RowData{
				UID: v.UID,
				Columns: []string{
					dash(v.Result),
					repo,
					shortDigest(v.Digest),
					dash(v.PolicyName),
					dash(v.Mode),
					decisionDay(v.PulledOn),
					truncate(dash(v.Reason), 120),
				},
				Raw: v,
			}
		})
		res.Status = decisionSummary(page.Items, deniedOnly)
		return res, nil
	}
	label := "policy decisions"
	if repoName != "" {
		label = repoName + " policy"
	}
	return newListPage("policydecisions", scopeUID, cols, load, nil).
		WithLabel(label).
		WithBoolToggle("x", "denied only", &deniedOnly)
}

// NewImagePolicyOverridesPage lists the waivers that let a specific image
// through despite a policy denying it.
func NewImagePolicyOverridesPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "DIGEST", Width: 20},
		{Title: "POLICY", Width: 30},
		{Title: "GRANTED BY", Width: 30},
		{Title: "GRANTED", Width: 14},
		{Title: "REASON", Width: 40},
	}
	names := newPolicyNameCache(client, orgUID)
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListImagePolicyOverrides(orgUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		byUID := names.lookup()
		return toPageResult(page, func(v api.ImagePolicyOverride) RowData {
			return RowData{
				UID: v.UID,
				Columns: []string{
					shortDigest(v.Digest),
					policyDisplayName(v.PolicyUID, v.PolicyName, byUID),
					dash(v.CreatedBy),
					relativeTime(v.CreateTime),
					truncate(dash(v.Reason), 120),
				},
				Raw: v,
			}
		}), nil
	}
	return newListPage("policyoverrides", orgUID, cols, load, nil).
		WithLabel("policy overrides")
}

// nameCache resolves a lookup table once and reuses it for later pages. A failed
// lookup is not cached, so r retries it; the ids it would have named simply show
// as ids in the meantime, which is worth more than failing the whole page.
type nameCache struct {
	mu    sync.Mutex
	names map[string]string
	fetch func() (map[string]string, error)
}

func (c *nameCache) lookup() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.names != nil {
		return c.names
	}
	names, err := c.fetch()
	if err != nil {
		return nil
	}
	c.names = names
	return names
}

func newPolicyNameCache(client *api.Client, orgUID string) *nameCache {
	return &nameCache{fetch: func() (map[string]string, error) { return client.ImagePolicyNames(orgUID) }}
}

func newRepoNameCache(client *api.Client, orgUID string) *nameCache {
	return &nameCache{fetch: func() (map[string]string, error) { return client.RepoNames(orgUID) }}
}

// policyDisplayName prefers the name the record carries, then the one from the
// policy list, and falls back to the id so a row is never blank.
func policyDisplayName(uid, name string, byUID map[string]string) string {
	if name != "" {
		return name
	}
	if n := byUID[uid]; n != "" {
		return n
	}
	return shortUID(uid)
}

func repoDisplayName(uid string, byUID map[string]string) string {
	if n := byUID[uid]; n != "" {
		return n
	}
	return shortUID(uid)
}

// shortResourceType drops the API prefix from a resource type, leaving the part
// that distinguishes it: "registry.chainguard.dev/Repo@v1" -> "Repo@v1".
func shortResourceType(t string) string {
	if i := strings.LastIndex(t, "/"); i >= 0 {
		return t[i+1:]
	}
	return t
}

func shortResourceTypes(types []string) string {
	out := make([]string, 0, len(types))
	for _, t := range types {
		out = append(out, shortResourceType(t))
	}
	return strings.Join(out, ", ")
}

func policyParameterNames(params []api.ImagePolicyParameter) string {
	out := make([]string, 0, len(params))
	for _, p := range params {
		name := p.Name
		if p.Required {
			name += "*"
		}
		out = append(out, name)
	}
	return strings.Join(out, ", ")
}

// policyParameterPairs renders a binding's configured values, sorted so the row
// does not reshuffle between refreshes.
func policyParameterPairs(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+params[k])
	}
	return strings.Join(pairs, ", ")
}

// decisionDay renders the day a pull was evaluated. The engine records a date
// rather than an instant, so relativeTime's "3h ago" would overstate precision.
func decisionDay(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02")
}

// decisionSummary tallies the outcomes on the current page.
func decisionSummary(items []api.ImagePolicyDecision, deniedOnly bool) string {
	if len(items) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, d := range items {
		counts[d.Result]++
	}
	parts := make([]string, 0, len(counts)+1)
	parts = append(parts, fmt.Sprintf("%d decisions", len(items)))
	for _, result := range []string{"denied", "allowed", "error"} {
		if n := counts[result]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s %d", result, n))
		}
	}
	if deniedOnly {
		parts = append(parts, "denied only")
	}
	return strings.Join(parts, "  │  ")
}

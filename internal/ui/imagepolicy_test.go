package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tomsk73/chaintui/internal/api"
)

func TestImagePolicyMenuEntries(t *testing.T) {
	t.Parallel()
	menu := NewImagePolicyMenuPage(nil, "org/1", "acme")
	if got := menu.Label(); got != "acme container policy" {
		t.Fatalf("label=%q", got)
	}
	page, err := menu.loadFn("", 50, "", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range page.Rows {
		names = append(names, r.Columns[0])
	}
	want := []string{"policies", "bindings", "decisions", "overrides"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("options=%v, want %v", names, want)
	}
	for _, r := range page.Rows {
		p := pushedPage(t, menu.enterFn(r))
		if p.GroupContext() != "org/1" {
			t.Errorf("%s: groupCtx=%q", r.Columns[0], p.GroupContext())
		}
	}
}

// Container policy has to use its own command prefixes: the plain policy words
// were already bound to the Libraries pages.
func TestImagePolicyCommands(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"cpolicy":         "containerpolicy",
		"containerpolicy": "containerpolicy",
		"imagepolicy":     "containerpolicy",
		"repopolicy":      "containerpolicy",
		"imagepolicies":   "imagepolicies",
		"cbindings":       "imagepolicybindings",
		"decisions":       "policydecisions",
		"overrides":       "policyoverrides",
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
	// The Libraries pages keep the words they already had.
	for cmd, want := range map[string]string{
		"policy":   "librariespolicy",
		"policies": "libraries-policies",
		"bindings": "policybindings",
	} {
		if got := resolveResourcePage(nil, cmd, "org/1", "acme").ResourceType(); got != want {
			t.Errorf("%q -> %q, want %q (libraries should keep it)", cmd, got, want)
		}
	}
}

func TestPolicyDisplayHelpers(t *testing.T) {
	t.Parallel()
	if got := shortResourceType("registry.chainguard.dev/Repo@v1"); got != "Repo@v1" {
		t.Errorf("shortResourceType=%q", got)
	}
	if got := shortResourceTypes([]string{"registry.chainguard.dev/Repo", "Other"}); got != "Repo, Other" {
		t.Errorf("shortResourceTypes=%q", got)
	}

	// A record's own name wins, then the policy list, then the bare id.
	byUID := map[string]string{"org/1/pol": "no-critical-cves"}
	if got := policyDisplayName("org/1/pol", "stamped", byUID); got != "stamped" {
		t.Errorf("got %q, want the stamped name", got)
	}
	if got := policyDisplayName("org/1/pol", "", byUID); got != "no-critical-cves" {
		t.Errorf("got %q, want the looked-up name", got)
	}
	if got := policyDisplayName("org/1/missing", "", byUID); got != "missing" {
		t.Errorf("got %q, want the short id", got)
	}
	if got := policyDisplayName("org/1/missing", "", nil); got != "missing" {
		t.Errorf("a failed lookup should still name the row, got %q", got)
	}

	// Parameters are sorted so a row does not reshuffle between refreshes.
	pairs := policyParameterPairs(map[string]string{"max": "5", "allow": "a, b"})
	if pairs != "allow=a, b, max=5" {
		t.Errorf("pairs=%q", pairs)
	}
	if got := policyParameterPairs(nil); got != "" {
		t.Errorf("pairs=%q, want empty", got)
	}
	if got := policyParameterNames([]api.ImagePolicyParameter{
		{Name: "severity", Required: true}, {Name: "allowlist"},
	}); got != "severity*, allowlist" {
		t.Errorf("names=%q, required should be marked", got)
	}
}

func TestDecisionDayAndSummary(t *testing.T) {
	t.Parallel()
	// The engine records the day of a pull, not the instant, so no "3h ago".
	if got := decisionDay(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)); got != "2026-08-20" {
		t.Errorf("day=%q", got)
	}
	if got := decisionDay(time.Time{}); got != "-" {
		t.Errorf("zero day=%q", got)
	}

	items := []api.ImagePolicyDecision{
		{Result: "denied"}, {Result: "allowed"}, {Result: "denied"}, {Result: "error"},
	}
	got := decisionSummary(items, false)
	for _, want := range []string{"4 decisions", "denied 2", "allowed 1", "error 1"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q missing %q", got, want)
		}
	}
	if strings.Contains(got, "denied only") {
		t.Errorf("summary should not claim a filter that is off: %q", got)
	}
	if got := decisionSummary(items, true); !strings.Contains(got, "denied only") {
		t.Errorf("summary should name the active filter: %q", got)
	}
	if got := decisionSummary(nil, false); got != "" {
		t.Errorf("empty page summary=%q", got)
	}
}

func TestDecisionsPageScoping(t *testing.T) {
	t.Parallel()
	// A repo-scoped page is scoped to the repo but labels itself with the name.
	repo := NewImagePolicyDecisionsPage(nil, "org/1", "org/1/repo", "nginx")
	if repo.GroupContext() != "org/1/repo" {
		t.Errorf("groupCtx=%q, want the repo", repo.GroupContext())
	}
	if repo.Label() != "nginx policy" {
		t.Errorf("label=%q", repo.Label())
	}
	org := NewImagePolicyDecisionsPage(nil, "org/1", "org/1", "")
	if org.Label() != "policy decisions" {
		t.Errorf("label=%q", org.Label())
	}
	if org.boolToggleKey != "x" {
		t.Errorf("denied-only toggle not bound: %q", org.boolToggleKey)
	}
}

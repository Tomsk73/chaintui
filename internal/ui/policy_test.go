package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/Tomsk73/chaintui/internal/api"
)

func TestCooldownFormatting(t *testing.T) {
	t.Parallel()
	if got := cooldownDays(0); got != "none" {
		t.Errorf("entitlement 0 = %q", got)
	}
	if got := cooldownDays(7); got != "7d" {
		t.Errorf("entitlement 7 = %q", got)
	}
	if got := policyCooldown(nil); got != "default (7d)" {
		t.Errorf("policy nil = %q", got)
	}
	zero, seven := int32(0), int32(7)
	if got := policyCooldown(&zero); got != "off" {
		t.Errorf("policy 0 = %q", got)
	}
	if got := policyCooldown(&seven); got != "7d" {
		t.Errorf("policy 7 = %q", got)
	}
}

func TestBindingSummaries(t *testing.T) {
	t.Parallel()
	bindings := []api.LibraryPolicyBinding{
		{PolicyName: "default-7d-cooldown", Mode: "enforced"},
		{PolicyUID: "org/abc123", Mode: "log"}, // policy not visible: fall back to UID
	}
	if got := bindingPolicyNames(bindings); got != "default-7d-cooldown, abc123" {
		t.Errorf("names=%q", got)
	}
	if got := bindingModes(bindings); got != "enforced, log" {
		t.Errorf("modes=%q", got)
	}
	if bindingPolicyNames(nil) != "" || bindingModes(nil) != "" {
		t.Error("no bindings should render empty")
	}
}

func TestLibraryPolicyMenuEntries(t *testing.T) {
	t.Parallel()
	p := NewLibraryPolicyMenuPage(nil, "org/1", "acme")
	if p.GroupContext() != "org/1" {
		t.Errorf("groupCtx=%q", p.GroupContext())
	}
	if p.Label() != "acme libraries policy" {
		t.Errorf("label=%q", p.Label())
	}
	res, err := p.loadFn("", 50, "", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range res.Rows {
		names = append(names, r.UID)
	}
	want := "entitlements policies bindings blocked"
	if got := strings.Join(names, " "); got != want {
		t.Errorf("entries=%q want %q", got, want)
	}
}

func TestUntilTime(t *testing.T) {
	t.Parallel()
	now := time.Now()
	cases := []struct {
		in   time.Time
		want string
	}{
		{time.Time{}, "-"},
		{now.Add(30 * time.Second), "in <1m"},
		{now.Add(20 * time.Minute), "in 19m"},
		{now.Add(5 * time.Hour), "in 4h"},
		{now.Add(5 * 24 * time.Hour), "in 4d"},
	}
	for _, tc := range cases {
		if got := untilTime(tc.in); got != tc.want {
			t.Errorf("untilTime(%v)=%q want %q", tc.in, got, tc.want)
		}
	}
	// A cooldown that has already elapsed reads as a past time, not a countdown.
	if got := untilTime(now.Add(-2 * time.Hour)); got != "2h ago" {
		t.Errorf("past unblock=%q", got)
	}
}

func TestLibrariesEcosystemPageWithoutOrg(t *testing.T) {
	t.Parallel()
	// Reachable via `:libraries` before an org is picked: the catalogue is global,
	// so the page still lists ecosystems, with policy columns blank.
	p := NewLibrariesEcosystemPage(nil, "", "")
	if p.Label() != "libraries" {
		t.Errorf("label=%q", p.Label())
	}
	res, err := p.loadFn("", 50, "", "")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(res.Rows) != 3 {
		t.Fatalf("rows=%d", len(res.Rows))
	}
	row := res.Rows[0]
	if row.UID != "java" {
		t.Errorf("first row=%q", row.UID)
	}
	for i, col := range row.Columns[1:6] {
		if col != "-" {
			t.Errorf("column %d = %q, want %q with no org", i+1, col, "-")
		}
	}
}

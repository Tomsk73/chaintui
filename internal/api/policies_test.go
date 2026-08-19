package api

import (
	"fmt"
	"testing"

	capabilities "chainguard.dev/sdk/proto/capabilities"
)

func TestChartCatalogFolder(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"charts", "Charts", " charts ", "iamguarded-charts", "community-charts"} {
		if !chartCatalogFolder(name) {
			t.Errorf("%q should be a chart folder", name)
		}
	}
	for _, name := range []string{"skills", "uploads", "chartsx", "chart", ""} {
		if chartCatalogFolder(name) {
			t.Errorf("%q should not be a chart folder", name)
		}
	}
}

func TestEcosystemStatuses(t *testing.T) {
	t.Parallel()
	pol := LibraryOrgPolicy{
		Entitlements: []LibraryEntitlement{
			{UID: "o/1", Ecosystem: "python", Access: "chainguard+upstream", CooldownDays: 7},
			{UID: "o/2", Ecosystem: "java-athena", Access: "chainguard"},
		},
		Bindings: []LibraryPolicyBinding{
			{UID: "o/b1", Ecosystem: "python", Mode: "enforced", PolicyName: "default"},
			{UID: "o/b2", Ecosystem: "python", Mode: "log", PolicyName: "shadow"},
			{UID: "o/b3", Ecosystem: "java", Mode: "enforced", PolicyName: "default"},
		},
	}
	got := pol.EcosystemStatuses([]string{"java", "python", "javascript"})
	if len(got) != 3 {
		t.Fatalf("got %d statuses", len(got))
	}

	// An Athena entitlement counts towards its base ecosystem.
	if got[0].Ecosystem != "java" || got[0].Entitlement == nil || got[0].Entitlement.Ecosystem != "java-athena" {
		t.Fatalf("java: %+v", got[0])
	}
	if len(got[0].Bindings) != 1 {
		t.Fatalf("java bindings: %+v", got[0].Bindings)
	}
	if got[1].Entitlement == nil || got[1].Entitlement.CooldownDays != 7 {
		t.Fatalf("python: %+v", got[1])
	}
	if len(got[1].Bindings) != 2 {
		t.Fatalf("python should keep both bindings: %+v", got[1].Bindings)
	}
	// Not entitled: no entitlement, no bindings.
	if got[2].Entitlement != nil || len(got[2].Bindings) != 0 {
		t.Fatalf("javascript: %+v", got[2])
	}
}

func TestEcosystemStatusesPrefersExactOverAthena(t *testing.T) {
	t.Parallel()
	pol := LibraryOrgPolicy{Entitlements: []LibraryEntitlement{
		{UID: "o/athena", Ecosystem: "python-athena", Access: "chainguard"},
		{UID: "o/plain", Ecosystem: "python", Access: "chainguard+upstream"},
	}}
	got := pol.EcosystemStatuses([]string{"python"})
	if got[0].Entitlement == nil || got[0].Entitlement.UID != "o/plain" {
		t.Fatalf("want the exact-match entitlement, got %+v", got[0].Entitlement)
	}
}

func TestLibEcosystemLabelsAndBase(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"java-athena": "java",
		"python":      "python",
		"dotnet":      "dotnet",
	}
	for in, want := range cases {
		if got := baseEcosystem(in); got != want {
			t.Errorf("baseEcosystem(%q)=%q want %q", in, got, want)
		}
	}
	if got := fmt.Sprint(libEcosystemLabel(0)); got != "unknown" {
		t.Errorf("unknown ecosystem label=%q", got)
	}
}

func TestUnescapePURLPart(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"2.24.0%2Bcgr.1": "2.24.0+cgr.1",
		"%40scope%2Fpkg": "@scope/pkg",
		"1.2.3":          "1.2.3",
		"100%":           "100%", // invalid encoding is left alone
	}
	for in, want := range cases {
		if got := unescapePURLPart(in); got != want {
			t.Errorf("unescapePURLPart(%q)=%q want %q", in, got, want)
		}
	}
}

func TestCapabilityNameFormatting(t *testing.T) {
	t.Parallel()
	cases := map[capabilities.Capability]string{
		capabilities.Capability_CAP_ADVISORIES_CREATE: "advisories.create",
		capabilities.Capability_CAP_INTERNAL:          "internal",
	}
	for in, want := range cases {
		if got := capabilityName(in); got != want {
			t.Errorf("capabilityName(%v)=%q want %q", in, got, want)
		}
	}
}

func TestRoleScopeHelpers(t *testing.T) {
	t.Parallel()
	// Built-in roles are root-level UIDPs; an org's roles are nested under it.
	if !isManagedRole("8ded3b7af5361980a27e53bd8b09e644c04a34cd") {
		t.Error("root-level UIDP should be managed")
	}
	if isManagedRole("org1/7991174e62b8a4e7") {
		t.Error("nested UIDP should not be managed")
	}

	if !inGroup("org1/role1", "org1") {
		t.Error("direct child should be in group")
	}
	if !inGroup("org1/folder/role1", "org1") {
		t.Error("nested role should be in group")
	}
	if inGroup("org2/role1", "org1") {
		t.Error("another org's role should not be in group")
	}
	// A UIDP that merely shares a prefix is not inside the group.
	if inGroup("org1extra/role1", "org1") {
		t.Error("prefix collision should not match")
	}
	if !inGroup("anything", "") {
		t.Error("no group selected should match everything")
	}
}

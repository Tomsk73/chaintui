package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Tomsk73/chaintui/internal/api"
)

func TestFixedIn(t *testing.T) {
	t.Parallel()
	cases := []struct {
		cve  api.ImageCVE
		want string
	}{
		{api.ImageCVE{FixVersion: "1.2.4", FixState: "fixed"}, "1.2.4"},
		{api.ImageCVE{FixState: "wont-fix"}, "wont-fix"},
		{api.ImageCVE{FixState: "not-fixed"}, "not-fixed"},
		{api.ImageCVE{FixState: "unknown"}, "-"},
		{api.ImageCVE{}, "-"},
	}
	for _, tc := range cases {
		if got := fixedIn(tc.cve); got != tc.want {
			t.Errorf("fixedIn(%+v)=%q want %q", tc.cve, got, tc.want)
		}
	}
}

func TestImageRef(t *testing.T) {
	t.Parallel()
	digest := "sha256:6244b69385c9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5"
	if got := imageRef("redis", "latest", ""); got != "redis:latest" {
		t.Errorf("repo+tag=%q", got)
	}
	if got := imageRef("redis", "", digest); got != "redis" {
		t.Errorf("repo only=%q", got)
	}
	if got := imageRef("", "", digest); !strings.HasPrefix(got, "sha256:") {
		t.Errorf("digest only=%q", got)
	}
	if got := imageRef("", "", ""); got != "image" {
		t.Errorf("nothing=%q", got)
	}
}

func TestCVESummary(t *testing.T) {
	t.Parallel()
	report := api.ImageVulnReport{
		Scanner:        "grype",
		ScannerVersion: "0.116.1",
		ScannedAt:      time.Now().Add(-2 * time.Hour),
		CVEs: []api.ImageCVE{
			{ID: "CVE-1", Severity: "high"},
			{ID: "CVE-2", Severity: "high", FixVersion: "1.0.1"},
			{ID: "CVE-3", Severity: "low"},
		},
	}
	full := cveSummary(report, report.CVEs)
	for _, want := range []string{"3 CVEs", "high 2", "low 1", "grype 0.116.1", "scanned 2h ago"} {
		if !strings.Contains(full, want) {
			t.Errorf("summary %q missing %q", full, want)
		}
	}

	// A filtered view must not claim counts the table is not showing.
	filtered := cveSummary(report, report.CVEs[1:2])
	if !strings.Contains(filtered, "1 of 3 CVEs") {
		t.Errorf("filtered summary=%q", filtered)
	}
	if strings.Contains(filtered, "low 1") {
		t.Errorf("filtered summary should only count shown rows: %q", filtered)
	}

	clean := cveSummary(api.ImageVulnReport{Scanner: "grype"}, nil)
	if !strings.Contains(clean, "0 CVEs") {
		t.Errorf("clean summary=%q", clean)
	}
}

func TestDescribeScanError(t *testing.T) {
	t.Parallel()
	tagErr := describeScanError(fmt.Errorf("%w for helm:latest", api.ErrNoScanReport), "latest", "")
	if !strings.Contains(tagErr.Error(), "Press v on a tag") {
		t.Errorf("tag path should suggest picking a tag: %v", tagErr)
	}
	digestErr := describeScanError(fmt.Errorf("%w for sha256:abc", api.ErrNoScanReport), "", "sha256:abc")
	if !strings.Contains(digestErr.Error(), "has not been scanned") {
		t.Errorf("digest path: %v", digestErr)
	}
	// Anything else is passed through untouched.
	other := fmt.Errorf("boom")
	if describeScanError(other, "latest", "") != other {
		t.Error("unrelated errors should pass through unchanged")
	}
}

func TestReposAndTagsOfferCVEs(t *testing.T) {
	t.Parallel()
	repos := NewReposPage(nil, "org/1")
	if fn := repos.rowActionFor("v"); fn == nil {
		t.Fatal("repos page should bind v")
	}
	tags := NewTagsPage(nil, "org/1/repo", "redis")
	if fn := tags.rowActionFor("v"); fn == nil {
		t.Fatal("tags page should bind v")
	}
	// v on a repo row opens the CVE page for that repo's latest image.
	cmd := repos.rowActionFor("v")(RowData{UID: "org/1/repo", Raw: api.Repo{UID: "org/1/repo", Name: "redis"}})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	msg, ok := cmd().(PushMsg)
	if !ok {
		t.Fatalf("msg type %T", cmd())
	}
	if msg.P.ResourceType() != "cves" || msg.P.Label() != "redis:latest cves" {
		t.Fatalf("pushed %q labelled %q", msg.P.ResourceType(), msg.P.Label())
	}
	// A row whose Raw is not a Repo is ignored rather than panicking.
	if repos.rowActionFor("v")(RowData{UID: "x"}) != nil {
		t.Error("unexpected command for a malformed row")
	}
}

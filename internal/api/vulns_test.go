package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// grypeFixture mirrors the shape Chainguard's grype reports come back in, with a
// fixed CVE, an unfixed one, a repeat of the same CVE in a second package, and an
// exact duplicate match from another layer.
const grypeFixture = `{
  "matches": [
    {
      "vulnerability": {
        "id": "CVE-2026-1111",
        "severity": "High",
        "description": "openssl thing",
        "dataSource": "https://nvd.nist.gov/vuln/detail/CVE-2026-1111",
        "fix": {"versions": [], "state": "unknown"},
        "cvss": [{"version": "3.1", "metrics": {"baseScore": 7.5}}]
      },
      "artifact": {"name": "libssl3", "version": "3.6.3-r4", "purl": "pkg:apk/wolfi/openssl@3.6.3-r4",
        "locations": [{"path": "/usr/lib/apk/db/installed"}]}
    },
    {
      "vulnerability": {
        "id": "CVE-2026-2222",
        "severity": "Critical",
        "description": "very bad",
        "fix": {"versions": ["1.2.4", "1.3.0"], "state": "fixed"},
        "urls": ["https://example.test/CVE-2026-2222"],
        "cvss": [{"version": "3.1", "metrics": {"baseScore": 8.1}}, {"version": "4.0", "metrics": {"baseScore": 9.8}}]
      },
      "artifact": {"name": "libfoo", "version": "1.2.3", "purl": "pkg:apk/wolfi/libfoo@1.2.3"}
    },
    {
      "vulnerability": {"id": "CVE-2026-1111", "severity": "High", "fix": {"state": "wont-fix"}},
      "artifact": {"name": "libcrypto3", "version": "3.6.3-r4", "purl": "pkg:apk/wolfi/openssl@3.6.3-r4x"}
    },
    {
      "vulnerability": {"id": "CVE-2026-1111", "severity": "High", "fix": {"versions": [], "state": "unknown"}},
      "artifact": {"name": "libssl3", "version": "3.6.3-r4", "purl": "pkg:apk/wolfi/openssl@3.6.3-r4",
        "locations": [{"path": "/usr/lib/apk/db/installed"}]}
    },
    {
      "vulnerability": {"id": "CVE-2025-9999", "severity": "Low", "fix": {"state": "not-fixed"}},
      "artifact": {"name": "redis", "version": "8.4.6-r0"}
    }
  ]
}`

func TestParseGrypeReport(t *testing.T) {
	t.Parallel()
	got, err := parseGrypeReport([]byte(grypeFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// The exact duplicate match is dropped; the same CVE against another package stays.
	if len(got) != 4 {
		t.Fatalf("got %d CVEs: %+v", len(got), got)
	}

	// Worst severity first.
	wantOrder := []string{"CVE-2026-2222", "CVE-2026-1111", "CVE-2026-1111", "CVE-2025-9999"}
	for i, want := range wantOrder {
		if got[i].ID != want {
			t.Fatalf("position %d = %s, want %s", i, got[i].ID, want)
		}
	}

	crit := got[0]
	if crit.Severity != "critical" || crit.Package != "libfoo" || crit.Version != "1.2.3" {
		t.Fatalf("critical row: %+v", crit)
	}
	// Highest CVSS base score across entries wins.
	if crit.CVSS != "9.8" {
		t.Fatalf("cvss=%q want 9.8", crit.CVSS)
	}
	// First fixed version, and the URL falls back to urls[] when dataSource is absent.
	if crit.FixVersion != "1.2.4" || crit.FixState != "fixed" || !crit.Fixable() {
		t.Fatalf("fix: %+v", crit)
	}
	if crit.URL != "https://example.test/CVE-2026-2222" {
		t.Fatalf("url=%q", crit.URL)
	}

	// Within a severity the two openssl rows are ordered by package name.
	if got[1].Package != "libcrypto3" || got[2].Package != "libssl3" {
		t.Fatalf("package order: %q then %q", got[1].Package, got[2].Package)
	}
	// wont-fix is reported as the state, and is not fixable.
	if got[1].FixState != "wont-fix" || got[1].Fixable() {
		t.Fatalf("wont-fix row: %+v", got[1])
	}
	// Location and dataSource URL come through on the row that carries them.
	if got[2].Location != "/usr/lib/apk/db/installed" || got[2].CVSS != "7.5" ||
		got[2].URL != "https://nvd.nist.gov/vuln/detail/CVE-2026-1111" || got[2].Fixable() {
		t.Fatalf("libssl3 row: %+v", got[2])
	}
	if got[3].Severity != "low" || got[3].Fixable() {
		t.Fatalf("low row: %+v", got[3])
	}
}

func TestParseGrypeReportEmptyAndInvalid(t *testing.T) {
	t.Parallel()
	got, err := parseGrypeReport([]byte(`{"matches":[]}`))
	if err != nil || len(got) != 0 {
		t.Fatalf("clean image: %d CVEs err=%v", len(got), err)
	}
	if _, err := parseGrypeReport([]byte("not json")); err == nil {
		t.Fatal("expected a parse error")
	}
}

const trivyFixture = `{
  "Results": [
    {
      "Target": "app/package-lock.json",
      "Vulnerabilities": [
        {
          "VulnerabilityID": "CVE-2026-3333",
          "PkgName": "lodash",
          "PkgIdentifier": {"PURL": "pkg:npm/lodash@4.17.20"},
          "InstalledVersion": "4.17.20",
          "FixedVersion": "4.17.21",
          "Status": "fixed",
          "Severity": "MEDIUM",
          "Title": "prototype pollution",
          "PrimaryURL": "https://avd.aquasec.com/nvd/cve-2026-3333",
          "CVSS": {"nvd": {"V3Score": 5.3}, "redhat": {"V3Score": 6.1}}
        },
        {
          "VulnerabilityID": "CVE-2026-4444",
          "PkgName": "openssl",
          "InstalledVersion": "3.0.0",
          "Severity": "CRITICAL",
          "Description": "fallback description",
          "CVSS": {"nvd": {"V2Score": 10}}
        }
      ]
    }
  ]
}`

func TestParseTrivyReport(t *testing.T) {
	t.Parallel()
	got, err := parseTrivyReport([]byte(trivyFixture))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d: %+v", len(got), got)
	}
	if got[0].ID != "CVE-2026-4444" || got[0].Severity != "critical" || got[0].CVSS != "10" {
		t.Fatalf("first row: %+v", got[0])
	}
	// Title is preferred as the description; Description is the fallback.
	if got[0].Description != "fallback description" {
		t.Fatalf("description=%q", got[0].Description)
	}
	med := got[1]
	if med.Severity != "medium" || med.CVSS != "6.1" || med.FixVersion != "4.17.21" || !med.Fixable() {
		t.Fatalf("medium row: %+v", med)
	}
	if med.Description != "prototype pollution" || med.Location != "app/package-lock.json" {
		t.Fatalf("medium detail: %+v", med)
	}
}

func TestSeverityCounts(t *testing.T) {
	t.Parallel()
	report := ImageVulnReport{CVEs: []ImageCVE{
		{ID: "a", Severity: "low"},
		{ID: "b", Severity: "critical"},
		{ID: "c", Severity: "high"},
		{ID: "d", Severity: "high"},
		{ID: "e", Severity: ""},
	}}
	got := report.SeverityCounts()
	want := "critical 1, high 2, low 1,  1"
	parts := make([]string, 0, len(got))
	for _, c := range got {
		parts = append(parts, fmt.Sprintf("%s %d", c.Severity, c.Count))
	}
	if strings.Join(parts, ", ") != want {
		t.Fatalf("counts=%q want %q", strings.Join(parts, ", "), want)
	}
}

func TestFetchRawReport(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			fmt.Fprint(w, `{"matches":[]}`)
		default:
			w.WriteHeader(http.StatusForbidden)
		}
	}))
	defer srv.Close()

	body, err := fetchRawReport(context.Background(), srv.URL+"/ok")
	if err != nil || string(body) != `{"matches":[]}` {
		t.Fatalf("body=%q err=%v", body, err)
	}
	if _, err := fetchRawReport(context.Background(), srv.URL+"/denied"); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestShortDigest(t *testing.T) {
	t.Parallel()
	digest := "sha256:6244b69385c9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5"
	got := shortDigest(digest)
	if !strings.HasPrefix(got, "sha256:") || !strings.Contains(got, "...") || len(got) != 19 {
		t.Fatalf("shortDigest=%q", got)
	}
	if shortDigest("short") != "short" {
		t.Fatal("short input should pass through")
	}
}

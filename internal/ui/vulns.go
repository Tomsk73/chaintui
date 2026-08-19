package ui

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"

	"github.com/Tomsk73/chaintui/internal/api"
)

// NewImageCVEsPage lists the CVEs in one image, worst severity first. Pass a
// digest for an exact image, or leave it empty to use the repo's tag (which
// defaults to "latest").
//
// `f` narrows the list to CVEs with a known fix, `s` saves it as CSV, `d` shows
// the full record for a row, and `/` filters locally across all columns (so
// "critical" or a package name both work).
func NewImageCVEsPage(client *api.Client, repoUID, repoName, tag, digest string) *ListPage {
	cols := []table.Column{
		{Title: "CVE", Width: 20},
		{Title: "SEVERITY", Width: 11},
		{Title: "CVSS", Width: 6},
		{Title: "PACKAGE", Width: 26},
		{Title: "VERSION", Width: 18},
		{Title: "FIXED IN", Width: 18},
		{Title: "DESCRIPTION", Width: 30},
	}
	fixableOnly := false
	load := func(string, int, string, string) (PageResult, error) {
		report, err := client.ListImageCVEs(repoUID, repoName, tag, digest)
		if err != nil {
			return PageResult{}, describeScanError(err, tag, digest)
		}
		shown := make([]api.ImageCVE, 0, len(report.CVEs))
		for _, cve := range report.CVEs {
			if fixableOnly && !cve.Fixable() {
				continue
			}
			shown = append(shown, cve)
		}
		rows := make([]RowData, 0, len(shown))
		for _, cve := range shown {
			rows = append(rows, RowData{
				UID: cve.ID + " " + cve.Purl,
				Columns: []string{
					cve.ID,
					dash(cve.Severity),
					dash(cve.CVSS),
					dash(cve.Package),
					dash(cve.Version),
					fixedIn(cve),
					truncate(cve.Description, 120),
				},
				Raw: cve,
			})
		}
		return PageResult{
			Rows:       rows,
			TotalCount: int64(len(rows)),
			Status:     cveSummary(report, shown),
		}, nil
	}
	save := func(filename string, rows []RowData) error {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		w := csv.NewWriter(f)
		w.Write([]string{"cve", "severity", "cvss", "package", "version", "fixed_in", "fix_state", "purl", "location", "url", "description"}) //nolint
		for _, row := range rows {
			cve, ok := row.Raw.(api.ImageCVE)
			if !ok {
				continue
			}
			w.Write([]string{ //nolint
				cve.ID, cve.Severity, cve.CVSS, cve.Package, cve.Version,
				cve.FixVersion, cve.FixState, cve.Purl, cve.Location, cve.URL, cve.Description,
			})
		}
		w.Flush()
		return w.Error()
	}
	return newListPage("cves", repoUID, cols, load, nil).
		WithLabel(imageRef(repoName, tag, digest)+" cves").
		WithBoolToggle("f", "fixable only", &fixableOnly).
		WithSave(save)
}

// describeScanError turns "no scan report" into something actionable: most often
// the repo simply has no image under that tag, and another tag will have one.
func describeScanError(err error, tag, digest string) error {
	if !errors.Is(err, api.ErrNoScanReport) {
		return err
	}
	if digest == "" && tag != "" {
		return fmt.Errorf("%w — no %s image, or it has not been scanned. Press v on a tag to pick an image", err, tag)
	}
	return fmt.Errorf("%w — this image has not been scanned", err)
}

// fixedIn describes a CVE's fix: the version when known, otherwise the scanner's
// state (not-fixed, wont-fix, unknown).
func fixedIn(cve api.ImageCVE) string {
	if cve.FixVersion != "" {
		return cve.FixVersion
	}
	switch cve.FixState {
	case "", "unknown":
		return "-"
	default:
		return cve.FixState
	}
}

// imageRef names the image a CVE list belongs to, for the breadcrumb.
func imageRef(repoName, tag, digest string) string {
	switch {
	case repoName != "" && tag != "":
		return repoName + ":" + tag
	case repoName != "":
		return repoName
	case digest != "":
		return shortDigest(digest)
	default:
		return "image"
	}
}

// shortDigest abbreviates a digest for display, as the tags list does.
func shortDigest(digest string) string {
	if len(digest) > 19 {
		return digest[:7] + "..." + digest[len(digest)-9:]
	}
	return digest
}

// cveSummary is the one-line scan summary shown under a CVE list. shown is the
// subset currently listed, so a filtered view reports "2 of 6" rather than
// claiming counts the table is not showing.
func cveSummary(report api.ImageVulnReport, shown []api.ImageCVE) string {
	count := fmt.Sprintf("%d CVEs", len(shown))
	if len(shown) != len(report.CVEs) {
		count = fmt.Sprintf("%d of %d CVEs", len(shown), len(report.CVEs))
	}
	parts := []string{count}
	for _, c := range (api.ImageVulnReport{CVEs: shown}).SeverityCounts() {
		parts = append(parts, fmt.Sprintf("%s %d", c.Severity, c.Count))
	}
	scanner := report.Scanner
	if report.ScannerVersion != "" {
		scanner += " " + report.ScannerVersion
	}
	if scanner != "" {
		parts = append(parts, scanner)
	}
	if !report.ScannedAt.IsZero() {
		parts = append(parts, "scanned "+relativeTime(report.ScannedAt))
	}
	return strings.Join(parts, "  │  ")
}

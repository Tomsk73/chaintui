package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	registryv1 "chainguard.dev/sdk/proto/platform/registry/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ErrNoScanReport means the image has no vulnerability scan to read: it was
// never scanned, or the tag does not exist.
var ErrNoScanReport = errors.New("no scan report")

// maxRawReportBytes caps a raw scan report download. node-sized reports run to a
// few MB; this is a sanity bound, not a real limit.
const maxRawReportBytes = 64 << 20

// rawReportTimeout bounds the direct download of a raw report.
const rawReportTimeout = 60 * time.Second

// ImageVulnReport is one image's CVE list, taken from the newest scan report.
type ImageVulnReport struct {
	Repo   string `json:"repo"`
	Tag    string `json:"tag,omitempty"`
	Digest string `json:"digest"`
	// Scanner and ScannerVersion identify the scanner that produced the report.
	Scanner        string     `json:"scanner"`
	ScannerVersion string     `json:"scannerVersion,omitempty"`
	DBBuildTime    time.Time  `json:"dbBuildTime,omitempty"`
	ScannedAt      time.Time  `json:"scannedAt"`
	CVEs           []ImageCVE `json:"cves"`
}

// ImageCVE is one vulnerability match: a CVE against a specific package in the
// image. The same CVE appears once per affected package.
type ImageCVE struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	// CVSS is the highest base score reported, blank when the scanner gave none.
	CVSS        string `json:"cvss,omitempty"`
	Package     string `json:"package,omitempty"`
	Version     string `json:"version,omitempty"`
	FixVersion  string `json:"fixVersion,omitempty"`
	FixState    string `json:"fixState,omitempty"`
	Purl        string `json:"purl,omitempty"`
	Location    string `json:"location,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

// Fixable reports whether the scanner knows of a fixed version.
func (c ImageCVE) Fixable() bool {
	return c.FixVersion != "" || c.FixState == "fixed"
}

// severityRank orders severities worst-first for display.
func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 0
	case "high":
		return 1
	case "medium":
		return 2
	case "low":
		return 3
	case "negligible":
		return 4
	default:
		return 5
	}
}

// SeverityCounts tallies the report's CVEs by severity, worst first.
func (r ImageVulnReport) SeverityCounts() []struct {
	Severity string
	Count    int
} {
	counts := map[string]int{}
	for _, c := range r.CVEs {
		counts[strings.ToLower(c.Severity)]++
	}
	out := make([]struct {
		Severity string
		Count    int
	}, 0, len(counts))
	for sev, n := range counts {
		out = append(out, struct {
			Severity string
			Count    int
		}{sev, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if severityRank(out[i].Severity) != severityRank(out[j].Severity) {
			return severityRank(out[i].Severity) < severityRank(out[j].Severity)
		}
		return out[i].Severity < out[j].Severity
	})
	return out
}

// ListImageCVEs returns the CVEs for one image. Pass a digest for an exact
// image, or a repo UIDP plus tag (tag defaults to "latest").
//
// The vuln report list RPC does not populate its vulnerabilities field, so this
// finds the newest report for the image and parses that scanner's raw output.
// Reports above the inline size limit come back as a signed URL, which is
// downloaded instead.
func (c *Client) ListImageCVEs(repoUID, repoName, tag, digest string) (ImageVulnReport, error) {
	if strings.TrimSpace(repoUID) == "" && strings.TrimSpace(digest) == "" {
		return ImageVulnReport{}, fmt.Errorf("repo or digest is required")
	}
	if digest == "" && tag == "" {
		tag = "latest"
	}
	ctx := context.Background()
	filter := &registryv1.VulnReportFilter{}
	if digest != "" {
		filter.Digest = digest
	} else {
		filter.RepoId = repoUID
		filter.Tag = tag
	}
	ref := repoName
	if tag != "" {
		ref += ":" + tag
	}
	if digest != "" {
		ref = shortDigest(digest)
	}
	resp, err := c.platform.Registry().Vulnerabilities().ListVulnReports(ctx, filter)
	if err != nil {
		// An unscanned image, or a tag that does not exist, comes back as NotFound.
		if status.Code(err) == codes.NotFound {
			return ImageVulnReport{}, fmt.Errorf("%w for %s", ErrNoScanReport, ref)
		}
		return ImageVulnReport{}, err
	}
	newest := newestReport(resp.GetItems())
	if newest == nil {
		return ImageVulnReport{}, fmt.Errorf("%w for %s", ErrNoScanReport, ref)
	}

	out := ImageVulnReport{
		Repo:           repoName,
		Tag:            tag,
		Digest:         newest.GetDigest(),
		Scanner:        strings.ToLower(newest.GetScanner().GetName().String()),
		ScannerVersion: newest.GetScanner().GetVersion(),
		DBBuildTime:    tsTime(newest.GetScanner().GetDbBuildTime()),
		ScannedAt:      tsTime(newest.GetCreatedAt()),
	}

	raw, err := c.platform.Registry().Vulnerabilities().GetRawVulnReport(ctx, &registryv1.GetRawVulnReportRequest{
		Digest:  newest.GetDigest(),
		Scanner: newest.GetScanner().GetName(),
	})
	if err != nil {
		return ImageVulnReport{}, fmt.Errorf("raw report: %w", err)
	}
	body := []byte(raw.GetRawReport())
	if len(body) == 0 && raw.GetRawReportUrl() != "" {
		body, err = fetchRawReport(ctx, raw.GetRawReportUrl())
		if err != nil {
			return ImageVulnReport{}, fmt.Errorf("download raw report: %w", err)
		}
	}
	if len(body) == 0 {
		// A clean image still gets a report; there is simply nothing in it.
		return out, nil
	}

	cves, err := parseRawReport(newest.GetScanner().GetName(), body)
	if err != nil {
		return ImageVulnReport{}, err
	}
	out.CVEs = cves
	return out, nil
}

// newestReport picks the most recently created report, preferring Grype when two
// scanners ran at the same time (its raw format is the one Chainguard produces).
func newestReport(items []*registryv1.VulnReport) *registryv1.VulnReport {
	var best *registryv1.VulnReport
	for _, r := range items {
		if best == nil {
			best = r
			continue
		}
		rt, bt := tsTime(r.GetCreatedAt()), tsTime(best.GetCreatedAt())
		if rt.After(bt) {
			best = r
			continue
		}
		if rt.Equal(bt) && r.GetScanner().GetName() == registryv1.Scanner_GRYPE {
			best = r
		}
	}
	return best
}

func fetchRawReport(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, rawReportTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	// The URL is pre-signed, so no Chainguard credentials are attached.
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %s", resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxRawReportBytes))
}

func parseRawReport(scanner registryv1.Scanner_Name, body []byte) ([]ImageCVE, error) {
	switch scanner {
	case registryv1.Scanner_TRIVY:
		return parseTrivyReport(body)
	default:
		// Grype is the scanner Chainguard runs; treat it as the default shape.
		return parseGrypeReport(body)
	}
}

// grypeReport is the subset of grype's JSON output that the CVE list needs.
type grypeReport struct {
	Matches []struct {
		Vulnerability struct {
			ID          string   `json:"id"`
			Severity    string   `json:"severity"`
			Description string   `json:"description"`
			DataSource  string   `json:"dataSource"`
			URLs        []string `json:"urls"`
			Fix         struct {
				Versions []string `json:"versions"`
				State    string   `json:"state"`
			} `json:"fix"`
			CVSS []struct {
				Version string `json:"version"`
				Metrics struct {
					BaseScore float64 `json:"baseScore"`
				} `json:"metrics"`
			} `json:"cvss"`
		} `json:"vulnerability"`
		Artifact struct {
			Name      string `json:"name"`
			Version   string `json:"version"`
			Purl      string `json:"purl"`
			Locations []struct {
				Path string `json:"path"`
			} `json:"locations"`
		} `json:"artifact"`
	} `json:"matches"`
}

func parseGrypeReport(body []byte) ([]ImageCVE, error) {
	var report grypeReport
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, fmt.Errorf("parse grype report: %w", err)
	}
	out := make([]ImageCVE, 0, len(report.Matches))
	for _, m := range report.Matches {
		v := m.Vulnerability
		cve := ImageCVE{
			ID:          v.ID,
			Severity:    strings.ToLower(v.Severity),
			Package:     m.Artifact.Name,
			Version:     m.Artifact.Version,
			Purl:        m.Artifact.Purl,
			FixState:    strings.ToLower(v.Fix.State),
			Description: strings.TrimSpace(v.Description),
			URL:         v.DataSource,
		}
		if len(v.Fix.Versions) > 0 {
			cve.FixVersion = v.Fix.Versions[0]
		}
		if cve.URL == "" && len(v.URLs) > 0 {
			cve.URL = v.URLs[0]
		}
		if len(m.Artifact.Locations) > 0 {
			cve.Location = m.Artifact.Locations[0].Path
		}
		best := 0.0
		for _, c := range v.CVSS {
			if c.Metrics.BaseScore > best {
				best = c.Metrics.BaseScore
			}
		}
		if best > 0 {
			cve.CVSS = strconv.FormatFloat(best, 'f', -1, 64)
		}
		out = append(out, cve)
	}
	return sortCVEs(dedupeCVEs(out)), nil
}

// trivyReport is the subset of trivy's JSON output that the CVE list needs.
type trivyReport struct {
	Results []struct {
		Target          string `json:"Target"`
		Vulnerabilities []struct {
			VulnerabilityID string `json:"VulnerabilityID"`
			PkgName         string `json:"PkgName"`
			PkgIdentifier   struct {
				PURL string `json:"PURL"`
			} `json:"PkgIdentifier"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
			Status           string `json:"Status"`
			Severity         string `json:"Severity"`
			Title            string `json:"Title"`
			Description      string `json:"Description"`
			PrimaryURL       string `json:"PrimaryURL"`
			CVSS             map[string]struct {
				V3Score float64 `json:"V3Score"`
				V2Score float64 `json:"V2Score"`
			} `json:"CVSS"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

func parseTrivyReport(body []byte) ([]ImageCVE, error) {
	var report trivyReport
	if err := json.Unmarshal(body, &report); err != nil {
		return nil, fmt.Errorf("parse trivy report: %w", err)
	}
	var out []ImageCVE
	for _, res := range report.Results {
		for _, v := range res.Vulnerabilities {
			desc := strings.TrimSpace(v.Title)
			if desc == "" {
				desc = strings.TrimSpace(v.Description)
			}
			cve := ImageCVE{
				ID:          v.VulnerabilityID,
				Severity:    strings.ToLower(v.Severity),
				Package:     v.PkgName,
				Version:     v.InstalledVersion,
				Purl:        v.PkgIdentifier.PURL,
				FixVersion:  v.FixedVersion,
				FixState:    strings.ToLower(v.Status),
				Location:    res.Target,
				Description: desc,
				URL:         v.PrimaryURL,
			}
			best := 0.0
			for _, score := range v.CVSS {
				if score.V3Score > best {
					best = score.V3Score
				}
				if score.V2Score > best {
					best = score.V2Score
				}
			}
			if best > 0 {
				cve.CVSS = strconv.FormatFloat(best, 'f', -1, 64)
			}
			out = append(out, cve)
		}
	}
	return sortCVEs(dedupeCVEs(out)), nil
}

// dedupeCVEs drops repeats of the same CVE against the same package, which
// scanners emit when a package is found in several layers.
func dedupeCVEs(in []ImageCVE) []ImageCVE {
	seen := make(map[string]bool, len(in))
	out := make([]ImageCVE, 0, len(in))
	for _, c := range in {
		key := c.ID + "\x00" + c.Purl + "\x00" + c.Package + "\x00" + c.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c)
	}
	return out
}

// sortCVEs orders worst severity first, then by CVE id and package so the list
// is stable between refreshes.
func sortCVEs(in []ImageCVE) []ImageCVE {
	sort.SliceStable(in, func(i, j int) bool {
		if ri, rj := severityRank(in[i].Severity), severityRank(in[j].Severity); ri != rj {
			return ri < rj
		}
		if in[i].ID != in[j].ID {
			return in[i].ID > in[j].ID // newest CVE ids first within a severity
		}
		return in[i].Package < in[j].Package
	})
	return in
}

func shortDigest(digest string) string {
	if len(digest) > 19 {
		return digest[:7] + "..." + digest[len(digest)-9:]
	}
	return digest
}

package api

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	librariesv2 "chainguard.dev/sdk/proto/chainguard/platform/libraries/v2beta1"
	librariesv1 "chainguard.dev/sdk/proto/platform/libraries/v1"
)

// LibraryEcosystem is a Chainguard Libraries language ecosystem.
type LibraryEcosystem string

const (
	LibraryEcosystemJava       LibraryEcosystem = "java"
	LibraryEcosystemPython     LibraryEcosystem = "python"
	LibraryEcosystemJavaScript LibraryEcosystem = "javascript"
)

const npmArtifactPrefix = "npm:"

// isJavaScriptEcosystem reports whether s selects the JS/npm catalog.
// JS is not on libraries v2beta1 ListArtifacts (JAVA/PYTHON only); it uses
// platform v1 NpmPackages instead.
func isJavaScriptEcosystem(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "javascript", "js", "npm", "node", "ecosystem_javascript":
		return true
	default:
		return false
	}
}

func parseLibraryEcosystem(s string) (librariesv2.Ecosystem, error) {
	if isJavaScriptEcosystem(s) {
		return librariesv2.Ecosystem_ECOSYSTEM_UNSPECIFIED, fmt.Errorf("javascript uses npm packages API")
	}
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "java", "maven", "ECOSYSTEM_JAVA":
		return librariesv2.Ecosystem_ECOSYSTEM_JAVA, nil
	case "python", "pypi", "ECOSYSTEM_PYTHON":
		return librariesv2.Ecosystem_ECOSYSTEM_PYTHON, nil
	default:
		return librariesv2.Ecosystem_ECOSYSTEM_UNSPECIFIED, fmt.Errorf("unknown libraries ecosystem %q (want java, python, or javascript)", s)
	}
}

func ecosystemLabel(e librariesv2.Ecosystem) string {
	switch e {
	case librariesv2.Ecosystem_ECOSYSTEM_JAVA:
		return string(LibraryEcosystemJava)
	case librariesv2.Ecosystem_ECOSYSTEM_PYTHON:
		return string(LibraryEcosystemPython)
	default:
		return e.String()
	}
}

func npmSourceTypes(remediated bool) []librariesv1.NpmSourceType {
	if remediated {
		return []librariesv1.NpmSourceType{
			librariesv1.NpmSourceType_NPM_SOURCE_TYPE_INTERNAL_REMEDIATED,
		}
	}
	// Match chainctl default: Chainguard-built packages only (not upstream).
	return []librariesv1.NpmSourceType{
		librariesv1.NpmSourceType_NPM_SOURCE_TYPE_INTERNAL,
		librariesv1.NpmSourceType_NPM_SOURCE_TYPE_INTERNAL_REMEDIATED,
	}
}

func npmPackageName(artifactID string) string {
	return strings.TrimPrefix(strings.TrimSpace(artifactID), npmArtifactPrefix)
}

func npmSourceLabel(t librariesv1.NpmSourceType) string {
	switch t {
	case librariesv1.NpmSourceType_NPM_SOURCE_TYPE_INTERNAL:
		return "internal"
	case librariesv1.NpmSourceType_NPM_SOURCE_TYPE_INTERNAL_REMEDIATED:
		return "remediated"
	case librariesv1.NpmSourceType_NPM_SOURCE_TYPE_UPSTREAM_REGISTRY:
		return "upstream"
	default:
		return ""
	}
}

func artifactSourceLabel(t librariesv1.SourceType) string {
	switch t {
	case librariesv1.SourceType_SOURCE_TYPE_INTERNAL:
		return "internal"
	case librariesv1.SourceType_SOURCE_TYPE_INTERNAL_REMEDIATED:
		return "remediated"
	case librariesv1.SourceType_SOURCE_TYPE_UPSTREAM_REGISTRY:
		return "upstream"
	default:
		return ""
	}
}

func isRemediatedSource(label string) bool {
	return label == "remediated"
}

// ListArtifacts returns one page of Chainguard Libraries artifacts for an ecosystem.
// Query is free-text search; remediated restricts to remediated packages when true.
//
// Java/Python use libraries v2beta1 ListArtifacts. JavaScript uses platform v1
// NpmPackages.List (JS is not in the v2beta1 Ecosystem enum yet).
//
// License and source type are populated for JavaScript (npm). Java/Python
// artifact list responses do not include those fields yet.
func (c *Client) ListArtifacts(ecosystem string, opts PageOpts, remediated bool) (Page[LibraryArtifact], error) {
	return c.listArtifacts(context.Background(), ecosystem, opts, remediated)
}

func (c *Client) listArtifacts(ctx context.Context, ecosystem string, opts PageOpts, remediated bool) (Page[LibraryArtifact], error) {
	if isJavaScriptEcosystem(ecosystem) {
		return c.listNpmArtifacts(ctx, opts, remediated)
	}
	eco, err := parseLibraryEcosystem(ecosystem)
	if err != nil {
		return Page[LibraryArtifact]{}, err
	}
	resp, err := c.libraries.ArtifactsService().ListArtifacts(ctx, &librariesv2.ListArtifactsRequest{
		Ecosystems: []librariesv2.Ecosystem{eco},
		Query:      strings.TrimSpace(opts.Query),
		Remediated: remediated,
		PageSize:   opts.size(),
		PageToken:  opts.PageToken,
		OrderBy:    opts.OrderBy,
	})
	if err != nil {
		return Page[LibraryArtifact]{}, err
	}
	out := make([]LibraryArtifact, len(resp.GetArtifacts()))
	for i, a := range resp.GetArtifacts() {
		out[i] = LibraryArtifact{
			UID:           a.GetUid(),
			Name:          a.GetName(),
			Description:   a.GetDescription(),
			Ecosystem:     ecosystemLabel(a.GetEcosystem()),
			LatestVersion: a.GetLatestVersion(),
			VersionCount:  a.GetVersionCount(),
			CreateTime:    tsTime(a.GetCreateTime()),
			UpdateTime:    tsTime(a.GetUpdateTime()),
		}
	}
	return Page[LibraryArtifact]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) listNpmArtifacts(ctx context.Context, opts PageOpts, remediated bool) (Page[LibraryArtifact], error) {
	resp, err := c.platform.Libraries().NpmPackages().List(ctx, &librariesv1.NpmPackageFilter{
		Query:       strings.TrimSpace(opts.Query),
		PageSize:    int64(opts.size()),
		PageToken:   opts.PageToken,
		SourceTypes: npmSourceTypes(remediated),
	})
	if err != nil {
		return Page[LibraryArtifact]{}, err
	}
	out := make([]LibraryArtifact, len(resp.GetItems()))
	for i, a := range resp.GetItems() {
		name := a.GetPackageName()
		out[i] = LibraryArtifact{
			UID:           npmArtifactPrefix + name,
			Name:          name,
			Description:   a.GetDescription(),
			Ecosystem:     string(LibraryEcosystemJavaScript),
			LatestVersion: a.GetLatestVersion(),
			VersionCount:  int32(a.GetVersionCount()),
			License:       a.GetLicense(),
			SourceType:    npmSourceLabel(a.GetSourceType()),
		}
	}
	return Page[LibraryArtifact]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

// ListArtifactVersions returns one page of versions for a Libraries artifact.
// remediated filters to remediated builds when true (npm source types; Java/Python
// via platform v1 source_type).
//
// Java/Python versions use platform v1 Artifacts.ListVersions so describe can
// show source type and malware scan fields. JavaScript uses NpmPackages.ListVersions
// (license, source, malware, provenance).
func (c *Client) ListArtifactVersions(artifactID string, opts PageOpts, remediated bool) (Page[LibraryArtifactVersion], error) {
	return c.listArtifactVersions(context.Background(), artifactID, opts, remediated)
}

func (c *Client) listArtifactVersions(ctx context.Context, artifactID string, opts PageOpts, remediated bool) (Page[LibraryArtifactVersion], error) {
	if strings.TrimSpace(artifactID) == "" {
		return Page[LibraryArtifactVersion]{}, fmt.Errorf("artifact id is required")
	}
	if strings.HasPrefix(artifactID, npmArtifactPrefix) {
		return c.listNpmVersions(ctx, npmPackageName(artifactID), opts, remediated)
	}
	return c.listJavaPythonVersions(ctx, artifactID, opts, remediated)
}

func (c *Client) listJavaPythonVersions(ctx context.Context, artifactID string, opts PageOpts, remediated bool) (Page[LibraryArtifactVersion], error) {
	// v1 returns the full version set (no page token). Prefer it over v2beta1 so
	// source type and malware fields are available on describe.
	resp, err := c.platform.Libraries().Artifacts().ListVersions(ctx, &librariesv1.ArtifactVersionFilter{
		Id: artifactID,
	})
	if err != nil {
		return Page[LibraryArtifactVersion]{}, err
	}
	out := make([]LibraryArtifactVersion, 0, len(resp.GetItems()))
	for _, v := range resp.GetItems() {
		src := artifactSourceLabel(v.GetSourceType())
		if remediated && !isRemediatedSource(src) {
			continue
		}
		// Default Chainguard catalog view: skip upstream mirrors.
		if !remediated && src == "upstream" {
			continue
		}
		out = append(out, LibraryArtifactVersion{
			UID:              v.GetId(),
			Name:             v.GetName(),
			Version:          v.GetVersion(),
			Description:      v.GetDescription(),
			SourceType:       src,
			SizeBytes:        v.GetSizeBytes(),
			MalwareScanned:   v.GetMalwareScanned(),
			MalwareMalicious: v.GetMalwareMalicious(),
			CreateTime:       tsTime(v.GetCreatedAt()),
			UpdateTime:       tsTime(v.GetUpdatedAt()),
		})
	}
	return pageSlice(out, opts), nil
}

func (c *Client) listNpmVersions(ctx context.Context, packageName string, opts PageOpts, remediated bool) (Page[LibraryArtifactVersion], error) {
	resp, err := c.platform.Libraries().NpmPackages().ListVersions(ctx, &librariesv1.NpmPackageVersionFilter{
		PackageName: packageName,
		PageSize:    int64(opts.size()),
		PageToken:   opts.PageToken,
		SourceTypes: npmSourceTypes(remediated),
	})
	if err != nil {
		return Page[LibraryArtifactVersion]{}, err
	}
	out := make([]LibraryArtifactVersion, len(resp.GetItems()))
	for i, v := range resp.GetItems() {
		out[i] = LibraryArtifactVersion{
			UID:              fmt.Sprintf("%s%s@%s", npmArtifactPrefix, v.GetPackageName(), v.GetVersion()),
			Name:             v.GetPackageName(),
			Version:          v.GetVersion(),
			License:          v.GetLicense(),
			SourceType:       npmSourceLabel(v.GetSourceType()),
			SizeBytes:        v.GetFileSize(),
			Provenance:       v.GetProvenancePredicateType(),
			MalwareScanned:   v.GetMalwareScanned(),
			MalwareMalicious: v.GetMalwareMalicious(),
			MalwareScannedAt: tsTime(v.GetMalwareScannedAt()),
			CreateTime:       tsTime(v.GetCreatedAt()),
			UpdateTime:       tsTime(v.GetUpdatedAt()),
		}
	}
	return Page[LibraryArtifactVersion]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

// pageSlice applies PageOpts page size / opaque numeric skip tokens to an
// already-fetched slice (used when the backend returns a full list).
func pageSlice[T any](items []T, opts PageOpts) Page[T] {
	pageSize := opts.size()
	if pageSize <= 0 {
		pageSize = 50
	}
	start := 0
	if tok := strings.TrimSpace(opts.PageToken); tok != "" {
		if n, err := parseAdvisorySkip(tok); err == nil {
			start = int(n)
		}
	}
	if start > len(items) {
		start = len(items)
	}
	end := start + int(pageSize)
	if end > len(items) {
		end = len(items)
	}
	next := ""
	if end < len(items) {
		next = fmt.Sprintf("%d", end)
	}
	return Page[T]{
		Items:         items[start:end],
		NextPageToken: next,
		TotalCount:    int64(len(items)),
	}
}

// ---- Full-ecosystem inventory ----

// LibraryInventory is a point-in-time snapshot of every package and version in
// one Chainguard Libraries ecosystem.
type LibraryInventory struct {
	Ecosystem  string `json:"ecosystem"`
	Remediated bool   `json:"remediated"`
	// GeneratedAt is when the sweep completed, in UTC. It names the export file.
	GeneratedAt  time.Time `json:"generatedAt"`
	PackageCount int       `json:"packageCount"`
	VersionCount int       `json:"versionCount"`
	// ErrorCount is how many packages have an Error instead of versions.
	ErrorCount int                `json:"errorCount"`
	Packages   []InventoryPackage `json:"packages"`
}

// InventoryPackage is one package in a LibraryInventory. Error is set (and
// Versions left empty) when that package's version list could not be fetched;
// one bad package does not fail the whole sweep.
type InventoryPackage struct {
	Name          string   `json:"name"`
	UID           string   `json:"uid,omitempty"`
	LatestVersion string   `json:"latestVersion,omitempty"`
	Versions      []string `json:"versions"`
	Error         string   `json:"error,omitempty"`
}

const (
	// maxSweepPages bounds a page walk so a repeated page token cannot loop forever.
	maxSweepPages = 10000
	// inventoryWorkers is how many per-package version fetches run concurrently.
	inventoryWorkers = 16
)

// collectPages walks a cursor-paginated list to exhaustion. fetch is called with
// "" for the first page and the previous NextPageToken thereafter. A repeated
// token ends the walk so a misbehaving server cannot spin forever. onPage, if
// non-nil, is called with the running item count after each page.
func collectPages[T any](ctx context.Context, fetch func(token string) (Page[T], error), onPage func(count int)) ([]T, error) {
	var out []T
	seen := make(map[string]bool)
	token := ""
	for i := 0; i < maxSweepPages; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		page, err := fetch(token)
		if err != nil {
			return nil, err
		}
		out = append(out, page.Items...)
		if onPage != nil {
			onPage(len(out))
		}
		next := page.NextPageToken
		if next == "" || seen[next] {
			return out, nil
		}
		seen[next] = true
		token = next
	}
	return out, fmt.Errorf("exceeded %d pages", maxSweepPages)
}

// ListAllArtifacts returns every artifact in an ecosystem, walking all pages.
// onPage, if non-nil, reports the running count as pages arrive — the java
// catalogue is ~85k artifacts, so this walk takes minutes.
func (c *Client) ListAllArtifacts(ctx context.Context, ecosystem string, remediated bool, onPage func(count int)) ([]LibraryArtifact, error) {
	return collectPages(ctx, func(token string) (Page[LibraryArtifact], error) {
		return c.listArtifacts(ctx, ecosystem, PageOpts{PageSize: MaxPageSize, PageToken: token}, remediated)
	}, onPage)
}

// ListAllArtifactVersions returns every version of one artifact, walking all pages.
func (c *Client) ListAllArtifactVersions(ctx context.Context, artifactID string, remediated bool) ([]LibraryArtifactVersion, error) {
	return collectPages(ctx, func(token string) (Page[LibraryArtifactVersion], error) {
		return c.listArtifactVersions(ctx, artifactID, PageOpts{PageSize: MaxPageSize, PageToken: token}, remediated)
	}, nil)
}

// versionStrings reduces version records to unique version strings in API order.
// A version can appear more than once when several source types build it.
func versionStrings(versions []LibraryArtifactVersion) []string {
	out := make([]string, 0, len(versions))
	seen := make(map[string]bool, len(versions))
	for _, v := range versions {
		if v.Version == "" || seen[v.Version] {
			continue
		}
		seen[v.Version] = true
		out = append(out, v.Version)
	}
	return out
}

// BuildLibraryInventory sweeps a whole ecosystem: every package, then every
// version of each package. That is one request per page of the artifact list
// plus one or more per package, so expect minutes (tens of minutes for java,
// ~85k packages) — cancel via ctx.
//
// progress, if non-nil, is called as work completes: total is 0 while the
// artifact list is still being walked (done is then the number of packages
// discovered so far), and the package count once version fetches begin.
//
// A package whose versions cannot be fetched is recorded with an Error and
// counted in ErrorCount rather than failing the whole sweep.
func (c *Client) BuildLibraryInventory(ctx context.Context, ecosystem string, remediated bool, progress func(done, total int)) (LibraryInventory, error) {
	listed := func(count int) {
		if progress != nil {
			progress(count, 0)
		}
	}
	artifacts, err := c.ListAllArtifacts(ctx, ecosystem, remediated, listed)
	if err != nil {
		return LibraryInventory{}, fmt.Errorf("list %s artifacts: %w", ecosystem, err)
	}
	if progress != nil {
		progress(0, len(artifacts))
	}

	packages := make([]InventoryPackage, len(artifacts))
	jobs := make(chan int)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		done int
	)
	for w := 0; w < inventoryWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				a := artifacts[i]
				pkg := InventoryPackage{Name: a.Name, UID: a.UID, LatestVersion: a.LatestVersion}
				versions, err := c.ListAllArtifactVersions(ctx, a.UID, remediated)
				if err != nil {
					pkg.Error = err.Error()
				} else {
					pkg.Versions = versionStrings(versions)
				}
				mu.Lock()
				packages[i] = pkg
				done++
				if progress != nil {
					progress(done, len(artifacts))
				}
				mu.Unlock()
			}
		}()
	}
feed:
	for i := range artifacts {
		select {
		case jobs <- i:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	// A cancelled sweep is incomplete: report it rather than writing a partial snapshot.
	if err := ctx.Err(); err != nil {
		return LibraryInventory{}, err
	}

	total, failed := 0, 0
	for _, p := range packages {
		total += len(p.Versions)
		if p.Error != "" {
			failed++
		}
	}
	return LibraryInventory{
		Ecosystem:    ecosystem,
		Remediated:   remediated,
		GeneratedAt:  time.Now().UTC(),
		PackageCount: len(packages),
		VersionCount: total,
		ErrorCount:   failed,
		Packages:     packages,
	}, nil
}

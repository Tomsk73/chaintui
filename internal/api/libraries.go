package api

import (
	"context"
	"fmt"
	"strings"

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
	if isJavaScriptEcosystem(ecosystem) {
		return c.listNpmArtifacts(opts, remediated)
	}
	eco, err := parseLibraryEcosystem(ecosystem)
	if err != nil {
		return Page[LibraryArtifact]{}, err
	}
	ctx := context.Background()
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

func (c *Client) listNpmArtifacts(opts PageOpts, remediated bool) (Page[LibraryArtifact], error) {
	ctx := context.Background()
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
	if strings.TrimSpace(artifactID) == "" {
		return Page[LibraryArtifactVersion]{}, fmt.Errorf("artifact id is required")
	}
	if strings.HasPrefix(artifactID, npmArtifactPrefix) {
		return c.listNpmVersions(npmPackageName(artifactID), opts, remediated)
	}
	return c.listJavaPythonVersions(artifactID, opts, remediated)
}

func (c *Client) listJavaPythonVersions(artifactID string, opts PageOpts, remediated bool) (Page[LibraryArtifactVersion], error) {
	ctx := context.Background()
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

func (c *Client) listNpmVersions(packageName string, opts PageOpts, remediated bool) (Page[LibraryArtifactVersion], error) {
	ctx := context.Background()
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

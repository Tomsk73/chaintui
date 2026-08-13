package api

import (
	"context"
	"fmt"
	"strings"

	librariesv2 "chainguard.dev/sdk/proto/chainguard/platform/libraries/v2beta1"
)

// LibraryEcosystem is a Chainguard Libraries language ecosystem.
type LibraryEcosystem string

const (
	LibraryEcosystemJava   LibraryEcosystem = "java"
	LibraryEcosystemPython LibraryEcosystem = "python"
)

func parseLibraryEcosystem(s string) (librariesv2.Ecosystem, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "java", "maven", "ECOSYSTEM_JAVA":
		return librariesv2.Ecosystem_ECOSYSTEM_JAVA, nil
	case "python", "pypi", "ECOSYSTEM_PYTHON":
		return librariesv2.Ecosystem_ECOSYSTEM_PYTHON, nil
	default:
		return librariesv2.Ecosystem_ECOSYSTEM_UNSPECIFIED, fmt.Errorf("unknown libraries ecosystem %q (want java or python)", s)
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

// ListArtifacts returns one page of Chainguard Libraries artifacts for an ecosystem.
// Query is free-text search; remediated restricts to remediated packages when true.
func (c *Client) ListArtifacts(ecosystem string, opts PageOpts, remediated bool) (Page[LibraryArtifact], error) {
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

// ListArtifactVersions returns one page of versions for a Libraries artifact.
func (c *Client) ListArtifactVersions(artifactID string, opts PageOpts) (Page[LibraryArtifactVersion], error) {
	if strings.TrimSpace(artifactID) == "" {
		return Page[LibraryArtifactVersion]{}, fmt.Errorf("artifact id is required")
	}
	ctx := context.Background()
	resp, err := c.libraries.ArtifactsService().ListArtifactVersions(ctx, &librariesv2.ListArtifactVersionsRequest{
		ArtifactId: artifactID,
		PageSize:   opts.size(),
		PageToken:  opts.PageToken,
		OrderBy:    opts.OrderBy,
	})
	if err != nil {
		return Page[LibraryArtifactVersion]{}, err
	}
	out := make([]LibraryArtifactVersion, len(resp.GetArtifactVersions()))
	for i, v := range resp.GetArtifactVersions() {
		out[i] = LibraryArtifactVersion{
			UID:         v.GetUid(),
			Name:        v.GetName(),
			Version:     v.GetVersion(),
			Description: v.GetDescription(),
			SizeBytes:   v.GetSizeBytes(),
			CreateTime:  tsTime(v.GetCreateTime()),
			UpdateTime:  tsTime(v.GetUpdateTime()),
		}
	}
	return Page[LibraryArtifactVersion]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

package api

import (
	"context"
	"strconv"
	"strings"
	"time"

	iamv2 "chainguard.dev/sdk/proto/chainguard/platform/iam/v2beta1"
	registryv2 "chainguard.dev/sdk/proto/chainguard/platform/registry/v2beta1"
	vulnv2 "chainguard.dev/sdk/proto/chainguard/platform/vulnerabilities/v2beta1"
	commonv1 "chainguard.dev/sdk/proto/platform/common/v1"
	registryv1 "chainguard.dev/sdk/proto/platform/registry/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// parsePURL extracts name and version from a PURL string.
// PURL format: pkg:type/namespace/name@version
func parsePURL(purl string) (name, version string) {
	s := purl
	if i := strings.Index(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[i+1:] // strip type
	}
	if i := strings.Index(s, "/"); i >= 0 {
		s = s[i+1:] // strip namespace
	}
	s, _, _ = strings.Cut(s, "?")
	name, version, _ = strings.Cut(s, "@")
	return
}

func uidpFilter(groupUID string) *commonv1.UIDPFilter {
	if groupUID != "" {
		return &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	return &commonv1.UIDPFilter{InRoot: true}
}

func tsTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// ListMyOrganizations returns one page of root-level groups (orgs) the current
// user belongs to, using uidp.ancestorsOf scoped to the user's subject UIDP.
func (c *Client) ListMyOrganizations(opts PageOpts) (Page[Group], error) {
	ctx := context.Background()
	req := &iamv2.ListGroupsRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if sub := c.Subject(); sub != "" {
		req.Uidp = &commonv1.UIDPFilter{AncestorsOf: sub}
	} else {
		req.Uidp = &commonv1.UIDPFilter{InRoot: true}
	}
	resp, err := c.v2.IAM().GroupsService().ListGroups(ctx, req)
	if err != nil {
		return Page[Group]{}, err
	}
	out := make([]Group, len(resp.GetGroups()))
	for i, g := range resp.GetGroups() {
		out[i] = Group{
			UID:         g.GetUid(),
			Name:        g.GetName(),
			Description: g.GetDescription(),
			CreateTime:  tsTime(g.GetCreateTime()),
			UpdateTime:  tsTime(g.GetUpdateTime()),
		}
	}
	return Page[Group]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListGroups(parentUID string, opts PageOpts) (Page[Group], error) {
	ctx := context.Background()
	resp, err := c.v2.IAM().GroupsService().ListGroups(ctx, &iamv2.ListGroupsRequest{
		Uidp:      uidpFilter(parentUID),
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	})
	if err != nil {
		return Page[Group]{}, err
	}
	out := make([]Group, len(resp.GetGroups()))
	for i, g := range resp.GetGroups() {
		out[i] = Group{
			UID:         g.GetUid(),
			Name:        g.GetName(),
			Description: g.GetDescription(),
			CreateTime:  tsTime(g.GetCreateTime()),
			UpdateTime:  tsTime(g.GetUpdateTime()),
		}
	}
	return Page[Group]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListIdentities(groupUID string, opts PageOpts) (Page[Identity], error) {
	ctx := context.Background()
	req := &iamv2.ListIdentitiesRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.IAM().IdentitiesService().ListIdentities(ctx, req)
	if err != nil {
		return Page[Identity]{}, err
	}
	out := make([]Identity, len(resp.GetIdentities()))
	for i, v := range resp.GetIdentities() {
		out[i] = Identity{
			UID:         v.GetUid(),
			Name:        v.GetName(),
			Description: v.GetDescription(),
			CreateTime:  tsTime(v.GetCreateTime()),
			UpdateTime:  tsTime(v.GetUpdateTime()),
		}
	}
	return Page[Identity]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListRoles(groupUID string, opts PageOpts) (Page[Role], error) {
	ctx := context.Background()
	req := &iamv2.ListRolesRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.IAM().RolesService().ListRoles(ctx, req)
	if err != nil {
		return Page[Role]{}, err
	}
	out := make([]Role, len(resp.GetRoles()))
	for i, v := range resp.GetRoles() {
		caps := make([]string, len(v.GetCapabilities()))
		for j, cap := range v.GetCapabilities() {
			caps[j] = cap.String()
		}
		out[i] = Role{
			UID:          v.GetUid(),
			Name:         v.GetName(),
			Description:  v.GetDescription(),
			Capabilities: caps,
			CreateTime:   tsTime(v.GetCreateTime()),
			UpdateTime:   tsTime(v.GetUpdateTime()),
		}
	}
	return Page[Role]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListRoleBindings(groupUID string, opts PageOpts) (Page[RoleBinding], error) {
	ctx := context.Background()
	req := &iamv2.ListRoleBindingsRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.IAM().RoleBindingsService().ListRoleBindings(ctx, req)
	if err != nil {
		return Page[RoleBinding]{}, err
	}
	out := make([]RoleBinding, len(resp.GetRoleBindings()))
	for i, v := range resp.GetRoleBindings() {
		out[i] = RoleBinding{
			UID:         v.GetUid(),
			IdentityUID: v.GetIdentityUid(),
			RoleUID:     v.GetRoleUid(),
			CreateTime:  tsTime(v.GetCreateTime()),
		}
	}
	return Page[RoleBinding]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListIdentityProviders(groupUID string, opts PageOpts) (Page[IdentityProvider], error) {
	ctx := context.Background()
	req := &iamv2.ListIdentityProvidersRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.IAM().IdentityProvidersService().ListIdentityProviders(ctx, req)
	if err != nil {
		return Page[IdentityProvider]{}, err
	}
	out := make([]IdentityProvider, len(resp.GetIdentityProviders()))
	for i, v := range resp.GetIdentityProviders() {
		out[i] = IdentityProvider{
			UID:         v.GetUid(),
			Name:        v.GetName(),
			Description: v.GetDescription(),
			CreateTime:  tsTime(v.GetCreateTime()),
			UpdateTime:  tsTime(v.GetUpdateTime()),
		}
	}
	return Page[IdentityProvider]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListGroupInvites(groupUID string, opts PageOpts) (Page[GroupInvite], error) {
	ctx := context.Background()
	req := &iamv2.ListGroupInvitesRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.IAM().GroupInvitesService().ListGroupInvites(ctx, req)
	if err != nil {
		return Page[GroupInvite]{}, err
	}
	out := make([]GroupInvite, len(resp.GetGroupInvites()))
	for i, v := range resp.GetGroupInvites() {
		roleUID := v.GetRoleUid()
		if roleUID == "" && v.GetRole() != nil {
			roleUID = v.GetRole().GetUid()
		}
		out[i] = GroupInvite{
			UID:            v.GetUid(),
			Email:          v.GetEmail(),
			RoleUID:        roleUID,
			ExpirationTime: tsTime(v.GetExpirationTime()),
			CreateTime:     tsTime(v.GetCreateTime()),
		}
	}
	return Page[GroupInvite]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListAccountAssociations(groupUID string, opts PageOpts) (Page[AccountAssociation], error) {
	ctx := context.Background()
	req := &iamv2.ListAccountAssociationsRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.IAM().AccountAssociationsService().ListAccountAssociations(ctx, req)
	if err != nil {
		return Page[AccountAssociation]{}, err
	}
	out := make([]AccountAssociation, len(resp.GetAccountAssociations()))
	for i, v := range resp.GetAccountAssociations() {
		aa := AccountAssociation{
			UID:         v.GetUid(),
			Name:        v.GetName(),
			Description: v.GetDescription(),
		}
		if am := v.GetAmazon(); am != nil {
			aa.Amazon = &AccountAssociationAmazon{Account: am.GetAccount()}
		}
		if g := v.GetGoogle(); g != nil {
			aa.Google = &AccountAssociationGoogle{
				ProjectID:     g.GetProjectId(),
				ProjectNumber: g.GetProjectNumber(),
			}
		}
		if cg := v.GetChainguard(); cg != nil {
			aa.Chainguard = &AccountAssociationChainguard{ServiceBindings: cg.GetServiceBindings()}
		}
		if gh := v.GetGithub(); gh != nil {
			installs := make(map[string]AccountAssociationGitHubAppInstallations, len(gh.GetAppInstallations()))
			for appID, set := range gh.GetAppInstallations() {
				entries := make([]AccountAssociationGitHubInstallation, 0, len(set.GetInstallations()))
				for _, inst := range set.GetInstallations() {
					entries = append(entries, AccountAssociationGitHubInstallation{
						InstallationID: strconv.FormatInt(inst.GetInstallationId(), 10),
						Name:           inst.GetName(),
					})
				}
				installs[strconv.FormatInt(appID, 10)] = AccountAssociationGitHubAppInstallations{
					Installations: entries,
				}
			}
			aa.GitHub = &AccountAssociationGitHub{AppInstallations: installs}
		}
		out[i] = aa
	}
	return Page[AccountAssociation]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListRepos(groupUID string, opts PageOpts) (Page[Repo], error) {
	ctx := context.Background()
	req := &registryv2.ListReposRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.Registry().ReposService().ListRepos(ctx, req)
	if err != nil {
		return Page[Repo]{}, err
	}
	out := make([]Repo, len(resp.GetRepos()))
	for i, v := range resp.GetRepos() {
		out[i] = Repo{
			UID:         v.GetUid(),
			Name:        v.GetName(),
			Description: v.GetDescription(),
			CreateTime:  tsTime(v.GetCreateTime()),
			UpdateTime:  tsTime(v.GetUpdateTime()),
		}
	}
	return Page[Repo]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

func (c *Client) ListTags(repoUID string, opts PageOpts) (Page[Tag], error) {
	ctx := context.Background()
	resp, err := c.v2.Registry().TagsService().ListTags(ctx, &registryv2.ListTagsRequest{
		Uidp:      &commonv1.UIDPFilter{ChildrenOf: repoUID},
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	})
	if err != nil {
		return Page[Tag]{}, err
	}
	out := make([]Tag, len(resp.GetTags()))
	for i, v := range resp.GetTags() {
		out[i] = Tag{
			UID:        v.GetUid(),
			Name:       v.GetName(),
			Digest:     v.GetDigest(),
			UpdateTime: tsTime(v.GetUpdateTime()),
		}
	}
	return Page[Tag]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

// GetTagSBOM fetches SBOM packages for a tag digest via the v1 registry API
// (not a cursor-paginated list resource).
func (c *Client) GetTagSBOM(repoUID, digest string) ([]SBOMPackage, error) {
	ctx := context.Background()

	// Chainguard tags point to OCI index manifests (multi-arch), so we use
	// IndexFilter rather than ImageDigest to match against the index digest.
	resp, err := c.platform.Registry().Registry().ListManifestMetadata(ctx, &registryv1.ManifestMetadataFilter{
		RepoId: repoUID,
		Items: []*registryv1.ManifestMetadataFilterEntry{
			{Filter: &registryv1.ManifestMetadataFilterEntry_IndexFilter{
				IndexFilter: &registryv1.ManifestMetadataIndexFilter{Digest: digest, Arch: "amd64"},
			}},
		},
	})
	if err != nil {
		return nil, err
	}
	var out []SBOMPackage
	for _, m := range resp.GetItems() {
		for _, pkg := range m.GetPkgMetadata() {
			name, version := parsePURL(pkg.GetPurl())
			out = append(out, SBOMPackage{
				Name:    name,
				Version: version,
				Purl:    pkg.GetPurl(),
				License: pkg.GetLicense(),
			})
		}
	}
	return out, nil
}

func (c *Client) ListAdvisories(groupUID string, opts PageOpts) (Page[Advisory], error) {
	ctx := context.Background()
	// The advisories List RPC returns Internal on page 2 when page_size is
	// >= ~40 (verified against console-api). Cap well below that.
	const maxAdvisoryPage int32 = 25
	pageSize := opts.size()
	if pageSize > maxAdvisoryPage {
		pageSize = maxAdvisoryPage
	}
	req := &vulnv2.ListAdvisoriesRequest{
		PageSize:  pageSize,
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
		Query:     opts.Query,
	}
	if groupUID != "" {
		req.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.v2.Vulnerabilities().AdvisoriesService().ListAdvisories(ctx, req)
	if err != nil {
		return Page[Advisory]{}, err
	}
	out := make([]Advisory, len(resp.GetAdvisories()))
	for i, v := range resp.GetAdvisories() {
		out[i] = Advisory{
			UID:                  v.GetUid(),
			AdvisoryID:           v.GetAdvisoryId(),
			LegacyAdvisoryID:     v.GetLegacyAdvisoryId(),
			Aliases:              v.GetAliases(),
			ArtifactName:         v.GetArtifactName(),
			ArtifactType:         v.GetArtifactType(),
			ArtifactArchitecture: v.GetArtifactArchitecture(),
			ComponentName:        v.GetComponentName(),
			ComponentLocation:    v.GetComponentLocation(),
			ComponentType:        v.GetComponentType(),
			Author:               v.GetAuthor(),
			CreateTime:           tsTime(v.GetCreateTime()),
			UpdateTime:           tsTime(v.GetUpdateTime()),
		}
	}
	return Page[Advisory]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

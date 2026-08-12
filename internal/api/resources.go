package api

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	iamv2 "chainguard.dev/sdk/proto/chainguard/platform/iam/v2beta1"
	registryv2 "chainguard.dev/sdk/proto/chainguard/platform/registry/v2beta1"
	vulnv2 "chainguard.dev/sdk/proto/chainguard/platform/vulnerabilities/v2beta1"
	commonv1 "chainguard.dev/sdk/proto/platform/common/v1"
	registryv1 "chainguard.dev/sdk/proto/platform/registry/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// uidpChildren scopes to direct children only — used for group hierarchy
// navigation (folders one level down) and tags under a repo.
func uidpChildren(parentUID string) *commonv1.UIDPFilter {
	if parentUID != "" {
		return &commonv1.UIDPFilter{ChildrenOf: parentUID}
	}
	return &commonv1.UIDPFilter{InRoot: true}
}

// uidpScope includes the whole subtree under a group (org/folder). Resource
// lists like repos and IAM objects often live in nested groups, so
// descendants_of matches how the console and chainctl typically scope orgs.
func uidpScope(groupUID string) *commonv1.UIDPFilter {
	if groupUID == "" {
		return nil
	}
	return &commonv1.UIDPFilter{DescendantsOf: groupUID}
}

func tsTime(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	return ts.AsTime()
}

// exactName returns opts.Query trimmed, or "" when unset. Used for List RPCs
// that filter by exact resource name rather than free-text query.
func exactName(opts PageOpts) string {
	return strings.TrimSpace(opts.Query)
}

// ListMyOrganizations returns one page of root-level groups (orgs) the current
// user belongs to, using uidp.ancestorsOf scoped to the user's subject UIDP.
func (c *Client) ListMyOrganizations(opts PageOpts) (Page[Group], error) {
	ctx := context.Background()
	req := &iamv2.ListGroupsRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
		Name:      exactName(opts),
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
		Uidp:      uidpChildren(parentUID),
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
		Name:      exactName(opts),
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
		Name:      exactName(opts),
	}
	req.Uidp = uidpScope(groupUID)
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
		Name:      exactName(opts),
	}
	req.Uidp = uidpScope(groupUID)
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
	req.Uidp = uidpScope(groupUID)
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
		Name:      exactName(opts),
	}
	req.Uidp = uidpScope(groupUID)
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
	req.Uidp = uidpScope(groupUID)
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
	req.Uidp = uidpScope(groupUID)
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
	if name := exactName(opts); name != "" {
		req.Name = &name
	}
	req.Uidp = uidpScope(groupUID)
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
		Name:      exactName(opts),
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

func mapAdvisory(v *vulnv2.Advisory) Advisory {
	return Advisory{
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

func parseAdvisorySkip(token string) (int32, error) {
	if token == "" {
		return 0, nil
	}
	n, err := strconv.ParseInt(token, 10, 32)
	if err != nil || n < 0 {
		// Opaque API page tokens are not used for advisories (they break on
		// certain records). Ask the UI to refresh from the start.
		return 0, fmt.Errorf("stale advisory page token; press r to refresh")
	}
	return int32(n), nil
}

func (c *Client) listAdvisoriesRaw(ctx context.Context, groupUID string, opts PageOpts, pageSize, skip int32) (*vulnv2.ListAdvisoriesResponse, error) {
	req := &vulnv2.ListAdvisoriesRequest{
		PageSize: pageSize,
		Skip:     skip,
		OrderBy:  opts.OrderBy,
		Query:    opts.Query,
	}
	req.Uidp = uidpScope(groupUID)
	return c.v2.Vulnerabilities().AdvisoriesService().ListAdvisories(ctx, req)
}

// ListAdvisories returns one page of advisories.
//
// The console-api ListAdvisories RPC returns Internal for some individual
// records (observed around offset 66 in the default ordering). Opaque
// page_token pagination also fails once a window includes such a record.
// We therefore paginate with Skip, encode the next skip offset in
// NextPageToken, and step over poison rows one at a time.
func (c *Client) ListAdvisories(groupUID string, opts PageOpts) (Page[Advisory], error) {
	ctx := context.Background()

	const (
		maxAdvisoryPage int32 = 25
		apiBatch        int32 = 10
		maxPoisonSkip         = 50
	)
	pageSize := opts.size()
	if pageSize > maxAdvisoryPage {
		pageSize = maxAdvisoryPage
	}

	skip, err := parseAdvisorySkip(opts.PageToken)
	if err != nil {
		return Page[Advisory]{}, err
	}

	out := make([]Advisory, 0, pageSize)
	cursor := skip
	var totalCount int64
	poisonSkips := 0

	for int32(len(out)) < pageSize {
		batch := pageSize - int32(len(out))
		if batch > apiBatch {
			batch = apiBatch
		}

		resp, err := c.listAdvisoriesRaw(ctx, groupUID, opts, batch, cursor)
		if err == nil {
			if resp.GetTotalCount() > 0 {
				totalCount = resp.GetTotalCount()
			}
			items := resp.GetAdvisories()
			if len(items) == 0 {
				break // end of list
			}
			for _, v := range items {
				out = append(out, mapAdvisory(v))
			}
			cursor += int32(len(items))
			poisonSkips = 0
			if int32(len(items)) < batch {
				break // end of list
			}
			continue
		}

		if status.Code(err) != codes.Internal {
			return Page[Advisory]{}, err
		}

		// Batch hit a poison row — walk singles and skip failures.
		advanced := false
		for int32(len(out)) < pageSize {
			one, err := c.listAdvisoriesRaw(ctx, groupUID, opts, 1, cursor)
			if err != nil {
				if status.Code(err) != codes.Internal {
					return Page[Advisory]{}, err
				}
				poisonSkips++
				if poisonSkips > maxPoisonSkip {
					return Page[Advisory]{}, fmt.Errorf("too many unreadable advisories near offset %d: %w", cursor, err)
				}
				cursor++
				advanced = true
				continue
			}
			if one.GetTotalCount() > 0 {
				totalCount = one.GetTotalCount()
			}
			items := one.GetAdvisories()
			if len(items) == 0 {
				return Page[Advisory]{
					Items:      out,
					TotalCount: totalCount,
				}, nil
			}
			out = append(out, mapAdvisory(items[0]))
			cursor++
			poisonSkips = 0
			advanced = true
			// After clearing the poison zone, prefer batching again.
			break
		}
		if !advanced {
			break
		}
	}

	next := ""
	if int32(len(out)) == pageSize {
		next = strconv.FormatInt(int64(cursor), 10)
	}

	return Page[Advisory]{
		Items:         out,
		NextPageToken: next,
		TotalCount:    totalCount,
	}, nil
}

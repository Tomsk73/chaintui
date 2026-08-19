package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	capabilities "chainguard.dev/sdk/proto/capabilities"
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

// unescapePURLPart decodes one percent-encoded PURL component, leaving it
// unchanged if it is not valid encoding.
func unescapePURLPart(s string) string {
	decoded, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return decoded
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

// ListMyOrganizations returns one page of root-level groups (orgs) visible to
// the caller.
//
// It filters on uidp.inRoot rather than ancestorsOf: the token's subject is an
// OIDC subject (e.g. "google-oauth2|1234"), not a UIDP, so ancestorsOf matches
// nothing. IAM already restricts the response to groups the caller can see,
// which is how chainctl lists orgs.
func (c *Client) ListMyOrganizations(opts PageOpts) (Page[Group], error) {
	ctx := context.Background()
	req := &iamv2.ListGroupsRequest{
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
		Name:      exactName(opts),
		Uidp:      &commonv1.UIDPFilter{InRoot: true},
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

// capabilityName renders a capability enum in the form Chainguard documents and
// chainctl accepts: CAP_ARGOS_DOCUMENTS_CREATE -> argos.documents.create.
func capabilityName(c capabilities.Capability) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimPrefix(c.String(), "CAP_")), "_", ".")
}

// isManagedRole reports whether a role UIDP is root-level, i.e. one of
// Chainguard's built-in roles rather than an org's own.
func isManagedRole(uid string) bool {
	return !strings.Contains(uid, "/")
}

// inGroup reports whether a UIDP sits anywhere under groupUID. An empty group
// matches everything (no org selected).
func inGroup(uid, groupUID string) bool {
	if groupUID == "" {
		return true
	}
	return strings.HasPrefix(uid, groupUID+"/")
}

// ListRoles returns the roles that can be bound in a group: the group's own
// custom roles plus Chainguard's managed (built-in) roles.
//
// Built-in roles live outside the group hierarchy — their UIDP has no parent —
// so neither uidp.descendantsOf nor uidp.inRoot matches them, and an org with no
// custom roles would otherwise show an empty page. This lists unfiltered and
// keeps root-level roles plus anything under groupUID. Roles number in the tens,
// so the merged list is sorted (custom first) and paged locally.
//
// customOnly drops the built-ins, leaving just the group's own roles.
func (c *Client) ListRoles(groupUID string, opts PageOpts, customOnly bool) (Page[Role], error) {
	ctx := context.Background()
	all, err := collectPages(ctx, func(token string) (Page[Role], error) {
		resp, err := c.v2.IAM().RolesService().ListRoles(ctx, &iamv2.ListRolesRequest{
			PageSize:  MaxPageSize,
			PageToken: token,
			Name:      exactName(opts),
		})
		if err != nil {
			return Page[Role]{}, err
		}
		items := make([]Role, len(resp.GetRoles()))
		for i, v := range resp.GetRoles() {
			caps := make([]string, len(v.GetCapabilities()))
			for j, capability := range v.GetCapabilities() {
				caps[j] = capabilityName(capability)
			}
			items[i] = Role{
				UID:          v.GetUid(),
				Name:         v.GetName(),
				Description:  v.GetDescription(),
				Managed:      isManagedRole(v.GetUid()),
				Capabilities: caps,
				CreateTime:   tsTime(v.GetCreateTime()),
				UpdateTime:   tsTime(v.GetUpdateTime()),
			}
		}
		return Page[Role]{
			Items:         items,
			NextPageToken: resp.GetNextPageToken(),
			TotalCount:    resp.GetTotalCount(),
		}, nil
	}, nil)
	if err != nil {
		return Page[Role]{}, err
	}
	out := make([]Role, 0, len(all))
	for _, r := range all {
		if r.Managed && customOnly {
			continue
		}
		if r.Managed || inGroup(r.UID, groupUID) {
			out = append(out, r)
		}
	}
	// The org's own roles first — the built-ins are the same everywhere.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Managed != out[j].Managed {
			return !out[i].Managed
		}
		return out[i].Name < out[j].Name
	})
	return pageSlice(out, opts), nil
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

// chartCatalogFolder reports whether an org child folder holds Helm charts.
// AddChart places charts in a catalog folder named "charts"; the iamguarded
// catalog uses "iamguarded-charts".
func chartCatalogFolder(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	return n == "charts" || strings.HasSuffix(n, "-charts")
}

// ListCharts returns the Helm charts in an org's chart catalog folders. The
// registry API has no chart list RPC — charts are repos inside a catalog folder
// — so this resolves the org's chart folders and lists the repos in each.
// Results are merged and paged locally; chart counts are small (tens).
// opts.Query filters by exact chart name.
func (c *Client) ListCharts(orgUID string, opts PageOpts) (Page[Chart], error) {
	if strings.TrimSpace(orgUID) == "" {
		return Page[Chart]{}, fmt.Errorf("org is required")
	}
	ctx := context.Background()
	folders, err := collectPages(ctx, func(token string) (Page[Group], error) {
		return c.ListGroups(orgUID, PageOpts{PageSize: MaxPageSize, PageToken: token})
	}, nil)
	if err != nil {
		return Page[Chart]{}, fmt.Errorf("list chart folders: %w", err)
	}
	name := exactName(opts)
	var out []Chart
	for _, f := range folders {
		if !chartCatalogFolder(f.Name) {
			continue
		}
		repos, err := collectPages(ctx, func(token string) (Page[Repo], error) {
			return c.ListRepos(f.UID, PageOpts{PageSize: MaxPageSize, PageToken: token, Query: name})
		}, nil)
		if err != nil {
			return Page[Chart]{}, fmt.Errorf("list charts in %s: %w", f.Name, err)
		}
		for _, r := range repos {
			out = append(out, Chart{
				UID:         r.UID,
				Name:        r.Name,
				Catalog:     f.Name,
				Description: r.Description,
				CreateTime:  r.CreateTime,
				UpdateTime:  r.UpdateTime,
			})
		}
	}
	// Same chart in two catalogs sits together, so the CATALOG column reads as
	// the difference between them.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Catalog < out[j].Catalog
	})
	return pageSlice(out, opts), nil
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
				IndexFilter: &registryv1.ManifestMetadataIndexFilter{Digest: digest, Arch: sbomArch},
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

// ErrNoImage means the repo has no image under the requested tag.
var ErrNoImage = errors.New("no image")

// A multi-arch tag points at an index, so reading its SBOM means picking one
// architecture. sbomArch is the registry's name for it and advisoryArch is the
// advisory catalogue's name for the same thing; keep the two in step.
const (
	sbomArch     = "amd64"
	advisoryArch = "x86_64"
)

// ImagePackages is what one image ships, as named by its SBOM.
type ImagePackages struct {
	Tag    string
	Digest string
	// Names are the distro package names in the image, deduped and sorted.
	// These match the component name on an APK advisory.
	Names []string
	// Total counts every SBOM package, including the language-ecosystem ones
	// that Names leaves out.
	Total int
	// Architecture is the arch these packages were read for, named as the
	// advisory catalogue names it.
	Architecture string
}

// ListImagePackages resolves a repo's tag to an image and returns the packages
// its SBOM names. Tag defaults to "latest".
func (c *Client) ListImagePackages(repoUID, tag string) (ImagePackages, error) {
	if strings.TrimSpace(tag) == "" {
		tag = "latest"
	}
	// ListTags with a Query matches the name exactly, so this is a lookup of the
	// one tag rather than a scan of the repo.
	page, err := c.ListTags(repoUID, PageOpts{Query: tag, PageSize: 1})
	if err != nil {
		return ImagePackages{}, err
	}
	digest := digestForTag(page.Items, tag)
	if digest == "" {
		return ImagePackages{}, fmt.Errorf("%w tagged %s", ErrNoImage, tag)
	}
	pkgs, err := c.GetTagSBOM(repoUID, digest)
	if err != nil {
		return ImagePackages{}, err
	}
	return ImagePackages{
		Tag:          tag,
		Digest:       digest,
		Names:        distroPackageNames(pkgs),
		Total:        len(pkgs),
		Architecture: advisoryArch,
	}, nil
}

// digestForTag finds the exact tag in a page of results. The name filter is a
// server-side exact match, but the response is still a list, and a tag without a
// digest is one we cannot resolve to an image.
func digestForTag(tags []Tag, tag string) string {
	for _, t := range tags {
		if t.Name == tag {
			return t.Digest
		}
	}
	return ""
}

// distroPackageNames picks the APK packages out of an SBOM, deduped and sorted.
// Language-ecosystem entries (Go modules, npm, ...) are left out: advisories are
// filed against distro package names.
//
// An SBOM whose entries carry no PURL at all is of an unrecognised shape rather
// than one without distro packages, so every name is returned instead of none.
func distroPackageNames(pkgs []SBOMPackage) []string {
	apk, all := map[string]bool{}, map[string]bool{}
	purled := false
	for _, p := range pkgs {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		all[name] = true
		if strings.HasPrefix(p.Purl, "pkg:") {
			purled = true
		}
		if strings.HasPrefix(p.Purl, "pkg:apk/") {
			apk[name] = true
		}
	}
	names := apk
	if !purled {
		names = all
	}
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
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
		Events:               mapAdvisoryEvents(v.GetEvents()),
		CreateTime:           tsTime(v.GetCreateTime()),
		UpdateTime:           tsTime(v.GetUpdateTime()),
	}
}

func mapAdvisoryEvents(in []*vulnv2.AdvisoryEvent) []AdvisoryEvent {
	if len(in) == 0 {
		return nil
	}
	out := make([]AdvisoryEvent, 0, len(in))
	for _, e := range in {
		out = append(out, mapAdvisoryEvent(e))
	}
	return out
}

func mapAdvisoryEvent(e *vulnv2.AdvisoryEvent) AdvisoryEvent {
	out := AdvisoryEvent{
		UID:         e.GetUid(),
		Author:      e.GetAuthor(),
		Reviewer:    e.GetReviewer(),
		ReviewState: ReviewState(e.GetReviewState().String()),
		Issue:       e.GetIssue(),
		Findings:    e.GetFindings(),
		CreateTime:  tsTime(e.GetCreateTime()),
	}
	// The oneof arm is both the event's type and its payload.
	switch t := e.GetType().(type) {
	case *vulnv2.AdvisoryEvent_Detection_:
		out.Type = AdvisoryEventTypeDetection
		out.Detection = mapAdvisoryDetection(t.Detection)
	case *vulnv2.AdvisoryEvent_TruePositiveDetermination_:
		out.Type = AdvisoryEventTypeTruePositive
		out.TruePositiveDetermination = &AdvisoryEventTruePositive{Note: t.TruePositiveDetermination.GetNote()}
	case *vulnv2.AdvisoryEvent_FalsePositiveDetermination_:
		out.Type = AdvisoryEventTypeFalsePositive
		out.FalsePositiveDetermination = &AdvisoryEventFalsePositive{
			Type: t.FalsePositiveDetermination.GetType().String(),
			Note: t.FalsePositiveDetermination.GetNote(),
		}
	case *vulnv2.AdvisoryEvent_Fixed_:
		out.Type = AdvisoryEventTypeFixed
		out.Fixed = &AdvisoryEventFixed{FixedVersion: t.Fixed.GetFixedVersion(), Note: t.Fixed.GetNote()}
	case *vulnv2.AdvisoryEvent_Patched_:
		out.Type = AdvisoryEventTypePatched
		out.Patched = &AdvisoryEventPatched{PatchedVersions: t.Patched.GetPatchedVersions(), Note: t.Patched.GetNote()}
	case *vulnv2.AdvisoryEvent_FixNotPlanned_:
		out.Type = AdvisoryEventTypeFixNotPlanned
		out.FixNotPlanned = &AdvisoryEventFixNotPlanned{Note: t.FixNotPlanned.GetNote()}
	case *vulnv2.AdvisoryEvent_AnalysisNotPlanned_:
		out.Type = AdvisoryEventTypeAnalysisNotPlanned
		out.AnalysisNotPlanned = &AdvisoryEventAnalysisNotPlanned{Note: t.AnalysisNotPlanned.GetNote()}
	case *vulnv2.AdvisoryEvent_PendingUpstreamFix_:
		out.Type = AdvisoryEventTypePendingUpstreamFix
		out.PendingUpstreamFix = &AdvisoryEventPendingUpstreamFix{Note: t.PendingUpstreamFix.GetNote()}
	}
	return out
}

func mapAdvisoryDetection(d *vulnv2.AdvisoryEvent_Detection) *AdvisoryEventDetection {
	if d == nil {
		return nil
	}
	out := &AdvisoryEventDetection{}
	switch t := d.GetType().(type) {
	case *vulnv2.AdvisoryEvent_Detection_Scanv1:
		out.ScanV1 = &AdvisoryEventDetectionScanV1{
			Scanner:           t.Scanv1.GetScanner(),
			Subpackage:        t.Scanv1.GetSubpackage(),
			Component:         t.Scanv1.GetComponent(),
			ComponentID:       t.Scanv1.GetComponentId(),
			ComponentVersion:  t.Scanv1.GetComponentVersion(),
			ComponentType:     t.Scanv1.GetComponentType(),
			ComponentLocation: t.Scanv1.GetComponentLocation(),
		}
	case *vulnv2.AdvisoryEvent_Detection_Nvdapi:
		out.NVDAPI = &AdvisoryEventDetectionNVDAPI{
			CPESearched: t.Nvdapi.GetCpeSearched(),
			CPEFound:    t.Nvdapi.GetCpeFound(),
		}
	case *vulnv2.AdvisoryEvent_Detection_Manual_:
		out.Manual = &AdvisoryEventDetectionManual{}
	}
	return out
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

// listAdvisoriesRaw is deliberately unscoped. Advisories are a single global
// catalogue rather than per-org data — every record lives under Chainguard's own
// group — so filtering by the caller's org UIDP matches nothing at all.
//
// Of the request's artifact/component filters only component_names has any
// effect: the server accepts artifact_names and then ignores it, returning the
// unfiltered catalogue. For APK advisories the component is the installed
// package name, which is what an image's SBOM lists, so component_names is the
// right filter anyway.
func (c *Client) listAdvisoriesRaw(ctx context.Context, filter AdvisoryFilter, opts PageOpts, pageSize, skip int32) (*vulnv2.ListAdvisoriesResponse, error) {
	req := advisoryRequest(filter, opts, pageSize, skip)
	return c.v2.Vulnerabilities().AdvisoriesService().ListAdvisories(ctx, req)
}

// advisoryRequest builds the list request. Notably it sets no Uidp — see
// listAdvisoriesRaw.
func advisoryRequest(filter AdvisoryFilter, opts PageOpts, pageSize, skip int32) *vulnv2.ListAdvisoriesRequest {
	req := &vulnv2.ListAdvisoriesRequest{
		PageSize:       pageSize,
		Skip:           skip,
		OrderBy:        opts.OrderBy,
		Query:          opts.Query,
		ComponentNames: filter.ComponentNames,
	}
	if filter.Architecture != "" {
		req.ArtifactArchitectures = []string{filter.Architecture}
	}
	return req
}

// advisoryFetcher loads a window of advisories starting at skip.
// Used so collectAdvisoriesPage can be unit-tested without a live API.
type advisoryFetcher func(pageSize, skip int32) (*vulnv2.ListAdvisoriesResponse, error)

const (
	maxAdvisoryPage int32 = 25
	advisoryBatch   int32 = 10
	maxPoisonSkip         = 50
)

// collectAdvisoriesPage returns one page of advisories using Skip-based paging.
//
// The console-api ListAdvisories RPC returns Internal for some individual
// records. Opaque page_token pagination also fails once a window includes
// such a record. We therefore paginate with Skip, encode the next skip in
// NextPageToken, and step over poison rows one at a time.
func collectAdvisoriesPage(opts PageOpts, fetch advisoryFetcher) (Page[Advisory], error) {
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
		if batch > advisoryBatch {
			batch = advisoryBatch
		}

		resp, err := fetch(batch, cursor)
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
			one, err := fetch(1, cursor)
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

// ListAdvisories returns one page of advisories.
// AdvisoryFilter narrows the advisory catalogue to a subset worth reading.
type AdvisoryFilter struct {
	// ComponentNames restricts results to advisories whose component is one of
	// these installed package names.
	ComponentNames []string
	// Architecture restricts results to one artifact architecture, e.g. x86_64.
	// Advisories are filed per architecture, so leaving this empty lists a
	// dual-arch package's advisories twice.
	Architecture string
}

// ListAdvisories returns one page of the Chainguard advisory catalogue. It takes
// no group: see listAdvisoriesRaw for why advisories are not org-scoped.
func (c *Client) ListAdvisories(opts PageOpts) (Page[Advisory], error) {
	return c.ListAdvisoriesFiltered(AdvisoryFilter{}, opts)
}

// ListAdvisoriesFiltered narrows the catalogue before paging it. Opts still
// apply, so a free-text Query searches within the filtered set rather than the
// whole catalogue.
func (c *Client) ListAdvisoriesFiltered(filter AdvisoryFilter, opts PageOpts) (Page[Advisory], error) {
	ctx := context.Background()
	return collectAdvisoriesPage(opts, func(pageSize, skip int32) (*vulnv2.ListAdvisoriesResponse, error) {
		return c.listAdvisoriesRaw(ctx, filter, opts, pageSize, skip)
	})
}

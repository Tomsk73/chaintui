package api

import (
	"context"
	"strconv"
	"time"

	advisoryv1 "chainguard.dev/sdk/proto/platform/advisory/v1"
	commonv1 "chainguard.dev/sdk/proto/platform/common/v1"
	iamv1 "chainguard.dev/sdk/proto/platform/iam/v1"
	registryv1 "chainguard.dev/sdk/proto/platform/registry/v1"
)

func uidpFilter(groupUID string) *commonv1.UIDPFilter {
	if groupUID != "" {
		return &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	return &commonv1.UIDPFilter{InRoot: true}
}

// ListMyOrganizations returns the root-level groups (orgs) the current user
// belongs to, using uidp.ancestorsOf scoped to the user's own subject UIDP.
// Falls back to all root groups when the subject is unavailable.
func (c *Client) ListMyOrganizations() ([]Group, error) {
	ctx := context.Background()
	filter := &iamv1.GroupFilter{}
	if sub := c.Subject(); sub != "" {
		filter.Uidp = &commonv1.UIDPFilter{AncestorsOf: sub}
	} else {
		filter.Uidp = &commonv1.UIDPFilter{InRoot: true}
	}
	resp, err := c.platform.IAM().Groups().List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]Group, len(resp.Items))
	for i, g := range resp.Items {
		out[i] = Group{UID: g.GetId(), Name: g.GetName(), Description: g.GetDescription()}
	}
	return out, nil
}

func (c *Client) ListGroups(parentUID string) ([]Group, error) {
	ctx := context.Background()
	resp, err := c.platform.IAM().Groups().List(ctx, &iamv1.GroupFilter{Uidp: uidpFilter(parentUID)})
	if err != nil {
		return nil, err
	}
	out := make([]Group, len(resp.Items))
	for i, g := range resp.Items {
		out[i] = Group{UID: g.GetId(), Name: g.GetName(), Description: g.GetDescription()}
	}
	return out, nil
}

func (c *Client) ListIdentities(groupUID string) ([]Identity, error) {
	ctx := context.Background()
	filter := &iamv1.IdentityFilter{}
	if groupUID != "" {
		filter.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.platform.IAM().Identities().List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]Identity, len(resp.Items))
	for i, v := range resp.Items {
		out[i] = Identity{UID: v.GetId(), Name: v.GetName(), Description: v.GetDescription()}
	}
	return out, nil
}

func (c *Client) ListRoles(groupUID string) ([]Role, error) {
	ctx := context.Background()
	filter := &iamv1.RoleFilter{}
	if groupUID != "" {
		filter.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.platform.IAM().Roles().List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]Role, len(resp.Items))
	for i, v := range resp.Items {
		out[i] = Role{UID: v.GetId(), Name: v.GetName(), Description: v.GetDescription(), Capabilities: v.GetCapabilities()}
	}
	return out, nil
}

func (c *Client) ListRoleBindings(groupUID string) ([]RoleBinding, error) {
	ctx := context.Background()
	filter := &iamv1.RoleBindingFilter{}
	if groupUID != "" {
		filter.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.platform.IAM().RoleBindings().List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]RoleBinding, len(resp.Items))
	for i, v := range resp.Items {
		rb := RoleBinding{
			UID:         v.GetId(),
			IdentityUID: v.GetIdentity(),
		}
		if r := v.GetRole(); r != nil {
			rb.RoleUID = r.GetId()
		}
		out[i] = rb
	}
	return out, nil
}

func (c *Client) ListIdentityProviders(groupUID string) ([]IdentityProvider, error) {
	ctx := context.Background()
	filter := &iamv1.IdentityProviderFilter{}
	if groupUID != "" {
		filter.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.platform.IAM().IdentityProviders().List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]IdentityProvider, len(resp.Items))
	for i, v := range resp.Items {
		out[i] = IdentityProvider{UID: v.GetId(), Name: v.GetName(), Description: v.GetDescription()}
	}
	return out, nil
}

func (c *Client) ListGroupInvites(groupUID string) ([]GroupInvite, error) {
	ctx := context.Background()
	resp, err := c.platform.IAM().GroupInvites().List(ctx, &iamv1.GroupInviteFilter{Group: groupUID})
	if err != nil {
		return nil, err
	}
	out := make([]GroupInvite, len(resp.Items))
	for i, v := range resp.Items {
		roleUID := ""
		if v.GetRole() != nil {
			roleUID = v.GetRole().GetId()
		}
		var expirationTime, createTime time.Time
		if v.GetExpiration() != nil {
			expirationTime = v.GetExpiration().AsTime()
		}
		if v.GetCreatedAt() != nil {
			createTime = v.GetCreatedAt().AsTime()
		}
		out[i] = GroupInvite{
			UID:            v.GetId(),
			Email:          v.GetEmail(),
			RoleUID:        roleUID,
			ExpirationTime: expirationTime,
			CreateTime:     createTime,
		}
	}
	return out, nil
}

func (c *Client) ListAccountAssociations(groupUID string) ([]AccountAssociation, error) {
	ctx := context.Background()
	filter := &iamv1.AccountAssociationsFilter{}
	if groupUID != "" {
		filter.Group = groupUID
	}
	resp, err := c.platform.IAM().AccountAssociations().List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]AccountAssociation, len(resp.Items))
	for i, v := range resp.Items {
		aa := AccountAssociation{
			UID:         v.GetGroup(),
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
			appID := strconv.FormatInt(gh.GetAppId(), 10)
			aa.GitHub = &AccountAssociationGitHub{
				AppInstallations: map[string]AccountAssociationGitHubAppInstallations{
					appID: {
						Installations: []AccountAssociationGitHubInstallation{
							{
								InstallationID: strconv.FormatInt(gh.GetInstallationId(), 10),
								Name:           gh.GetName(),
							},
						},
					},
				},
			}
		}
		out[i] = aa
	}
	return out, nil
}

func (c *Client) ListRepos(groupUID string) ([]Repo, error) {
	ctx := context.Background()
	filter := &registryv1.RepoFilter{}
	if groupUID != "" {
		filter.Uidp = &commonv1.UIDPFilter{ChildrenOf: groupUID}
	}
	resp, err := c.platform.Registry().Registry().ListRepos(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]Repo, len(resp.Items))
	for i, v := range resp.Items {
		out[i] = Repo{UID: v.GetId(), Name: v.GetName()}
	}
	return out, nil
}

func (c *Client) ListTags(repoUID string) ([]Tag, error) {
	ctx := context.Background()
	filter := &registryv1.TagFilter{
		Uidp: &commonv1.UIDPFilter{ChildrenOf: repoUID},
	}
	resp, err := c.platform.Registry().Registry().ListTags(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]Tag, len(resp.Items))
	for i, v := range resp.Items {
		var updateTime time.Time
		if v.GetLastUpdated() != nil {
			updateTime = v.GetLastUpdated().AsTime()
		}
		out[i] = Tag{UID: v.GetId(), Name: v.GetName(), Digest: v.GetDigest(), UpdateTime: updateTime}
	}
	return out, nil
}

func (c *Client) ListAdvisories(groupUID string) ([]Advisory, error) {
	ctx := context.Background()
	// The gRPC advisory API is document-centric (one Document per package,
	// containing multiple Advisory entries). Flatten into our Advisory type.
	// Group filtering is not supported by the SDK's DocumentFilter.
	resp, err := c.platform.Advisory().SecurityAdvisory().ListDocuments(ctx, &advisoryv1.DocumentFilter{})
	if err != nil {
		return nil, err
	}
	var out []Advisory
	for _, doc := range resp.Items {
		for _, adv := range doc.GetAdvisories() {
			out = append(out, Advisory{
				UID:     adv.GetId(),
				Aliases: adv.GetAliases(),
			})
		}
	}
	return out, nil
}

package api

import (
	"context"
	"fmt"
	"strings"

	librariesv1 "chainguard.dev/sdk/proto/platform/libraries/v1"
)

// libEcosystemLabel names a platform v1 libraries ecosystem. The Athena tiers
// grant Chainguard-remediated builds for their paired ecosystem.
func libEcosystemLabel(e librariesv1.Ecosystem) string {
	switch e {
	case librariesv1.Ecosystem_JAVA:
		return string(LibraryEcosystemJava)
	case librariesv1.Ecosystem_PYTHON:
		return string(LibraryEcosystemPython)
	case librariesv1.Ecosystem_JAVASCRIPT:
		return string(LibraryEcosystemJavaScript)
	case librariesv1.Ecosystem_JAVA_ATHENA:
		return "java-athena"
	case librariesv1.Ecosystem_PYTHON_ATHENA:
		return "python-athena"
	case librariesv1.Ecosystem_JAVASCRIPT_ATHENA:
		return "javascript-athena"
	case librariesv1.Ecosystem_DOTNET:
		return "dotnet"
	case librariesv1.Ecosystem_DOTNET_ATHENA:
		return "dotnet-athena"
	default:
		return strings.ToLower(e.String())
	}
}

// baseEcosystem strips an Athena suffix so an entitlement lines up with the
// ecosystem the catalogue browser uses.
func baseEcosystem(label string) string {
	return strings.TrimSuffix(label, "-athena")
}

func entitlementAccessLabel(p librariesv1.Policy) string {
	switch p {
	case librariesv1.Policy_POLICY_CHAINGUARD:
		return "chainguard"
	case librariesv1.Policy_POLICY_CHAINGUARD_AND_UPSTREAM:
		return "chainguard+upstream"
	default:
		// POLICY_UNKNOWN behaves as CHAINGUARD.
		return "chainguard"
	}
}

func entitlementSourceLabel(s librariesv1.Source) string {
	switch s {
	case librariesv1.Source_SOURCE_TRIAL:
		return "trial"
	case librariesv1.Source_SOURCE_SFDC:
		return "sfdc"
	default:
		return ""
	}
}

func policyTypeLabel(t librariesv1.LibraryPolicyType) string {
	switch t {
	case librariesv1.LibraryPolicyType_LIBRARY_POLICY_TYPE_SYSTEM:
		return "system"
	case librariesv1.LibraryPolicyType_LIBRARY_POLICY_TYPE_CUSTOM:
		return "custom"
	default:
		return ""
	}
}

func bindingModeLabel(m librariesv1.LibraryPolicyBindingMode) string {
	switch m {
	case librariesv1.LibraryPolicyBindingMode_LIBRARY_POLICY_BINDING_MODE_ENFORCED:
		return "enforced"
	case librariesv1.LibraryPolicyBindingMode_LIBRARY_POLICY_BINDING_MODE_LOG:
		return "log"
	default:
		return ""
	}
}

// ListLibraryEntitlements returns the ecosystems an org may pull. The API
// returns the full set in one response (no paging).
func (c *Client) ListLibraryEntitlements(orgUID string) ([]LibraryEntitlement, error) {
	if strings.TrimSpace(orgUID) == "" {
		return nil, fmt.Errorf("org is required")
	}
	ctx := context.Background()
	resp, err := c.platform.Libraries().Entitlements().List(ctx, &librariesv1.EntitlementFilter{
		ParentId: orgUID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]LibraryEntitlement, len(resp.GetItems()))
	for i, e := range resp.GetItems() {
		out[i] = LibraryEntitlement{
			UID:          e.GetId(),
			Ecosystem:    libEcosystemLabel(e.GetEcosystem()),
			Access:       entitlementAccessLabel(e.GetPolicy()),
			Source:       entitlementSourceLabel(e.GetSource()),
			CooldownDays: e.GetCooldownDays(),
		}
	}
	return out, nil
}

// ListLibraryPolicies returns every policy available to an org: Chainguard
// system policies plus the org's own custom policies.
func (c *Client) ListLibraryPolicies(orgUID string) ([]LibraryPolicy, error) {
	if strings.TrimSpace(orgUID) == "" {
		return nil, fmt.Errorf("org is required")
	}
	ctx := context.Background()
	return collectPages(ctx, func(token string) (Page[LibraryPolicy], error) {
		resp, err := c.platform.Libraries().LibraryPolicies().ListPolicies(ctx, &librariesv1.LibraryPolicyFilter{
			ParentId:  orgUID,
			PageSize:  MaxPageSize,
			PageToken: token,
		})
		if err != nil {
			return Page[LibraryPolicy]{}, err
		}
		items := make([]LibraryPolicy, len(resp.GetItems()))
		for i, p := range resp.GetItems() {
			block := make([]string, 0, len(p.GetBlockList()))
			for _, b := range p.GetBlockList() {
				block = append(block, b.GetPurl())
			}
			allow := make([]LibraryAllowEntry, 0, len(p.GetAllowList()))
			for _, a := range p.GetAllowList() {
				allow = append(allow, LibraryAllowEntry{
					Purl:           a.GetPurl(),
					BypassCooldown: a.GetBypassCooldown(),
					BypassMalware:  a.GetBypassMalware(),
					Justification:  a.GetJustification(),
				})
			}
			items[i] = LibraryPolicy{
				UID:             p.GetId(),
				Name:            p.GetName(),
				Description:     p.GetDescription(),
				Type:            policyTypeLabel(p.GetPolicyType()),
				CooldownDays:    p.CooldownDays,
				BlockList:       block,
				AllowList:       allow,
				BlockedLicenses: p.GetBlockedLicenses(),
				Expression:      p.GetExpression(),
				CreateTime:      tsTime(p.GetCreatedAt()),
				UpdateTime:      tsTime(p.GetUpdatedAt()),
			}
		}
		return Page[LibraryPolicy]{
			Items:         items,
			NextPageToken: resp.GetNextPageToken(),
			TotalCount:    resp.GetTotalCount(),
		}, nil
	}, nil)
}

// ListLibraryPolicyBindings returns which policy is active for each of an org's
// ecosystems, and whether it enforces or only logs.
func (c *Client) ListLibraryPolicyBindings(orgUID string) ([]LibraryPolicyBinding, error) {
	if strings.TrimSpace(orgUID) == "" {
		return nil, fmt.Errorf("org is required")
	}
	ctx := context.Background()
	return collectPages(ctx, func(token string) (Page[LibraryPolicyBinding], error) {
		resp, err := c.platform.Libraries().LibraryPolicyBindings().ListBindings(ctx, &librariesv1.LibraryPolicyBindingFilter{
			ParentId:  orgUID,
			PageSize:  MaxPageSize,
			PageToken: token,
		})
		if err != nil {
			return Page[LibraryPolicyBinding]{}, err
		}
		items := make([]LibraryPolicyBinding, len(resp.GetItems()))
		for i, b := range resp.GetItems() {
			items[i] = LibraryPolicyBinding{
				UID:        b.GetId(),
				PolicyUID:  b.GetPolicy(),
				Ecosystem:  libEcosystemLabel(b.GetEcosystem()),
				Mode:       bindingModeLabel(b.GetMode()),
				CreateTime: tsTime(b.GetCreatedAt()),
				UpdateTime: tsTime(b.GetUpdatedAt()),
			}
		}
		return Page[LibraryPolicyBinding]{
			Items:         items,
			NextPageToken: resp.GetNextPageToken(),
			TotalCount:    resp.GetTotalCount(),
		}, nil
	}, nil)
}

// GetLibraryOrgPolicy fetches an org's entitlements, policies and bindings
// together, with binding policy names resolved. Each list is small (a handful of
// rows), so callers page over the result locally.
func (c *Client) GetLibraryOrgPolicy(orgUID string) (LibraryOrgPolicy, error) {
	ents, err := c.ListLibraryEntitlements(orgUID)
	if err != nil {
		return LibraryOrgPolicy{}, fmt.Errorf("entitlements: %w", err)
	}
	policies, err := c.ListLibraryPolicies(orgUID)
	if err != nil {
		return LibraryOrgPolicy{}, fmt.Errorf("policies: %w", err)
	}
	bindings, err := c.ListLibraryPolicyBindings(orgUID)
	if err != nil {
		return LibraryOrgPolicy{}, fmt.Errorf("policy bindings: %w", err)
	}
	names := make(map[string]string, len(policies))
	for _, p := range policies {
		names[p.UID] = p.Name
	}
	for i := range bindings {
		bindings[i].PolicyName = names[bindings[i].PolicyUID]
	}
	return LibraryOrgPolicy{Entitlements: ents, Policies: policies, Bindings: bindings}, nil
}

// EcosystemStatuses reports the org's posture for each browsable ecosystem, in
// the order the picker lists them. Athena entitlements count towards their base
// ecosystem.
func (p LibraryOrgPolicy) EcosystemStatuses(ecosystems []string) []EcosystemStatus {
	out := make([]EcosystemStatus, len(ecosystems))
	for i, eco := range ecosystems {
		st := EcosystemStatus{Ecosystem: eco}
		for j := range p.Entitlements {
			if baseEcosystem(p.Entitlements[j].Ecosystem) == eco {
				// Prefer an exact match over an Athena tier for the same base.
				if st.Entitlement == nil || p.Entitlements[j].Ecosystem == eco {
					st.Entitlement = &p.Entitlements[j]
				}
			}
		}
		for _, b := range p.Bindings {
			if baseEcosystem(b.Ecosystem) == eco {
				st.Bindings = append(st.Bindings, b)
			}
		}
		out[i] = st
	}
	return out
}

// ListLibraryBlockEvents returns packages policy withheld from an org, most
// recent first. opts.Query matches the PURL name component exactly
// (case-insensitive). The API defaults to enforced-mode events from the last 30
// days; logMode switches to shadow (log-only) violations.
func (c *Client) ListLibraryBlockEvents(orgUID string, opts PageOpts, logMode bool) (Page[LibraryBlockEvent], error) {
	if strings.TrimSpace(orgUID) == "" {
		return Page[LibraryBlockEvent]{}, fmt.Errorf("org is required")
	}
	mode := librariesv1.LibraryPolicyBindingMode_LIBRARY_POLICY_BINDING_MODE_ENFORCED
	if logMode {
		mode = librariesv1.LibraryPolicyBindingMode_LIBRARY_POLICY_BINDING_MODE_LOG
	}
	ctx := context.Background()
	resp, err := c.platform.Libraries().LibraryPolicyBlockEvents().ListBlockEvents(ctx, &librariesv1.LibraryPolicyBlockEventFilter{
		ParentId:    orgUID,
		PackageName: exactName(opts),
		Mode:        mode,
		PageSize:    opts.size(),
		PageToken:   opts.PageToken,
	})
	if err != nil {
		return Page[LibraryBlockEvent]{}, err
	}
	out := make([]LibraryBlockEvent, len(resp.GetItems()))
	for i, e := range resp.GetItems() {
		name, version := parsePURL(e.GetPurl())
		// PURL components are percent-encoded (e.g. "2.24.0%2Bcgr.1"); show them
		// as the version string a user would type.
		name, version = unescapePURLPart(name), unescapePURLPart(version)
		out[i] = LibraryBlockEvent{
			UID:            e.GetId(),
			Purl:           e.GetPurl(),
			Package:        name,
			Version:        version,
			Ecosystem:      strings.ToLower(e.GetEcosystem()),
			Mode:           bindingModeLabel(e.GetMode()),
			Reason:         e.GetReason(),
			PolicyUID:      e.GetPolicy(),
			CooldownDays:   e.GetCooldownDays(),
			PublishDate:    tsTime(e.GetPublishDate()),
			UnblocksAt:     tsTime(e.GetUnblocksAt()),
			FirstBlockedAt: tsTime(e.GetFirstBlockedAt()),
			LastBlockedAt:  tsTime(e.GetLastBlockedAt()),
			AttemptCount:   e.GetAttemptCount(),
		}
	}
	return Page[LibraryBlockEvent]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

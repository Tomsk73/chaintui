package api

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	policiesv1 "chainguard.dev/sdk/proto/platform/policies/v1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/structpb"
)

// RepoResourceType is the resource an image policy evaluates. The policy list
// carries a schema version suffix, bindings do not.
const RepoResourceType = "registry.chainguard.dev/Repo"

// ListImagePolicies returns the policies available to a group: its own custom
// policies plus Chainguard's system ones.
//
// System policies live outside the group hierarchy, exactly as built-in IAM
// roles do, so a descendants_of filter would hide them and an org with no custom
// policies would show an empty page. This lists unfiltered — the RPC allows it —
// and keeps root-level policies plus anything under groupUID. Policies number in
// the tens, so the merged list is sorted (custom first) and paged locally.
func (c *Client) ListImagePolicies(groupUID string, opts PageOpts) (Page[ImagePolicy], error) {
	ctx := context.Background()
	all, err := collectPages(ctx, func(token string) (Page[ImagePolicy], error) {
		resp, err := c.platform.Policies().Policies().ListPolicies(ctx, &policiesv1.PolicyFilter{
			PageSize:  MaxPageSize,
			PageToken: token,
			Name:      exactName(opts),
		})
		if err != nil {
			return Page[ImagePolicy]{}, err
		}
		items := make([]ImagePolicy, 0, len(resp.GetItems()))
		for _, v := range resp.GetItems() {
			items = append(items, mapImagePolicy(v))
		}
		return Page[ImagePolicy]{Items: items, NextPageToken: resp.GetNextPageToken()}, nil
	}, nil)
	if err != nil {
		return Page[ImagePolicy]{}, err
	}

	return pageSlice(policiesForGroup(all, groupUID), opts), nil
}

// policiesForGroup keeps the policies a group can actually bind: its own, plus
// the root-level system ones, custom first.
func policiesForGroup(all []ImagePolicy, groupUID string) []ImagePolicy {
	kept := make([]ImagePolicy, 0, len(all))
	for _, p := range all {
		// isManagedRole tests for a root-level UIDP, which is what a
		// Chainguard-managed record looks like whatever the resource.
		if isManagedRole(p.UID) || inGroup(p.UID, groupUID) {
			kept = append(kept, p)
		}
	}
	// Custom policies first: they are the ones an org acts on.
	sort.SliceStable(kept, func(i, j int) bool {
		ci, cj := kept[i].Type == policyTypeCustom, kept[j].Type == policyTypeCustom
		if ci != cj {
			return ci
		}
		return kept[i].Name < kept[j].Name
	})
	return kept
}

// ImagePolicyNames maps policy UIDP to name, for the lists that reference a
// policy by id alone.
func (c *Client) ImagePolicyNames(groupUID string) (map[string]string, error) {
	page, err := c.ListImagePolicies(groupUID, PageOpts{PageSize: MaxPageSize})
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(page.Items))
	for _, p := range page.Items {
		out[p.UID] = p.Name
	}
	return out, nil
}

// ListImagePolicyBindings returns the bindings in scope, newest first. Each
// binding names the policy it activates and the mode it runs in.
func (c *Client) ListImagePolicyBindings(groupUID string, opts PageOpts) (Page[ImagePolicyBinding], error) {
	ctx := context.Background()
	resp, err := c.platform.Policies().Bindings().ListBindings(ctx, &policiesv1.BindingFilter{
		Uidp:      uidpScope(groupUID),
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	})
	if err != nil {
		return Page[ImagePolicyBinding]{}, err
	}
	out := make([]ImagePolicyBinding, 0, len(resp.GetItems()))
	for _, v := range resp.GetItems() {
		out = append(out, ImagePolicyBinding{
			UID:           v.GetId(),
			PolicyUID:     v.GetPolicy(),
			Mode:          imagePolicyModeLabel(v.GetMode()),
			ResourceTypes: v.GetResourceTypes(),
			Parameters:    policyParameterValues(v.GetParameters()),
			CreateTime:    tsTime(v.GetCreatedAt()),
			UpdateTime:    tsTime(v.GetUpdatedAt()),
		})
	}
	return Page[ImagePolicyBinding]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

// ImagePolicyDecisionFilter narrows a decision list beyond its scope.
type ImagePolicyDecisionFilter struct {
	// DeniedOnly keeps only the pulls a policy refused.
	DeniedOnly bool
}

// ListImagePolicyDecisions returns pull-time policy outcomes under scopeUID,
// which may be an org or a single repo.
func (c *Client) ListImagePolicyDecisions(scopeUID string, filter ImagePolicyDecisionFilter, opts PageOpts) (Page[ImagePolicyDecision], error) {
	ctx := context.Background()
	req := &policiesv1.DecisionFilter{
		Uidp:      uidpScope(scopeUID),
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	}
	if filter.DeniedOnly {
		req.Result = policiesv1.Result_RESULT_DENIED
	}
	resp, err := c.platform.Policies().Decisions().ListDecisions(ctx, req)
	if err != nil {
		return Page[ImagePolicyDecision]{}, err
	}
	out := make([]ImagePolicyDecision, 0, len(resp.GetItems()))
	for _, v := range resp.GetItems() {
		out = append(out, ImagePolicyDecision{
			UID:        v.GetId(),
			RepoUID:    v.GetRepoId(),
			Digest:     v.GetDigest(),
			PolicyUID:  v.GetPolicyId(),
			PolicyName: v.GetPolicyName(),
			Mode:       imagePolicyModeLabel(v.GetMode()),
			Result:     policyResultLabel(v.GetResult()),
			Reason:     v.GetReason(),
			PulledOn:   dateTime(v.GetPulledOn()),
		})
	}
	return Page[ImagePolicyDecision]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

// ListImagePolicyOverrides returns the waivers granted in scope.
func (c *Client) ListImagePolicyOverrides(groupUID string, opts PageOpts) (Page[ImagePolicyOverride], error) {
	ctx := context.Background()
	resp, err := c.platform.Policies().Overrides().ListOverrides(ctx, &policiesv1.OverrideFilter{
		Uidp:      uidpScope(groupUID),
		PageSize:  opts.size(),
		PageToken: opts.PageToken,
		OrderBy:   opts.OrderBy,
	})
	if err != nil {
		return Page[ImagePolicyOverride]{}, err
	}
	out := make([]ImagePolicyOverride, 0, len(resp.GetItems()))
	for _, v := range resp.GetItems() {
		out = append(out, ImagePolicyOverride{
			UID:        v.GetId(),
			PolicyUID:  v.GetPolicyId(),
			Digest:     v.GetDigest(),
			Reason:     v.GetReason(),
			CreatedBy:  v.GetCreatedBy(),
			CreateTime: tsTime(v.GetCreatedAt()),
		})
	}
	return Page[ImagePolicyOverride]{
		Items:         out,
		NextPageToken: resp.GetNextPageToken(),
		TotalCount:    resp.GetTotalCount(),
	}, nil
}

// RepoNames maps repo UIDP to name for a group, so lists that carry only a repo
// id can show something readable.
func (c *Client) RepoNames(groupUID string) (map[string]string, error) {
	ctx := context.Background()
	repos, err := collectPages(ctx, func(token string) (Page[Repo], error) {
		return c.ListRepos(groupUID, PageOpts{PageSize: MaxPageSize, PageToken: token})
	}, nil)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string, len(repos))
	for _, r := range repos {
		out[r.UID] = r.Name
	}
	return out, nil
}

const (
	policyTypeSystem = "system"
	policyTypeCustom = "custom"
)

func mapImagePolicy(v *policiesv1.Policy) ImagePolicy {
	resourceType := v.GetSupportedResourceType()
	if resourceType == "" && len(v.GetSupportedResourceTypes()) > 0 {
		// Older records only populate the deprecated repeated field.
		resourceType = v.GetSupportedResourceTypes()[0]
	}
	out := ImagePolicy{
		UID:          v.GetId(),
		Name:         v.GetName(),
		Description:  v.GetDescription(),
		Type:         imagePolicyTypeLabel(v.GetPolicyType()),
		ResourceType: resourceType,
		Expression:   v.GetExpression(),
		CreateTime:   tsTime(v.GetCreatedAt()),
		UpdateTime:   tsTime(v.GetUpdatedAt()),
	}
	for _, p := range v.GetParameterSchemas() {
		out.Parameters = append(out.Parameters, ImagePolicyParameter{
			Name:        p.GetName(),
			Type:        parameterTypeLabel(p.GetType()),
			Description: p.GetDescription(),
			Required:    p.GetRequired(),
		})
	}
	return out
}

func imagePolicyTypeLabel(t policiesv1.PolicyType) string {
	switch t {
	case policiesv1.PolicyType_POLICY_TYPE_SYSTEM:
		return policyTypeSystem
	case policiesv1.PolicyType_POLICY_TYPE_CUSTOM:
		return policyTypeCustom
	default:
		return ""
	}
}

func imagePolicyModeLabel(m policiesv1.PolicyMode) string {
	switch m {
	case policiesv1.PolicyMode_POLICY_MODE_ENFORCED:
		return "enforced"
	case policiesv1.PolicyMode_POLICY_MODE_DRY_RUN:
		return "dry-run"
	default:
		return ""
	}
}

func policyResultLabel(r policiesv1.Result) string {
	switch r {
	case policiesv1.Result_RESULT_ALLOWED:
		return "allowed"
	case policiesv1.Result_RESULT_DENIED:
		return "denied"
	case policiesv1.Result_RESULT_ERROR:
		return "error"
	default:
		return ""
	}
}

func parameterTypeLabel(t policiesv1.ParameterType) string {
	switch t {
	case policiesv1.ParameterType_PARAMETER_TYPE_STRING:
		return "string"
	case policiesv1.ParameterType_PARAMETER_TYPE_INTEGER:
		return "integer"
	case policiesv1.ParameterType_PARAMETER_TYPE_STRING_LIST:
		return "string list"
	case policiesv1.ParameterType_PARAMETER_TYPE_BOOLEAN:
		return "boolean"
	default:
		return ""
	}
}

// policyParameterValues flattens a binding's parameter values to strings for
// display; the structured form stays available in the raw record.
func policyParameterValues(in map[string]*structpb.Value) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = parameterValueString(v)
	}
	return out
}

func parameterValueString(v *structpb.Value) string {
	if v == nil {
		return ""
	}
	if l := v.GetListValue(); l != nil {
		parts := make([]string, 0, len(l.GetValues()))
		for _, item := range l.GetValues() {
			parts = append(parts, parameterValueString(item))
		}
		return strings.Join(parts, ", ")
	}
	if s, ok := v.GetKind().(*structpb.Value_StringValue); ok {
		return s.StringValue
	}
	return strings.TrimSpace(fmt.Sprint(v.AsInterface()))
}

// dateTime converts a whole-day date to a time. The policy engine records the
// day a pull happened, not the instant.
func dateTime(d *date.Date) time.Time {
	if d == nil || d.GetYear() == 0 {
		return time.Time{}
	}
	return time.Date(int(d.GetYear()), time.Month(d.GetMonth()), int(d.GetDay()), 0, 0, 0, 0, time.UTC)
}

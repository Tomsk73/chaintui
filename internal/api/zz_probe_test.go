package api

import (
	"context"
	"os"
	"testing"

	commonv1 "chainguard.dev/sdk/proto/platform/common/v1"
	policiesv1 "chainguard.dev/sdk/proto/platform/policies/v1"
)

func TestZZPolicies(t *testing.T) {
	if os.Getenv("PROBE") == "" {
		t.Skip("set PROBE=1")
	}
	c, err := NewClient()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	p := c.platform.Policies()

	orgs, err := c.ListMyOrganizations(PageOpts{PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	org := ""
	if v := os.Getenv("ORG"); v != "" {
		for _, o := range orgs.Items {
			if o.Name == v {
				org = o.UID
			}
		}
	}
	if org == "" {
		org = orgs.Items[0].UID
	}
	t.Logf("org=%s", org)

	// Policies: unscoped and scoped, to learn which returns anything.
	for _, tc := range []struct {
		label string
		f     *policiesv1.PolicyFilter
	}{
		{"unscoped", &policiesv1.PolicyFilter{PageSize: 10}},
		{"descendantsOf org", &policiesv1.PolicyFilter{PageSize: 10, Uidp: &commonv1.UIDPFilter{DescendantsOf: org}}},
	} {
		list, err := p.Policies().ListPolicies(ctx, tc.f)
		if err != nil {
			t.Logf("policies %s: ERROR %v", tc.label, err)
			continue
		}
		t.Logf("policies %s: items=%d total=%d", tc.label, len(list.GetItems()), list.GetTotalCount())
		for _, pol := range list.GetItems() {
			t.Logf("   id=%s name=%q type=%s resourceType=%q params=%d expr=%.60q",
				pol.GetId(), pol.GetName(), pol.GetPolicyType(), pol.GetSupportedResourceType(), len(pol.GetParameterSchemas()), pol.GetExpression())
		}
	}

	list, err := p.Bindings().ListBindings(ctx, &policiesv1.BindingFilter{PageSize: 10})
	if err != nil {
		t.Logf("bindings: ERROR %v", err)
	} else {
		t.Logf("bindings: items=%d total=%d", len(list.GetItems()), list.GetTotalCount())
		for _, b := range list.GetItems() {
			t.Logf("   id=%s policy=%s mode=%s types=%v params=%v", b.GetId(), b.GetPolicy(), b.GetMode(), b.GetResourceTypes(), b.GetParameters())
		}
	}

	dl, err := p.Decisions().ListDecisions(ctx, &policiesv1.DecisionFilter{PageSize: 10})
	if err != nil {
		t.Logf("decisions: ERROR %v", err)
	} else {
		t.Logf("decisions: items=%d total=%d", len(dl.GetItems()), dl.GetTotalCount())
		for _, d := range dl.GetItems() {
			t.Logf("   id=%s repo=%s policy=%q mode=%s result=%s digest=%.20s pulled=%v reason=%.40q",
				d.GetId(), d.GetRepoId(), d.GetPolicyName(), d.GetMode(), d.GetResult(), d.GetDigest(), d.GetPulledOn(), d.GetReason())
		}
	}

	ol, err := p.Overrides().ListOverrides(ctx, &policiesv1.OverrideFilter{PageSize: 10})
	if err != nil {
		t.Logf("overrides: ERROR %v", err)
	} else {
		t.Logf("overrides: items=%d total=%d", len(ol.GetItems()), ol.GetTotalCount())
		for _, o := range ol.GetItems() {
			t.Logf("   id=%s policy=%s digest=%.20s by=%s reason=%.40q", o.GetId(), o.GetPolicyId(), o.GetDigest(), o.GetCreatedBy(), o.GetReason())
		}
	}
}

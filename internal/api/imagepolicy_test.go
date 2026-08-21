package api

import (
	"fmt"
	"testing"
	"time"

	policiesv1 "chainguard.dev/sdk/proto/platform/policies/v1"
	"google.golang.org/genproto/googleapis/type/date"
	"google.golang.org/protobuf/types/known/structpb"
)

// System policies sit outside the group hierarchy, like built-in IAM roles, so
// an org with no custom policies must still see them.
func TestPoliciesForGroup(t *testing.T) {
	t.Parallel()
	all := []ImagePolicy{
		{UID: "sys-b", Name: "no-critical-cves", Type: policyTypeSystem},
		{UID: "sys-a", Name: "signed-images", Type: policyTypeSystem},
		{UID: "org/1/pol", Name: "internal-only", Type: policyTypeCustom},
		{UID: "org/2/pol", Name: "someone-elses", Type: policyTypeCustom},
	}
	got := policiesForGroup(all, "org/1")

	var names []string
	for _, p := range got {
		names = append(names, p.Name)
	}
	// Own custom policy first, then system ones by name; another org's is dropped.
	want := []string{"internal-only", "no-critical-cves", "signed-images"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", names, want)
	}

	// An org with nothing of its own still gets the system policies.
	if got := policiesForGroup(all, "org/9"); len(got) != 2 {
		t.Fatalf("got %d policies, want the 2 system ones", len(got))
	}
}

func TestImagePolicyBindingScope(t *testing.T) {
	t.Parallel()
	// A binding is a child of the scope it applies to.
	if got := (ImagePolicyBinding{UID: "org/1/repo/binding"}).Scope(); got != "org/1/repo" {
		t.Errorf("scope=%q", got)
	}
	if got := (ImagePolicyBinding{UID: "root"}).Scope(); got != "root" {
		t.Errorf("scope=%q, want the id itself when it has no parent", got)
	}
}

func TestImagePolicyLabels(t *testing.T) {
	t.Parallel()
	if got := imagePolicyTypeLabel(policiesv1.PolicyType_POLICY_TYPE_SYSTEM); got != "system" {
		t.Errorf("type=%q", got)
	}
	if got := imagePolicyTypeLabel(policiesv1.PolicyType_POLICY_TYPE_UNSPECIFIED); got != "" {
		t.Errorf("type=%q, want empty", got)
	}
	if got := imagePolicyModeLabel(policiesv1.PolicyMode_POLICY_MODE_DRY_RUN); got != "dry-run" {
		t.Errorf("mode=%q", got)
	}
	if got := imagePolicyModeLabel(policiesv1.PolicyMode_POLICY_MODE_ENFORCED); got != "enforced" {
		t.Errorf("mode=%q", got)
	}
	if got := policyResultLabel(policiesv1.Result_RESULT_DENIED); got != "denied" {
		t.Errorf("result=%q", got)
	}
	if got := parameterTypeLabel(policiesv1.ParameterType_PARAMETER_TYPE_STRING_LIST); got != "string list" {
		t.Errorf("param type=%q", got)
	}
}

func TestMapImagePolicyResourceType(t *testing.T) {
	t.Parallel()
	// The current field wins.
	p := mapImagePolicy(&policiesv1.Policy{
		Id:                     "org/1/pol",
		Name:                   "internal-only",
		SupportedResourceType:  "registry.chainguard.dev/Repo@v1",
		SupportedResourceTypes: []string{"old"},
		PolicyType:             policiesv1.PolicyType_POLICY_TYPE_CUSTOM,
		ParameterSchemas: []*policiesv1.ParameterSchema{
			{Name: "severity", Type: policiesv1.ParameterType_PARAMETER_TYPE_STRING, Required: true},
		},
	})
	if p.ResourceType != "registry.chainguard.dev/Repo@v1" {
		t.Errorf("resourceType=%q", p.ResourceType)
	}
	if len(p.Parameters) != 1 || p.Parameters[0].Name != "severity" || !p.Parameters[0].Required {
		t.Errorf("parameters=%+v", p.Parameters)
	}

	// Records that only populate the deprecated repeated field still resolve.
	old := mapImagePolicy(&policiesv1.Policy{SupportedResourceTypes: []string{"registry.chainguard.dev/Repo"}})
	if old.ResourceType != "registry.chainguard.dev/Repo" {
		t.Errorf("resourceType=%q, want the deprecated field as fallback", old.ResourceType)
	}
	if got := mapImagePolicy(&policiesv1.Policy{}).ResourceType; got != "" {
		t.Errorf("resourceType=%q, want empty", got)
	}
}

func TestParameterValueString(t *testing.T) {
	t.Parallel()
	list, _ := structpb.NewList([]any{"critical", "high"})
	for _, tc := range []struct {
		name string
		in   *structpb.Value
		want string
	}{
		{"string", structpb.NewStringValue("critical"), "critical"},
		{"number", structpb.NewNumberValue(5), "5"},
		{"bool", structpb.NewBoolValue(true), "true"},
		{"list", structpb.NewListValue(list), "critical, high"},
		{"nil", nil, ""},
	} {
		if got := parameterValueString(tc.in); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestDateTime(t *testing.T) {
	t.Parallel()
	got := dateTime(&date.Date{Year: 2026, Month: 8, Day: 20})
	if !got.Equal(time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("got %v", got)
	}
	if got := dateTime(nil); !got.IsZero() {
		t.Errorf("nil date=%v, want zero", got)
	}
	if got := dateTime(&date.Date{}); !got.IsZero() {
		t.Errorf("empty date=%v, want zero", got)
	}
}

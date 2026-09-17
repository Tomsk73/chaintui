package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	iamv2 "chainguard.dev/sdk/proto/chainguard/platform/iam/v2beta1"
	librariesv2 "chainguard.dev/sdk/proto/chainguard/platform/libraries/v2beta1"
	vulnv2 "chainguard.dev/sdk/proto/chainguard/platform/vulnerabilities/v2beta1"
	librariesv1 "chainguard.dev/sdk/proto/platform/libraries/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestExactName(t *testing.T) {
	t.Parallel()
	if got := exactName(PageOpts{Query: "  nginx  "}); got != "nginx" {
		t.Fatalf("got %q", got)
	}
	if got := exactName(PageOpts{}); got != "" {
		t.Fatalf("empty query: got %q", got)
	}
}

func TestUIDPHelpers(t *testing.T) {
	t.Parallel()

	children := uidpChildren("org/1")
	if children.GetChildrenOf() != "org/1" || children.GetDescendantsOf() != "" {
		t.Fatalf("uidpChildren: %+v", children)
	}
	root := uidpChildren("")
	if !root.GetInRoot() {
		t.Fatal("uidpChildren(\"\") should set InRoot")
	}

	scope := uidpScope("org/1")
	if scope.GetDescendantsOf() != "org/1" || scope.GetChildrenOf() != "" {
		t.Fatalf("uidpScope: %+v", scope)
	}
	if uidpScope("") != nil {
		t.Fatal("uidpScope(\"\") should be nil")
	}
}

func TestParseAdvisorySkip(t *testing.T) {
	t.Parallel()

	n, err := parseAdvisorySkip("")
	if err != nil || n != 0 {
		t.Fatalf("empty: n=%d err=%v", n, err)
	}
	n, err = parseAdvisorySkip("42")
	if err != nil || n != 42 {
		t.Fatalf("42: n=%d err=%v", n, err)
	}
	if _, err := parseAdvisorySkip("-1"); err == nil {
		t.Fatal("expected error for negative skip")
	}
	if _, err := parseAdvisorySkip("opaque-token"); err == nil {
		t.Fatal("expected error for opaque token")
	}
}

func TestParsePURL(t *testing.T) {
	t.Parallel()
	name, ver := parsePURL("pkg:apk/wolfi/curl@8.5.0-r0")
	if name != "curl" || ver != "8.5.0-r0" {
		t.Fatalf("got name=%q ver=%q", name, ver)
	}
	name, ver = parsePURL("pkg:apk/wolfi/glibc@2.39?arch=x86_64")
	if name != "glibc" || ver != "2.39" {
		t.Fatalf("query strip: name=%q ver=%q", name, ver)
	}
}

func TestIsNotLoggedIn(t *testing.T) {
	t.Parallel()
	if !IsNotLoggedIn(ErrNotLoggedIn) {
		t.Fatal("bare ErrNotLoggedIn")
	}
	if !IsNotLoggedIn(fmt.Errorf("%w: details", ErrNotLoggedIn)) {
		t.Fatal("wrapped ErrNotLoggedIn")
	}
	if IsNotLoggedIn(errors.New("other")) {
		t.Fatal("unrelated error")
	}
}

func TestParseLibraryEcosystem(t *testing.T) {
	t.Parallel()
	java, err := parseLibraryEcosystem("java")
	if err != nil || java != librariesv2.Ecosystem_ECOSYSTEM_JAVA {
		t.Fatalf("java: %v %v", java, err)
	}
	py, err := parseLibraryEcosystem("pypi")
	if err != nil || py != librariesv2.Ecosystem_ECOSYSTEM_PYTHON {
		t.Fatalf("pypi: %v %v", py, err)
	}
	if _, err := parseLibraryEcosystem("npm"); err == nil {
		// javascript is handled separately via isJavaScriptEcosystem
		t.Fatal("expected parseLibraryEcosystem error for npm (use isJavaScriptEcosystem)")
	}
	if !isJavaScriptEcosystem("javascript") || !isJavaScriptEcosystem("npm") || !isJavaScriptEcosystem("js") {
		t.Fatal("expected javascript aliases")
	}
	if isJavaScriptEcosystem("java") {
		t.Fatal("java should not be javascript")
	}
}

func TestLibrarySourceLabels(t *testing.T) {
	t.Parallel()
	if got := npmSourceLabel(librariesv1.NpmSourceType_NPM_SOURCE_TYPE_INTERNAL_REMEDIATED); got != "remediated" {
		t.Fatalf("npm remediated: %q", got)
	}
	if got := artifactSourceLabel(librariesv1.SourceType_SOURCE_TYPE_INTERNAL); got != "internal" {
		t.Fatalf("artifact internal: %q", got)
	}
	if !isRemediatedSource("remediated") || isRemediatedSource("internal") {
		t.Fatal("isRemediatedSource")
	}
}

func TestPageSlice(t *testing.T) {
	t.Parallel()
	items := []int{0, 1, 2, 3, 4}
	page := pageSlice(items, PageOpts{PageSize: 2})
	if len(page.Items) != 2 || page.Items[0] != 0 || page.NextPageToken != "2" || page.TotalCount != 5 {
		t.Fatalf("first page: %+v", page)
	}
	page = pageSlice(items, PageOpts{PageSize: 2, PageToken: "2"})
	if len(page.Items) != 2 || page.Items[0] != 2 || page.NextPageToken != "4" {
		t.Fatalf("second page: %+v", page)
	}
	page = pageSlice(items, PageOpts{PageSize: 2, PageToken: "4"})
	if len(page.Items) != 1 || page.Items[0] != 4 || page.NextPageToken != "" {
		t.Fatalf("last page: %+v", page)
	}
}

func advisoryAt(i int) *vulnv2.Advisory {
	return &vulnv2.Advisory{
		Uid:          fmt.Sprintf("uid-%d", i),
		AdvisoryId:   fmt.Sprintf("CGA-%04d", i),
		ArtifactName: fmt.Sprintf("pkg-%d", i),
	}
}

func totalCountPtr(n int) *int64 {
	v := int64(n)
	return &v
}

// fakeAdvisories simulates a skip-based list where poisonOffsets return Internal.
func fakeAdvisories(total int, poison map[int]bool) advisoryFetcher {
	return func(pageSize, skip int32) (*vulnv2.ListAdvisoriesResponse, error) {
		if pageSize == 1 {
			idx := int(skip)
			if idx >= total {
				return &vulnv2.ListAdvisoriesResponse{TotalCount: totalCountPtr(total)}, nil
			}
			if poison[idx] {
				return nil, status.Error(codes.Internal, "poison")
			}
			return &vulnv2.ListAdvisoriesResponse{
				Advisories: []*vulnv2.Advisory{advisoryAt(idx)},
				TotalCount: totalCountPtr(total),
			}, nil
		}

		var out []*vulnv2.Advisory
		for i := 0; i < int(pageSize); i++ {
			idx := int(skip) + i
			if idx >= total {
				break
			}
			if poison[idx] {
				return nil, status.Error(codes.Internal, "poison in batch")
			}
			out = append(out, advisoryAt(idx))
		}
		return &vulnv2.ListAdvisoriesResponse{
			Advisories: out,
			TotalCount: totalCountPtr(total),
		}, nil
	}
}

func TestCollectAdvisoriesPageHappyPath(t *testing.T) {
	t.Parallel()
	page, err := collectAdvisoriesPage(PageOpts{PageSize: 5}, fakeAdvisories(20, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 5 {
		t.Fatalf("items=%d", len(page.Items))
	}
	if page.NextPageToken != "5" {
		t.Fatalf("next=%q", page.NextPageToken)
	}
	if page.TotalCount != 20 {
		t.Fatalf("total=%d", page.TotalCount)
	}
	if page.Items[0].AdvisoryID != "CGA-0000" {
		t.Fatalf("first=%q", page.Items[0].AdvisoryID)
	}
}

func TestCollectAdvisoriesPageSkipsPoison(t *testing.T) {
	t.Parallel()
	// Offsets 2 and 3 are unreadable; page should still fill with later rows.
	poison := map[int]bool{2: true, 3: true}
	page, err := collectAdvisoriesPage(PageOpts{PageSize: 5}, fakeAdvisories(20, poison))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 5 {
		t.Fatalf("items=%d want 5", len(page.Items))
	}
	ids := make([]string, len(page.Items))
	for i, a := range page.Items {
		ids[i] = a.AdvisoryID
	}
	want := []string{"CGA-0000", "CGA-0001", "CGA-0004", "CGA-0005", "CGA-0006"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids=%v want %v", ids, want)
		}
	}
	// Consumed through index 7 (next skip after 0,1,skip2,skip3,4,5,6).
	if page.NextPageToken != "7" {
		t.Fatalf("next=%q want 7", page.NextPageToken)
	}
}

func TestCollectAdvisoriesPageFromSkipToken(t *testing.T) {
	t.Parallel()
	page, err := collectAdvisoriesPage(PageOpts{PageSize: 3, PageToken: "10"}, fakeAdvisories(20, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 3 || page.Items[0].AdvisoryID != "CGA-0010" {
		t.Fatalf("got %+v", page.Items)
	}
	if page.NextPageToken != "13" {
		t.Fatalf("next=%q", page.NextPageToken)
	}
}

func TestCollectAdvisoriesPageEndOfList(t *testing.T) {
	t.Parallel()
	page, err := collectAdvisoriesPage(PageOpts{PageSize: 10, PageToken: "18"}, fakeAdvisories(20, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items=%d", len(page.Items))
	}
	if page.NextPageToken != "" {
		t.Fatalf("expected empty next, got %q", page.NextPageToken)
	}
}

func TestCollectAdvisoriesPageCapsSize(t *testing.T) {
	t.Parallel()
	page, err := collectAdvisoriesPage(PageOpts{PageSize: 100}, fakeAdvisories(100, nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != int(maxAdvisoryPage) {
		t.Fatalf("items=%d want capped %d", len(page.Items), maxAdvisoryPage)
	}
}

func TestCollectAdvisoriesPageStaleToken(t *testing.T) {
	t.Parallel()
	_, err := collectAdvisoriesPage(PageOpts{PageToken: "abc"}, fakeAdvisories(5, nil))
	if err == nil {
		t.Fatal("expected stale token error")
	}
}

func TestCollectAdvisoriesPageNonInternalError(t *testing.T) {
	t.Parallel()
	fetch := func(pageSize, skip int32) (*vulnv2.ListAdvisoriesResponse, error) {
		return nil, status.Error(codes.PermissionDenied, "nope")
	}
	_, err := collectAdvisoriesPage(PageOpts{PageSize: 5}, fetch)
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("got %v", err)
	}
}

func TestDistroPackageNames(t *testing.T) {
	t.Parallel()
	// APKs are kept, deduped and sorted; language-ecosystem packages are not
	// advisory artifacts and are dropped.
	got := distroPackageNames([]SBOMPackage{
		{Name: "openssl", Purl: "pkg:apk/wolfi/openssl@3.1.4-r0"},
		{Name: "glibc", Purl: "pkg:apk/wolfi/glibc@2.38-r0"},
		{Name: "openssl", Purl: "pkg:apk/wolfi/openssl@3.1.4-r1"},
		{Name: "golang.org/x/net", Purl: "pkg:golang/golang.org/x/net@v0.17.0"},
		{Name: "", Purl: "pkg:apk/wolfi/@1"},
	})
	want := []string{"glibc", "openssl"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// An SBOM that carries PURLs but no APKs genuinely has no distro packages.
	if got := distroPackageNames([]SBOMPackage{
		{Name: "golang.org/x/net", Purl: "pkg:golang/golang.org/x/net@v0.17.0"},
	}); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}

	// One with no PURLs at all is an unrecognised shape, so fall back to every
	// name rather than silently filtering the page down to nothing.
	if got := distroPackageNames([]SBOMPackage{{Name: "openssl"}, {Name: "glibc"}}); fmt.Sprint(got) != fmt.Sprint([]string{"glibc", "openssl"}) {
		t.Fatalf("no-purl fallback: got %v", got)
	}

	if got := distroPackageNames(nil); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestDigestForTag(t *testing.T) {
	t.Parallel()
	tags := []Tag{
		{Name: "latest-dev", Digest: "sha256:dev"},
		{Name: "latest", Digest: "sha256:abc"},
	}
	if got := digestForTag(tags, "latest"); got != "sha256:abc" {
		t.Fatalf("got %q, want the exact match not the prefix one", got)
	}
	if got := digestForTag(tags, "1.2.3"); got != "" {
		t.Fatalf("got %q for a missing tag", got)
	}
	if got := digestForTag([]Tag{{Name: "latest"}}, "latest"); got != "" {
		t.Fatalf("a tag with no digest cannot resolve to an image, got %q", got)
	}
}

// The advisory catalogue is global. Scoping the request to the caller's org
// returned nothing at all, which is what made both advisory pages come up empty.
func TestAdvisoryRequestIsNotOrgScoped(t *testing.T) {
	t.Parallel()
	req := advisoryRequest(AdvisoryFilter{}, PageOpts{Query: "nginx", OrderBy: "created_at desc"}, 25, 50)
	if req.GetUidp() != nil {
		t.Fatalf("uidp=%v, want none", req.GetUidp())
	}
	if req.GetQuery() != "nginx" || req.GetOrderBy() != "created_at desc" {
		t.Fatalf("opts not carried: %+v", req)
	}
	if req.GetPageSize() != 25 || req.GetSkip() != 50 {
		t.Fatalf("paging not carried: size=%d skip=%d", req.GetPageSize(), req.GetSkip())
	}
}

func TestAdvisoryRequestFilter(t *testing.T) {
	t.Parallel()
	// component_names is the filter the server honours; artifact_names is
	// accepted and then ignored, so nothing should ever set it.
	req := advisoryRequest(AdvisoryFilter{
		ComponentNames: []string{"glibc", "zlib"},
		Architecture:   "x86_64",
	}, PageOpts{}, 25, 0)
	if fmt.Sprint(req.GetComponentNames()) != fmt.Sprint([]string{"glibc", "zlib"}) {
		t.Fatalf("componentNames=%v", req.GetComponentNames())
	}
	if len(req.GetArtifactNames()) != 0 {
		t.Fatalf("artifactNames=%v, but the server ignores that filter", req.GetArtifactNames())
	}
	if fmt.Sprint(req.GetArtifactArchitectures()) != fmt.Sprint([]string{"x86_64"}) {
		t.Fatalf("arches=%v", req.GetArtifactArchitectures())
	}

	// No architecture means no filter, rather than an empty-string one that
	// would match nothing.
	if got := advisoryRequest(AdvisoryFilter{}, PageOpts{}, 25, 0).GetArtifactArchitectures(); len(got) != 0 {
		t.Fatalf("arches=%v, want none", got)
	}
}

func TestAdvisoryStatus(t *testing.T) {
	t.Parallel()
	at := func(h int) time.Time { return time.Date(2026, 8, 1, h, 0, 0, 0, time.UTC) }

	// The status is the most recent approved event.
	a := Advisory{Events: []AdvisoryEvent{
		{Type: AdvisoryEventTypeDetection, ReviewState: ReviewStateApproved, CreateTime: at(1)},
		{Type: AdvisoryEventTypeFixed, ReviewState: ReviewStateApproved, CreateTime: at(2)},
	}}
	if got := a.Status(); got != AdvisoryEventTypeFixed {
		t.Errorf("got %q, want fixed", got)
	}
	if got := a.Status().Label(); got != "Fixed" {
		t.Errorf("label=%q", got)
	}

	// Pending and rejected events do not count: an advisory whose
	// false-positive claim was rejected is not a false positive. This mirrors
	// CGA-9644-6q9c-r6j6, which the API itself classifies by its last approved
	// event despite later rejected ones.
	rejected := Advisory{Events: []AdvisoryEvent{
		{Type: AdvisoryEventTypeDetection, ReviewState: ReviewStateApproved, CreateTime: at(1)},
		{Type: AdvisoryEventTypeFixed, ReviewState: ReviewStateApproved, CreateTime: at(2)},
		{Type: AdvisoryEventTypeFalsePositive, ReviewState: ReviewStateRejected, CreateTime: at(3)},
		{Type: AdvisoryEventTypePendingUpstreamFix, ReviewState: ReviewStatePending, CreateTime: at(4)},
	}}
	if got := rejected.Status(); got != AdvisoryEventTypeFixed {
		t.Errorf("got %q, want the last approved event", got)
	}

	// Out-of-order events still resolve to the newest.
	unordered := Advisory{Events: []AdvisoryEvent{
		{Type: AdvisoryEventTypePatched, ReviewState: ReviewStateApproved, CreateTime: at(5)},
		{Type: AdvisoryEventTypeDetection, ReviewState: ReviewStateApproved, CreateTime: at(1)},
	}}
	if got := unordered.Status(); got != AdvisoryEventTypePatched {
		t.Errorf("got %q, want patched", got)
	}

	// A detection nobody has ruled on is still being triaged.
	detected := Advisory{Events: []AdvisoryEvent{
		{Type: AdvisoryEventTypeDetection, ReviewState: ReviewStateApproved, CreateTime: at(1)},
	}}
	if got := detected.Status().Label(); got != "Under Investigation" {
		t.Errorf("label=%q", got)
	}

	// Nothing to go on.
	if got := (Advisory{}).Status(); got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got := (Advisory{Events: []AdvisoryEvent{{Type: AdvisoryEventTypeFixed, ReviewState: ReviewStatePending}}}).Status(); got != "" {
		t.Errorf("a pending-only advisory should have no status, got %q", got)
	}
	if got := AdvisoryEventType("something_new").Label(); got != "" {
		t.Errorf("unknown type should render blank, got %q", got)
	}
}

// The list response hydrates the identity, role and group; dropping them is what
// left the role bindings page showing three opaque UIDPs.
func TestMapRoleBindingKeepsHydratedFields(t *testing.T) {
	t.Parallel()
	v := &iamv2.RoleBinding{
		Uid: "org/1/rb-1",
		Identity: &iamv2.RoleBindingIdentity{
			Uid:    "org/1/id-1",
			Name:   "tom",
			Email:  "tom@example.com",
			Issuer: "https://accounts.google.com",
		},
		Role:  &iamv2.RoleBindingRole{Uid: "role-owner", Name: "owner"},
		Group: &iamv2.RoleBindingGroup{Uid: "org/1", Name: "acme"},
	}
	got := mapRoleBinding(v)

	if got.Identity == nil || got.Identity.Email != "tom@example.com" {
		t.Fatalf("identity=%+v", got.Identity)
	}
	if got.Role == nil || got.Role.Name != "owner" {
		t.Fatalf("role=%+v", got.Role)
	}
	if got.Group == nil || got.Group.Name != "acme" {
		t.Fatalf("group=%+v", got.Group)
	}
	// identity_uid/role_uid are only set on writes, so reads fall back to the
	// hydrated sub-messages for them.
	if got.IdentityUID != "org/1/id-1" || got.RoleUID != "role-owner" {
		t.Fatalf("identityUID=%q roleUID=%q", got.IdentityUID, got.RoleUID)
	}
	if got.Holder() != "tom@example.com" || got.RoleName() != "owner" {
		t.Fatalf("holder=%q role=%q", got.Holder(), got.RoleName())
	}
	if !got.Identity.IsHuman() {
		t.Error("an identity with a verified email and issuer is a person")
	}

	// An unhydrated binding still names its holder, by UIDP.
	bare := mapRoleBinding(&iamv2.RoleBinding{Uid: "org/1/rb-2", IdentityUid: "org/1/id-2", RoleUid: "role-viewer"})
	if bare.Holder() != "org/1/id-2" || bare.RoleName() != "role-viewer" {
		t.Fatalf("holder=%q role=%q", bare.Holder(), bare.RoleName())
	}
}

func TestMapIdentityKeepsEmailAndRelationship(t *testing.T) {
	t.Parallel()
	seen := time.Date(2026, 9, 15, 10, 0, 0, 0, time.UTC)
	human := mapIdentity(&iamv2.Identity{
		Uid:          "org/1/id-1",
		Name:         "tom",
		Email:        "tom@example.com",
		LastSeenTime: timestamppb.New(seen),
		Relationship: &iamv2.Identity_ClaimMatch_{ClaimMatch: &iamv2.Identity_ClaimMatch{
			Iss: &iamv2.Identity_ClaimMatch_Issuer{Issuer: "https://accounts.google.com"},
			Sub: &iamv2.Identity_ClaimMatch_Subject{Subject: "1234"},
		}},
	})
	if human.Email != "tom@example.com" || !human.LastSeenTime.Equal(seen) {
		t.Fatalf("email=%q lastSeen=%v", human.Email, human.LastSeenTime)
	}
	if human.ClaimMatch == nil || human.ClaimMatch.Issuer != "https://accounts.google.com" {
		t.Fatalf("claimMatch=%+v", human.ClaimMatch)
	}
	if got := human.Relationship(); got != "claim match" {
		t.Errorf("relationship=%q", got)
	}

	// The other relationship arms each land in their own field.
	keys := mapIdentity(&iamv2.Identity{Relationship: &iamv2.Identity_StaticKeys_{
		StaticKeys: &iamv2.Identity_StaticKeys{Issuer: "iss", Subject: "sub"},
	}})
	if keys.StaticKeys == nil || keys.Relationship() != "static keys" {
		t.Errorf("staticKeys=%+v relationship=%q", keys.StaticKeys, keys.Relationship())
	}
	aws := mapIdentity(&iamv2.Identity{Relationship: &iamv2.Identity_AwsIdentity{
		AwsIdentity: &iamv2.Identity_AWSIdentity{AwsAccount: "1234"},
	}})
	if aws.AWSIdentity == nil || aws.Relationship() != "aws" {
		t.Errorf("aws=%+v relationship=%q", aws.AWSIdentity, aws.Relationship())
	}
	sp := mapIdentity(&iamv2.Identity{Relationship: &iamv2.Identity_ServicePrincipal{
		ServicePrincipal: iamv2.ServicePrincipal_SERVICE_PRINCIPAL_INGESTER,
	}})
	if sp.ServicePrincipal != ServicePrincipalIngester || sp.Relationship() != "service principal" {
		t.Errorf("sp=%q relationship=%q", sp.ServicePrincipal, sp.Relationship())
	}
	if got := mapIdentity(&iamv2.Identity{}).Relationship(); got != "" {
		t.Errorf("relationship=%q, want empty", got)
	}
}

func TestParseTokenSeparatesIdentityFromActor(t *testing.T) {
	t.Parallel()
	// sub is who the session acts as; act.sub is the human who assumed it.
	assumed := fakeJWT(t, map[string]any{
		"sub":   "org/1/support-identity",
		"email": "tom@chainguard.dev",
		"act":   map[string]any{"sub": "google-oauth2|1234"},
	})
	subject, identity, email := parseToken(assumed)
	if identity != "org/1/support-identity" {
		t.Errorf("identity=%q, want the token's own sub", identity)
	}
	if subject != "google-oauth2|1234" {
		t.Errorf("subject=%q, want the actor", subject)
	}
	if email != "tom@chainguard.dev" {
		t.Errorf("email=%q", email)
	}

	// Without an actor claim the two are the same and nothing is assumed.
	plain := fakeJWT(t, map[string]any{"sub": "google-oauth2|1234", "email": "tom@chainguard.dev"})
	subject, identity, _ = parseToken(plain)
	if subject != identity || identity != "google-oauth2|1234" {
		t.Errorf("subject=%q identity=%q", subject, identity)
	}
}

// fakeJWT builds an unsigned token with the given claims. parseToken reads the
// payload without verifying, which is all these tests need.
func fakeJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(payload) + ".sig"
}

func TestLoginCommand(t *testing.T) {
	t.Parallel()
	cmd, err := LoginCommand("org/1/support")
	if err != nil {
		t.Skipf("chainctl not installed: %v", err)
	}
	if got := cmd.Args[len(cmd.Args)-1]; got != "--identity=org/1/support" {
		t.Errorf("args=%v", cmd.Args)
	}
	// No identity means log in as yourself, which is how you stop assuming one.
	plain, err := LoginCommand("  ")
	if err != nil {
		t.Fatal(err)
	}
	for _, arg := range plain.Args {
		if strings.HasPrefix(arg, "--identity") {
			t.Errorf("unexpected identity flag: %v", plain.Args)
		}
	}
}

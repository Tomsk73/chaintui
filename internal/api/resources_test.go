package api

import (
	"errors"
	"fmt"
	"testing"

	librariesv2 "chainguard.dev/sdk/proto/chainguard/platform/libraries/v2beta1"
	vulnv2 "chainguard.dev/sdk/proto/chainguard/platform/vulnerabilities/v2beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
		t.Fatal("expected error for npm")
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

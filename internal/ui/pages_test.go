package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tomsk73/chaintui/internal/api"
)

func TestWriteLibraryInventory(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	inv := api.LibraryInventory{
		Ecosystem:    "python",
		GeneratedAt:  time.Date(2026, 8, 18, 14, 30, 0, 0, time.UTC),
		PackageCount: 1,
		VersionCount: 2,
		Packages: []api.InventoryPackage{
			{Name: "requests", LatestVersion: "2.32.3", Versions: []string{"2.31.0", "2.32.3"}},
		},
	}

	name, err := writeLibraryInventory(inv)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if name != "20260818T143000Z-python.json" {
		t.Fatalf("name=%q", name)
	}

	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	var got api.LibraryInventory
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Ecosystem != "python" || len(got.Packages) != 1 {
		t.Fatalf("round trip: %+v", got)
	}
	if pkg := got.Packages[0]; pkg.Name != "requests" || len(pkg.Versions) != 2 {
		t.Fatalf("package: %+v", got.Packages[0])
	}

	// Same timestamp again must not clobber the existing snapshot.
	if _, err := writeLibraryInventory(inv); !os.IsExist(err) {
		t.Fatalf("second write err=%v, want already-exists", err)
	}
}

// pushedPage runs a page-opening command and returns the page it pushes.
func pushedPage(t *testing.T, cmd tea.Cmd) Page {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command returned")
	}
	msg, ok := cmd().(PushMsg)
	if !ok {
		t.Fatalf("msg type %T, want PushMsg", cmd())
	}
	return msg.P
}

func TestReposEnterOpensRepoMenu(t *testing.T) {
	t.Parallel()
	repos := NewReposPage(nil, "org/1")
	row := RowData{
		UID:     "org/1/repo",
		Columns: []string{"nginx", "web server", "1d ago"},
		Raw:     api.Repo{UID: "org/1/repo", Name: "nginx"},
	}
	menu := pushedPage(t, repos.enterFn(row))
	if got := menu.ResourceType(); got != "repo" {
		t.Fatalf("resource=%q, want the repo menu", got)
	}
	if got := menu.Label(); got != "nginx" {
		t.Fatalf("label=%q", got)
	}
	// The menu keeps the org context so advisories and `:` commands stay scoped
	// to the org rather than to the repo.
	if got := menu.GroupContext(); got != "org/1" {
		t.Fatalf("groupCtx=%q, want the org", got)
	}
}

func TestRepoMenuEntries(t *testing.T) {
	t.Parallel()
	menu := NewRepoMenuPage(nil, "org/1", "org/1/repo", "nginx")
	page, err := menu.loadFn("", 50, "", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range page.Rows {
		names = append(names, r.Columns[0])
	}
	want := []string{"tags", "cves", "advisories", "policy"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("options=%v, want %v", names, want)
	}

	// Each option opens the page it advertises.
	opened := map[string]Page{}
	for _, r := range page.Rows {
		opened[r.Columns[0]] = pushedPage(t, menu.enterFn(r))
	}
	for _, tc := range []struct{ option, resource, label string }{
		{"tags", "tags", "nginx"},
		{"cves", "cves", "nginx:latest cves"},
		{"advisories", "advisories", "nginx:latest advisories"},
		{"policy", "policydecisions", "nginx policy"},
	} {
		p := opened[tc.option]
		if got := p.ResourceType(); got != tc.resource {
			t.Errorf("%s: resource=%q, want %q", tc.option, got, tc.resource)
		}
		if got := p.Label(); got != tc.label {
			t.Errorf("%s: label=%q, want %q", tc.option, got, tc.label)
		}
	}
}

func TestDescribeImageError(t *testing.T) {
	t.Parallel()
	got := describeImageError(fmt.Errorf("%w tagged latest", api.ErrNoImage), "nginx", "latest")
	if !errors.Is(got, api.ErrNoImage) {
		t.Fatal("should still wrap ErrNoImage")
	}
	if !strings.Contains(got.Error(), "tags") {
		t.Fatalf("should point at the tag list: %v", got)
	}
	other := errors.New("permission denied")
	if describeImageError(other, "nginx", "latest") != other {
		t.Fatal("unrelated errors should pass through untouched")
	}
}

// The API accepts only uid and created_at as order_by, rejecting anything else
// with InvalidArgument, and its own default of "uid asc" reads as random.
func TestAdvisoryOrder(t *testing.T) {
	t.Parallel()
	if got := advisoryOrder(""); got != "created_at desc" {
		t.Errorf("default order = %q, want the newest advisories first", got)
	}
	if got := advisoryOrder("uid asc"); got != "uid asc" {
		t.Errorf("an explicit order should win, got %q", got)
	}
}

func TestAdvisoryRowShowsStatus(t *testing.T) {
	t.Parallel()
	cols := advisoryCols()
	if cols[2].Title != "STATUS" {
		t.Fatalf("column 2 is %q, want STATUS", cols[2].Title)
	}
	for _, c := range cols {
		if c.Title == "CREATED" {
			t.Fatal("the created date column should be gone")
		}
	}

	at := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	row := advisoryRow(api.Advisory{
		AdvisoryID:   "CGA-1234",
		ArtifactName: "openssl",
		Aliases:      []string{"CVE-2026-1", "GHSA-x"},
		Events: []api.AdvisoryEvent{
			{Type: api.AdvisoryEventTypeDetection, ReviewState: api.ReviewStateApproved, CreateTime: at},
			{Type: api.AdvisoryEventTypeFixed, ReviewState: api.ReviewStateApproved, CreateTime: at.Add(time.Hour)},
		},
	})
	want := []string{"CGA-1234", "openssl", "Fixed", "CVE-2026-1, GHSA-x"}
	if fmt.Sprint(row.Columns) != fmt.Sprint(want) {
		t.Fatalf("row=%v, want %v", row.Columns, want)
	}

	// An advisory with no events to go on still renders a cell.
	if got := advisoryRow(api.Advisory{AdvisoryID: "CGA-2"}).Columns[2]; got != "-" {
		t.Fatalf("unknown status rendered as %q", got)
	}
}

package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
)

func testListPage(loadFn func(string, int, string, string) (PageResult, error)) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 20},
		{Title: "UID", Width: 10},
		{Title: "CREATED", Width: 10},
	}
	if loadFn == nil {
		loadFn = func(string, int, string, string) (PageResult, error) {
			return PageResult{}, nil
		}
	}
	p := newListPage("repos", "org", cols, loadFn, nil)
	p.loading = false
	p.SetSize(80, 24)
	return p
}

func rows(names ...string) []RowData {
	out := make([]RowData, len(names))
	for i, n := range names {
		out[i] = RowData{
			UID:     "uid-" + n,
			Columns: []string{n, "id-" + n, "1d ago"},
		}
	}
	return out
}

func TestApplyFilterLocal(t *testing.T) {
	t.Parallel()
	p := testListPage(nil)
	p.allRows = rows("nginx", "curl", "openssl", "nginx-fips")
	p.filter = "nginx"
	p.applyFilter()
	if len(p.displayedRows) != 2 {
		t.Fatalf("got %d rows", len(p.displayedRows))
	}
	if p.displayedRows[0].Columns[0] != "nginx" || p.displayedRows[1].Columns[0] != "nginx-fips" {
		t.Fatalf("got %+v", p.displayedRows)
	}
}

func TestApplyFilterServerSkipsLocal(t *testing.T) {
	t.Parallel()
	p := testListPage(nil).WithServerFilter()
	p.allRows = rows("nginx", "curl")
	p.filter = "nginx"
	p.applyFilter()
	// Server filter trusts the loaded page; local substring filter is not applied.
	if len(p.displayedRows) != 2 {
		t.Fatalf("got %d", len(p.displayedRows))
	}
}

func TestApplyFilterLocalSort(t *testing.T) {
	t.Parallel()
	p := testListPage(nil)
	p.allRows = rows("curl", "nginx", "openssl")
	p.sortCol = 0
	p.sortAsc = true
	p.applyFilter()
	if p.displayedRows[0].Columns[0] != "curl" {
		t.Fatalf("asc first=%q", p.displayedRows[0].Columns[0])
	}
	p.sortAsc = false
	p.applyFilter()
	if p.displayedRows[0].Columns[0] != "openssl" {
		t.Fatalf("desc first=%q", p.displayedRows[0].Columns[0])
	}
}

func TestOrderByArgAndServerSort(t *testing.T) {
	t.Parallel()
	p := testListPage(nil).WithServerSort(map[int]string{0: "name", 2: "create_time"})
	if p.orderByArg() != "" {
		t.Fatal("unsorted should have empty orderBy")
	}
	p.sortCol = 0
	p.sortAsc = true
	if !p.usesServerSort() {
		t.Fatal("expected server sort")
	}
	if got := p.orderByArg(); got != "name asc" {
		t.Fatalf("got %q", got)
	}
	p.sortAsc = false
	if got := p.orderByArg(); got != "name desc" {
		t.Fatalf("got %q", got)
	}
	p.sortCol = 1 // unmapped
	if p.usesServerSort() || p.orderByArg() != "" {
		t.Fatal("unmapped column should fall back to local sort")
	}
}

func TestServerSortSkipsLocalReorder(t *testing.T) {
	t.Parallel()
	p := testListPage(nil).WithServerSort(map[int]string{0: "name"})
	p.allRows = rows("nginx", "curl") // already server-ordered
	p.sortCol = 0
	p.sortAsc = true
	p.applyFilter()
	if p.displayedRows[0].Columns[0] != "nginx" {
		t.Fatalf("should keep API order, got %q", p.displayedRows[0].Columns[0])
	}
}

func TestDoLoadPassesQueryAndOrderBy(t *testing.T) {
	t.Parallel()
	var gotToken string
	var gotSize int
	var gotQuery, gotOrderBy string
	p := testListPage(func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		gotToken, gotSize, gotQuery, gotOrderBy = token, pageSize, query, orderBy
		return PageResult{Rows: rows("a"), NextPageToken: "n1", TotalCount: 10}, nil
	}).WithServerNameFilter().WithServerSort(map[int]string{0: "name"})
	p.filter = "nginx"
	p.sortCol = 0
	p.sortAsc = false
	p.pageSize = 25

	cmd := p.doLoad("tok")
	msg := cmd()
	loaded, ok := msg.(LoadedMsg)
	if !ok {
		t.Fatalf("msg type %T", msg)
	}
	if gotToken != "tok" || gotSize != 25 || gotQuery != "nginx" || gotOrderBy != "name desc" {
		t.Fatalf("token=%q size=%d query=%q orderBy=%q", gotToken, gotSize, gotQuery, gotOrderBy)
	}
	if loaded.RequestToken != "tok" || loaded.NextPageToken != "n1" {
		t.Fatalf("%+v", loaded)
	}
}

func TestPaginationKeys(t *testing.T) {
	t.Parallel()
	p := testListPage(func(token string, _ int, _, _ string) (PageResult, error) {
		next := ""
		if token == "" {
			next = "page2"
		} else if token == "page2" {
			next = "page3"
		}
		return PageResult{Rows: rows("x"), NextPageToken: next, TotalCount: 100}, nil
	})

	m, _ := p.Update(LoadedMsg{
		PageResult:   PageResult{Rows: rows("a"), NextPageToken: "page2", TotalCount: 100},
		RequestToken: "",
	})
	p = m.(*ListPage)
	if p.pageNum != 1 || p.nextPageToken != "page2" {
		t.Fatalf("page1 state: num=%d next=%q", p.pageNum, p.nextPageToken)
	}

	m, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	p = m.(*ListPage)
	if cmd == nil {
		t.Fatal("expected load cmd for ]")
	}
	if p.pageNum != 2 {
		t.Fatalf("pageNum=%d after ]", p.pageNum)
	}
	if len(p.prevTokens) != 1 || p.prevTokens[0] != "" {
		t.Fatalf("prevTokens=%v", p.prevTokens)
	}

	m, _ = p.Update(LoadedMsg{
		PageResult:   PageResult{Rows: rows("b"), NextPageToken: "page3", TotalCount: 100},
		RequestToken: "page2",
	})
	p = m.(*ListPage)

	m, cmd = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	p = m.(*ListPage)
	if cmd == nil {
		t.Fatal("expected load cmd for [")
	}
	if p.pageNum != 1 {
		t.Fatalf("pageNum=%d after [", p.pageNum)
	}
	if len(p.prevTokens) != 0 {
		t.Fatalf("prevTokens should be empty, got %v", p.prevTokens)
	}
}

func TestResetPagination(t *testing.T) {
	t.Parallel()
	p := testListPage(nil)
	p.pageToken = "t"
	p.nextPageToken = "n"
	p.prevTokens = []string{"", "a"}
	p.pageNum = 3
	p.totalCount = 99
	p.resetPagination()
	if p.pageToken != "" || p.nextPageToken != "" || p.prevTokens != nil || p.pageNum != 1 || p.totalCount != 0 {
		t.Fatalf("%+v", p)
	}
}

func TestWithBoolToggle(t *testing.T) {
	t.Parallel()
	flag := false
	var loads int
	p := testListPage(func(string, int, string, string) (PageResult, error) {
		loads++
		return PageResult{Rows: rows("a")}, nil
	}).WithBoolToggle("m", "remediated", &flag)
	p.loading = false

	m, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	p = m.(*ListPage)
	if !flag {
		t.Fatal("expected flag flipped on")
	}
	if cmd == nil || !p.loading {
		t.Fatal("expected reload")
	}
	_ = loads
}

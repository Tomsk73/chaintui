package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func keyPress(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

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

// drainExport pumps export events through Update until the export finishes.
func drainExport(t *testing.T, p *ListPage) *ListPage {
	t.Helper()
	run := p.exportEvents
	if run == nil {
		t.Fatal("no export running")
	}
	for i := 0; i < 100; i++ {
		msg := waitForExport(run)()
		ev, ok := msg.(exportEvent)
		if !ok {
			t.Fatalf("msg type %T", msg)
		}
		m, _ := p.Update(ev)
		p = m.(*ListPage)
		if ev.finished {
			return p
		}
	}
	t.Fatal("export never finished")
	return p
}

func TestExportKeyRunsToCompletion(t *testing.T) {
	t.Parallel()
	p := testListPage(nil).WithExport("x", "exporting java",
		func(_ context.Context, progress func(done, total int)) (string, error) {
			progress(0, 2)
			progress(2, 2)
			return "20260818T143000Z-java.json", nil
		})

	m, cmd := p.Update(keyPress('x'))
	p = m.(*ListPage)
	if !p.exporting || cmd == nil {
		t.Fatalf("expected export to start: exporting=%v cmd=%v", p.exporting, cmd)
	}

	p = drainExport(t, p)
	if p.exporting || p.exportEvents != nil {
		t.Fatal("export state should be cleared when finished")
	}
	if !strings.Contains(p.saveMsg, "20260818T143000Z-java.json") {
		t.Fatalf("saveMsg=%q", p.saveMsg)
	}
}

func TestExportProgressUpdatesStatus(t *testing.T) {
	t.Parallel()
	p := testListPage(nil).WithExport("x", "exporting java", nil)
	p.exporting = true
	if got := p.exportStatus(); !strings.Contains(got, "listing packages...") {
		t.Fatalf("pre-count status=%q", got)
	}
	// Listing phase: total unknown, done is the running discovery count.
	m, _ := p.Update(exportEvent{done: 400, total: 0})
	p = m.(*ListPage)
	if got := p.exportStatus(); !strings.Contains(got, "400 so far") {
		t.Fatalf("listing status=%q", got)
	}
	m, _ = p.Update(exportEvent{done: 3, total: 12})
	p = m.(*ListPage)
	got := p.exportStatus()
	if !strings.Contains(got, "3/12 packages") || !strings.Contains(got, "25%") {
		t.Fatalf("status=%q", got)
	}
	if !strings.Contains(got, "x to cancel") {
		t.Fatalf("status should offer cancel: %q", got)
	}
}

func TestExportKeyCancelsRunningExport(t *testing.T) {
	t.Parallel()
	p := testListPage(nil).WithExport("x", "exporting java",
		func(ctx context.Context, _ func(done, total int)) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		})

	m, _ := p.Update(keyPress('x'))
	p = m.(*ListPage)
	m, cmd := p.Update(keyPress('x')) // second press cancels
	p = m.(*ListPage)
	if cmd != nil {
		t.Fatal("cancel should not start another export")
	}
	p = drainExport(t, p)
	if p.exporting {
		t.Fatal("still exporting after cancel")
	}
	if !strings.Contains(p.saveMsg, "cancelled") {
		t.Fatalf("saveMsg=%q", p.saveMsg)
	}
}

func TestExportBlocksReloadKeys(t *testing.T) {
	t.Parallel()
	flag := false
	p := testListPage(nil).
		WithBoolToggle("m", "remediated", &flag).
		WithExport("x", "exporting java",
			func(ctx context.Context, _ func(done, total int)) (string, error) {
				<-ctx.Done()
				return "", ctx.Err()
			})

	m, _ := p.Update(keyPress('x'))
	p = m.(*ListPage)

	for _, key := range []rune{'r', '/', 'o', 'm', ']'} {
		m, cmd := p.Update(keyPress(key))
		p = m.(*ListPage)
		if cmd != nil {
			t.Fatalf("%q should be ignored while exporting", key)
		}
	}
	if p.filterMode || p.sortMode || flag || p.loading {
		t.Fatalf("export should not be interrupted: filter=%v sort=%v flag=%v loading=%v",
			p.filterMode, p.sortMode, flag, p.loading)
	}

	p.cancelExport()
	p = drainExport(t, p)
}

func TestInventoryFilename(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 8, 18, 14, 30, 0, 0, time.UTC)
	if got := inventoryFilename("python", false, at); got != "20260818T143000Z-python.json" {
		t.Fatalf("got %q", got)
	}
	if got := inventoryFilename("javascript", true, at); got != "20260818T143000Z-javascript-remediated.json" {
		t.Fatalf("remediated: got %q", got)
	}
	// Local-zone input is normalised to UTC so names always sort by real time.
	local := at.In(time.FixedZone("UTC+10", 10*60*60))
	if got := inventoryFilename("java", false, local); got != "20260818T143000Z-java.json" {
		t.Fatalf("zone: got %q", got)
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

// The bubbles table styles the selected row before the viewport pads or
// truncates it, so a row grid wider than the window wraps the highlight onto a
// second line and a narrower one leaves it short of the right edge. Both the
// header and the body grid must come out exactly window-wide.
func TestTableGridFillsWindowWidth(t *testing.T) {
	t.Parallel()
	for _, width := range []int{80, 120, 200} {
		p := testListPage(nil)
		p.allRows = rows("alpine", "nginx")
		p.applyFilter()
		p.SetSize(width, 24)

		// Rows are styled at grid width and only then clipped by the viewport,
		// so measure the grid rather than the clipped output.
		grid := 0
		for _, c := range p.table.Columns() {
			grid += c.Width + tableCellStyle.GetHorizontalFrameSize()
		}
		if grid != width {
			t.Errorf("width %d: row grid is %d cells wide", width, grid)
		}
		if got := lipgloss.Width(strings.Split(p.table.View(), "\n")[0]); got != width {
			t.Errorf("width %d: header is %d cells wide", width, got)
		}
	}
}

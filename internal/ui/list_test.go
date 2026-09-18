package ui

import (
	"context"
	"strings"
	"sync"
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
	}).WithServerFilter().WithServerSort(map[int]string{0: "name"})
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

// searchProbe records what a search asked its loader for. A search runs on its
// own goroutine, so the counters are guarded.
type searchProbe struct {
	mu      sync.Mutex
	calls   int
	size    int
	query   string
	orderBy string
}

func (s *searchProbe) record(size int, query, orderBy string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.size, s.query, s.orderBy = size, query, orderBy
}

func (s *searchProbe) snapshot() (calls, size int, query, orderBy string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls, s.size, s.query, s.orderBy
}

// runBatch runs every command in a tea.Batch, which a reload is (the spinner's
// tick alongside the load itself).
func runBatch(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("msg type %T, want a batch", msg)
	}
	for _, c := range batch {
		c()
	}
}

// submitFilter opens the "/" prompt, types term and presses enter.
func submitFilter(t *testing.T, p *ListPage, term string) (*ListPage, tea.Cmd) {
	t.Helper()
	m, _ := p.Update(keyPress('/'))
	p = m.(*ListPage)
	if !p.filterMode {
		t.Fatal("/ should open the filter prompt")
	}
	p.filterIn.SetValue(term)
	m, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return m.(*ListPage), cmd
}

// drainSearch pumps a running search's events through Update until it finishes.
func drainSearch(t *testing.T, p *ListPage) *ListPage {
	t.Helper()
	run := p.searchRun
	if run == nil {
		t.Fatal("no search running")
	}
	for i := 0; i < 500; i++ {
		msg := waitForSearch(run)()
		m, _ := p.Update(msg)
		p = m.(*ListPage)
		if _, done := msg.(searchDoneMsg); done {
			return p
		}
	}
	t.Fatal("search never finished")
	return p
}

// A search is of the whole resource, not of the page that happens to be loaded:
// matches on later API pages have to come back too.
func TestSearchSpansEveryPage(t *testing.T) {
	t.Parallel()
	var probe searchProbe
	p := testListPage(func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		probe.record(pageSize, query, orderBy)
		switch token {
		case "":
			return PageResult{Rows: rows("curl", "nginx"), NextPageToken: "p2", Status: "scanned now"}, nil
		case "p2":
			return PageResult{Rows: rows("openssl", "nginx-fips"), NextPageToken: "p3"}, nil
		default:
			return PageResult{Rows: rows("python")}, nil
		}
	})
	// The page on screen is the first of three.
	m, _ := p.Update(LoadedMsg{PageResult: PageResult{Rows: rows("curl", "nginx"), NextPageToken: "p2"}})
	p = m.(*ListPage)

	p, cmd := submitFilter(t, p, "nginx")
	if cmd == nil || !p.searching {
		t.Fatalf("expected a search to start: cmd=%v searching=%v", cmd != nil, p.searching)
	}
	p = drainSearch(t, p)

	if p.searching || !p.searchActive {
		t.Fatalf("searching=%v active=%v", p.searching, p.searchActive)
	}
	if len(p.displayedRows) != 2 ||
		p.displayedRows[0].Columns[0] != "nginx" || p.displayedRows[1].Columns[0] != "nginx-fips" {
		t.Fatalf("got %+v", p.displayedRows)
	}
	if p.searchScanned != 5 || p.searchTruncated {
		t.Fatalf("scanned=%d truncated=%v", p.searchScanned, p.searchTruncated)
	}
	if p.status != "scanned now" {
		t.Fatalf("status=%q, want the loader's own summary", p.status)
	}
	// Whole pages at a time, and no Name query: that field is an exact match, so
	// sending the typed term would find nothing.
	calls, size, query, _ := probe.snapshot()
	if calls != 3 || size != searchPageSize || query != "" {
		t.Fatalf("calls=%d size=%d query=%q", calls, size, query)
	}
}

// Results already covering the whole resource are searched in place: a loader
// that fetched everything in one call must not be asked again.
func TestSearchOfSinglePageMakesNoRequest(t *testing.T) {
	t.Parallel()
	var probe searchProbe
	p := testListPage(func(_ string, pageSize int, query, orderBy string) (PageResult, error) {
		probe.record(pageSize, query, orderBy)
		return PageResult{}, nil
	})
	m, _ := p.Update(LoadedMsg{PageResult: PageResult{Rows: rows("nginx", "curl", "nginx-fips")}})
	p = m.(*ListPage)

	p, cmd := submitFilter(t, p, "nginx")
	if cmd != nil || p.searching {
		t.Fatal("a fully loaded result should be searched without another request")
	}
	if !p.searchActive || len(p.displayedRows) != 2 {
		t.Fatalf("active=%v rows=%+v", p.searchActive, p.displayedRows)
	}
	if calls, _, _, _ := probe.snapshot(); calls != 0 {
		t.Fatalf("loader called %d times", calls)
	}
}

// Pages whose API searches for itself keep handing the query over instead.
func TestServerFilterPageQueriesTheAPI(t *testing.T) {
	t.Parallel()
	var probe searchProbe
	p := testListPage(func(_ string, pageSize int, query, orderBy string) (PageResult, error) {
		probe.record(pageSize, query, orderBy)
		return PageResult{Rows: rows("nginx")}, nil
	}).WithServerFilter()

	p, cmd := submitFilter(t, p, "nginx")
	if p.searching || p.searchActive {
		t.Fatal("a server-query page should not walk the pages itself")
	}
	if cmd == nil || !p.loading {
		t.Fatal("expected a reload with the query")
	}
	runBatch(t, cmd)
	if calls, _, query, _ := probe.snapshot(); calls != 1 || query != "nginx" {
		t.Fatalf("calls=%d query=%q", calls, query)
	}
}

// The matches are one list, so the API's page tokens no longer apply to what is
// on screen; clearing the search puts them back.
func TestSearchResultsIgnorePagingThenRestoreIt(t *testing.T) {
	t.Parallel()
	p := testListPage(func(token string, _ int, _, _ string) (PageResult, error) {
		if token == "" {
			return PageResult{Rows: rows("nginx", "curl"), NextPageToken: "p2"}, nil
		}
		return PageResult{Rows: rows("nginx-fips")}, nil
	})
	m, _ := p.Update(LoadedMsg{PageResult: PageResult{Rows: rows("nginx", "curl"), NextPageToken: "p2"}})
	p = m.(*ListPage)

	p, _ = submitFilter(t, p, "nginx")
	p = drainSearch(t, p)

	m, cmd := p.Update(keyPress(']'))
	p = m.(*ListPage)
	if cmd != nil || p.pageNum != 1 {
		t.Fatalf("] should do nothing during a search: cmd=%v pageNum=%d", cmd != nil, p.pageNum)
	}
	if p.hasMorePages() {
		t.Fatal("search results should not claim to have more pages")
	}

	p, cmd = submitFilter(t, p, "")
	if cmd == nil || !p.loading {
		t.Fatal("clearing the search should reload the first page")
	}
	if p.searchActive || p.searchRows != nil {
		t.Fatalf("search state left behind: active=%v rows=%v", p.searchActive, p.searchRows)
	}
}

// esc out of the prompt undoes the preview that typing applied, leaving the
// rows the page was already showing.
func TestFilterEscRestoresCommittedSearch(t *testing.T) {
	t.Parallel()
	p := testListPage(nil)
	m, _ := p.Update(LoadedMsg{PageResult: PageResult{Rows: rows("nginx", "curl", "nginx-fips")}})
	p = m.(*ListPage)

	p, _ = submitFilter(t, p, "nginx")
	if len(p.displayedRows) != 2 {
		t.Fatalf("rows=%+v", p.displayedRows)
	}

	m, _ = p.Update(keyPress('/'))
	p = m.(*ListPage)
	p.filterIn.SetValue("nginx-f")
	m, _ = p.Update(keyPress('x')) // typing previews over the rows on screen
	p = m.(*ListPage)
	m, _ = p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	p = m.(*ListPage)

	if p.filter != "nginx" || len(p.displayedRows) != 2 {
		t.Fatalf("filter=%q rows=%+v", p.filter, p.displayedRows)
	}
}

// A search is re-run rather than re-filtered when the rows it searched change,
// since the new ones were never looked at.
func TestSearchRerunsOnRefreshAndToggle(t *testing.T) {
	t.Parallel()
	flag := false
	var probe searchProbe
	p := testListPage(func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		probe.record(pageSize, query, orderBy)
		if token == "" {
			return PageResult{Rows: rows("nginx"), NextPageToken: "p2"}, nil
		}
		return PageResult{Rows: rows("nginx-fips")}, nil
	}).WithBoolToggle("m", "remediated", &flag)
	m, _ := p.Update(LoadedMsg{PageResult: PageResult{Rows: rows("nginx"), NextPageToken: "p2"}})
	p = m.(*ListPage)

	p, _ = submitFilter(t, p, "nginx")
	p = drainSearch(t, p)

	m, cmd := p.Update(keyPress('r'))
	p = m.(*ListPage)
	if cmd == nil || !p.searching {
		t.Fatal("r should re-run the search, not reload one page")
	}
	p = drainSearch(t, p)

	m, cmd = p.Update(keyPress('m'))
	p = m.(*ListPage)
	if cmd == nil || !p.searching || !flag {
		t.Fatalf("toggle should re-run the search: cmd=%v searching=%v flag=%v", cmd != nil, p.searching, flag)
	}
	p = drainSearch(t, p)
	if len(p.displayedRows) != 2 {
		t.Fatalf("rows=%+v", p.displayedRows)
	}
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

// A search that finds a handful of rows must show them, not leave the cursor
// stranded past the end of the new list and render an empty table.
func TestSearchKeepsCursorInRange(t *testing.T) {
	t.Parallel()
	p := testListPage(nil)
	names := make([]string, 0, 40)
	for i := 0; i < 39; i++ {
		names = append(names, "curl")
	}
	names = append(names, "nginx")
	m, _ := p.Update(LoadedMsg{PageResult: PageResult{Rows: rows(names...)}})
	p = m.(*ListPage)
	p.table.SetCursor(39)

	p, _ = submitFilter(t, p, "nginx")
	if len(p.displayedRows) != 1 {
		t.Fatalf("rows=%d", len(p.displayedRows))
	}
	if p.table.Cursor() != 0 {
		t.Fatalf("cursor=%d, want it clamped to the only match", p.table.Cursor())
	}
	if row, ok := p.selectedRow(); !ok || row.Columns[0] != "nginx" {
		t.Fatalf("selected=%+v ok=%v", row, ok)
	}
}

// Lists load newest-first: the default order goes to the API until the user
// sorts a column themselves, and a column the API cannot sort leaves it in place
// so the rows still arrive newest-first underneath the local sort.
func TestDefaultOrderUntilTheUserSorts(t *testing.T) {
	t.Parallel()
	p := testListPage(nil).
		WithServerSort(map[int]string{0: "name"}).
		WithDefaultOrder("created_at desc")

	if got := p.orderByArg(); got != "created_at desc" {
		t.Fatalf("unsorted order=%q", got)
	}
	p.sortCol, p.sortAsc = 0, true
	if got := p.orderByArg(); got != "name asc" {
		t.Fatalf("sorted order=%q", got)
	}
	p.sortCol = 2 // no order_by field for this column
	if got := p.orderByArg(); got != "created_at desc" {
		t.Fatalf("locally sorted order=%q, want the default still sent", got)
	}
}

// A relative time reads as "3d ago" and sorts as nothing useful, so time columns
// carry the instant they stand for as their sort key.
func TestSortUsesTimeSortKeys(t *testing.T) {
	t.Parallel()
	p := testListPage(nil)
	now := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	rowAt := func(name string, t time.Time, shown string) RowData {
		return RowData{
			UID:      name,
			Columns:  []string{name, "id-" + name, shown},
			SortKeys: map[int]string{2: timeKey(t)},
		}
	}
	p.allRows = []RowData{
		rowAt("old", now.AddDate(0, 0, -3), "3d ago"),
		rowAt("newest", now.Add(-10*time.Hour), "10h ago"),
		rowAt("never", time.Time{}, "-"),
		rowAt("older", now.AddDate(0, -2, 0), "2026-07-18"),
	}
	// Newest first, with the row that has no time at all last.
	p.sortCol, p.sortAsc = 2, false
	p.applyFilter()
	var got []string
	for _, rd := range p.displayedRows {
		got = append(got, rd.Columns[0])
	}
	want := []string{"newest", "old", "older", "never"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v, want %v", got, want)
	}
	// Without a sort key the display text would decide, putting "3d ago" first.
	if sortValue(p.allRows[0], 0) != "old" {
		t.Fatal("a column without a key should sort on its text")
	}
}

// The footer distinguishes an ordering the API applied to the whole resource
// from one applied to the rows on screen.
func TestOrderLabelMarksServerOrdering(t *testing.T) {
	t.Parallel()
	server := testListPage(nil).WithDefaultOrder("created_at desc")
	if got := server.orderLabel(); got != "sorted by created_at ▼ (server)" {
		t.Errorf("default order label=%q", got)
	}
	server.sortCol, server.sortAsc = 0, true
	server.serverSortFields = map[int]string{0: "name"}
	if got := server.orderLabel(); got != "sorted by NAME ▲ (server)" {
		t.Errorf("server sort label=%q", got)
	}
	local := testListPage(nil).WithDefaultSort(2, false)
	if got := local.orderLabel(); got != "sorted by CREATED ▼" {
		t.Errorf("local sort label=%q", got)
	}
	if got := testListPage(nil).orderLabel(); got != "" {
		t.Errorf("unordered label=%q", got)
	}
}

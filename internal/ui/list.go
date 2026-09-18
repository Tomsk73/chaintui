package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Tomsk73/chaintui/internal/api"
)

// RowData is the display+identity data for a single table row.
type RowData struct {
	UID     string
	Columns []string // values matching ListPage.cols
	// SortKeys gives the value a column sorts on, by column index, for columns
	// whose display text does not sort the way it reads: a relative time like
	// "3d ago" is the case that matters, since it sorts nothing like the instant
	// it stands for. Columns without an entry sort on their text. See timeKey.
	SortKeys map[int]string
	Raw      any // original typed value, marshalled for detail view
}

// timeKey is the sort key for a column showing a time. Zero times sort before
// every real one, which puts "never" last in the newest-first order that lists
// default to.
func timeKey(t time.Time) string {
	return t.UTC().Format(time.RFC3339Nano)
}

// PageResult is one page of rows from a loadFn (API or local).
type PageResult struct {
	Rows          []RowData
	NextPageToken string
	TotalCount    int64 // 0 if unknown
	// Status is an optional summary for the footer, e.g. a scan's severity
	// counts and scanner version.
	Status string
}

// LoadedMsg carries a freshly fetched page to the list page.
type LoadedMsg struct {
	PageResult
	// RequestToken is the page token that was used for this fetch ("" = first page).
	RequestToken string
}

// ListPage is the generic resource list view used for every resource type.
type ListPage struct {
	resource string // "groups", "identities", etc.
	groupCtx string // scoping UIDP
	label    string // breadcrumb display name for this page

	cols    []table.Column
	allRows []RowData // current API page (unfiltered)
	table   table.Model

	loading bool
	spinner spinner.Model
	err     error

	filterMode bool
	filterIn   textinput.Model
	filter     string
	// filterPrev is the term the rows on screen were produced with, kept so esc
	// out of the prompt undoes the preview that typing applied.
	filterPrev string

	// serverFilter, when true, reloads from the API with Query=filter instead of
	// searching every page. Only for resources whose List RPC does a real
	// free-text search (advisories, Libraries artifacts) — see WithServerFilter.
	serverFilter bool
	// serverFilterHint is shown next to the filter prompt (e.g. "server query").
	serverFilterHint string

	// Submitting the filter on a page without serverFilter searches the whole
	// resource: every API page is fetched and matched, and the matches are then
	// shown as one list. searchRows holds them, and the counters describe how
	// much was searched to find them.
	searching       bool // a search is running
	searchActive    bool // searchRows is what the table is showing
	searchRows      []RowData
	searchScanned   int
	searchTruncated bool
	searchRun       *searchRun
	searchSeq       int

	// serverSortFields maps column index → API order_by field name.
	// When the user sorts a mapped column, we re-fetch with OrderBy and skip
	// local sorting for that column.
	serverSortFields map[int]string

	// defaultOrder is the API order_by a list is loaded with until the user
	// sorts a column themselves — newest first, wherever the API can do it.
	defaultOrder string

	// Optional boolean toggle (e.g. remediated filter). Flipping reloads page 1.
	boolToggleKey   string
	boolToggleLabel string
	boolToggle      *bool

	// Optional secondary actions on the selected row, alongside Enter, in
	// binding order.
	rowActions []rowAction

	saveMode bool
	saveIn   textinput.Model
	// saveMsg is the shared status line for anything written to disk (save, export).
	saveMsg string
	saveFn  func(filename string, rows []RowData) error

	// Background export of the full result set (every page), written to a file
	// by exportFn. See WithExport.
	exportKey    string
	exportLabel  string
	exportFn     func(ctx context.Context, progress func(done, total int)) (filename string, err error)
	exporting    bool
	exportCancel context.CancelFunc
	exportEvents *exportRun
	exportDone   int
	exportTotal  int

	sortMode bool
	sortCol  int // -1 = unsorted
	sortAsc  bool

	pageSize int

	// status is the optional loader-supplied summary shown in the footer.
	status string

	// API cursor pagination state.
	pageToken     string   // token used for the current page ("" = first)
	nextPageToken string   // token for the next page (empty = last)
	prevTokens    []string // stack of tokens for prior pages
	pageNum       int      // 1-based display page number
	totalCount    int64

	displayedRows []RowData // rows after local filter+sort (for selectedRow / save)
	filteredRows  []RowData // alias of displayedRows for save compatibility

	width  int
	height int

	loadFn  func(pageToken string, pageSize int, query, orderBy string) (PageResult, error)
	enterFn func(RowData) tea.Cmd // emits a Cmd on Enter (nil = no action)
}

func newListPage(
	resource, groupCtx string,
	cols []table.Column,
	loadFn func(pageToken string, pageSize int, query, orderBy string) (PageResult, error),
	enterFn func(RowData) tea.Cmd,
) *ListPage {
	fi := textinput.New()
	fi.Placeholder = "search..."
	fi.CharLimit = 60

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(yellow)

	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
	)
	s := table.DefaultStyles()
	s.Header = tableHeaderStyle
	s.Cell = tableCellStyle
	s.Selected = selectedRowStyle
	t.SetStyles(s)

	return &ListPage{
		resource: resource,
		groupCtx: groupCtx,
		cols:     cols,
		table:    t,
		spinner:  sp,
		loading:  true,
		filterIn: fi,
		loadFn:   loadFn,
		enterFn:  enterFn,
		sortCol:  -1,
		sortAsc:  true,
		pageSize: 50,
		pageNum:  1,
	}
}

func (p *ListPage) ResourceType() string { return p.resource }
func (p *ListPage) GroupContext() string { return p.groupCtx }
func (p *ListPage) Label() string {
	if p.label != "" {
		return p.label
	}
	return p.resource
}

func (p *ListPage) WithLabel(label string) *ListPage {
	p.label = label
	return p
}

func (p *ListPage) WithPageSize(n int) *ListPage {
	if n > 0 {
		p.pageSize = n
	}
	return p
}

// WithServerFilter hands the filter to the API as its free-text Query instead of
// searching the pages here. Only for resources whose List RPC searches properly
// — advisories and Libraries artifacts, whose catalogues are far too large to
// walk — since the query then decides what every page contains.
//
// The List RPCs that take a Name instead do not qualify: that field is an exact
// match, so a partial name finds nothing. Those pages use the default search.
func (p *ListPage) WithServerFilter() *ListPage {
	p.serverFilter = true
	p.serverFilterHint = "server query"
	p.filterIn.Placeholder = "query..."
	return p
}

// WithServerSort maps table column indices to API order_by field names
// (e.g. 0 → "name"). Sorting a mapped column reloads from the API.
//
// Only fields the RPC actually accepts: it rejects anything else with
// InvalidArgument, which fails the whole load rather than falling back to an
// unsorted list. The accepted set differs between APIs — the IAM v2beta1 lists
// take created_at and updated_at, the registry's repos take create_time and
// update_time, and its tags take no time field at all.
func (p *ListPage) WithServerSort(fields map[int]string) *ListPage {
	p.serverSortFields = fields
	return p
}

// WithDefaultOrder sets the API order_by a list loads with before the user
// sorts anything, which is newest first for every list whose RPC can order by
// time. The field has to be one that RPC accepts — see WithServerSort.
//
// This is the server ordering the whole result set, so it holds across pages
// and across a search, unlike the local sort WithDefaultSort asks for.
func (p *ListPage) WithDefaultOrder(order string) *ListPage {
	p.defaultOrder = order
	return p
}

// WithDefaultSort sorts by one column from the start. For pages whose API has
// no order_by field for the column — registry tags, and the lists merged and
// paged here rather than by the API — so the ordering is of the rows on screen,
// which the footer says by leaving off "(server)".
//
// Pass a column with a SortKeys entry for a time column: its display text is
// relative ("3d ago") and does not sort chronologically.
func (p *ListPage) WithDefaultSort(col int, asc bool) *ListPage {
	p.sortCol = col
	p.sortAsc = asc
	return p
}

// WithBoolToggle binds a key that flips *flag and reloads from the first page.
// The flag is read by the caller's loadFn closure. Label is shown in the footer.
func (p *ListPage) WithBoolToggle(key, label string, flag *bool) *ListPage {
	p.boolToggleKey = key
	p.boolToggleLabel = label
	p.boolToggle = flag
	return p
}

// exportEvent is one update from a running export: either progress (done/total
// packages) or, when finished is set, the outcome.
type exportEvent struct {
	done, total int
	finished    bool
	filename    string
	err         error
}

// WithExport binds a key that runs a long-running export of the entire result
// set in the background — not just the current page. fn writes the file itself
// and returns its name; it reports progress through the callback it is given.
// Pressing the key while an export is running cancels it.
func (p *ListPage) WithExport(key, label string, fn func(ctx context.Context, progress func(done, total int)) (string, error)) *ListPage {
	p.exportKey = key
	p.exportLabel = label
	p.exportFn = fn
	return p
}

// exportRun carries events from a running export back to the update loop.
// Progress is droppable; done is buffered so the worker never blocks on it even
// if the user has navigated away from this page.
type exportRun struct {
	progress chan exportEvent
	done     chan exportEvent
}

// startExport runs exportFn in a goroutine and streams its events into the
// update loop.
func (p *ListPage) startExport() tea.Cmd {
	run := &exportRun{
		progress: make(chan exportEvent, 32),
		done:     make(chan exportEvent, 1),
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.exporting = true
	p.exportCancel = cancel
	p.exportEvents = run
	p.exportDone, p.exportTotal = 0, 0
	p.saveMsg = ""
	fn := p.exportFn
	go func() {
		defer cancel()
		name, err := fn(ctx, func(done, total int) {
			select {
			case run.progress <- exportEvent{done: done, total: total}:
			default: // UI is behind; skip this tick rather than stall the export
			}
		})
		run.done <- exportEvent{finished: true, filename: name, err: err}
	}()
	return tea.Batch(p.spinner.Tick, waitForExport(run))
}

func waitForExport(run *exportRun) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev := <-run.done:
			return ev
		case ev := <-run.progress:
			return ev
		}
	}
}

// cancelExport asks a running export to stop; the worker still reports a final
// event, which flips the page out of its exporting state.
func (p *ListPage) cancelExport() {
	if p.exportCancel != nil {
		p.exportCancel()
	}
}

const (
	// searchPageSize is the page size a search asks for. The largest page the API
	// allows is the fewest round trips for the same rows.
	searchPageSize = int(api.MaxPageSize)
	// searchMaxRows and searchMaxPages bound a search, so a resource with far
	// more rows than anyone wants to read cannot keep it fetching. Hitting
	// either marks the result partial rather than passing it off as complete.
	searchMaxRows  = 20000
	searchMaxPages = 200
)

// searchRun carries a running search's events back to the update loop, the same
// way exportRun does. Progress is droppable; done is buffered and always sent,
// including for a cancelled search, so the waiting Cmd never blocks forever.
type searchRun struct {
	seq      int
	progress chan int
	done     chan searchDoneMsg
	cancel   chan struct{}
}

// searchProgressMsg reports how many rows a running search has looked at.
type searchProgressMsg struct {
	seq     int
	scanned int
}

// searchDoneMsg is the outcome of a search: every matching row across every
// page, or the error that stopped it.
type searchDoneMsg struct {
	seq       int
	rows      []RowData
	scanned   int
	truncated bool
	cancelled bool
	status    string
	err       error
}

// startSearch searches the whole resource rather than the page on screen: it
// walks every API page and keeps the rows matching the filter. Matches are shown
// as a single list, so `[` and `]` have nothing to page through while one is up.
//
// The query is left empty on purpose. The List RPCs that accept one match a name
// exactly, which is the opposite of what a typed search wants; the sort order is
// still passed so a server-sorted column comes back in order.
func (p *ListPage) startSearch() tea.Cmd {
	p.cancelSearch()
	p.searchSeq++
	run := &searchRun{
		seq:      p.searchSeq,
		progress: make(chan int, 32),
		done:     make(chan searchDoneMsg, 1),
		cancel:   make(chan struct{}),
	}
	p.searchRun = run
	p.searching = true
	p.searchActive = false
	p.searchRows = nil
	p.searchScanned = 0
	p.searchTruncated = false
	p.loading = true
	p.err = nil
	p.resetPagination()

	fn := p.loadFn
	orderBy := p.orderByArg()
	filter := strings.ToLower(p.filter)
	go func() {
		var (
			out       []RowData
			scanned   int
			truncated bool
			token     string
			status    string
		)
		for pages := 0; ; pages++ {
			select {
			case <-run.cancel:
				run.done <- searchDoneMsg{seq: run.seq, cancelled: true}
				return
			default:
			}
			res, err := fn(token, searchPageSize, "", orderBy)
			if err != nil {
				run.done <- searchDoneMsg{seq: run.seq, err: err}
				return
			}
			if pages == 0 {
				status = res.Status
			}
			for _, rd := range res.Rows {
				if rowMatches(rd, filter) {
					out = append(out, rd)
				}
			}
			scanned += len(res.Rows)
			select {
			case run.progress <- scanned:
			default: // UI is behind; the next page's count supersedes this one
			}
			token = res.NextPageToken
			if token == "" || len(res.Rows) == 0 {
				break
			}
			if scanned >= searchMaxRows || pages+1 >= searchMaxPages {
				truncated = true
				break
			}
		}
		run.done <- searchDoneMsg{
			seq:       run.seq,
			rows:      out,
			scanned:   scanned,
			truncated: truncated,
			status:    status,
		}
	}()
	return tea.Batch(p.spinner.Tick, waitForSearch(run))
}

func waitForSearch(run *searchRun) tea.Cmd {
	return func() tea.Msg {
		select {
		case msg := <-run.done:
			return msg
		case scanned := <-run.progress:
			return searchProgressMsg{seq: run.seq, scanned: scanned}
		}
	}
}

// cancelSearch stops a running search at its next page boundary. It is called
// whenever the results would be replaced anyway — a new search, or a return to
// server paging — so the old one does not keep fetching pages nobody will see.
func (p *ListPage) cancelSearch() {
	if p.searchRun != nil {
		close(p.searchRun.cancel)
		p.searchRun = nil
	}
	p.searching = false
}

// clearSearch returns the page to plain server paging.
func (p *ListPage) clearSearch() {
	p.cancelSearch()
	p.searchActive = false
	p.searchRows = nil
	p.searchScanned = 0
	p.searchTruncated = false
}

// wholeResultLoaded reports whether the rows in hand are already the entire
// result set: the first page, with nothing before or after it. Loaders that
// fetch everything in one call (a scan report, an SBOM) are always in this
// state, so their searches cost no further requests.
func (p *ListPage) wholeResultLoaded() bool {
	return p.pageToken == "" && p.nextPageToken == "" && len(p.prevTokens) == 0
}

// searchLoaded matches the filter against the rows already in hand, when those
// are the whole result set. Used instead of startSearch where re-fetching would
// only hand back the same rows.
func (p *ListPage) searchLoaded() {
	p.cancelSearch()
	f := strings.ToLower(p.filter)
	matches := make([]RowData, 0, len(p.allRows))
	for _, rd := range p.allRows {
		if rowMatches(rd, f) {
			matches = append(matches, rd)
		}
	}
	p.searchActive = true
	p.searchRows = matches
	p.searchScanned = len(p.allRows)
	p.searchTruncated = false
	p.applyFilter()
}

// rowAction is a key bound to something done with the selected row.
type rowAction struct {
	key string
	fn  func(RowData) tea.Cmd
}

// WithRowAction binds another key that acts on the selected row, for pages where
// Enter already means something else (e.g. v for CVEs on repos, where Enter
// drills into tags). It may be called more than once to bind several keys.
func (p *ListPage) WithRowAction(key string, fn func(RowData) tea.Cmd) *ListPage {
	p.rowActions = append(p.rowActions, rowAction{key: key, fn: fn})
	return p
}

func (p *ListPage) WithSave(fn func(filename string, rows []RowData) error) *ListPage {
	si := textinput.New()
	si.Placeholder = "filename..."
	si.CharLimit = 120
	p.saveIn = si
	p.saveFn = fn
	return p
}

func (p *ListPage) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.table.SetWidth(w)
	p.table.SetHeight(h - 1) // leave a line for filter/error
	// Grow the last column so the row grid is exactly w cells wide: any less and
	// the selected row's highlight stops short of the window edge, any more and
	// it wraps onto the next line. Each cell occupies its column width plus the
	// cell style's padding.
	if len(p.cols) > 0 {
		pad := tableCellStyle.GetHorizontalFrameSize()
		used := 0
		for _, c := range p.cols[:len(p.cols)-1] {
			used += c.Width + pad
		}
		last := w - used - pad
		if last > 10 {
			cols := make([]table.Column, len(p.cols))
			copy(cols, p.cols)
			cols[len(cols)-1].Width = last
			p.table.SetColumns(cols)
		}
	}
}

// InputActive reports whether one of this page's own prompts is open, in which
// case it, not the App, should receive plain keystrokes.
func (p *ListPage) InputActive() bool {
	return p.filterMode || p.saveMode || p.sortMode
}

func (p *ListPage) Init() tea.Cmd {
	return tea.Batch(p.spinner.Tick, p.doLoad(""))
}

func (p *ListPage) resetPagination() {
	p.pageToken = ""
	p.nextPageToken = ""
	p.prevTokens = nil
	p.pageNum = 1
	p.totalCount = 0
}

// orderByArg is the order_by sent with every load: the column the user sorted,
// or the list's default order. A column the API cannot sort is sorted here
// instead, and the rows still arrive in the default order underneath.
func (p *ListPage) orderByArg() string {
	if p.sortCol < 0 || p.serverSortFields == nil {
		return p.defaultOrder
	}
	field, ok := p.serverSortFields[p.sortCol]
	if !ok || field == "" {
		return p.defaultOrder
	}
	dir := "asc"
	if !p.sortAsc {
		dir = "desc"
	}
	return field + " " + dir
}

func (p *ListPage) usesServerSort() bool {
	if p.sortCol < 0 || p.serverSortFields == nil {
		return false
	}
	_, ok := p.serverSortFields[p.sortCol]
	return ok
}

func (p *ListPage) doLoad(token string) tea.Cmd {
	fn := p.loadFn
	pageSize := p.pageSize
	query := ""
	if p.serverFilter {
		query = p.filter
	}
	orderBy := p.orderByArg()
	return func() tea.Msg {
		res, err := fn(token, pageSize, query, orderBy)
		if err != nil {
			return errMsg{err}
		}
		return LoadedMsg{PageResult: res, RequestToken: token}
	}
}

func (p *ListPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadedMsg:
		// A page fetched before a search started is no longer what the page is
		// showing, so it is dropped rather than allowed to replace the results.
		if p.searching {
			return p, nil
		}
		p.loading = false
		p.pageToken = msg.RequestToken
		p.nextPageToken = msg.NextPageToken
		p.totalCount = msg.TotalCount
		p.status = msg.Status
		p.allRows = msg.Rows
		p.applyFilter()
		return p, nil

	case searchProgressMsg:
		if p.searchRun == nil || msg.seq != p.searchRun.seq {
			return p, nil
		}
		p.searchScanned = msg.scanned
		return p, waitForSearch(p.searchRun)

	case searchDoneMsg:
		if p.searchRun == nil || msg.seq != p.searchRun.seq {
			return p, nil
		}
		p.searchRun = nil
		p.searching = false
		p.loading = false
		if msg.cancelled {
			return p, nil
		}
		if msg.err != nil {
			p.err = msg.err
			return p, nil
		}
		p.status = msg.status
		p.searchActive = true
		p.searchRows = msg.rows
		p.searchScanned = msg.scanned
		p.searchTruncated = msg.truncated
		p.applyFilter()
		return p, nil

	case errMsg:
		p.loading = false
		p.err = msg.err
		return p, nil

	case actionDoneMsg:
		if msg.err != nil {
			p.saveMsg = errStyle.Render(msg.failed() + ": " + msg.err.Error())
			return p, nil
		}
		p.saveMsg = dimStyle.Render(msg.done + " " + msg.what)
		p.err = nil
		// Re-run whatever the page is showing so the row goes away: the search,
		// or the API page being viewed rather than the first one.
		if p.searchActive || p.searching {
			return p, p.startSearch()
		}
		p.loading = true
		return p, tea.Batch(p.spinner.Tick, p.doLoad(p.pageToken))

	case exportEvent:
		// An export whose page was popped mid-run still finishes and writes its
		// file; its trailing events land on whatever page is now on top, so
		// ignore anything that is not ours.
		if !p.exporting {
			return p, nil
		}
		if !msg.finished {
			p.exportDone, p.exportTotal = msg.done, msg.total
			if p.exportEvents == nil {
				return p, nil
			}
			return p, waitForExport(p.exportEvents)
		}
		p.exporting = false
		p.exportEvents = nil
		p.exportCancel = nil
		switch {
		case errors.Is(msg.err, context.Canceled):
			p.saveMsg = dimStyle.Render("export cancelled")
		case msg.err != nil:
			p.saveMsg = errStyle.Render("export failed: " + msg.err.Error())
		default:
			p.saveMsg = dimStyle.Render("exported to " + msg.filename)
		}
		return p, nil

	case spinner.TickMsg:
		if p.loading || p.exporting || p.searching {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return p, cmd
		}
		return p, nil

	case tea.KeyMsg:
		if p.filterMode {
			return p.updateFilter(msg)
		}
		if p.saveMode {
			return p.updateSave(msg)
		}
		if p.sortMode {
			return p.updateSort(msg)
		}
		// A running export owns the current result set: allow only cursor
		// movement and the export key (which cancels).
		if p.exporting && !isTableNavKey(msg) && msg.String() != p.exportKey {
			return p, nil
		}
		switch msg.String() {
		case "r":
			p.err = nil
			// Refreshing a search means searching again, including one still
			// running: startSearch drops the old walk and begins another.
			if p.searchActive || p.searching {
				return p, p.startSearch()
			}
			p.loading = true
			p.resetPagination()
			return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
		case "/":
			p.filterMode = true
			p.filterPrev = p.filter
			p.filterIn.SetValue(p.filter)
			p.filterIn.Focus()
			return p, textinput.Blink
		case "s":
			if p.saveFn != nil {
				p.saveMode = true
				p.saveMsg = ""
				p.saveIn.SetValue("")
				p.saveIn.Focus()
				return p, textinput.Blink
			}
		case "o":
			if !p.loading {
				p.sortMode = true
			}
			return p, nil
		case "[":
			// A search's matches are one list, not a run of API pages.
			if p.loading || p.searchActive || len(p.prevTokens) == 0 {
				return p, nil
			}
			prev := p.prevTokens[len(p.prevTokens)-1]
			p.prevTokens = p.prevTokens[:len(p.prevTokens)-1]
			p.pageNum--
			if p.pageNum < 1 {
				p.pageNum = 1
			}
			p.loading = true
			p.err = nil
			return p, tea.Batch(p.spinner.Tick, p.doLoad(prev))
		case "]":
			if p.loading || p.searchActive || p.nextPageToken == "" {
				return p, nil
			}
			p.prevTokens = append(p.prevTokens, p.pageToken)
			p.pageNum++
			token := p.nextPageToken
			p.loading = true
			p.err = nil
			return p, tea.Batch(p.spinner.Tick, p.doLoad(token))
		case "d":
			if row, ok := p.selectedRow(); ok {
				return p, func() tea.Msg { return PushMsg{P: newDetailPage(p.resource, row)} }
			}
		case "enter":
			if p.enterFn != nil {
				if row, ok := p.selectedRow(); ok {
					return p, p.enterFn(row)
				}
			}
		default:
			for _, action := range p.rowActions {
				if action.key == "" || msg.String() != action.key {
					continue
				}
				if row, ok := p.selectedRow(); ok {
					return p, action.fn(row)
				}
				return p, nil
			}
			if p.exportFn != nil && p.exportKey != "" && msg.String() == p.exportKey {
				if p.exporting {
					p.cancelExport()
					return p, nil
				}
				if p.loading {
					return p, nil
				}
				return p, p.startExport()
			}
			if p.boolToggle != nil && p.boolToggleKey != "" && msg.String() == p.boolToggleKey && !p.loading {
				*p.boolToggle = !*p.boolToggle
				p.err = nil
				// The toggle changes which rows exist, so a search has to be run
				// again over the new set rather than re-filtered.
				if p.searchActive {
					return p, p.startSearch()
				}
				p.loading = true
				p.resetPagination()
				return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
			}
		}
	}

	var cmd tea.Cmd
	p.table, cmd = p.table.Update(msg)
	return p, cmd
}

// updateFilter drives the "/" prompt. Typing narrows the rows on screen as an
// immediate preview; submitting searches the whole resource, so what the table
// ends up showing is never limited to the page the user happened to be on.
func (p *ListPage) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.filterMode = false
		p.filterIn.Blur()
		// Typing was only a preview, so abandoning it puts back the term the rows
		// on screen were actually produced with.
		p.filter = p.filterPrev
		p.applyFilter()
		return p, nil
	case "enter":
		p.filterMode = false
		p.filterIn.Blur()
		p.filter = strings.TrimSpace(p.filterIn.Value())
		p.err = nil
		switch {
		case p.filter == "":
			// Back to plain server paging from the first page.
			p.clearSearch()
			p.loading = true
			p.resetPagination()
			return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
		case p.serverFilter:
			// The API does the searching; every page it returns is already
			// narrowed to the query.
			p.clearSearch()
			p.loading = true
			p.resetPagination()
			return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
		case p.wholeResultLoaded():
			p.searchLoaded()
			return p, nil
		default:
			return p, p.startSearch()
		}
	}
	var cmd tea.Cmd
	p.filterIn, cmd = p.filterIn.Update(msg)
	if !p.serverFilter {
		p.filter = p.filterIn.Value()
		p.applyFilter()
	}
	return p, cmd
}

func (p *ListPage) updateSave(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.saveMode = false
		p.saveIn.Blur()
		return p, nil
	case "enter":
		p.saveMode = false
		p.saveIn.Blur()
		name := strings.TrimSpace(p.saveIn.Value())
		if name == "" {
			return p, nil
		}
		if err := p.saveFn(name, p.filteredRows); err != nil {
			p.saveMsg = errStyle.Render("save failed: " + err.Error())
		} else {
			p.saveMsg = dimStyle.Render("saved to " + name)
		}
		return p, nil
	}
	var cmd tea.Cmd
	p.saveIn, cmd = p.saveIn.Update(msg)
	return p, cmd
}

func (p *ListPage) updateSort(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "o":
		p.sortMode = false
		return p, nil
	}
	// Number keys 1–9 pick a column.
	if len(msg.String()) == 1 && msg.String() >= "1" && msg.String() <= "9" {
		col := int(msg.String()[0] - '1')
		if col < len(p.cols) {
			if p.sortCol == col {
				p.sortAsc = !p.sortAsc
			} else {
				p.sortCol = col
				p.sortAsc = true
			}
			p.sortMode = false
			if p.usesServerSort() {
				p.err = nil
				// The API decides the order for a mapped column, so a search has
				// to walk the pages again to get its matches in the new one.
				if p.searchActive {
					return p, p.startSearch()
				}
				p.loading = true
				p.resetPagination()
				return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
			}
			p.applyFilter()
		}
	}
	return p, nil
}

// isTableNavKey reports whether a key only moves the table cursor, matching the
// bubbles table default keymap.
func isTableNavKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "up", "down", "j", "k", "pgup", "pgdown", "b", "f", " ", "home", "end", "g", "G":
		return true
	}
	return false
}

// applyFilter rebuilds the table from whichever rows the page holds: a search's
// matches from every API page, or the one page the API last returned.
func (p *ListPage) applyFilter() {
	source := p.allRows
	if p.searchActive {
		source = p.searchRows
	}
	filtered := p.sortRows(p.localFilter(source))
	p.filteredRows = filtered
	p.displayedRows = filtered

	rows := make([]table.Row, len(p.displayedRows))
	for i, rd := range p.displayedRows {
		rows[i] = rd.Columns
	}
	p.table.SetRows(rows)
	// A search or a filter can leave far fewer rows than the cursor's position,
	// and a cursor past the end renders as an empty table rather than as the
	// matches that were found. SetCursor clamps it back into the new list.
	if p.table.Cursor() >= len(rows) {
		p.table.SetCursor(len(rows) - 1)
	}
}

// localFilter narrows rows to those matching the filter term. On a committed
// search this changes nothing — every row in hand already matched — so its real
// job is the live preview while the user types.
//
// Rows from a serverFilter page are left alone: the API applied the query, and
// matching them again here would drop the rows it matched on fields the table
// does not show.
func (p *ListPage) localFilter(in []RowData) []RowData {
	f := strings.ToLower(p.filter)
	if p.serverFilter || f == "" {
		return in
	}
	out := make([]RowData, 0, len(in))
	for _, rd := range in {
		if rowMatches(rd, f) {
			out = append(out, rd)
		}
	}
	return out
}

// sortRows applies the sort the user chose with o, or the page's default one. A
// column mapped to an API order_by field is already in order, whether it came
// back as one page or as the many a search walked, so only unmapped columns are
// sorted here.
func (p *ListPage) sortRows(in []RowData) []RowData {
	out := append([]RowData(nil), in...)
	if p.usesServerSort() || p.sortCol < 0 || p.sortCol >= len(p.cols) {
		return out
	}
	col, asc := p.sortCol, p.sortAsc
	sort.SliceStable(out, func(i, j int) bool {
		if asc {
			return sortValue(out[i], col) < sortValue(out[j], col)
		}
		return sortValue(out[i], col) > sortValue(out[j], col)
	})
	return out
}

// sortValue is what a row sorts on in a column: its SortKeys entry when it has
// one, otherwise the text the column shows.
func sortValue(rd RowData, col int) string {
	if key, ok := rd.SortKeys[col]; ok {
		return key
	}
	if col < len(rd.Columns) {
		return rd.Columns[col]
	}
	return ""
}

// rowMatches reports whether any of a row's columns contains f, which must
// already be lowercased.
func rowMatches(rd RowData, f string) bool {
	for _, col := range rd.Columns {
		if strings.Contains(strings.ToLower(col), f) {
			return true
		}
	}
	return false
}

func (p *ListPage) selectedRow() (RowData, bool) {
	sel := p.table.SelectedRow()
	if sel == nil {
		return RowData{}, false
	}
	for _, rd := range p.displayedRows {
		if len(rd.Columns) > 0 && rd.Columns[0] == sel[0] {
			return rd, true
		}
	}
	return RowData{}, false
}

// exportStatus is the progress text shown while an export runs. A zero total
// means the export is still discovering how much work there is.
func (p *ListPage) exportStatus() string {
	label := p.exportLabel
	if label == "" {
		label = "exporting"
	}
	switch {
	case p.exportTotal > 0:
		pct := p.exportDone * 100 / p.exportTotal
		label += fmt.Sprintf(": %d/%d packages (%d%%)", p.exportDone, p.exportTotal, pct)
	case p.exportDone > 0:
		label += fmt.Sprintf(": listing packages (%d so far)...", p.exportDone)
	default:
		label += ": listing packages..."
	}
	if p.exportKey != "" {
		label += "  │  " + p.exportKey + " to cancel"
	}
	return label
}

// searchStatus is the progress text shown while a search runs, so a walk over
// many API pages says how far it has got rather than looking stuck.
func (p *ListPage) searchStatus() string {
	msg := fmt.Sprintf("Searching all %s for %q", p.resource, p.filter)
	if p.searchScanned > 0 {
		msg += fmt.Sprintf(" — %d searched", p.searchScanned)
	}
	return msg + "..."
}

// searchSummary describes a finished search: how many rows matched, out of how
// many were looked at, and whether the walk stopped before the end.
func (p *ListPage) searchSummary() string {
	msg := fmt.Sprintf("search %q: %d of %d %s",
		p.filter, len(p.searchRows), p.searchScanned, p.resource)
	if p.searchTruncated {
		msg += fmt.Sprintf(" — stopped after %d rows, narrow the search", p.searchScanned)
	}
	return msg
}

// orderLabel says how the rows are ordered: the column the user sorted, or the
// order the list loads with by default. "(server)" marks an ordering the API
// applied to the whole resource; without it, only the rows on screen are sorted.
func (p *ListPage) orderLabel() string {
	arrow := func(asc bool) string {
		if asc {
			return "▲"
		}
		return "▼"
	}
	if p.sortCol >= 0 && p.sortCol < len(p.cols) {
		label := fmt.Sprintf("sorted by %s %s", p.cols[p.sortCol].Title, arrow(p.sortAsc))
		if p.usesServerSort() {
			label += " (server)"
		}
		return label
	}
	if p.defaultOrder == "" {
		return ""
	}
	field, dir, _ := strings.Cut(p.defaultOrder, " ")
	return fmt.Sprintf("sorted by %s %s (server)", field, arrow(dir != "desc"))
}

func (p *ListPage) hasMorePages() bool {
	// A search holds every match at once, so there are no pages to step through.
	if p.searchActive {
		return false
	}
	return p.nextPageToken != "" || len(p.prevTokens) > 0
}

func (p *ListPage) View() string {
	if p.searching {
		return p.spinner.View() + " " + p.searchStatus()
	}
	if p.loading {
		return p.spinner.View() + " Loading " + p.resource + "..."
	}
	if p.err != nil {
		return errStyle.Render("Error: " + p.err.Error())
	}

	tableView := p.table.View()

	var bottom string
	switch {
	case p.exporting:
		bottom = cmdBarStyle.Render(p.spinner.View() + " " + p.exportStatus())
	case p.saveMode:
		bottom = cmdBarStyle.Render("save to: " + p.saveIn.View())
	case p.saveMsg != "":
		bottom = p.saveMsg
	case p.sortMode:
		parts := make([]string, len(p.cols))
		for i, c := range p.cols {
			parts[i] = fmt.Sprintf("%d:%s", i+1, c.Title)
		}
		bottom = cmdBarStyle.Render("sort by: " + strings.Join(parts, "  "))
	case p.filterMode:
		// Say where enter will look, since it is not the page on screen.
		hint := " (enter searches all " + p.resource + ")"
		if p.serverFilter {
			hint = " (" + p.serverFilterHint + ")"
		}
		bottom = cmdBarStyle.Render("/ " + p.filterIn.View() + hint)
	default:
		var parts []string
		if p.status != "" {
			parts = append(parts, p.status)
		}
		if label := p.orderLabel(); label != "" {
			parts = append(parts, label)
		}
		switch {
		case p.searchActive:
			parts = append(parts, p.searchSummary())
		case p.filter != "":
			parts = append(parts, fmt.Sprintf("filter: %q", p.filter))
		}
		if p.boolToggle != nil && p.boolToggleLabel != "" {
			state := "off"
			if *p.boolToggle {
				state = "on"
			}
			parts = append(parts, fmt.Sprintf("%s:%s (%s)", p.boolToggleLabel, state, p.boolToggleKey))
		}
		if p.hasMorePages() || p.pageNum > 1 || p.totalCount > 0 {
			pageInfo := fmt.Sprintf("page %d", p.pageNum)
			if p.totalCount > 0 {
				pageInfo += fmt.Sprintf("/%d", (p.totalCount+int64(p.pageSize)-1)/int64(p.pageSize))
			} else if p.nextPageToken != "" {
				pageInfo += "+"
			}
			pageInfo += "  [ ]"
			parts = append(parts, pageInfo)
		}
		if len(parts) > 0 {
			bottom = dimStyle.Render(strings.Join(parts, "  │  "))
		}
	}

	if bottom != "" {
		return lipgloss.JoinVertical(lipgloss.Left, tableView, bottom)
	}
	return tableView
}

// detailPage shows the raw JSON of a resource.
type detailPage struct {
	resource string
	row      RowData
	content  string
	width    int
	height   int
}

func newDetailPage(resource string, row RowData) *detailPage {
	var content string
	if row.Raw != nil {
		b, err := json.MarshalIndent(row.Raw, "", "  ")
		if err == nil {
			content = string(b)
		}
	}
	if content == "" {
		content = strings.Join(row.Columns, "\n")
	}
	return &detailPage{resource: resource, row: row, content: content}
}

func (d *detailPage) ResourceType() string { return d.resource }
func (d *detailPage) GroupContext() string { return "" }
func (d *detailPage) Label() string        { return d.resource }
func (d *detailPage) SetSize(w, h int)     { d.width = w; d.height = h }
func (d *detailPage) Init() tea.Cmd        { return nil }

func (d *detailPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		// q is the global quit binding, so esc is the only way back from here.
		if msg.String() == "esc" {
			return d, func() tea.Msg { return PopMsg{} }
		}
	}
	return d, nil
}

func (d *detailPage) View() string {
	return lipgloss.NewStyle().
		Foreground(white).
		Padding(1, 2).
		Render(d.content)
}

// rowActionFor returns the action bound to key, or nil when the page has none.
func (p *ListPage) rowActionFor(key string) func(RowData) tea.Cmd {
	for _, action := range p.rowActions {
		if action.key == key {
			return action.fn
		}
	}
	return nil
}

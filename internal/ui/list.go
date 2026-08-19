package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RowData is the display+identity data for a single table row.
type RowData struct {
	UID     string
	Columns []string // values matching ListPage.cols
	Raw     any      // original typed value, marshalled for detail view
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

	// serverFilter, when true, reloads from the API with Query=filter
	// instead of filtering the current page locally.
	serverFilter bool
	// serverFilterHint is shown next to the filter prompt (e.g. "server", "name").
	serverFilterHint string

	// serverSortFields maps column index → API order_by field name.
	// When the user sorts a mapped column, we re-fetch with OrderBy and skip
	// local sorting for that column.
	serverSortFields map[int]string

	// Optional boolean toggle (e.g. remediated filter). Flipping reloads page 1.
	boolToggleKey   string
	boolToggleLabel string
	boolToggle      *bool

	// Optional secondary action on the selected row, alongside Enter.
	rowActionKey string
	rowActionFn  func(RowData) tea.Cmd

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
	fi.Placeholder = "filter..."
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
func (p *ListPage) GroupContext() string  { return p.groupCtx }
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

// WithServerFilter causes "/" filter submits to re-fetch with query instead of
// filtering the current page locally (free-text Query, e.g. advisories).
func (p *ListPage) WithServerFilter() *ListPage {
	p.serverFilter = true
	p.serverFilterHint = "server"
	p.filterIn.Placeholder = "query..."
	return p
}

// WithServerNameFilter is like WithServerFilter but for exact-name List RPCs
// (repos, tags, identities, groups, roles).
func (p *ListPage) WithServerNameFilter() *ListPage {
	p.serverFilter = true
	p.serverFilterHint = "exact name"
	p.filterIn.Placeholder = "exact name..."
	return p
}

// WithServerSort maps table column indices to API order_by field names
// (e.g. 0 → "name"). Sorting a mapped column reloads from the API.
func (p *ListPage) WithServerSort(fields map[int]string) *ListPage {
	p.serverSortFields = fields
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

// WithRowAction binds a second key that acts on the selected row, for pages
// where Enter already means something else (e.g. v for CVEs on repos, where
// Enter drills into tags).
func (p *ListPage) WithRowAction(key string, fn func(RowData) tea.Cmd) *ListPage {
	p.rowActionKey = key
	p.rowActionFn = fn
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

func (p *ListPage) orderByArg() string {
	if p.sortCol < 0 || p.serverSortFields == nil {
		return ""
	}
	field, ok := p.serverSortFields[p.sortCol]
	if !ok || field == "" {
		return ""
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
		p.loading = false
		p.pageToken = msg.RequestToken
		p.nextPageToken = msg.NextPageToken
		p.totalCount = msg.TotalCount
		p.status = msg.Status
		p.allRows = msg.Rows
		p.applyFilter()
		return p, nil

	case errMsg:
		p.loading = false
		p.err = msg.err
		return p, nil

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
		if p.loading || p.exporting {
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
			p.loading = true
			p.err = nil
			p.resetPagination()
			return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
		case "/":
			p.filterMode = true
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
			if p.loading || len(p.prevTokens) == 0 {
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
			if p.loading || p.nextPageToken == "" {
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
			if p.rowActionFn != nil && p.rowActionKey != "" && msg.String() == p.rowActionKey {
				if row, ok := p.selectedRow(); ok {
					return p, p.rowActionFn(row)
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
				p.loading = true
				p.err = nil
				p.resetPagination()
				return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
			}
		}
	}

	var cmd tea.Cmd
	p.table, cmd = p.table.Update(msg)
	return p, cmd
}

func (p *ListPage) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		p.filterMode = false
		p.filterIn.Blur()
		return p, nil
	case "enter":
		p.filterMode = false
		p.filterIn.Blur()
		p.filter = p.filterIn.Value()
		if p.serverFilter {
			p.loading = true
			p.err = nil
			p.resetPagination()
			return p, tea.Batch(p.spinner.Tick, p.doLoad(""))
		}
		p.applyFilter()
		return p, nil
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
				p.loading = true
				p.err = nil
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

func (p *ListPage) applyFilter() {
	f := strings.ToLower(p.filter)
	var filtered []RowData
	if p.serverFilter || f == "" {
		filtered = append([]RowData(nil), p.allRows...)
	} else {
		for _, rd := range p.allRows {
			if p.rowMatches(rd, f) {
				filtered = append(filtered, rd)
			}
		}
	}
	if !p.usesServerSort() && p.sortCol >= 0 && p.sortCol < len(p.cols) {
		col, asc := p.sortCol, p.sortAsc
		sort.SliceStable(filtered, func(i, j int) bool {
			if asc {
				return filtered[i].Columns[col] < filtered[j].Columns[col]
			}
			return filtered[i].Columns[col] > filtered[j].Columns[col]
		})
	}
	p.filteredRows = filtered
	p.displayedRows = filtered

	rows := make([]table.Row, len(p.displayedRows))
	for i, rd := range p.displayedRows {
		rows[i] = rd.Columns
	}
	p.table.SetRows(rows)
}

func (p *ListPage) rowMatches(rd RowData, f string) bool {
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

func (p *ListPage) hasMorePages() bool {
	return p.nextPageToken != "" || len(p.prevTokens) > 0
}

func (p *ListPage) View() string {
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
		hint := ""
		if p.serverFilter {
			if p.serverFilterHint != "" {
				hint = " (" + p.serverFilterHint + ")"
			} else {
				hint = " (server)"
			}
		}
		bottom = cmdBarStyle.Render("/ " + p.filterIn.View() + hint)
	default:
		var parts []string
		if p.status != "" {
			parts = append(parts, p.status)
		}
		if p.sortCol >= 0 {
			dir := "▲"
			if !p.sortAsc {
				dir = "▼"
			}
			label := fmt.Sprintf("sorted by %s %s", p.cols[p.sortCol].Title, dir)
			if p.usesServerSort() {
				label += " (server)"
			}
			parts = append(parts, label)
		}
		if p.filter != "" {
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
func (d *detailPage) GroupContext() string  { return "" }
func (d *detailPage) Label() string         { return d.resource }
func (d *detailPage) SetSize(w, h int)      { d.width = w; d.height = h }
func (d *detailPage) Init() tea.Cmd         { return nil }

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

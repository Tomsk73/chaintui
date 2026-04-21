package ui

import (
	"encoding/json"
	"fmt"
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

// LoadedMsg carries freshly fetched rows to the list page.
type LoadedMsg struct {
	Rows []RowData
}

// ListPage is the generic resource list view used for every resource type.
type ListPage struct {
	resource string // "groups", "identities", etc.
	groupCtx string // scoping UIDP

	cols    []table.Column
	allRows []RowData // unfiltered
	table   table.Model

	loading bool
	spinner spinner.Model
	err     error

	filterMode bool
	filterIn   textinput.Model
	filter     string

	width  int
	height int

	loadFn  func() ([]RowData, error) // fetches rows from API
	enterFn func(RowData) Page         // builds next page on Enter (nil = no drill-down)
}

func newListPage(
	resource, groupCtx string,
	cols []table.Column,
	loadFn func() ([]RowData, error),
	enterFn func(RowData) Page,
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
	}
}

func (p *ListPage) ResourceType() string { return p.resource }
func (p *ListPage) GroupContext() string  { return p.groupCtx }

func (p *ListPage) SetSize(w, h int) {
	p.width = w
	p.height = h
	p.table.SetWidth(w)
	p.table.SetHeight(h - 1) // leave a line for filter/error
	// Proportionally resize last column to fill width.
	if len(p.cols) > 0 {
		used := 0
		for _, c := range p.cols[:len(p.cols)-1] {
			used += c.Width + 1
		}
		last := w - used - 2
		if last > 10 {
			cols := make([]table.Column, len(p.cols))
			copy(cols, p.cols)
			cols[len(cols)-1].Width = last
			p.table.SetColumns(cols)
		}
	}
}

func (p *ListPage) Init() tea.Cmd {
	return tea.Batch(p.spinner.Tick, p.doLoad())
}

func (p *ListPage) doLoad() tea.Cmd {
	fn := p.loadFn
	return func() tea.Msg {
		rows, err := fn()
		if err != nil {
			return errMsg{err}
		}
		return LoadedMsg{rows}
	}
}

func (p *ListPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadedMsg:
		p.loading = false
		p.allRows = msg.Rows
		p.applyFilter()
		return p, nil

	case errMsg:
		p.loading = false
		p.err = msg.err
		return p, nil

	case spinner.TickMsg:
		if p.loading {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			return p, cmd
		}
		return p, nil

	case tea.KeyMsg:
		if p.filterMode {
			return p.updateFilter(msg)
		}
		switch msg.String() {
		case "r":
			p.loading = true
			p.err = nil
			return p, tea.Batch(p.spinner.Tick, p.doLoad())
		case "/":
			p.filterMode = true
			p.filterIn.SetValue("")
			p.filterIn.Focus()
			return p, textinput.Blink
		case "d":
			if row, ok := p.selectedRow(); ok {
				return p, func() tea.Msg { return PushMsg{P: newDetailPage(p.resource, row)} }
			}
		case "enter":
			if p.enterFn != nil {
				if row, ok := p.selectedRow(); ok {
					next := p.enterFn(row)
					if next != nil {
						return p, func() tea.Msg { return PushMsg{P: next} }
					}
				}
			}
		}
	}

	var cmd tea.Cmd
	p.table, cmd = p.table.Update(msg)
	return p, cmd
}

func (p *ListPage) updateFilter(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "enter":
		p.filterMode = false
		p.filterIn.Blur()
		p.filter = p.filterIn.Value()
		p.applyFilter()
		return p, nil
	}
	var cmd tea.Cmd
	p.filterIn, cmd = p.filterIn.Update(msg)
	p.filter = p.filterIn.Value()
	p.applyFilter()
	return p, cmd
}

func (p *ListPage) applyFilter() {
	f := strings.ToLower(p.filter)
	var rows []table.Row
	for _, rd := range p.allRows {
		if f == "" || p.rowMatches(rd, f) {
			rows = append(rows, rd.Columns)
		}
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
	// Match back to allRows by UID (first visible column isn't always UID, so match by columns).
	for _, rd := range p.allRows {
		if len(rd.Columns) > 0 && rd.Columns[0] == sel[0] {
			return rd, true
		}
	}
	return RowData{}, false
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
	if p.filterMode {
		bottom = cmdBarStyle.Render("/ " + p.filterIn.View())
	} else if p.filter != "" {
		bottom = dimStyle.Render(fmt.Sprintf("filter: %q  (/ to change, esc to clear)", p.filter))
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
func (d *detailPage) SetSize(w, h int)     { d.width = w; d.height = h }
func (d *detailPage) Init() tea.Cmd        { return nil }

func (d *detailPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		switch msg.String() {
		case "esc", "q":
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

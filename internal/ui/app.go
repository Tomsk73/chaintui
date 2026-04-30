package ui

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Tomsk73/chaintui/internal/api"
)

var debugLog *log.Logger

func InitDebugLog() error {
	f, err := os.OpenFile("/tmp/chaintui-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	debugLog = log.New(f, "", log.Ltime|log.Lmicroseconds)
	return nil
}

func logMsg(msg tea.Msg) {
	if debugLog != nil {
		debugLog.Printf("%T %s", msg, fmt.Sprintf("%+v", msg))
	}
}

const (
	headerH = 3
	footerH = 2
)

// Page is the interface implemented by every view in the navigation stack.
type Page interface {
	tea.Model
	ResourceType() string
	GroupContext() string
	Label() string
	SetSize(w, h int)
}

// Navigation messages understood by the App.
type (
	PushMsg   struct{ P Page }
	PopMsg    struct{}
	SwitchMsg struct{ Resource, GroupCtx string }
	errMsg    struct{ err error }
)

// App is the root bubbletea model – owns the navigation stack.
type App struct {
	client  *api.Client
	stack   []Page
	width   int
	height  int
	cmdMode bool
	cmd     textinput.Model
	orgCtx  string // active organisation UIDP
	orgName string // display name for the active organisation
}

func New(client *api.Client) App {
	c := textinput.New()
	c.Placeholder = "resource (groups, identities, roles, rb, repos, tags, adv)..."
	c.CharLimit = 40

	root := NewGroupsPage(client, "")
	return App{
		client: client,
		stack:  []Page{root},
		cmd:    c,
	}
}

func (a App) top() Page { return a.stack[len(a.stack)-1] }

func (a App) contentH() int {
	h := a.height - headerH - footerH
	if h < 1 {
		return 1
	}
	return h
}

func (a App) Init() tea.Cmd {
	return a.top().Init()
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	logMsg(msg)
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		for i := range a.stack {
			a.stack[i].SetSize(a.width, a.contentH())
		}
		return a, nil

		//	case SelectOrgMsg:
		//		a.orgCtx = msg.UID
		//		a.orgName = msg.Name
		//		page := NewGroupsPage(a.client, msg.UID)
		//		page.SetSize(a.width, a.contentH())
		//		a.stack = []Page{page}
		//		return a, page.Init()

	case PushMsg:
		msg.P.SetSize(a.width, a.contentH())
		a.stack = append(append([]Page{}, a.stack...), msg.P)
		return a, msg.P.Init()

	case PopMsg:
		if len(a.stack) > 1 {
			a.stack = append([]Page{}, a.stack[:len(a.stack)-1]...)
		}
		return a, nil

	case SwitchMsg:
		page := resolveResourcePage(a.client, msg.Resource, msg.GroupCtx)
		if page == nil {
			return a, nil
		}
		page.SetSize(a.width, a.contentH())
		a.stack = []Page{page}
		return a, page.Init()

	case tea.KeyMsg:
		if a.cmdMode {
			return a.handleCmdKey(msg)
		}
		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			if len(a.stack) == 1 {
				return a, tea.Quit
			}
			a.stack = append([]Page{}, a.stack[:len(a.stack)-1]...)
			return a, nil
		case "esc":
			if len(a.stack) > 1 {
				a.stack = append([]Page{}, a.stack[:len(a.stack)-1]...)
			}
			return a, nil
		case ":":
			a.cmdMode = true
			a.cmd.SetValue("")
			a.cmd.Focus()
			return a, textinput.Blink
			//		case "o":
			//			page := NewOrgSelectorPage(a.client)
			//			page.SetSize(a.width, a.contentH())
			//			a.stack = append(append([]Page{}, a.stack...), page)
			//			return a, page.Init()
		}
	}

	// Delegate to top of stack.
	updated, cmd := a.top().Update(msg)
	newStack := append([]Page{}, a.stack...)
	newStack[len(newStack)-1] = updated.(Page)
	a.stack = newStack
	return a, cmd
}

func (a App) handleCmdKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		a.cmdMode = false
		a.cmd.Blur()
		return a, nil
	case "enter":
		val := strings.TrimSpace(a.cmd.Value())
		a.cmdMode = false
		a.cmd.Blur()
		if val == "" {
			return a, nil
		}
		ctx := a.top().GroupContext()
		if ctx == "" {
			ctx = a.orgCtx
		}
		return a, func() tea.Msg { return SwitchMsg{Resource: val, GroupCtx: ctx} }
	}
	var cmd tea.Cmd
	a.cmd, cmd = a.cmd.Update(msg)
	return a, cmd
}

func (a App) View() string {
	if a.width == 0 {
		return "Initializing..."
	}
	top := a.top()
	header := renderHeader(a.width, top.ResourceType(), a.groupPath(), a.breadcrumb())
	content := lipgloss.NewStyle().Height(a.contentH()).Render(top.View())
	var footer string
	if a.cmdMode {
		footer = renderCmdBar(a.width, a.cmd.View())
	} else {
		footer = renderFooter(a.width, top.ResourceType(), len(a.stack) > 1)
	}
	return strings.Join([]string{header, content, footer}, "\n")
}

func (a App) breadcrumb() string {
	parts := make([]string, len(a.stack))
	for i, p := range a.stack {
		parts[i] = p.Label()
	}
	return strings.Join(parts, " > ")
}

// groupPath builds a human-readable org/group context from the org name and
// any named group pages that have been drilled into on the navigation stack.
func (a App) groupPath() string {
	var parts []string
	if a.orgName != "" {
		parts = append(parts, a.orgName)
	}
	for _, p := range a.stack {
		if p.ResourceType() == "groups" && p.Label() != "groups" {
			parts = append(parts, p.Label())
		}
	}
	if len(parts) == 0 {
		return "no org"
	}
	return strings.Join(parts, " / ")
}

func resolveResourcePage(client *api.Client, resource, groupCtx string) Page {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "g", "group", "groups":
		return NewGroupsPage(client, groupCtx)
	case "id", "identity", "identities":
		return NewIdentitiesPage(client, groupCtx)
	case "r", "role", "roles":
		return NewRolesPage(client, groupCtx)
	case "rb", "rolebinding", "rolebindings":
		return NewRoleBindingsPage(client, groupCtx)
	case "idp", "identityprovider", "identityproviders":
		return NewIDPsPage(client, groupCtx)
	case "inv", "invite", "invites", "groupinvites":
		return NewGroupInvitesPage(client, groupCtx)
	case "repo", "repos", "repository":
		return NewReposPage(client, groupCtx)
		//	case "tag", "tags":
		//		return NewTagsPage(client, groupCtx)
	case "adv", "advisory", "advisories":
		return NewAdvisoriesPage(client, groupCtx)
	}
	return nil
}

func renderHeader(width int, resource, groupPath, breadcrumb string) string {
	left := lipgloss.JoinHorizontal(lipgloss.Left,
		logo(),
		appNameStyle.Render("chaintui"),
		sepStyle.Render("  │  "),
		ctxStyle.Render(groupPath),
		sepStyle.Render("  │  "),
		resTypeStyle.Render(resource),
	)
	right := dimStyle.Render(breadcrumb)

	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}

	line1 := headerStyle.Width(width).Render(
		left + strings.Repeat(" ", pad) + right,
	)
	line2 := dimStyle.Render(strings.Repeat("─", width))
	return line1 + "\n" + line2
}

func renderFooter(width int, resource string, canGoBack bool) string {
	hints := []string{
		//	keyHint("o", "select org"),
		keyHint(":", "cmd"),
		keyHint("/", "filter"),
		keyHint("d", "describe"),
		keyHint("r", "refresh"),
	}
	if resource == "groups" || resource == "repos" || resource == "tags" {
		hints = append(hints, keyHint("↵", "drill down"))
	}
	if resource == "sbom" {
		hints = append(hints, keyHint("s", "save csv"))
	}
	if canGoBack {
		hints = append(hints, keyHint("esc", "back"))
	}
	hints = append(hints, keyHint("q", "quit"))

	line := footerStyle.Width(width).Render(strings.Join(hints, dimStyle.Render("  ")))
	sep := dimStyle.Render(strings.Repeat("─", width))
	return sep + "\n" + line
}

func renderCmdBar(width int, input string) string {
	prompt := cmdBarStyle.Render(":" + input)
	hint := dimStyle.Render("  enter to switch resource, esc to cancel")
	pad := width - lipgloss.Width(prompt) - lipgloss.Width(hint)
	if pad < 0 {
		pad = 0
	}
	line := prompt + strings.Repeat(" ", pad) + hint
	sep := dimStyle.Render(strings.Repeat("─", width))
	return sep + "\n" + line
}

package ui

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

// Rows the header and footer bands take; the page gets the rest. Each band is
// pinned to exactly its rows by band, so a line that would wrap cannot push the
// footer off the bottom of the screen.
const (
	headerH = 2
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

// AssumeIdentityMsg asks the App to log the session in as another identity.
// Pages raise it from a ConfirmMsg action, never directly.
type AssumeIdentityMsg struct {
	UID  string
	Name string
}

// ReloginMsg asks the App to log in again as the caller themselves, which both
// drops an assumed identity and renews an expired token.
type ReloginMsg struct{}

// identitySwitchedMsg carries the outcome of the chainctl login that ran while
// the TUI was suspended.
type identitySwitchedMsg struct {
	name string
	err  error
}

// ConfirmMsg asks the App to put a yes/no pop-up up and only run Action if the
// answer is yes. Pages raise this rather than acting straight away, so anything
// destructive is confirmed the same way everywhere.
type ConfirmMsg struct {
	// Prompt is the question, Detail names the thing it is about, and Warning
	// says what answering yes will do.
	Prompt  string
	Detail  string
	Warning string
	Action  tea.Cmd
}

// App is the root bubbletea model – owns the navigation stack.
type App struct {
	client  *api.Client
	stack   []Page
	width   int
	height  int
	cmdMode bool
	cmd     textinput.Model
	// quitting is set while the "are you sure" dialog is up; nothing has been
	// torn down yet, so cancelling leaves the session exactly as it was.
	quitting bool
	// confirm holds a page's pending yes/no question. Its Action has not run and
	// will not unless the answer is yes.
	confirm *ConfirmMsg
	// notice reports the result of something the App did itself, rather than a
	// page — switching identity, for instance. Cleared by the next keypress.
	notice    string
	noticeErr bool
	orgCtx    string // active organisation UIDP
	orgName   string // display name for the active organisation
}

// inputCapture is implemented by pages that own the keyboard while one of their
// own prompts is open (filter, save, sort). The App defers its single-key
// bindings to them so a keystroke meant for a text box is not read as a command.
type inputCapture interface {
	InputActive() bool
}

func (a App) inputActive() bool {
	p, ok := a.top().(inputCapture)
	return ok && p.InputActive()
}

func New(client *api.Client) App {
	c := textinput.New()
	c.Placeholder = "resource (repos, charts, libraries, libpolicy, adv)..."
	c.CharLimit = 40

	// Everything is scoped to an org, so the org picker is the root page.
	root := NewOrgSelectorPage(client)
	return App{
		client: client,
		stack:  []Page{root},
		cmd:    c,
	}
}

// orgListResource is the resource type of the org picker, which is the root page.
const orgListResource = "organizations"

func (a App) top() Page { return a.stack[len(a.stack)-1] }

// pop removes the top page. Landing back on the org picker clears the active org
// so the header and `:` commands do not carry a stale context.
func (a *App) pop() {
	if len(a.stack) <= 1 {
		return
	}
	a.stack = append([]Page{}, a.stack[:len(a.stack)-1]...)
	if a.top().ResourceType() == orgListResource {
		a.orgCtx, a.orgName = "", ""
	}
}

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

	case SelectOrgMsg:
		// Picking an org sets the context every other page is scoped by, then
		// opens that org's menu. The org list stays below it, so esc goes back.
		a.orgCtx = msg.UID
		a.orgName = msg.Name
		page := NewOrgMenuPage(a.client, msg.UID, msg.Name)
		page.SetSize(a.width, a.contentH())
		a.stack = append(append([]Page{}, a.stack...), page)
		return a, page.Init()

	case PushMsg:
		msg.P.SetSize(a.width, a.contentH())
		a.stack = append(append([]Page{}, a.stack...), msg.P)
		return a, msg.P.Init()

	case PopMsg:
		a.pop()
		return a, nil

	case SwitchMsg:
		page := resolveResourcePage(a.client, msg.Resource, msg.GroupCtx, a.orgName)
		if page == nil {
			return a, nil
		}
		page.SetSize(a.width, a.contentH())
		// Keep the org picker underneath so esc still walks back to it, unless
		// the command switched to the picker itself.
		if page.ResourceType() == orgListResource {
			a.orgCtx, a.orgName = "", ""
			a.stack = []Page{page}
		} else {
			a.stack = []Page{a.stack[0], page}
		}
		return a, page.Init()

	case ConfirmMsg:
		confirm := msg
		a.confirm = &confirm
		return a, nil

	case AssumeIdentityMsg:
		return a.login(msg.UID, msg.Name)

	case ReloginMsg:
		return a.login("", "")

	case identitySwitchedMsg:
		return a.finishLogin(msg)

	case tea.KeyMsg:
		// Any keypress acknowledges the last notice.
		a.notice, a.noticeErr = "", false
		if a.confirm != nil {
			return a.handleConfirmKey(msg)
		}
		if a.quitting {
			return a.handleQuitKey(msg)
		}
		if a.cmdMode {
			return a.handleCmdKey(msg)
		}
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		// A page owns the keyboard while one of its own prompts is open, so a
		// single-key global does not swallow what the user is typing.
		if !a.inputActive() {
			switch msg.String() {
			case "q":
				a.quitting = true
				return a, nil
			case "esc":
				a.pop()
				return a, nil
			case ":":
				a.cmdMode = true
				a.cmd.SetValue("")
				a.cmd.Focus()
				return a, textinput.Blink
			}
		}
	}

	// Delegate to top of stack.
	updated, cmd := a.top().Update(msg)
	newStack := append([]Page{}, a.stack...)
	newStack[len(newStack)-1] = updated.(Page)
	a.stack = newStack
	return a, cmd
}

// login hands the terminal to chainctl so it can run its interactive login,
// assuming identityUID when one is given. tea.ExecProcess suspends the program
// for the duration, which is the only way a browser-based login can work from
// inside a full-screen TUI.
func (a App) login(identityUID, name string) (tea.Model, tea.Cmd) {
	// chainctl writes its result to the token cache, which CHAINGUARD_TOKEN
	// overrides. Logging in would appear to succeed and change nothing.
	if api.TokenFromEnv() {
		a.notice = "CHAINGUARD_TOKEN is set, so it pins this session's identity — unset it to switch"
		a.noticeErr = true
		return a, nil
	}
	cmd, err := api.LoginCommand(identityUID)
	if err != nil {
		a.notice = err.Error()
		a.noticeErr = true
		return a, nil
	}
	return a, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return identitySwitchedMsg{name: name, err: err}
	})
}

// finishLogin rebuilds the session around whatever token chainctl just cached.
//
// The stack is reset to the org picker rather than kept: a different principal
// may not see the same orgs, or the same resources within them, so carrying the
// old pages over would show a view the new session cannot refresh.
func (a App) finishLogin(msg identitySwitchedMsg) (tea.Model, tea.Cmd) {
	who := msg.name
	if who == "" {
		who = "your own login"
	}
	if msg.err != nil {
		a.notice = "login as " + who + " failed: " + msg.err.Error()
		a.noticeErr = true
		return a, nil
	}
	client, err := api.NewClient()
	if err != nil {
		a.notice = "no usable token after login: " + err.Error()
		a.noticeErr = true
		return a, nil
	}
	old := a.client
	a.client = client
	a.orgCtx, a.orgName = "", ""
	a.notice = "now acting as " + who
	a.noticeErr = false
	root := NewOrgSelectorPage(client)
	root.SetSize(a.width, a.contentH())
	a.stack = []Page{root}
	if old != nil {
		// The old connections carried the previous credentials; anything still
		// running on them is meant to stop.
		_ = old.Close()
	}
	return a, root.Init()
}

// handleConfirmKey answers a page's pending question. Only an explicit yes runs
// the action; anything unrecognised is ignored rather than taken as an answer,
// which matters when the action is destructive.
func (a App) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		action := a.confirm.Action
		a.confirm = nil
		return a, action
	case "n", "N", "esc", "ctrl+c":
		a.confirm = nil
		return a, nil
	}
	return a, nil
}

// handleQuitKey drives the quit confirmation. Only an explicit yes quits;
// unrecognised keys are ignored rather than treated as either answer.
func (a App) handleQuitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter", "ctrl+c":
		return a, tea.Quit
	case "n", "N", "esc":
		a.quitting = false
		return a, nil
	}
	return a, nil
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
		// A couple of commands act on the session rather than navigating.
		switch strings.ToLower(val) {
		case "login", "relogin", "whoami":
			return a, func() tea.Msg { return ReloginMsg{} }
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
	header := renderHeader(a.width, top.ResourceType(), a.groupPath(), a.breadcrumb(), a.whoami())
	var footer string
	switch {
	case a.cmdMode:
		footer = renderCmdBar(a.width, a.cmd.View())
	case a.notice != "":
		footer = renderNotice(a.width, a.notice, a.noticeErr)
	default:
		footer = renderFooter(a.width, top.ResourceType(), len(a.stack) > 1)
	}
	view := strings.Join([]string{
		band(header, a.width, headerH),
		band(top.View(), a.width, a.contentH()),
		band(footer, a.width, footerH),
	}, "\n")
	switch {
	case a.confirm != nil:
		return overlayCenter(view, confirmDialog(a.confirm.Prompt, a.confirm.Detail, a.confirm.Warning))
	case a.quitting:
		return overlayCenter(view, quitDialog())
	}
	return view
}

// band pins one region of the screen to exactly h rows and at most width cells
// per row. The view is the header, the page and the footer stacked to the
// window's height, so a region that renders larger would push the footer past
// the bottom of the screen — and bubbletea cannot repaint rows it believes are
// off-screen, so the previous frame's footer stays visible under the new one.
func band(s string, width, h int) string {
	return lipgloss.NewStyle().Height(h).MaxHeight(h).MaxWidth(width).Render(s)
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
	// Only folders that have been drilled into ("group"), not folder lists, so
	// the path reads org / folder / subfolder.
	for _, p := range a.stack {
		if p.ResourceType() == "group" {
			parts = append(parts, p.Label())
		}
	}
	if len(parts) == 0 {
		return "no org"
	}
	return strings.Join(parts, " / ")
}

// whoami describes the session for the header. While an identity is assumed it
// names both the identity being acted as and the human who assumed it, because
// that difference decides what the session is allowed to do.
func (a App) whoami() string {
	if a.client == nil {
		return ""
	}
	me := a.client.Email()
	if me == "" {
		me = shortUID(a.client.Subject())
	}
	if !a.client.Assumed() {
		return me
	}
	return "as " + shortUID(a.client.Identity()) + " (" + me + ")"
}

// resolveResourcePage maps a `:` command to a page, scoped to the active org
// (groupCtx). orgName is used only for page labels.
func resolveResourcePage(client *api.Client, resource, groupCtx, orgName string) Page {
	switch strings.ToLower(strings.TrimSpace(resource)) {
	case "home", "menu", "root", "org", "orgs", "organization", "organizations":
		return NewOrgSelectorPage(client)
	case "g", "group", "groups", "folder", "folders":
		return NewGroupsPage(client, groupCtx)
	case "u", "user", "users", "member", "members":
		return NewUsersPage(client, groupCtx, orgName)
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
	case "chart", "charts", "helm":
		return NewChartsPage(client, groupCtx)
	case "adv", "advisory", "advisories":
		return NewAdvisoriesPage(client, groupCtx)
	case "lib", "libs", "library", "libraries", "artifact", "artifacts":
		return NewLibrariesEcosystemPage(client, groupCtx, orgName)
	case "libpolicy", "librariespolicy", "libraries-policy", "policy":
		return NewLibraryPolicyMenuPage(client, groupCtx, orgName)
	// Container policy keeps its own prefixes: the bare policy words above were
	// already claimed by Libraries.
	case "cpolicy", "containerpolicy", "imagepolicy", "repopolicy":
		return NewImagePolicyMenuPage(client, groupCtx, orgName)
	case "imagepolicies", "cpolicies":
		return NewImagePoliciesPage(client, groupCtx)
	case "cbindings", "imagebindings":
		return NewImagePolicyBindingsPage(client, groupCtx)
	case "decisions", "pulls":
		return NewImagePolicyDecisionsPage(client, groupCtx, groupCtx, "")
	case "overrides", "waivers":
		return NewImagePolicyOverridesPage(client, groupCtx)
	case "ent", "ents", "entitlement", "entitlements":
		return NewLibraryEntitlementsPage(client, groupCtx)
	case "policies", "libpolicies":
		return NewLibraryPoliciesPage(client, groupCtx)
	case "binding", "bindings", "policybindings":
		return NewLibraryPolicyBindingsPage(client, groupCtx)
	case "blocked", "blocks", "blockevents", "blockedpackages":
		return NewLibraryBlockEventsPage(client, groupCtx)
	case "java", "libraries/java", "lib/java":
		return NewLibraryArtifactsPage(client, string(api.LibraryEcosystemJava))
	case "python", "libraries/python", "lib/python", "pypi":
		return NewLibraryArtifactsPage(client, string(api.LibraryEcosystemPython))
	case "javascript", "js", "npm", "node", "libraries/javascript", "lib/javascript", "lib/js", "lib/npm":
		return NewLibraryArtifactsPage(client, string(api.LibraryEcosystemJavaScript))
	}
	return nil
}

func renderHeader(width int, resource, groupPath, breadcrumb, whoami string) string {
	segments := []string{
		appNameStyle.Render("chaintui"),
		sepStyle.Render("  │  "),
		ctxStyle.Render(groupPath),
		sepStyle.Render("  │  "),
		resTypeStyle.Render(resource),
	}
	if whoami != "" {
		// An assumed identity changes what every page can see, so it is called
		// out rather than dimmed away.
		style := dimStyle
		if strings.HasPrefix(whoami, "as ") {
			style = assumedStyle
		}
		segments = append(segments, sepStyle.Render("  │  "), style.Render(whoami))
	}
	left := lipgloss.JoinHorizontal(lipgloss.Left, segments...)
	right := dimStyle.Render(breadcrumb)

	// The breadcrumb gives way to the context on the left, and is cut from its
	// start so the page you are on is the part that stays visible.
	if over := lipgloss.Width(left) + 1 + lipgloss.Width(right) - width; over > 0 {
		right = ansi.TruncateLeft(right, over+1, "…")
	}
	pad := width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}

	line1 := headerStyle.Width(width).Render(
		fitLine(left+strings.Repeat(" ", pad)+right, width),
	)
	line2 := dimStyle.Render(strings.Repeat("─", width))
	return line1 + "\n" + line2
}

func renderFooter(width int, resource string, canGoBack bool) string {
	global := []string{
		keyHint(":", "cmd"),
		keyHint("/", "search"),
		keyHint("o", "sort"),
		keyHint("d", "describe"),
		keyHint("r", "refresh"),
		keyHint("[", "prev page"),
		keyHint("]", "next page"),
	}
	var hints []string
	switch resource {
	case "groups", "group", "repos", "repo", "tags", "charts", "libraries", "artifacts",
		"org", "librariespolicy", "containerpolicy":
		hints = append(hints, keyHint("↵", "drill down"))
	case orgListResource:
		hints = append(hints, keyHint("↵", "select org"))
	}
	if resource == "artifacts" {
		hints = append(hints, keyHint("m", "remediated"), keyHint("x", "export json"))
	}
	if resource == "blocked" {
		hints = append(hints, keyHint("l", "log mode"))
	}
	if resource == "policydecisions" {
		hints = append(hints, keyHint("x", "denied only"))
	}
	if resource == "roles" {
		hints = append(hints, keyHint("c", "custom only"))
	}
	if resource == "repos" || resource == "tags" {
		hints = append(hints, keyHint("v", "cves"))
	}
	if resource == "repos" {
		hints = append(hints, keyHint("D", "delete repo"))
	}
	if resource == "users" || resource == "identities" {
		hints = append(hints, keyHint("a", "assume"), keyHint("D", "revoke access"))
	}
	if resource == "cves" {
		hints = append(hints, keyHint("f", "fixable only"), keyHint("s", "save csv"))
	}
	if resource == "sbom" {
		hints = append(hints, keyHint("s", "save csv"))
	}
	if canGoBack {
		hints = append(hints, keyHint("esc", "back"))
	}
	hints = append(hints, keyHint("q", "quit"))

	line := footerStyle.Width(width).Render(fitHints(global, hints, width))
	sep := dimStyle.Render(strings.Repeat("─", width))
	return sep + "\n" + line
}

// fitHints lays the key hints out on one row. When they do not all fit, the
// generic ones go first, from the right — every page has those, and they are
// documented — so the page's own keys stay visible. If those still overflow,
// page keys go from the left, keeping the way out: q quit is last and is never
// dropped, and esc back goes only once it is the last thing before it. The cut
// is a backstop for a window too narrow even for that.
func fitHints(global, page []string, width int) string {
	sep := dimStyle.Render("  ")
	for {
		all := append(append(make([]string, 0, len(global)+len(page)), global...), page...)
		line := strings.Join(all, sep)
		switch {
		case lipgloss.Width(line) <= width:
			return line
		case len(global) > 0:
			global = global[:len(global)-1]
		case len(page) > 1:
			page = page[1:]
		default:
			return fitLine(line, width)
		}
	}
}

// renderNotice replaces the footer hints with the result of something the App
// did, until the next keypress.
func renderNotice(width int, notice string, isErr bool) string {
	style := noticeStyle
	if isErr {
		style = errStyle
	}
	sep := dimStyle.Render(strings.Repeat("─", width))
	return sep + "\n" + footerStyle.Width(width).Render(fitLine(style.Render(notice), width))
}

func renderCmdBar(width int, input string) string {
	prompt := cmdBarStyle.Render(":" + input)
	hint := dimStyle.Render("  enter to switch resource, esc to cancel")
	pad := width - lipgloss.Width(prompt) - lipgloss.Width(hint)
	if pad < 0 {
		pad = 0
	}
	line := fitLine(prompt+strings.Repeat(" ", pad)+hint, width)
	sep := dimStyle.Render(strings.Repeat("─", width))
	return sep + "\n" + line
}

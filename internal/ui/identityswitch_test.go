package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// chainctl writes its result to the token cache, which CHAINGUARD_TOKEN
// overrides — so a login would appear to succeed and change nothing.
func TestAssumeRefusedWhenTokenPinnedByEnv(t *testing.T) {
	t.Setenv("CHAINGUARD_TOKEN", "pinned-token")
	a := sized(New(nil))
	m, cmd := a.Update(AssumeIdentityMsg{UID: "org/1/support", Name: "support"})
	a = m.(App)
	if cmd != nil {
		t.Fatal("must not run chainctl while the env pins the token")
	}
	if !a.noticeErr || !strings.Contains(a.notice, "CHAINGUARD_TOKEN") {
		t.Fatalf("notice=%q err=%v", a.notice, a.noticeErr)
	}
	if !strings.Contains(a.View(), "CHAINGUARD_TOKEN") {
		t.Errorf("notice not rendered:\n%s", a.View())
	}
}

func TestLoginFailureIsReportedAndSessionKept(t *testing.T) {
	t.Setenv("CHAINGUARD_TOKEN", "")
	a := sized(New(nil))
	m, _ := a.Update(SelectOrgMsg{UID: "org/1", Name: "acme"})
	a = m.(App)
	stack := len(a.stack)

	m, cmd := a.Update(identitySwitchedMsg{name: "support", err: errors.New("browser closed")})
	a = m.(App)
	if cmd != nil {
		t.Error("a failed login should not go on to rebuild the session")
	}
	if !a.noticeErr || !strings.Contains(a.notice, "support") || !strings.Contains(a.notice, "browser closed") {
		t.Fatalf("notice=%q", a.notice)
	}
	// The session is untouched: still the same org, same stack.
	if len(a.stack) != stack || a.orgCtx != "org/1" {
		t.Fatalf("stack=%d orgCtx=%q, want the session left alone", len(a.stack), a.orgCtx)
	}
}

// A successful login needs a usable token afterwards; without one the session is
// left as it was rather than half-swapped.
func TestLoginWithNoUsableTokenKeepsSession(t *testing.T) {
	t.Setenv("CHAINGUARD_TOKEN", "")
	t.Setenv("PATH", t.TempDir()) // no chainctl, so no token can be resolved
	a := sized(New(nil))
	m, _ := a.Update(SelectOrgMsg{UID: "org/1", Name: "acme"})
	a = m.(App)

	m, cmd := a.Update(identitySwitchedMsg{name: "support"})
	a = m.(App)
	if cmd != nil {
		t.Error("expected no follow-on command")
	}
	if !a.noticeErr || !strings.Contains(a.notice, "no usable token") {
		t.Fatalf("notice=%q", a.notice)
	}
	if a.orgCtx != "org/1" {
		t.Errorf("orgCtx=%q, want the session left alone", a.orgCtx)
	}
}

// A notice is an acknowledgement, not a mode: the next keypress clears it.
func TestNoticeClearedByNextKey(t *testing.T) {
	t.Setenv("CHAINGUARD_TOKEN", "pinned-token")
	a := sized(New(nil))
	m, _ := a.Update(AssumeIdentityMsg{UID: "org/1/support"})
	a = m.(App)
	if a.notice == "" {
		t.Fatal("expected a notice")
	}
	m, _ = a.Update(keyPress('j'))
	if got := m.(App).notice; got != "" {
		t.Errorf("notice=%q, want cleared", got)
	}
}

func TestWhoamiNamesAssumedIdentity(t *testing.T) {
	t.Parallel()
	// No client yet (the app can be constructed before auth), so nothing to say.
	if got := (App{}).whoami(); got != "" {
		t.Errorf("whoami=%q", got)
	}
	// The rendered header carries whatever whoami returns.
	header := renderHeader(200, "users", "acme", "acme > users", "as support (tom@chainguard.dev)")
	for _, want := range []string{"as support", "tom@chainguard.dev"} {
		if !strings.Contains(header, want) {
			t.Errorf("header missing %q", want)
		}
	}
	if strings.Contains(renderHeader(200, "users", "acme", "b", ""), "  │    │") {
		t.Error("no identity should mean no empty segment")
	}
}

func TestReloginUsesOwnLogin(t *testing.T) {
	t.Setenv("CHAINGUARD_TOKEN", "pinned-token")
	a := sized(New(nil))
	// Refused for the same reason as an assume, and says so once.
	m, cmd := a.Update(ReloginMsg{})
	if cmd != nil {
		t.Error("must not run chainctl while the env pins the token")
	}
	if !strings.Contains(m.(App).notice, "CHAINGUARD_TOKEN") {
		t.Errorf("notice=%q", m.(App).notice)
	}
}

// The confirmation owns the keyboard, so an assume cannot be triggered by a
// stray key while it is open.
func TestAssumeOnlyRunsOnYes(t *testing.T) {
	t.Setenv("CHAINGUARD_TOKEN", "pinned-token")
	a := sized(New(nil))
	m, _ := a.Update(ConfirmMsg{
		Prompt: "Switch to this identity?",
		Action: func() tea.Msg { return AssumeIdentityMsg{UID: "org/1/support"} },
	})
	a = m.(App)

	m, cmd := a.Update(keyPress('z'))
	a = m.(App)
	if a.confirm == nil {
		t.Error("a stray key is not an answer")
	}
	if cmd != nil {
		t.Error("a stray key must not act")
	}

	m, cmd = a.Update(keyPress('y'))
	a = m.(App)
	if a.confirm != nil {
		t.Error("dialog should be dismissed")
	}
	if cmd == nil {
		t.Fatal("yes should run the action")
	}
	if _, ok := cmd().(AssumeIdentityMsg); !ok {
		t.Fatalf("msg type %T, want AssumeIdentityMsg", cmd())
	}
}

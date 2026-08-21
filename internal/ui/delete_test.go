package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tomsk73/chaintui/internal/api"
)

// confirmFrom runs a row action and returns the ConfirmMsg it raises.
func confirmFrom(t *testing.T, cmd tea.Cmd) ConfirmMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("no command returned")
	}
	msg, ok := cmd().(ConfirmMsg)
	if !ok {
		t.Fatalf("msg type %T, want ConfirmMsg", cmd())
	}
	return msg
}

func repoRow() RowData {
	repo := api.Repo{UID: "org/1/repo", Name: "nginx", Description: "custom image"}
	return RowData{UID: repo.UID, Columns: []string{repo.Name, repo.Description, "1d ago"}, Raw: repo}
}

func TestDeleteRepoAsksBeforeDeleting(t *testing.T) {
	t.Parallel()
	repos := NewReposPage(nil, "org/1")
	del := repos.rowActionFor("D")
	if del == nil {
		t.Fatal("repos page should bind D to delete")
	}
	// d is describe; delete must not be sitting on it.
	if repos.rowActionFor("d") != nil {
		t.Fatal("lowercase d must stay describe")
	}
	// Tag delete was the wrong surface — D belongs on repos, not tags.
	tags := NewTagsPage(nil, "org/1/repo", "nginx")
	if tags.rowActionFor("D") != nil {
		t.Fatal("tags page must not bind D; delete the repo instead")
	}

	msg := confirmFrom(t, del(repoRow()))
	if msg.Prompt != "Are you sure you want to delete this repository?" {
		t.Errorf("prompt=%q", msg.Prompt)
	}
	if msg.Detail != "nginx" {
		t.Errorf("detail=%q", msg.Detail)
	}
	if !strings.Contains(msg.Warning, "repository") || !strings.Contains(msg.Warning, "tags") {
		t.Errorf("warning=%q", msg.Warning)
	}
	if msg.Action == nil {
		t.Fatal("no action to run on yes")
	}

	// A row that is not a repo is ignored rather than panicking.
	if del(RowData{UID: "x"}) != nil {
		t.Error("unexpected command for a malformed row")
	}
}

// Nothing may be deleted until the answer is yes.
func TestConfirmDialogGatesTheAction(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  tea.KeyMsg
		run  bool
	}{
		{"y", keyPress('y'), true},
		{"Y", keyPress('Y'), true},
		{"enter", tea.KeyMsg{Type: tea.KeyEnter}, true},
		{"n", keyPress('n'), false},
		{"N", keyPress('N'), false},
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}, false},
	} {
		ran := false
		a := sized(New(nil))
		m, _ := a.Update(ConfirmMsg{
			Prompt: "Are you sure you want to delete this repository?",
			Action: func() tea.Msg { ran = true; return nil },
		})
		a = m.(App)
		if a.confirm == nil {
			t.Fatal("dialog should be up")
		}
		if !strings.Contains(a.View(), "delete this repository?") {
			t.Fatalf("dialog not rendered:\n%s", a.View())
		}

		m, cmd := a.Update(tc.key)
		a = m.(App)
		if a.confirm != nil {
			t.Errorf("%s: dialog should be dismissed", tc.name)
		}
		if cmd != nil {
			cmd()
		}
		if ran != tc.run {
			t.Errorf("%s: action ran=%v, want %v", tc.name, ran, tc.run)
		}
	}

	// An unrecognised key is not an answer: the dialog stays and nothing runs.
	ran := false
	a := sized(New(nil))
	m, _ := a.Update(ConfirmMsg{Prompt: "delete?", Action: func() tea.Msg { ran = true; return nil }})
	a = m.(App)
	m, cmd := a.Update(keyPress('z'))
	if m.(App).confirm == nil {
		t.Error("dialog should still be up")
	}
	if cmd != nil {
		cmd()
	}
	if ran {
		t.Error("a stray key must not delete anything")
	}
}

// While the dialog is up it owns the keyboard, so q cannot start a second one.
func TestConfirmDialogOwnsTheKeyboard(t *testing.T) {
	t.Parallel()
	a := sized(New(nil))
	m, _ := a.Update(ConfirmMsg{Prompt: "delete?", Action: nil})
	a = m.(App)
	m, _ = a.Update(keyPress('q'))
	a = m.(App)
	if a.quitting {
		t.Error("q should be consumed by the open dialog, not open a second one")
	}
	if a.confirm == nil {
		t.Error("q is not an answer, so the dialog should still be up")
	}
}

func TestDeletedMsgRefreshesTheList(t *testing.T) {
	t.Parallel()
	loads := 0
	p := testListPage(func(token string, _ int, _, _ string) (PageResult, error) {
		loads++
		return PageResult{Rows: rows("a")}, nil
	})
	p.pageToken = "page2"

	m, cmd := p.Update(deletedMsg{what: "nginx"})
	p = m.(*ListPage)
	if !strings.Contains(p.saveMsg, "deleted nginx") {
		t.Errorf("saveMsg=%q", p.saveMsg)
	}
	if cmd == nil || !p.loading {
		t.Fatal("a successful delete should reload the list")
	}

	// A failure reports and leaves the list alone.
	m, cmd = p.Update(deletedMsg{what: "nginx", err: errors.New("permission denied")})
	p = m.(*ListPage)
	if !strings.Contains(p.saveMsg, "permission denied") {
		t.Errorf("saveMsg=%q", p.saveMsg)
	}
	if cmd != nil {
		t.Error("a failed delete should not reload")
	}
}

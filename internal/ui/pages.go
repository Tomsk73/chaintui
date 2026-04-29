package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Tomsk73/chaintui/internal/api"
)

// SelectOrgMsg is emitted when the user picks an organisation in the org selector.
type SelectOrgMsg struct {
	UID  string
	Name string
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

func shortUID(uid string) string {
	parts := strings.Split(uid, "/")
	return parts[len(parts)-1]
}

func pushPage(p Page) tea.Cmd {
	return func() tea.Msg { return PushMsg{P: p} }
}

// --- Org selector ---

// NewOrgSelectorPage lists the organisations the current user belongs to.
// Selecting one emits SelectOrgMsg so the App can set the active org context.
func NewOrgSelectorPage(client *api.Client) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 35},
		{Title: "UID", Width: 25},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		orgs, err := client.ListMyOrganizations()
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(orgs))
		for i, g := range orgs {
			rows[i] = RowData{
				UID:     g.UID,
				Columns: []string{g.Name, shortUID(g.UID), g.Description, relativeTime(g.CreateTime)},
				Raw:     g,
			}
		}
		return rows, nil
	}
	enter := func(row RowData) tea.Cmd {
		return func() tea.Msg { return SelectOrgMsg{UID: row.UID, Name: row.Columns[0]} }
	}
	return newListPage("organizations", "", cols, load, enter)
}

// --- Groups ---

func NewGroupsPage(client *api.Client, parentUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "UID", Width: 20},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		groups, err := client.ListGroups(parentUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(groups))
		for i, g := range groups {
			rows[i] = RowData{
				UID:     g.UID,
				Columns: []string{g.Name, shortUID(g.UID), g.Description, relativeTime(g.CreateTime)},
				Raw:     g,
			}
		}
		return rows, nil
	}
	enter := func(row RowData) tea.Cmd {
		return pushPage(NewGroupsPage(client, row.UID).WithLabel(row.Columns[0]))
	}
	return newListPage("groups", parentUID, cols, load, enter)
}

// --- Identities ---

func NewIdentitiesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "UID", Width: 20},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListIdentities(groupUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{v.Name, shortUID(v.UID), v.Description, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	return newListPage("identities", groupUID, cols, load, nil)
}

// --- Roles ---

func NewRolesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "CAPABILITIES", Width: 40},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListRoles(groupUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			caps := strings.Join(v.Capabilities, ", ")
			if len(caps) > 38 {
				caps = caps[:35] + "..."
			}
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{v.Name, caps, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	return newListPage("roles", groupUID, cols, load, nil)
}

// --- RoleBindings ---

func NewRoleBindingsPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "UID", Width: 20},
		{Title: "IDENTITY", Width: 30},
		{Title: "ROLE", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListRoleBindings(groupUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{shortUID(v.UID), shortUID(v.IdentityUID), shortUID(v.RoleUID), relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	return newListPage("rolebindings", groupUID, cols, load, nil)
}

// --- IdentityProviders ---

func NewIDPsPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "UID", Width: 20},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListIdentityProviders(groupUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{v.Name, shortUID(v.UID), v.Description, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	return newListPage("identityproviders", groupUID, cols, load, nil)
}

// --- GroupInvites ---

func NewGroupInvitesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "EMAIL", Width: 35},
		{Title: "ROLE", Width: 20},
		{Title: "EXPIRES", Width: 14},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListGroupInvites(groupUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{v.Email, shortUID(v.RoleUID), relativeTime(v.ExpirationTime), relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	return newListPage("groupinvites", groupUID, cols, load, nil)
}

// --- Repos ---

func NewReposPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 35},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListRepos(groupUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{v.Name, v.Description, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	enter := func(row RowData) tea.Cmd {
		return pushPage(NewTagsPage(client, row.UID).WithLabel(row.Columns[0]))
	}
	return newListPage("repos", groupUID, cols, load, enter)
}

// --- Tags ---

func NewTagsPage(client *api.Client, repoUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "DIGEST", Width: 40},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListTags(repoUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			digest := v.Digest
			if len(digest) > 19 {
				digest = digest[:7] + "..." + digest[len(digest)-9:]
			}
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{v.Name, digest, relativeTime(v.UpdateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	enter := func(row RowData) tea.Cmd {
		tag, ok := row.Raw.(api.Tag)
		if !ok {
			return nil
		}
		return pushPage(NewSBOMPage(client, repoUID, tag.Name, tag.Digest).WithLabel(tag.Name + " sbom"))
	}
	return newListPage("tags", repoUID, cols, load, enter)
}

// --- SBOM ---

func NewSBOMPage(client *api.Client, repoUID, tagName, digest string) *ListPage {
	cols := []table.Column{
		{Title: "PACKAGE", Width: 35},
		{Title: "VERSION", Width: 25},
		{Title: "PURL", Width: 50},
	}
	load := func() ([]RowData, error) {
		pkgs, err := client.GetTagSBOM(repoUID, digest)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(pkgs))
		for i, p := range pkgs {
			rows[i] = RowData{
				UID:     fmt.Sprintf("%s@%s", p.Name, p.Version),
				Columns: []string{p.Name, p.Version, p.Purl},
				Raw:     p,
			}
		}
		return rows, nil
	}
	return newListPage("sbom", repoUID, cols, load, nil)
}

// --- Advisories ---

func NewAdvisoriesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "ID", Width: 20},
		{Title: "ARTIFACT", Width: 30},
		{Title: "ALIASES", Width: 40},
		{Title: "CREATED", Width: 14},
	}
	load := func() ([]RowData, error) {
		items, err := client.ListAdvisories(groupUID)
		if err != nil {
			return nil, err
		}
		rows := make([]RowData, len(items))
		for i, v := range items {
			id := v.AdvisoryID
			if id == "" {
				id = v.UID
			}
			aliases := strings.Join(v.Aliases, ", ")
			rows[i] = RowData{
				UID:     v.UID,
				Columns: []string{id, v.ArtifactName, aliases, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}
		return rows, nil
	}
	return newListPage("advisories", groupUID, cols, load, nil)
}

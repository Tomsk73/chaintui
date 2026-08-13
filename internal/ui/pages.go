package ui

import (
	"encoding/csv"
	"fmt"
	"os"
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

func pageOpts(token string, pageSize int, query, orderBy string) api.PageOpts {
	return api.PageOpts{
		PageSize:  int32(pageSize),
		PageToken: token,
		Query:     query,
		OrderBy:   orderBy,
	}
}

func toPageResult[T any](page api.Page[T], mapRow func(T) RowData) PageResult {
	rows := make([]RowData, len(page.Items))
	for i, item := range page.Items {
		rows[i] = mapRow(item)
	}
	return PageResult{
		Rows:          rows,
		NextPageToken: page.NextPageToken,
		TotalCount:    page.TotalCount,
	}
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
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListMyOrganizations(pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(g api.Group) RowData {
			return RowData{
				UID:     g.UID,
				Columns: []string{g.Name, shortUID(g.UID), g.Description, relativeTime(g.CreateTime)},
				Raw:     g,
			}
		}), nil
	}
	enter := func(row RowData) tea.Cmd {
		return func() tea.Msg { return SelectOrgMsg{UID: row.UID, Name: row.Columns[0]} }
	}
	return newListPage("organizations", "", cols, load, enter).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 1: "uid", 3: "create_time"})
}

// --- Groups ---

func NewGroupsPage(client *api.Client, parentUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "UID", Width: 20},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListGroups(parentUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(g api.Group) RowData {
			return RowData{
				UID:     g.UID,
				Columns: []string{g.Name, shortUID(g.UID), g.Description, relativeTime(g.CreateTime)},
				Raw:     g,
			}
		}), nil
	}
	enter := func(row RowData) tea.Cmd {
		return pushPage(NewGroupResourcesPage(client, row.UID, row.Columns[0]))
	}
	return newListPage("groups", parentUID, cols, load, enter).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 1: "uid", 3: "create_time"})
}

// --- Group resource selector ---

func NewGroupResourcesPage(client *api.Client, groupUID, groupName string) *ListPage {
	cols := []table.Column{
		{Title: "RESOURCE", Width: 25},
		{Title: "DESCRIPTION", Width: 50},
	}

	type entry struct {
		name, desc string
		make       func() Page
	}
	entries := []entry{
		{"groups", "Child groups", func() Page {
			return NewGroupsPage(client, groupUID).WithLabel(groupName + " groups")
		}},
		{"repos", "Container image repositories", func() Page {
			return NewReposPage(client, groupUID).WithLabel(groupName + " repos")
		}},
		{"identities", "Workload identities", func() Page {
			return NewIdentitiesPage(client, groupUID).WithLabel(groupName + " identities")
		}},
		{"roles", "IAM roles", func() Page {
			return NewRolesPage(client, groupUID).WithLabel(groupName + " roles")
		}},
		{"rolebindings", "Role bindings", func() Page {
			return NewRoleBindingsPage(client, groupUID).WithLabel(groupName + " rolebindings")
		}},
		{"identityproviders", "Identity providers", func() Page {
			return NewIDPsPage(client, groupUID).WithLabel(groupName + " idps")
		}},
		{"groupinvites", "Group invites", func() Page {
			return NewGroupInvitesPage(client, groupUID).WithLabel(groupName + " invites")
		}},
		{"advisories", "Security advisories", func() Page {
			return NewAdvisoriesPage(client, groupUID).WithLabel(groupName + " advisories")
		}},
	}

	load := func(string, int, string, string) (PageResult, error) {
		rows := make([]RowData, len(entries))
		for i, e := range entries {
			rows[i] = RowData{
				UID:     e.name,
				Columns: []string{e.name, e.desc},
				Raw:     e.make,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	enter := func(row RowData) tea.Cmd {
		makePage, ok := row.Raw.(func() Page)
		if !ok {
			return nil
		}
		return pushPage(makePage())
	}
	return newListPage("group", groupUID, cols, load, enter).WithLabel(groupName)
}

// --- Identities ---

func NewIdentitiesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "UID", Width: 20},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListIdentities(groupUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.Identity) RowData {
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Name, shortUID(v.UID), v.Description, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}), nil
	}
	return newListPage("identities", groupUID, cols, load, nil).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 1: "uid", 3: "create_time"})
}

// --- Roles ---

func NewRolesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "CAPABILITIES", Width: 40},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListRoles(groupUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.Role) RowData {
			caps := strings.Join(v.Capabilities, ", ")
			if len(caps) > 38 {
				caps = caps[:35] + "..."
			}
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Name, caps, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}), nil
	}
	return newListPage("roles", groupUID, cols, load, nil).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 2: "create_time"})
}

// --- RoleBindings ---

func NewRoleBindingsPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "UID", Width: 20},
		{Title: "IDENTITY", Width: 30},
		{Title: "ROLE", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, _, orderBy string) (PageResult, error) {
		page, err := client.ListRoleBindings(groupUID, pageOpts(token, pageSize, "", orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.RoleBinding) RowData {
			return RowData{
				UID:     v.UID,
				Columns: []string{shortUID(v.UID), shortUID(v.IdentityUID), shortUID(v.RoleUID), relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}), nil
	}
	return newListPage("rolebindings", groupUID, cols, load, nil).
		WithServerSort(map[int]string{0: "uid", 3: "create_time"})
}

// --- IdentityProviders ---

func NewIDPsPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "UID", Width: 20},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListIdentityProviders(groupUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.IdentityProvider) RowData {
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Name, shortUID(v.UID), v.Description, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}), nil
	}
	return newListPage("identityproviders", groupUID, cols, load, nil).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 1: "uid", 3: "create_time"})
}

// --- GroupInvites ---

func NewGroupInvitesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "EMAIL", Width: 35},
		{Title: "ROLE", Width: 20},
		{Title: "EXPIRES", Width: 14},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, _, orderBy string) (PageResult, error) {
		page, err := client.ListGroupInvites(groupUID, pageOpts(token, pageSize, "", orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.GroupInvite) RowData {
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Email, shortUID(v.RoleUID), relativeTime(v.ExpirationTime), relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}), nil
	}
	return newListPage("groupinvites", groupUID, cols, load, nil).
		WithServerSort(map[int]string{3: "created_at"})
}

// --- Repos ---

func NewReposPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 35},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListRepos(groupUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.Repo) RowData {
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Name, v.Description, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}), nil
	}
	enter := func(row RowData) tea.Cmd {
		return pushPage(NewTagsPage(client, row.UID).WithLabel(row.Columns[0]))
	}
	return newListPage("repos", groupUID, cols, load, enter).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 2: "create_time"})
}

// --- Tags ---

func NewTagsPage(client *api.Client, repoUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 30},
		{Title: "DIGEST", Width: 40},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListTags(repoUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.Tag) RowData {
			digest := v.Digest
			if len(digest) > 19 {
				digest = digest[:7] + "..." + digest[len(digest)-9:]
			}
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Name, digest, relativeTime(v.UpdateTime)},
				Raw:     v,
			}
		}), nil
	}
	enter := func(row RowData) tea.Cmd {
		tag, ok := row.Raw.(api.Tag)
		if !ok {
			return nil
		}
		return pushPage(NewSBOMPage(client, repoUID, tag.Name, tag.Digest).WithLabel(tag.Name + " sbom"))
	}
	return newListPage("tags", repoUID, cols, load, enter).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 2: "update_time"})
}

// --- SBOM ---

func NewSBOMPage(client *api.Client, repoUID, tagName, digest string) *ListPage {
	cols := []table.Column{
		{Title: "PACKAGE", Width: 35},
		{Title: "VERSION", Width: 25},
		{Title: "PURL", Width: 50},
	}
	load := func(string, int, string, string) (PageResult, error) {
		pkgs, err := client.GetTagSBOM(repoUID, digest)
		if err != nil {
			return PageResult{}, err
		}
		rows := make([]RowData, len(pkgs))
		for i, p := range pkgs {
			rows[i] = RowData{
				UID:     fmt.Sprintf("%s@%s", p.Name, p.Version),
				Columns: []string{p.Name, p.Version, p.Purl},
				Raw:     p,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	save := func(filename string, rows []RowData) error {
		f, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer f.Close()
		w := csv.NewWriter(f)
		w.Write([]string{"name", "version", "purl", "license"}) //nolint
		for _, row := range rows {
			pkg, ok := row.Raw.(api.SBOMPackage)
			if !ok {
				continue
			}
			w.Write([]string{pkg.Name, pkg.Version, pkg.Purl, pkg.License}) //nolint
		}
		w.Flush()
		return w.Error()
	}
	return newListPage("sbom", repoUID, cols, load, nil).WithSave(save)
}

// --- Advisories ---

func NewAdvisoriesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "ID", Width: 20},
		{Title: "ARTIFACT", Width: 30},
		{Title: "ALIASES", Width: 40},
		{Title: "CREATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListAdvisories(groupUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.Advisory) RowData {
			id := v.AdvisoryID
			if id == "" {
				id = v.UID
			}
			aliases := strings.Join(v.Aliases, ", ")
			return RowData{
				UID:     v.UID,
				Columns: []string{id, v.ArtifactName, aliases, relativeTime(v.CreateTime)},
				Raw:     v,
			}
		}), nil
	}
	return newListPage("advisories", groupUID, cols, load, nil).
		WithServerFilter().
		WithServerSort(map[int]string{0: "advisory_id", 1: "artifact_name", 3: "create_time"}).
		WithPageSize(25)
}

func formatBytes(n int64) string {
	if n <= 0 {
		return "-"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

// --- Root menu ---

// NewRootMenuPage is the app home: top-level areas (groups, libraries).
func NewRootMenuPage(client *api.Client) *ListPage {
	cols := []table.Column{
		{Title: "RESOURCE", Width: 20},
		{Title: "DESCRIPTION", Width: 50},
	}
	type entry struct {
		name, desc string
		make       func() Page
	}
	entries := []entry{
		{"groups", "Organizations and folders", func() Page {
			return NewGroupsPage(client, "")
		}},
		{"libraries", "Chainguard Libraries artifacts", func() Page {
			return NewLibrariesEcosystemPage(client)
		}},
	}
	load := func(string, int, string, string) (PageResult, error) {
		rows := make([]RowData, len(entries))
		for i, e := range entries {
			rows[i] = RowData{
				UID:     e.name,
				Columns: []string{e.name, e.desc},
				Raw:     e.make,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	enter := func(row RowData) tea.Cmd {
		makePage, ok := row.Raw.(func() Page)
		if !ok {
			return nil
		}
		return pushPage(makePage())
	}
	return newListPage("home", "", cols, load, enter).WithLabel("home")
}

// --- Libraries ---

// NewLibrariesEcosystemPage lets the user pick Java or Python before listing artifacts.
func NewLibrariesEcosystemPage(client *api.Client) *ListPage {
	cols := []table.Column{
		{Title: "ECOSYSTEM", Width: 16},
		{Title: "DESCRIPTION", Width: 40},
	}
	type entry struct {
		id, label, desc string
	}
	entries := []entry{
		{string(api.LibraryEcosystemJava), "java", "Maven / Java packages"},
		{string(api.LibraryEcosystemPython), "python", "PyPI / Python packages"},
	}
	load := func(string, int, string, string) (PageResult, error) {
		rows := make([]RowData, len(entries))
		for i, e := range entries {
			rows[i] = RowData{
				UID:     e.id,
				Columns: []string{e.label, e.desc},
				Raw:     e.id,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	enter := func(row RowData) tea.Cmd {
		eco, _ := row.Raw.(string)
		if eco == "" {
			eco = row.UID
		}
		return pushPage(NewLibraryArtifactsPage(client, eco).WithLabel(eco + " artifacts"))
	}
	return newListPage("libraries", "", cols, load, enter).WithLabel("libraries")
}

// NewLibraryArtifactsPage lists Chainguard Libraries artifacts for one ecosystem.
func NewLibraryArtifactsPage(client *api.Client, ecosystem string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 28},
		{Title: "LATEST", Width: 16},
		{Title: "VERSIONS", Width: 10},
		{Title: "DESCRIPTION", Width: 36},
		{Title: "CREATED", Width: 14},
		{Title: "UPDATED", Width: 14},
	}
	remediated := false
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListArtifacts(ecosystem, pageOpts(token, pageSize, query, orderBy), remediated)
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.LibraryArtifact) RowData {
			return RowData{
				UID: v.UID,
				Columns: []string{
					v.Name,
					v.LatestVersion,
					fmt.Sprintf("%d", v.VersionCount),
					truncate(v.Description, 80),
					relativeTime(v.CreateTime),
					relativeTime(v.UpdateTime),
				},
				Raw: v,
			}
		}), nil
	}
	enter := func(row RowData) tea.Cmd {
		art, ok := row.Raw.(api.LibraryArtifact)
		if !ok {
			return nil
		}
		label := art.Name
		if label == "" {
			label = art.UID
		}
		return pushPage(NewLibraryVersionsPage(client, art.UID, label).WithLabel(label + " versions"))
	}
	return newListPage("artifacts", "", cols, load, enter).
		WithLabel(ecosystem + " artifacts").
		WithServerFilter().
		WithServerSort(map[int]string{
			0: "name",
			1: "latest_version",
			2: "version_count",
			4: "create_time",
			5: "update_time",
		}).
		WithBoolToggle("m", "remediated", &remediated)
}

// NewLibraryVersionsPage lists versions for one Libraries artifact.
func NewLibraryVersionsPage(client *api.Client, artifactID, artifactName string) *ListPage {
	cols := []table.Column{
		{Title: "VERSION", Width: 24},
		{Title: "SIZE", Width: 12},
		{Title: "UPDATED", Width: 14},
	}
	load := func(token string, pageSize int, _, orderBy string) (PageResult, error) {
		page, err := client.ListArtifactVersions(artifactID, pageOpts(token, pageSize, "", orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.LibraryArtifactVersion) RowData {
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Version, formatBytes(v.SizeBytes), relativeTime(v.UpdateTime)},
				Raw:     v,
			}
		}), nil
	}
	return newListPage("versions", "", cols, load, nil).
		WithLabel(artifactName + " versions").
		WithServerSort(map[int]string{0: "version", 1: "size_bytes", 2: "update_time"})
}

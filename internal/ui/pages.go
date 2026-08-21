package ui

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
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

// untilTime renders a future timestamp as a countdown ("in 6d"). Past times fall
// back to relativeTime, which reads correctly for them.
func untilTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	d := time.Until(t)
	if d <= 0 {
		return relativeTime(t)
	}
	switch {
	case d < time.Minute:
		return "in <1m"
	case d < time.Hour:
		return fmt.Sprintf("in %dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("in %dh", int(d.Hours()))
	default:
		return fmt.Sprintf("in %dd", int(d.Hours()/24))
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
	entries := []menuEntry{
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
	return newMenuPage("group", groupUID, groupName, entries)
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

// NewRolesPage lists the roles bindable in a group: its own custom roles first,
// then Chainguard's built-in ones. `c` hides the built-ins.
func NewRolesPage(client *api.Client, groupUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 34},
		{Title: "TYPE", Width: 8},
		{Title: "CAPABILITIES", Width: 44},
		{Title: "DESCRIPTION", Width: 30},
	}
	customOnly := false
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListRoles(groupUID, pageOpts(token, pageSize, query, orderBy), customOnly)
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.Role) RowData {
			kind := "custom"
			if v.Managed {
				kind = "built-in"
			}
			return RowData{
				UID: v.UID,
				Columns: []string{
					v.Name,
					kind,
					truncate(strings.Join(v.Capabilities, ", "), 120),
					truncate(v.Description, 60),
				},
				Raw: v,
			}
		}), nil
	}
	// Roles are merged from two scopes and paged locally, so sorting stays local too.
	return newListPage("roles", groupUID, cols, load, nil).
		WithServerNameFilter().
		WithBoolToggle("c", "custom only", &customOnly)
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
		return pushPage(NewRepoMenuPage(client, groupUID, row.UID, row.Columns[0]))
	}
	// v is a shortcut straight to the menu's cves entry, for the repo's latest
	// image. Press v on a tag for a specific image.
	cves := func(row RowData) tea.Cmd {
		repo, ok := row.Raw.(api.Repo)
		if !ok {
			return nil
		}
		return pushPage(NewImageCVEsPage(client, repo.UID, repo.Name, "latest", ""))
	}
	return newListPage("repos", groupUID, cols, load, enter).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 2: "create_time"}).
		WithRowAction("v", cves)
}

// --- Repo menu ---

// NewRepoMenuPage is what Enter on a repo opens: the views available for one
// image. groupUID is carried through because the advisory list is scoped to the
// org, not to the repo.
func NewRepoMenuPage(client *api.Client, groupUID, repoUID, repoName string) *ListPage {
	entries := []menuEntry{
		{"tags", "Image tags in this repository", func() Page {
			return NewTagsPage(client, repoUID, repoName)
		}},
		{"cves", "CVEs in the latest image", func() Page {
			return NewImageCVEsPage(client, repoUID, repoName, "latest", "")
		}},
		{"advisories", "Advisories for the packages in the latest image", func() Page {
			return NewImageAdvisoriesPage(client, groupUID, repoUID, repoName, "latest")
		}},
		{"policy", "Policy decisions on pulls of this image", func() Page {
			return NewImagePolicyDecisionsPage(client, groupUID, repoUID, repoName)
		}},
	}
	// The menu's own context stays the org so `:` commands still scope to it.
	return newMenuPage("repo", groupUID, repoName, entries)
}

// --- Tags ---

// NewTagsPage lists a repo's tags. repoName labels the page and names the image
// on the CVE list reached with v.
func NewTagsPage(client *api.Client, repoUID, repoName string) *ListPage {
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
	// CVEs for this exact image, resolved by digest rather than by tag.
	cves := func(row RowData) tea.Cmd {
		tag, ok := row.Raw.(api.Tag)
		if !ok {
			return nil
		}
		return pushPage(NewImageCVEsPage(client, repoUID, repoName, tag.Name, tag.Digest))
	}
	return newListPage("tags", repoUID, cols, load, enter).
		WithLabel(repoName).
		WithServerNameFilter().
		WithServerSort(map[int]string{0: "name", 2: "update_time"}).
		WithRowAction("v", cves)
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

// advisoryCols, advisoryRow and advisorySortFields are shared by the org-wide
// advisory list and the per-image one, so the two read identically.
func advisoryCols() []table.Column {
	return []table.Column{
		{Title: "ID", Width: 20},
		{Title: "ARTIFACT", Width: 30},
		{Title: "STATUS", Width: 21},
		{Title: "ALIASES", Width: 40},
	}
}

func advisoryRow(v api.Advisory) RowData {
	id := v.AdvisoryID
	if id == "" {
		id = v.UID
	}
	return RowData{
		UID:     v.UID,
		Columns: []string{id, v.ArtifactName, dash(v.Status().Label()), strings.Join(v.Aliases, ", ")},
		Raw:     v,
	}
}

// advisoryOrder defaults the feed to newest-first. The API's own default is
// "uid asc", which reads as random, and no column maps to a server sort now that
// the list shows status rather than the created date.
//
// created_at is one of only two order_by fields the API accepts (the other is
// uid); it rejects everything else with InvalidArgument.
func advisoryOrder(orderBy string) string {
	if orderBy == "" {
		return "created_at desc"
	}
	return orderBy
}

// NewAdvisoriesPage lists the Chainguard advisory catalogue. groupUID is the
// page's navigation context only: advisories are global, not org-scoped.
func NewAdvisoriesPage(client *api.Client, groupUID string) *ListPage {
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListAdvisories(pageOpts(token, pageSize, query, advisoryOrder(orderBy)))
		if err != nil {
			return PageResult{}, err
		}
		res := toPageResult(page, advisoryRow)
		res.Status = "Chainguard advisory catalogue — not scoped to this org"
		return res, nil
	}
	return newListPage("advisories", groupUID, advisoryCols(), load, nil).
		WithServerFilter().
		WithPageSize(25)
}

// NewImageAdvisoriesPage lists the advisories that apply to one image: those
// whose component is one of the distro packages its SBOM names. groupUID is the
// page's navigation context only — advisories are a global catalogue.
//
// The package list costs a tag lookup plus an SBOM fetch, so it is resolved once
// and reused for every page of advisories.
func NewImageAdvisoriesPage(client *api.Client, groupUID, repoUID, repoName, tag string) *ListPage {
	var (
		mu     sync.Mutex
		cached api.ImagePackages
	)
	// resolve caches the image's package list. A failure leaves the cache empty
	// so r retries the lookup rather than sticking on the error. The lock is held
	// across the call so two overlapping loads cannot both fetch it.
	resolve := func() (api.ImagePackages, error) {
		mu.Lock()
		defer mu.Unlock()
		if len(cached.Names) > 0 {
			return cached, nil
		}
		img, err := client.ListImagePackages(repoUID, tag)
		if err != nil {
			return api.ImagePackages{}, describeImageError(err, repoName, tag)
		}
		if len(img.Names) == 0 {
			return api.ImagePackages{}, fmt.Errorf("%s names no distro packages to match advisories against",
				imageRef(repoName, tag, ""))
		}
		cached = img
		return img, nil
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		img, err := resolve()
		if err != nil {
			return PageResult{}, err
		}
		page, err := client.ListAdvisoriesFiltered(api.AdvisoryFilter{
			ComponentNames: img.Names,
			Architecture:   img.Architecture,
		}, pageOpts(token, pageSize, query, advisoryOrder(orderBy)))
		if err != nil {
			return PageResult{}, err
		}
		res := toPageResult(page, advisoryRow)
		res.Status = fmt.Sprintf("%d distro packages in %s %s (SBOM has %d)",
			len(img.Names), imageRef(repoName, img.Tag, ""), img.Architecture, img.Total)
		return res, nil
	}
	return newListPage("advisories", groupUID, advisoryCols(), load, nil).
		WithLabel(imageRef(repoName, tag, "") + " advisories").
		WithServerFilter().
		WithPageSize(25)
}

// describeImageError says what to do about a repo with no image under the tag,
// which is the common failure here.
func describeImageError(err error, repoName, tag string) error {
	if !errors.Is(err, api.ErrNoImage) {
		return err
	}
	return fmt.Errorf("%w — %s has no %s image. Open its tags to see what it does have", err, repoName, tag)
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

func dash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
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

// --- Org menu ---

// menuEntry is one row of a static resource-picker page.
type menuEntry struct {
	name, desc string
	make       func() Page
}

// newMenuPage builds a static picker page whose rows push another page on Enter.
func newMenuPage(resource, groupCtx, label string, entries []menuEntry) *ListPage {
	cols := []table.Column{
		{Title: "RESOURCE", Width: 22},
		{Title: "DESCRIPTION", Width: 50},
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
	return newListPage(resource, groupCtx, cols, load, enter).WithLabel(label)
}

// NewOrgMenuPage is what you land on after picking an org: everything the TUI
// can show for that org. Its GroupContext is the org UIDP, so `:` commands
// resolve inside the selected org.
func NewOrgMenuPage(client *api.Client, orgUID, orgName string) *ListPage {
	entries := []menuEntry{
		{"repos", "Container image repositories", func() Page {
			return NewReposPage(client, orgUID).WithLabel(orgName + " repos")
		}},
		{"charts", "Helm charts in the org's chart catalogs", func() Page {
			return NewChartsPage(client, orgUID).WithLabel(orgName + " charts")
		}},
		{"containerpolicy", "Image policies, bindings, decisions and overrides", func() Page {
			return NewImagePolicyMenuPage(client, orgUID, orgName)
		}},
		{"libraries", "Chainguard Libraries by ecosystem", func() Page {
			return NewLibrariesEcosystemPage(client, orgUID, orgName)
		}},
		{"librariespolicy", "Libraries entitlements, policies and blocks", func() Page {
			return NewLibraryPolicyMenuPage(client, orgUID, orgName)
		}},
		{"advisories", "Security advisories", func() Page {
			return NewAdvisoriesPage(client, orgUID).WithLabel(orgName + " advisories")
		}},
		{"groups", "Folders within the org", func() Page {
			return NewGroupsPage(client, orgUID).WithLabel(orgName + " folders")
		}},
		{"identities", "Workload identities", func() Page {
			return NewIdentitiesPage(client, orgUID).WithLabel(orgName + " identities")
		}},
		{"roles", "IAM roles", func() Page {
			return NewRolesPage(client, orgUID).WithLabel(orgName + " roles")
		}},
		{"rolebindings", "Role bindings", func() Page {
			return NewRoleBindingsPage(client, orgUID).WithLabel(orgName + " rolebindings")
		}},
		{"identityproviders", "Identity providers", func() Page {
			return NewIDPsPage(client, orgUID).WithLabel(orgName + " idps")
		}},
		{"groupinvites", "Group invites", func() Page {
			return NewGroupInvitesPage(client, orgUID).WithLabel(orgName + " invites")
		}},
	}
	return newMenuPage("org", orgUID, orgName, entries)
}

// --- Charts ---

// NewChartsPage lists the Helm charts in an org's chart catalog folders.
// Enter drills into the chart's tags, as for an image repo.
func NewChartsPage(client *api.Client, orgUID string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 32},
		{Title: "CATALOG", Width: 20},
		{Title: "DESCRIPTION", Width: 30},
		{Title: "UPDATED", Width: 14},
	}
	load := func(token string, pageSize int, query, orderBy string) (PageResult, error) {
		page, err := client.ListCharts(orgUID, pageOpts(token, pageSize, query, orderBy))
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.Chart) RowData {
			return RowData{
				UID:     v.UID,
				Columns: []string{v.Name, v.Catalog, truncate(v.Description, 60), relativeTime(v.UpdateTime)},
				Raw:     v,
			}
		}), nil
	}
	enter := func(row RowData) tea.Cmd {
		chart, ok := row.Raw.(api.Chart)
		if !ok {
			return nil
		}
		return pushPage(NewTagsPage(client, chart.UID, chart.Name).WithLabel(chart.Name + " tags"))
	}
	return newListPage("charts", orgUID, cols, load, enter).
		WithLabel("charts").
		WithServerNameFilter()
}

// --- Libraries ---

// libraryEcosystems are the ecosystems the artifact browser can list, in
// display order.
var libraryEcosystems = []struct {
	id, desc string
}{
	{string(api.LibraryEcosystemJava), "Maven / Java packages"},
	{string(api.LibraryEcosystemPython), "PyPI / Python packages"},
	{string(api.LibraryEcosystemJavaScript), "npm / JavaScript packages"},
}

// NewLibrariesEcosystemPage picks an ecosystem to browse and shows the org's
// policy posture for each: whether it is entitled, which sources it may pull,
// and which policy is active. Press d on a row for the full entitlement and
// binding detail.
//
// The artifact catalogue itself is global, so browsing still works when no org
// is selected — the policy columns are then blank.
func NewLibrariesEcosystemPage(client *api.Client, orgUID, orgName string) *ListPage {
	cols := []table.Column{
		{Title: "ECOSYSTEM", Width: 14},
		{Title: "ENTITLED", Width: 9},
		{Title: "ACCESS", Width: 20},
		{Title: "COOLDOWN", Width: 9},
		{Title: "POLICY", Width: 22},
		{Title: "MODE", Width: 10},
		{Title: "DESCRIPTION", Width: 26},
	}
	load := func(string, int, string, string) (PageResult, error) {
		ids := make([]string, len(libraryEcosystems))
		for i, e := range libraryEcosystems {
			ids[i] = e.id
		}
		var statuses []api.EcosystemStatus
		if orgUID != "" {
			policy, err := client.GetLibraryOrgPolicy(orgUID)
			if err != nil {
				return PageResult{}, err
			}
			statuses = policy.EcosystemStatuses(ids)
		}
		rows := make([]RowData, len(libraryEcosystems))
		for i, e := range libraryEcosystems {
			var st api.EcosystemStatus
			if i < len(statuses) {
				st = statuses[i]
			} else {
				st = api.EcosystemStatus{Ecosystem: e.id}
			}
			entitled, access, cooldown := "-", "-", "-"
			if orgUID != "" {
				entitled = "no"
			}
			if ent := st.Entitlement; ent != nil {
				entitled = "yes"
				access = ent.Access
				cooldown = cooldownDays(ent.CooldownDays)
			}
			rows[i] = RowData{
				UID: e.id,
				Columns: []string{
					e.id,
					entitled,
					access,
					cooldown,
					dash(bindingPolicyNames(st.Bindings)),
					dash(bindingModes(st.Bindings)),
					e.desc,
				},
				Raw: st,
			}
		}
		return PageResult{Rows: rows}, nil
	}
	enter := func(row RowData) tea.Cmd {
		eco := row.UID
		if eco == "" {
			return nil
		}
		return pushPage(NewLibraryArtifactsPage(client, eco).WithLabel(eco + " artifacts"))
	}
	label := "libraries"
	if orgName != "" {
		label = orgName + " libraries"
	}
	return newListPage("libraries", orgUID, cols, load, enter).WithLabel(label)
}

// inventoryFilename names an inventory export after its timestamp and ecosystem,
// e.g. 20260818T143000Z-python.json. UTC and fixed-width so names sort by time.
func inventoryFilename(ecosystem string, remediated bool, at time.Time) string {
	name := at.UTC().Format("20060102T150405Z") + "-" + ecosystem
	if remediated {
		name += "-remediated"
	}
	return name + ".json"
}

// writeLibraryInventory writes inv as JSON in the working directory and returns
// the filename. It never overwrites: a name clash is reported as an error.
func writeLibraryInventory(inv api.LibraryInventory) (string, error) {
	name := inventoryFilename(inv.Ecosystem, inv.Remediated, inv.GeneratedAt)
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return "", err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(inv); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return name, nil
}

// NewLibraryArtifactsPage lists Chainguard Libraries artifacts for one ecosystem.
func NewLibraryArtifactsPage(client *api.Client, ecosystem string) *ListPage {
	cols := []table.Column{
		{Title: "NAME", Width: 26},
		{Title: "LATEST", Width: 14},
		{Title: "VERSIONS", Width: 9},
		{Title: "LICENSE", Width: 14},
		{Title: "SOURCE", Width: 12},
		{Title: "DESCRIPTION", Width: 28},
		{Title: "CREATED", Width: 12},
		{Title: "UPDATED", Width: 12},
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
					dash(v.License),
					dash(v.SourceType),
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
		return pushPage(NewLibraryVersionsPage(client, art.UID, label, remediated).WithLabel(label + " versions"))
	}
	export := func(ctx context.Context, progress func(done, total int)) (string, error) {
		inv, err := client.BuildLibraryInventory(ctx, ecosystem, remediated, progress)
		if err != nil {
			return "", err
		}
		return writeLibraryInventory(inv)
	}
	page := newListPage("artifacts", "", cols, load, enter).
		WithLabel(ecosystem + " artifacts").
		WithServerFilter().
		WithBoolToggle("m", "remediated", &remediated).
		WithExport("x", "exporting "+ecosystem, export)
	// Java/Python support server order_by; npm v1 list does not.
	// License/source are npm-only today, so they are not in the sort map.
	if ecosystem != string(api.LibraryEcosystemJavaScript) {
		page = page.WithServerSort(map[int]string{
			0: "name",
			1: "latest_version",
			2: "version_count",
			6: "create_time",
			7: "update_time",
		})
	}
	return page
}

// NewLibraryVersionsPage lists versions for one Libraries artifact.
// Press d to describe — malware/provenance appear in JSON when the API provides them.
func NewLibraryVersionsPage(client *api.Client, artifactID, artifactName string, remediated bool) *ListPage {
	cols := []table.Column{
		{Title: "VERSION", Width: 22},
		{Title: "LICENSE", Width: 14},
		{Title: "SOURCE", Width: 12},
		{Title: "SIZE", Width: 10},
		{Title: "UPDATED", Width: 12},
	}
	load := func(token string, pageSize int, _, orderBy string) (PageResult, error) {
		page, err := client.ListArtifactVersions(artifactID, pageOpts(token, pageSize, "", orderBy), remediated)
		if err != nil {
			return PageResult{}, err
		}
		return toPageResult(page, func(v api.LibraryArtifactVersion) RowData {
			return RowData{
				UID: v.UID,
				Columns: []string{
					v.Version,
					dash(v.License),
					dash(v.SourceType),
					formatBytes(v.SizeBytes),
					relativeTime(v.UpdateTime),
				},
				Raw: v,
			}
		}), nil
	}
	return newListPage("versions", "", cols, load, nil).
		WithLabel(artifactName + " versions")
}

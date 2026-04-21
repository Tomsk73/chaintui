package api

import (
	"net/url"
	"strconv"
)

const maxPage = "200"

func scopeParams(groupUID string) url.Values {
	p := url.Values{}
	p.Set("pageSize", maxPage)
	if groupUID == "" {
		p.Set("uidp.inRoot", "true")
	} else {
		p.Set("uidp.childrenOf", groupUID)
	}
	return p
}

// ListMyOrganizations returns the root-level groups (orgs) the current user
// belongs to, using uidp.ancestorsOf scoped to the user's own subject UIDP.
// Falls back to all root groups when the subject is unavailable.
func (c *Client) ListMyOrganizations() ([]Group, error) {
	p := url.Values{"pageSize": {maxPage}}
	if sub := c.Subject(); sub != "" {
		p.Set("uidp.ancestorsOf", sub)
	} else {
		p.Set("uidp.inRoot", "true")
	}
	var resp struct {
		Items []Group `json:"items"`
	}
	if err := c.get("/iam/v2beta1/groups", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListGroups(parentUID string) ([]Group, error) {
	var resp struct {
		Items []Group `json:"items"`
	}
	if err := c.get("/iam/v2beta1/groups", scopeParams(parentUID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListIdentities(groupUID string) ([]Identity, error) {
	var resp struct {
		Items []Identity `json:"items"`
	}
	p := url.Values{"pageSize": {maxPage}}
	if groupUID != "" {
		p.Set("uidp.childrenOf", groupUID)
	}
	if err := c.get("/iam/v2beta1/identities", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListRoles(groupUID string) ([]Role, error) {
	var resp struct {
		Items []Role `json:"items"`
	}
	p := url.Values{"pageSize": {maxPage}}
	if groupUID != "" {
		p.Set("uidp.childrenOf", groupUID)
	}
	if err := c.get("/iam/v2beta1/roles", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListRoleBindings(groupUID string) ([]RoleBinding, error) {
	var resp struct {
		Items []RoleBinding `json:"items"`
	}
	p := url.Values{"pageSize": {maxPage}}
	if groupUID != "" {
		p.Set("uidp.childrenOf", groupUID)
	}
	if err := c.get("/iam/v2beta1/roleBindings", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListIdentityProviders(groupUID string) ([]IdentityProvider, error) {
	var resp struct {
		Items []IdentityProvider `json:"items"`
	}
	p := url.Values{"pageSize": {maxPage}}
	if groupUID != "" {
		p.Set("uidp.childrenOf", groupUID)
	}
	if err := c.get("/iam/v2beta1/identityProviders", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListGroupInvites(groupUID string) ([]GroupInvite, error) {
	var resp struct {
		Items []GroupInvite `json:"items"`
	}
	p := url.Values{"pageSize": {maxPage}}
	if groupUID != "" {
		p.Set("uidp.childrenOf", groupUID)
	}
	if err := c.get("/iam/v2beta1/groupInvites", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListRepos(groupUID string) ([]Repo, error) {
	var resp struct {
		Items []Repo `json:"items"`
	}
	p := url.Values{"pageSize": {maxPage}}
	if groupUID != "" {
		p.Set("uidp.childrenOf", groupUID)
	}
	if err := c.get("/registry/v2beta1/repos", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListTags(repoUID string) ([]Tag, error) {
	var resp struct {
		Items []Tag `json:"items"`
	}
	p := url.Values{
		"pageSize":          {maxPage},
		"uidp.childrenOf":   {repoUID},
	}
	if err := c.get("/registry/v2beta1/tags", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

func (c *Client) ListAdvisories(groupUID string) ([]Advisory, error) {
	var resp struct {
		Items []Advisory `json:"items"`
	}
	p := url.Values{"pageSize": {strconv.Itoa(200)}}
	if groupUID != "" {
		p.Set("uidp.childrenOf", groupUID)
	}
	if err := c.get("/vulnerabilities/v2beta1/advisories", p, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

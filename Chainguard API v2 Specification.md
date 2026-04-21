# Chainguard API v2 Specification

Source: <https://edu.chainguard.dev/chainguard/api/spec-api-v2/>

---

# Chainguard API v2 Specification

- AccountAssociationsServiceClose Group   - List Account AssociationsHTTP Method:  GET
  - Update Account AssociationHTTP Method:  PATCH
  - Create Account AssociationHTTP Method:  POST
  - Delete Account AssociationHTTP Method:  DEL
  - Get Account AssociationHTTP Method:  GET
- GroupInvitesServiceOpen Group
- GroupsServiceOpen Group
- IdentitiesServiceOpen Group
- IdentityProvidersServiceOpen Group
- RoleBindingsServiceOpen Group
- RolesServiceOpen Group
- ReposServiceOpen Group
- TagsServiceOpen Group
- AdvisoriesServiceOpen Group
- AuthOpen Group
- SecurityTokenServiceClose Group   - Exchange2HTTP Method:  GET
  - ExchangeHTTP Method:  POST
  - Exchange Refresh TokenHTTP Method:  POST
- ModelsOpen Group

[Powered by Scalar](https://www.scalar.com)v2beta1OAS 3.0.3# Chainguard API v2beta1

 Download OpenAPI Document json Download OpenAPI Document yaml Server Server:<https://console-api.enforce.dev##> AccountAssociationsService

​Copy linkAccountAssociationsService Operations- get/iam/v2beta1/accountAssociations

- patch/iam/v2beta1/accountAssociations/{accountAssociation.uid}
- post/iam/v2beta1/accountAssociations/{parent}
- delete/iam/v2beta1/accountAssociations/{uid}
- get/iam/v2beta1/accountAssociations/{uid}

### List Account Associations

​Copy linkListAccountAssociations returns account associations with pagination support.

Query Parameters- uidp.ancestorsOfCopy link to uidp.ancestorsOfType: stringancestors_of are groups reachable by repeated proceeding from child to parent.

- uidp.descendantsOfCopy link to uidp.descendantsOfType: stringdescendants_of are groups reachable by repeated proceeding from parent to child.
- uidp.childrenOfCopy link to uidp.childrenOfType: stringchildren_of are groups reachable by directly proceeding from parent to children.
- uidp.inRootCopy link to uidp.inRootType: booleanin_root resticts responses to root level resources (organizations, user identities)
- uidp.idsCopy link to uidp.idsType: array string[]ids are a list of exact UIDPs of records.
- nameCopy link to nameType: stringOptional exact name to filter by.
- pageSizeCopy link to pageSizeType: integerFormat: int32Maximum number of results to return per page.
  Default: 50, Maximum: 200.
- pageTokenCopy link to pageTokenType: stringPage token from a previous List response for pagination.
  Opaque token with 3-day expiration.
- orderByCopy link to orderByType: stringOrder results by field. Format: "field [asc|desc]"
  Default: "group asc"
  Note: Changing order_by between pages invalidates the page token.
- skipCopy link to skipType: integerFormat: int32Number of results to skip before returning.
  Used for random-access pagination (jumping to arbitrary pages).
  Must be non-negative.

Responses- 200A successful response.

application/json

- defaultAn unexpected error response.

application/json

 get/iam/v2beta1/accountAssociationsStatus: 200Status: default Show Schema ```json
{
  "accountAssociations": [
    {
      "amazon": null,
      "azure": null,
      "chainguard": null,
      "createTime": "2026-04-17T13:08:56.983Z",
      "description": "string",
      "github": null,
      "google": null,
      "name": "string",
      "uid": "string",
      "updateTime": "2026-04-17T13:08:56.983Z"
    }
  ],
  "nextPageToken": "string",
  "skipped": 1,
  "totalCount": "string"
}

```

JSONCopyJSONCopyA successful response.

### Update Account Association

​Copy linkUpdateAccountAssociation updates an account association's fields.

Path Parameters- accountAssociation.uidCopy link to accountAssociation.uidType: string`Pattern:  .+` required Unique identifier (the group UIDP this association belongs to).

Body required */*The account association to update.

- nameCopy link to nameType: string required Human-readable name of the association.
- amazonCopy link to amazonType: object · chainguard.platform.iam.v2beta1.AccountAssociation.AmazonAmazon account association.

 Show Child Attributesfor amazon
- azureCopy link to azureType: object · chainguard.platform.iam.v2beta1.AccountAssociation.AzureAzure tenant and client association.

 Show Child Attributesfor azure
- chainguardCopy link to chainguardType: object · chainguard.platform.iam.v2beta1.AccountAssociation.ChainguardChainguard service principal bindings.

 Show Child Attributesfor chainguard
- descriptionCopy link to descriptionType: stringOptional description of the association.
- githubCopy link to githubType: object · chainguard.platform.iam.v2beta1.AccountAssociation.GitHubGitHub holds GitHub App installation associations for a group.

 Show Child Attributesfor github
- googleCopy link to googleType: object · chainguard.platform.iam.v2beta1.AccountAssociation.GoogleGoogle Cloud project association.

 Show Child Attributesfor google

Responses- 200A successful response.

application/json
- defaultAn unexpected error response.

application/json

 patch/iam/v2beta1/accountAssociations/*{accountAssociation.uid}*Status: 200Status: default Show Schema ```json
{
  "amazon": {
    "account": "string"
  },
  "azure": {
    "clientIds": {
      "additionalProperty": "string"
    },
    "tenantId": "string"
  },
  "chainguard": {
    "serviceBindings": {
      "additionalProperty": "string"
    }
  },
  "createTime": "2026-04-17T13:08:56.983Z",
  "description": "string",
  "github": {
    "appInstallations": {
      "additionalProperty": null
    }
  },
  "google": {
    "projectId": "string",
    "projectNumber": "string"
  },
  "name": "string",
  "uid": "string",
  "updateTime": "2026-04-17T13:08:56.983Z"
}
```

JSONCopyJSONCopyA successful response.

### Create Account Association

​Copy linkCreateAccountAssociation creates a new account association under a parent group.

Path Parameters- parentCopy link to parentType: string`Pattern:  .+` required Parent group UIDP under which the account association will be created.

Body required */*The account association to create.

AccountAssociation represents cloud provider account associations for a group.

- nameCopy link to nameType: string required Human-readable name of the association.
- amazonCopy link to amazonType: object · chainguard.platform.iam.v2beta1.AccountAssociation.AmazonAmazon account association.

 Show Child Attributesfor amazon

- azureCopy link to azureType: object · chainguard.platform.iam.v2beta1.AccountAssociation.AzureAzure tenant and client association.

 Show Child Attributesfor azure

- chainguardCopy link to chainguardType: object · chainguard.platform.iam.v2beta1.AccountAssociation.ChainguardChainguard service principal bindings.

 Show Child Attributesfor chainguard

- descriptionCopy link to descriptionType: stringOptional description of the association.
- githubCopy link to githubType: object · chainguard.platform.iam.v2beta1.AccountAssociation.GitHubGitHub holds GitHub App installation associations for a group.

 Show Child Attributesfor github

- googleCopy link to googleType: object · chainguard.platform.iam.v2beta1.AccountAssociation.GoogleGoogle Cloud project association.

 Show Child Attributesfor google

Responses- 200A successful response.

application/json

- defaultAn unexpected error response.

application/json

 post/iam/v2beta1/accountAssociations/*{parent}*Status: 200Status: default Show Schema ```json
{
  "amazon": {
    "account": "string"
  },
  "azure": {
    "clientIds": {
      "additionalProperty": "string"
    },
    "tenantId": "string"
  },
  "chainguard": {
    "serviceBindings": {
      "additionalProperty": "string"
    }
  },
  "createTime": "2026-04-17T13:08:56.983Z",
  "description": "string",
  "github": {
    "appInstallations": {
      "additionalProperty": null
    }
  },
  "google": {
    "projectId": "string",
    "projectNumber": "string"
  },
  "name": "string",
  "uid": "string",
  "updateTime": "2026-04-17T13:08:56.983Z"
}

```

JSONCopyJSONCopyA successful response.

### Delete Account Association

​Copy linkDeleteAccountAssociation deletes an account association by UID.
  Idempotent: returns success if the account association does not exist (AIP-135).

Path Parameters- uidCopy link to uidType: string`Pattern:  .+` required UID (group UIDP) of the account association to delete.

Responses- 200A successful response.

application/json
- defaultAn unexpected error response.

application/json

 delete/iam/v2beta1/accountAssociations/*{uid}*Status: 200Status: default Show Schema ```json
{}
```

CopyCopyA successful response.

### Get Account Association

​Copy linkGetAccountAssociation retrieves a single account association by group UID.

Path Parameters- uidCopy link to uidType: string`Pattern:  .+` required UID (group UIDP) of the account association to retrieve.

Responses- 200A successful response.

application/json

- defaultAn unexpected error response.

application/json

 get/iam/v2beta1/accountAssociations/*{uid}*Status: 200Status: default Show Schema ```json
{
  "amazon": {
    "account": "string"
  },
  "azure": {
    "clientIds": {
      "additionalProperty": "string"
    },
    "tenantId": "string"
  },
  "chainguard": {
    "serviceBindings": {
      "additionalProperty": "string"
    }
  },
  "createTime": "2026-04-17T13:08:56.983Z",
  "description": "string",
  "github": {
    "appInstallations": {
      "additionalProperty": null
    }
  },
  "google": {
    "projectId": "string",
    "projectNumber": "string"
  },
  "name": "string",
  "uid": "string",
  "updateTime": "2026-04-17T13:08:56.983Z"
}

```

JSONCopyJSONCopyA successful response.

## GroupInvitesService  (Collapsed)

​Copy linkGroupInvitesService Operations- get/iam/v2beta1/groupInvites
- post/iam/v2beta1/groupInvites/{parent}
- delete/iam/v2beta1/groupInvites/{uid}
- get/iam/v2beta1/groupInvites/{uid}

 Show More ## GroupsService  (Collapsed)

​Copy linkGroupsService Operations- get/iam/v2beta1/groups
- patch/iam/v2beta1/groups/{group.uid}
- post/iam/v2beta1/groups/{parent}
- delete/iam/v2beta1/groups/{uid}
- get/iam/v2beta1/groups/{uid}

 Show More ## IdentitiesService  (Collapsed)

​Copy linkIdentitiesService Operations- get/iam/v2beta1/identities
- patch/iam/v2beta1/identities/{identity.uid}
- post/iam/v2beta1/identities/{parent}
- delete/iam/v2beta1/identities/{uid}
- get/iam/v2beta1/identities/{uid}

 Show More ## IdentityProvidersService  (Collapsed)

​Copy linkIdentityProvidersService Operations- get/iam/v2beta1/identityProviders
- patch/iam/v2beta1/identityProviders/{identityProvider.uid}
- post/iam/v2beta1/identityProviders/{parent}
- delete/iam/v2beta1/identityProviders/{uid}
- get/iam/v2beta1/identityProviders/{uid}

 Show More ## RoleBindingsService  (Collapsed)

​Copy linkRoleBindingsService Operations- get/iam/v2beta1/roleBindings
- post/iam/v2beta1/roleBindings/{parent}
- patch/iam/v2beta1/roleBindings/{roleBinding.uid}
- delete/iam/v2beta1/roleBindings/{uid}
- get/iam/v2beta1/roleBindings/{uid}

 Show More ## RolesService  (Collapsed)

​Copy linkRolesService Operations- get/iam/v2beta1/roles
- post/iam/v2beta1/roles/{parent}
- patch/iam/v2beta1/roles/{role.uid}
- delete/iam/v2beta1/roles/{uid}
- get/iam/v2beta1/roles/{uid}

 Show More ## ReposService  (Collapsed)

​Copy linkReposService Operations- get/registry/v2beta1/repos
- get/registry/v2beta1/repos/{uid}

 Show More ## TagsService  (Collapsed)

​Copy linkTagsService Operations- get/registry/v2beta1/tags
- get/registry/v2beta1/tags/{uid}

 Show More ## AdvisoriesService  (Collapsed)

​Copy linkAdvisoriesService Operations- get/vulnerabilities/v2beta1/advisories
- get/vulnerabilities/v2beta1/advisories/{uid}

 Show More ## Auth  (Collapsed)

​Copy linkAuth Operations- get/sts/headless_sessions

 Show More ## SecurityTokenService

​Copy linkSecurityTokenService Operations- get/sts/exchange
- post/sts/exchange
- post/sts/exchange_refresh_token

### Exchange2

​Copy linkQuery Parameters- audCopy link to audType: array string[]
- scopeCopy link to scopeType: stringDeprecated: use scopes instead
- identityCopy link to identityType: string
- capCopy link to capType: array string[]List of capabilities to request for the token.
- identityProviderCopy link to identityProviderType: stringEmpty or the UIDP of the custom identity provider.
- scopesCopy link to scopesType: array string[]One or more group scopes to restrict the returned token to.
  If scope and scopes are both provided, the union of their values is
  considered, after deduplication.

The returned token will include role bindings for the requested scopes,
  if any exist, that are either granted directly at that scope or inherited from
  its ancestors. That is, a role binding granted at a "lower" scope in the ancestry
  applies to all descendants of that scope.

For example, given a role binding on group `foo` with id `foo/rb-id-1`
  and a role binding on group `foo/bar` with id `foo/bar/rb-id-2`:

  - given scopes = [foo, foo/bar] => {foo: [foo/rb-id-1], foo/bar: [foo/bar/rb-id-2]}
  - given scopes = [foo/bar] => {foo/bar: [foo/rb-id-1, foo/bar/rb-id-2]}

Responses- 200A successful response.

application/json
- defaultAn unexpected error response.

application/json

 get/sts/exchangeStatus: 200Status: default Show Schema ```json
{
  "expiry": "2026-04-17T13:08:56.983Z",
  "refreshToken": "string",
  "token": "string"
}
```

JSONCopyJSONCopyA successful response.

### Exchange

​Copy linkQuery Parameters- audCopy link to audType: array string[]

- scopeCopy link to scopeType: stringDeprecated: use scopes instead
- identityCopy link to identityType: string
- capCopy link to capType: array string[]List of capabilities to request for the token.
- identityProviderCopy link to identityProviderType: stringEmpty or the UIDP of the custom identity provider.
- scopesCopy link to scopesType: array string[]One or more group scopes to restrict the returned token to.
  If scope and scopes are both provided, the union of their values is
  considered, after deduplication.

The returned token will include role bindings for the requested scopes,
  if any exist, that are either granted directly at that scope or inherited from
  its ancestors. That is, a role binding granted at a "lower" scope in the ancestry
  applies to all descendants of that scope.

For example, given a role binding on group `foo` with id `foo/rb-id-1`
  and a role binding on group `foo/bar` with id `foo/bar/rb-id-2`:

- given scopes = [foo, foo/bar] => {foo: [foo/rb-id-1], foo/bar: [foo/bar/rb-id-2]}
- given scopes = [foo/bar] => {foo/bar: [foo/rb-id-1, foo/bar/rb-id-2]}

Responses- 200A successful response.

application/json

- defaultAn unexpected error response.

application/json

 post/sts/exchangeStatus: 200Status: default Show Schema ```json
{
  "expiry": "2026-04-17T13:08:56.983Z",
  "refreshToken": "string",
  "token": "string"
}

```

JSONCopyJSONCopyA successful response.

### Exchange Refresh Token

​Copy linkQuery Parameters- audCopy link to audType: array string[]
- scopeCopy link to scopeType: stringDeprecated: use scopes instead
- capCopy link to capType: array string[]List of capabilities to request for the token.
- scopesCopy link to scopesType: array string[]One or more group scopes to restrict the returned token to.
  If scope and scopes are both provided, the union of their values is
  considered, after deduplication.

The returned token will include role bindings for the requested scopes,
  if any exist, that are either granted directly at that scope or inherited from
  its ancestors. That is, a role binding granted at a "lower" scope in the ancestry
  applies to all descendants of that scope.

For example, given a role binding on group `foo` with id `foo/rb-id-1`
  and a role binding on group `foo/bar` with id `foo/bar/rb-id-2`:

  - given scopes = [foo, foo/bar] => {foo: [foo/rb-id-1], foo/bar: [foo/bar/rb-id-2]}
  - given scopes = [foo/bar] => {foo/bar: [foo/rb-id-1, foo/bar/rb-id-2]}

Responses- 200A successful response.

application/json
- defaultAn unexpected error response.

application/json

 post/sts/exchange_refresh_tokenStatus: 200Status: default Show Schema ```json
{
  "refreshToken": {
    "expiry": "2026-04-17T13:08:56.983Z",
    "refreshToken": "string",
    "token": "string"
  },
  "token": {
    "expiry": "2026-04-17T13:08:56.983Z",
    "refreshToken": "string",
    "token": "string"
  }
}
```

JSONCopyJSONCopyA successful response.

## Models

 Show More Show sidebarSearch- AccountAssociationsServiceClose Group   - List Account AssociationsHTTP Method:  GET

- Update Account AssociationHTTP Method:  PATCH
- Create Account AssociationHTTP Method:  POST
- Delete Account AssociationHTTP Method:  DEL
- Get Account AssociationHTTP Method:  GET
- GroupInvitesServiceOpen Group
- GroupsServiceOpen Group
- IdentitiesServiceOpen Group
- IdentityProvidersServiceOpen Group
- RoleBindingsServiceOpen Group
- RolesServiceOpen Group
- ReposServiceOpen Group
- TagsServiceOpen Group
- AdvisoriesServiceOpen Group
- AuthOpen Group
- SecurityTokenServiceOpen Group

GETServer: <https://console-api.enforce.dev/iam/v2beta1/accountAssociationsCopy> URLSend Send get request to <https://console-api.enforce.dev/iam/v2beta1/accountAssociationsClose> ClientList Account AssociationsAllCookiesHeadersQueryAll## Authentication

Select Auth Type  No authentication selected ## Variables

| Enabled | Key | Value |
| --- | --- | --- |

## Cookies

| Enabled | Key | Value |
| --- | --- | --- |
|  | Key | Value |

## Headers

| Enabled | Key | Value |
| --- | --- | --- |
|  | Accept | application/json |
|  | Key | Value |

## Query Parameters

 Clear All Query Parameters| Enabled | Key | Value |
| --- | --- | --- |
|  | uidp.ancestorsOf | Value |
|  | uidp.descendantsOf | Value |
|  | uidp.childrenOf | Value |
|  | uidp.inRoot |  |
|  | uidp.ids | Value |
|  | name | Value |
|  | pageSize | Value |
|  | pageToken | Value |
|  | orderBy | Value |
|  | skip | Value |
|  | Key | Value |

## Request Body

## Code Snippet (Collapsed)

 Response AllCookiesHeadersBodyAll[Powered By Scalar.com](https://www.scalar.com)                         .,,uod8B8bou,,.                ..,uod8BBBBBBBBBBBBBBBBRPFT?l!i:.           ||||||||||||||!?TFPRBBBBBBBBBBBBBBB8m=,           ||||   '""^^!!||||||||||TFPRBBBVT!:...!           ||||            '""^^!!|||||?!:.......!           ||||                     ||||.........!           ||||                     ||||.........!           ||||                     ||||.........!           ||||                     ||||.........!           ||||                     ||||.........!           ||||                     ||||.........!           ||||,                    ||||.........`|||||!!-._               ||||.......;.           ':!|||||||||!!-._        ||||.....bBBBBWdou,.         bBBBBB86foi!|||||||!!-..:|||!..bBBBBBBBBBBBBBBY!         ::!?TFPRBBBBBB86foi!||||||||!!bBBBBBBBBBBBBBBY..!         :::::::::!?TFPRBBBBBB86ftiaabBBBBBBBBBBBBBBY....!         :::;`"^!:;::::::!?TFPRBBBBBBBBBBBBBBBBBBBY......!         ;::::::...''^::::::::::!?TFPRBBBBBBBBBBY........!     .ob86foi;::::::::::::::::::::::::!?TFPRBY..........`.b888888888886foi;:::::::::::::::::::::::..........` .b888888888888888888886foi;::::::::::::::::...........b888888888888888888888888888886foi;:::::::::......`!Tf998888888888888888888888888888888886foi;:::....`  '"^!|Tf9988888888888888888888888888888888!::..`'"^!|Tf998888888888888888888888889!! '`             '"^!|Tf9988888888888888888!!`iBBbo.                  '"^!|Tf998888888889!`             WBBBBbo.                        '"^!|Tf9989!`YBBBP^'                              '"^!`               ` Send Request ⌘Command↵Enter#### Was this helpful?

YesNoThanks for the feedback!Tell us what could be improved:0/1000SubmitCancel### Related Articles

#### [Chainguard API v1 Specification](/chainguard/api/spec-api-v1/)

#### [Chainguard OpenAPI Specification](/chainguard/api/spec/)

#### [chainctl](/chainguard/chainctl/chainctl-docs/chainctl/)

chainctl Chainguard Control

  1 min read#### [chainctl agent](/chainguard/chainctl/chainctl-docs/chainctl_agent/)

chainctl agent Agent-powered commands.

  1 min read#### [chainctl agent accept-terms](/chainguard/chainctl/chainctl-docs/chainctl_agent_accept-terms/)

chainctl agent accept-terms Accept required legal terms.

  1 min readLast updated: 2026-04-06 08:48

[Chainguard API v1 SpecificationPrev](/chainguard/api/spec-api-v1/)[Chainguard API v2 Tutorial  Next](/chainguard/api/api-v2-tutorial/)

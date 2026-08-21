package api

import (
	"strings"
	"time"
)

// ---- Enum types ----

type OrgKind string

const (
	OrgKindUnspecified OrgKind = "ORG_KIND_UNSPECIFIED"
	OrgKindStarter     OrgKind = "ORG_KIND_STARTER"
	OrgKindCustomer    OrgKind = "ORG_KIND_CUSTOMER"
	OrgKindDev         OrgKind = "ORG_KIND_DEV"
	OrgKindInfra       OrgKind = "ORG_KIND_INFRA"
)

type OrgStatus string

const (
	OrgStatusUnspecified  OrgStatus = "ORG_STATUS_UNSPECIFIED"
	OrgStatusInitializing OrgStatus = "ORG_STATUS_INITIALIZING"
	OrgStatusReady        OrgStatus = "ORG_STATUS_READY"
)

type ServicePrincipal string

const (
	ServicePrincipalUnspecified       ServicePrincipal = "SERVICE_PRINCIPAL_UNSPECIFIED"
	ServicePrincipalCosigned          ServicePrincipal = "SERVICE_PRINCIPAL_COSIGNED"
	ServicePrincipalIngester          ServicePrincipal = "SERVICE_PRINCIPAL_INGESTER"
	ServicePrincipalCatalogSyncer     ServicePrincipal = "SERVICE_PRINCIPAL_CATALOG_SYNCER"
	ServicePrincipalApkoBuilder       ServicePrincipal = "SERVICE_PRINCIPAL_APKO_BUILDER"
	ServicePrincipalEntitlementSyncer ServicePrincipal = "SERVICE_PRINCIPAL_ENTITLEMENT_SYNCER"
	ServicePrincipalTenantScanner     ServicePrincipal = "SERVICE_PRINCIPAL_TENANT_SCANNER"
	ServicePrincipalSedimentology     ServicePrincipal = "SERVICE_PRINCIPAL_SEDIMENTOLOGY"
	ServicePrincipalSkillup           ServicePrincipal = "SERVICE_PRINCIPAL_SKILLUP"
	ServicePrincipalMaterializer      ServicePrincipal = "SERVICE_PRINCIPAL_MATERIALIZER"
)

type CatalogTier string

const (
	CatalogTierUnspecified CatalogTier = "CATALOG_TIER_UNSPECIFIED"
	CatalogTierApplication CatalogTier = "CATALOG_TIER_APPLICATION"
	CatalogTierBase        CatalogTier = "CATALOG_TIER_BASE"
	CatalogTierFIPS        CatalogTier = "CATALOG_TIER_FIPS"
	CatalogTierAI          CatalogTier = "CATALOG_TIER_AI"
	CatalogTierDevtools    CatalogTier = "CATALOG_TIER_DEVTOOLS"
	CatalogTierCommercial  CatalogTier = "CATALOG_TIER_COMMERCIAL"
)

type ReviewState string

const (
	ReviewStateUnspecified    ReviewState = "REVIEW_STATE_UNSPECIFIED"
	ReviewStatePending        ReviewState = "REVIEW_STATE_PENDING"
	ReviewStateApproved       ReviewState = "REVIEW_STATE_APPROVED"
	ReviewStateRequestChanges ReviewState = "REVIEW_STATE_REQUEST_CHANGES"
	ReviewStateRejected       ReviewState = "REVIEW_STATE_REJECTED"
)

// ---- IAM types ----

type Group struct {
	UID            string           `json:"uid"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	Kind           OrgKind          `json:"kind,omitempty"`
	Status         OrgStatus        `json:"status,omitempty"`
	Verified       bool             `json:"verified,omitempty"`
	ResourceLimits map[string]int32 `json:"resourceLimits,omitempty"`
	CreateTime     time.Time        `json:"createTime"`
	UpdateTime     time.Time        `json:"updateTime"`
}

type Identity struct {
	UID              string               `json:"uid"`
	Name             string               `json:"name"`
	Description      string               `json:"description"`
	Company          string               `json:"company,omitempty"`
	Email            string               `json:"email,omitempty"`
	EmailUnverified  string               `json:"emailUnverified,omitempty"`
	Providers        []string             `json:"providers,omitempty"`
	ServicePrincipal ServicePrincipal     `json:"servicePrincipal,omitempty"`
	ClaimMatch       *IdentityClaimMatch  `json:"claimMatch,omitempty"`
	AWSIdentity      *IdentityAWSIdentity `json:"awsIdentity,omitempty"`
	StaticKeys       *IdentityStaticKeys  `json:"staticKeys,omitempty"`
	LastSeenTime     time.Time            `json:"lastSeenTime,omitempty"`
	CreateTime       time.Time            `json:"createTime"`
	UpdateTime       time.Time            `json:"updateTime"`
}

type IdentityClaimMatch struct {
	Issuer          string            `json:"issuer,omitempty"`
	IssuerPattern   string            `json:"issuerPattern,omitempty"`
	Subject         string            `json:"subject,omitempty"`
	SubjectPattern  string            `json:"subjectPattern,omitempty"`
	Audience        string            `json:"audience,omitempty"`
	AudiencePattern string            `json:"audiencePattern,omitempty"`
	Claims          map[string]string `json:"claims,omitempty"`
	ClaimPatterns   map[string]string `json:"claimPatterns,omitempty"`
}

type IdentityAWSIdentity struct {
	AWSAccount    string `json:"awsAccount"`
	ARN           string `json:"arn,omitempty"`
	ARNPattern    string `json:"arnPattern,omitempty"`
	UserID        string `json:"userId,omitempty"`
	UserIDPattern string `json:"userIdPattern,omitempty"`
}

type IdentityStaticKeys struct {
	Issuer         string    `json:"issuer"`
	Subject        string    `json:"subject"`
	IssuerKeys     string    `json:"issuerKeys"`
	ExpirationTime time.Time `json:"expirationTime,omitempty"`
}

type Role struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Managed is true for Chainguard's built-in roles, which live outside the
	// group hierarchy and are bindable in any org.
	Managed      bool      `json:"managed"`
	Capabilities []string  `json:"capabilities"`
	CreateTime   time.Time `json:"createTime"`
	UpdateTime   time.Time `json:"updateTime"`
}

type RoleBinding struct {
	UID         string               `json:"uid"`
	IdentityUID string               `json:"identityUid,omitempty"`
	RoleUID     string               `json:"roleUid,omitempty"`
	Identity    *RoleBindingIdentity `json:"identity,omitempty"`
	Role        *RoleBindingRole     `json:"role,omitempty"`
	Group       *RoleBindingGroup    `json:"group,omitempty"`
	CreateTime  time.Time            `json:"createTime"`
}

type RoleBindingIdentity struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Email       string `json:"email,omitempty"`
	Issuer      string `json:"issuer,omitempty"`
	Subject     string `json:"subject,omitempty"`
}

type RoleBindingRole struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type RoleBindingGroup struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type IdentityProvider struct {
	UID         string                `json:"uid"`
	Name        string                `json:"name"`
	Description string                `json:"description"`
	DefaultRole string                `json:"defaultRole,omitempty"`
	OIDC        *IdentityProviderOIDC `json:"oidc,omitempty"`
	CreateTime  time.Time             `json:"createTime"`
	UpdateTime  time.Time             `json:"updateTime"`
}

type IdentityProviderOIDC struct {
	Issuer           string   `json:"issuer"`
	ClientID         string   `json:"clientId"`
	ClientSecret     string   `json:"clientSecret,omitempty"`
	AdditionalScopes []string `json:"additionalScopes,omitempty"`
}

type GroupInvite struct {
	UID            string    `json:"uid"`
	Code           string    `json:"code,omitempty"`
	Email          string    `json:"email,omitempty"`
	RoleUID        string    `json:"roleUid"`
	SingleUse      bool      `json:"singleUse,omitempty"`
	TTL            string    `json:"ttl,omitempty"`
	KeyID          string    `json:"keyId,omitempty"`
	CreateTime     time.Time `json:"createTime"`
	ExpirationTime time.Time `json:"expirationTime"`
}

type AccountAssociation struct {
	UID         string                        `json:"uid"`
	Name        string                        `json:"name"`
	Description string                        `json:"description,omitempty"`
	Amazon      *AccountAssociationAmazon     `json:"amazon,omitempty"`
	Azure       *AccountAssociationAzure      `json:"azure,omitempty"`
	Google      *AccountAssociationGoogle     `json:"google,omitempty"`
	GitHub      *AccountAssociationGitHub     `json:"github,omitempty"`
	Chainguard  *AccountAssociationChainguard `json:"chainguard,omitempty"`
	CreateTime  time.Time                     `json:"createTime"`
	UpdateTime  time.Time                     `json:"updateTime"`
}

type AccountAssociationAmazon struct {
	Account string `json:"account"`
}

type AccountAssociationAzure struct {
	TenantID  string            `json:"tenantId"`
	ClientIDs map[string]string `json:"clientIds"`
}

type AccountAssociationGoogle struct {
	ProjectID     string `json:"projectId"`
	ProjectNumber string `json:"projectNumber"`
}

type AccountAssociationGitHub struct {
	AppInstallations map[string]AccountAssociationGitHubAppInstallations `json:"appInstallations,omitempty"`
}

type AccountAssociationGitHubAppInstallations struct {
	Installations []AccountAssociationGitHubInstallation `json:"installations"`
}

type AccountAssociationGitHubInstallation struct {
	InstallationID string `json:"installationId"`
	Name           string `json:"name,omitempty"`
}

type AccountAssociationChainguard struct {
	ServiceBindings map[string]string `json:"serviceBindings"`
}

// ---- Registry types ----

type Repo struct {
	UID           string         `json:"uid"`
	Name          string         `json:"name"`
	Description   string         `json:"description,omitempty"`
	Aliases       []string       `json:"aliases,omitempty"`
	Bundles       []string       `json:"bundles,omitempty"`
	ActiveTags    []string       `json:"activeTags,omitempty"`
	CatalogTier   CatalogTier    `json:"catalogTier,omitempty"`
	Readme        string         `json:"readme,omitempty"`
	SyncConfig    *SyncConfig    `json:"sync_config,omitempty"`
	CustomOverlay *CustomOverlay `json:"customOverlay,omitempty"`
	CreateTime    time.Time      `json:"createTime"`
	UpdateTime    time.Time      `json:"updateTime"`
}

type SyncConfig struct {
	Source         string    `json:"source,omitempty"`
	Amazon         string    `json:"amazon,omitempty"`
	Azure          string    `json:"azure,omitempty"`
	Google         string    `json:"google,omitempty"`
	ApkoOverlay    string    `json:"apkoOverlay,omitempty"`
	UniqueTags     bool      `json:"uniqueTags,omitempty"`
	GracePeriod    bool      `json:"gracePeriod,omitempty"`
	ExpirationTime time.Time `json:"expirationTime,omitempty"`
}

type CustomOverlay struct {
	Environment  map[string]string        `json:"environment,omitempty"`
	Annotations  map[string]string        `json:"annotations,omitempty"`
	Contents     *CustomOverlayContents   `json:"contents,omitempty"`
	Accounts     *CustomOverlayAccounts   `json:"accounts,omitempty"`
	Certificates *CustomOverlayCerts      `json:"certificates,omitempty"`
}

type CustomOverlayContents struct {
	Packages []string `json:"packages,omitempty"`
}

type CustomOverlayAccounts struct {
	RunAs  string                      `json:"runAs,omitempty"`
	Users  []CustomOverlayAccountUser  `json:"users,omitempty"`
	Groups []CustomOverlayAccountGroup `json:"groups,omitempty"`
}

type CustomOverlayAccountUser struct {
	UID       int32  `json:"uid,omitempty"`
	GID       int32  `json:"gid,omitempty"`
	Username  string `json:"username,omitempty"`
	GroupName string `json:"groupName,omitempty"`
	HomeDir   string `json:"homeDir,omitempty"`
	Shell     string `json:"shell,omitempty"`
}

type CustomOverlayAccountGroup struct {
	GID       int32    `json:"gid,omitempty"`
	Groupname string   `json:"groupname,omitempty"`
	Members   []string `json:"members,omitempty"`
}

type CustomOverlayCerts struct {
	Providers  []string               `json:"providers,omitempty"`
	Additional []CustomOverlayCertEntry `json:"additional,omitempty"`
}

type CustomOverlayCertEntry struct {
	Name    string `json:"name,omitempty"`
	Content string `json:"content,omitempty"`
}

type Tag struct {
	UID        string    `json:"uid"`
	Name       string    `json:"name"`
	Digest     string    `json:"digest"`
	Deprecated bool      `json:"deprecated,omitempty"`
	Bundles    []string  `json:"bundles,omitempty"`
	UpdateTime time.Time `json:"updateTime"`
}

// ---- Vulnerability types ----

type Advisory struct {
	UID                  string          `json:"uid"`
	AdvisoryID           string          `json:"advisoryId,omitempty"`
	LegacyAdvisoryID     string          `json:"legacyAdvisoryId,omitempty"`
	Aliases              []string        `json:"aliases,omitempty"`
	ArtifactName         string          `json:"artifactName"`
	ArtifactType         string          `json:"artifactType"`
	ArtifactArchitecture string          `json:"artifactArchitecture"`
	ComponentName        string          `json:"componentName"`
	ComponentLocation    string          `json:"componentLocation"`
	ComponentType        string          `json:"componentType"`
	Author               string          `json:"author"`
	Events               []AdvisoryEvent `json:"events,omitempty"`
	CreateTime           time.Time       `json:"createTime"`
	UpdateTime           time.Time       `json:"updateTime"`
	DeleteTime           *time.Time      `json:"deleteTime,omitempty"`
}

// AdvisoryEventType names the kind of an advisory event — the oneof arm the
// API set on it.
// Status is where the advisory currently stands: the kind of its most recent
// approved event. Pending and rejected events are ignored — an advisory whose
// false-positive claim was rejected is not a false positive — which matches how
// the API's own latest_event_type filter classifies records.
//
// Returns "" when there is nothing to go on, e.g. a record whose events the
// caller did not fetch.
func (a Advisory) Status() AdvisoryEventType {
	var latest AdvisoryEvent
	var found bool
	for _, e := range a.Events {
		if e.ReviewState != ReviewStateApproved || e.Type == "" {
			continue
		}
		// Events arrive oldest-first, but do not rely on it.
		if !found || e.CreateTime.After(latest.CreateTime) || e.CreateTime.Equal(latest.CreateTime) {
			latest, found = e, true
		}
	}
	return latest.Type
}

// AdvisoryEventType names the kind of an advisory event — the oneof arm the
// API set on it.
type AdvisoryEventType string

const (
	AdvisoryEventTypeDetection          AdvisoryEventType = "detection"
	AdvisoryEventTypeTruePositive       AdvisoryEventType = "true_positive"
	AdvisoryEventTypeFalsePositive      AdvisoryEventType = "false_positive"
	AdvisoryEventTypeFixed              AdvisoryEventType = "fixed"
	AdvisoryEventTypePatched            AdvisoryEventType = "patched"
	AdvisoryEventTypeFixNotPlanned      AdvisoryEventType = "fix_not_planned"
	AdvisoryEventTypeAnalysisNotPlanned AdvisoryEventType = "analysis_not_planned"
	AdvisoryEventTypePendingUpstreamFix AdvisoryEventType = "pending_upstream_fix"
)

// Label is how an event type reads in the UI.
func (t AdvisoryEventType) Label() string {
	switch t {
	case AdvisoryEventTypeDetection:
		// A detection nobody has ruled on yet is still being triaged.
		return "Under Investigation"
	case AdvisoryEventTypeTruePositive:
		return "True Positive"
	case AdvisoryEventTypeFalsePositive:
		return "False Positive"
	case AdvisoryEventTypeFixed:
		return "Fixed"
	case AdvisoryEventTypePatched:
		return "Patched"
	case AdvisoryEventTypeFixNotPlanned:
		return "Fix Not Planned"
	case AdvisoryEventTypeAnalysisNotPlanned:
		return "Analysis Not Planned"
	case AdvisoryEventTypePendingUpstreamFix:
		return "Pending Upstream Fix"
	default:
		return ""
	}
}

type AdvisoryEvent struct {
	UID string `json:"uid"`
	// Type is the kind of event, mirroring which payload field below is set.
	Type                       AdvisoryEventType                `json:"type"`
	Author                     string                           `json:"author"`
	Reviewer                   string                           `json:"reviewer,omitempty"`
	ReviewState                ReviewState                      `json:"reviewState"`
	Issue                      string                           `json:"issue,omitempty"`
	Findings                   []byte                           `json:"findings,omitempty"`
	Detection                  *AdvisoryEventDetection          `json:"detection,omitempty"`
	TruePositiveDetermination  *AdvisoryEventTruePositive       `json:"truePositiveDetermination,omitempty"`
	FalsePositiveDetermination *AdvisoryEventFalsePositive      `json:"falsePositiveDetermination,omitempty"`
	Fixed                      *AdvisoryEventFixed              `json:"fixed,omitempty"`
	Patched                    *AdvisoryEventPatched            `json:"patched,omitempty"`
	FixNotPlanned              *AdvisoryEventFixNotPlanned      `json:"fixNotPlanned,omitempty"`
	AnalysisNotPlanned         *AdvisoryEventAnalysisNotPlanned `json:"analysisNotPlanned,omitempty"`
	PendingUpstreamFix         *AdvisoryEventPendingUpstreamFix `json:"pendingUpstreamFix,omitempty"`
	CreateTime                 time.Time                        `json:"createTime"`
}

type AdvisoryEventDetection struct {
	ScanV1 *AdvisoryEventDetectionScanV1 `json:"scanv1,omitempty"`
	NVDAPI *AdvisoryEventDetectionNVDAPI `json:"nvdapi,omitempty"`
	Manual *AdvisoryEventDetectionManual `json:"manual,omitempty"`
}

type AdvisoryEventDetectionScanV1 struct {
	Scanner           string `json:"scanner,omitempty"`
	Subpackage        string `json:"subpackage,omitempty"`
	Component         string `json:"component,omitempty"`
	ComponentID       string `json:"componentId,omitempty"`
	ComponentVersion  string `json:"componentVersion,omitempty"`
	ComponentType     string `json:"componentType,omitempty"`
	ComponentLocation string `json:"componentLocation,omitempty"`
}

type AdvisoryEventDetectionNVDAPI struct {
	CPESearched string `json:"cpeSearched,omitempty"`
	CPEFound    string `json:"cpeFound,omitempty"`
}

type AdvisoryEventDetectionManual struct{}

type AdvisoryEventTruePositive struct {
	Note string `json:"note,omitempty"`
}

type AdvisoryEventFalsePositive struct {
	Type string `json:"type,omitempty"`
	Note string `json:"note,omitempty"`
}

type AdvisoryEventFixed struct {
	FixedVersion string `json:"fixedVersion,omitempty"`
	Note         string `json:"note,omitempty"`
}

type AdvisoryEventPatched struct {
	PatchedVersions []string `json:"patchedVersions,omitempty"`
	Note            string   `json:"note,omitempty"`
}

type AdvisoryEventFixNotPlanned struct {
	Note string `json:"note,omitempty"`
}

type AdvisoryEventAnalysisNotPlanned struct {
	Note string `json:"note,omitempty"`
}

type AdvisoryEventPendingUpstreamFix struct {
	Note string `json:"note,omitempty"`
}

// ---- SBOM types ----

type SBOMPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Purl    string `json:"purl,omitempty"`
	License string `json:"license,omitempty"`
}

// ---- Libraries types ----

type LibraryArtifact struct {
	UID           string    `json:"uid"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Ecosystem     string    `json:"ecosystem"`
	LatestVersion string    `json:"latestVersion,omitempty"`
	VersionCount  int32     `json:"versionCount,omitempty"`
	License       string    `json:"license,omitempty"`
	SourceType    string    `json:"sourceType,omitempty"`
	CreateTime    time.Time `json:"createTime"`
	UpdateTime    time.Time `json:"updateTime"`
}

// ---- Image policy types ----

// ImagePolicy is a rule container images are evaluated against at pull time.
// Type is "system" (Chainguard-authored, read-only) or "custom" (org-managed).
type ImagePolicy struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	// ResourceType is the resource the policy evaluates, e.g.
	// "registry.chainguard.dev/Repo@v1".
	ResourceType string `json:"resourceType,omitempty"`
	// Expression is the policy body.
	Expression string `json:"expression,omitempty"`
	// Parameters are the knobs a binding may set when it activates the policy.
	Parameters []ImagePolicyParameter `json:"parameters,omitempty"`
	CreateTime time.Time              `json:"createTime"`
	UpdateTime time.Time              `json:"updateTime"`
}

// ImagePolicyParameter is one configurable knob declared by a policy.
type ImagePolicyParameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// ImagePolicyBinding activates a policy over everything beneath its parent.
type ImagePolicyBinding struct {
	UID string `json:"uid"`
	// PolicyUID is the bound policy; PolicyName is filled in by the caller from
	// the policy list, since the API returns only the id.
	PolicyUID  string `json:"policyUid"`
	PolicyName string `json:"policyName,omitempty"`
	// Mode is "enforced" (block on violation) or "dry-run" (log only).
	Mode          string            `json:"mode"`
	ResourceTypes []string          `json:"resourceTypes,omitempty"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	CreateTime    time.Time         `json:"createTime"`
	UpdateTime    time.Time         `json:"updateTime"`
}

// Scope is the UIDP the binding is attached to: it applies to that resource and
// everything under it. The binding's own id is a child of that scope.
func (b ImagePolicyBinding) Scope() string {
	if i := strings.LastIndex(b.UID, "/"); i > 0 {
		return b.UID[:i]
	}
	return b.UID
}

// ImagePolicyDecision is the outcome of evaluating one policy against one image
// digest at pull time.
type ImagePolicyDecision struct {
	UID        string `json:"uid"`
	RepoUID    string `json:"repoUid"`
	Digest     string `json:"digest"`
	PolicyUID  string `json:"policyUid"`
	PolicyName string `json:"policyName,omitempty"`
	Mode       string `json:"mode"`
	// Result is "allowed", "denied" or "error".
	Result string `json:"result"`
	Reason string `json:"reason,omitempty"`
	// PulledOn is the day the digest was pulled and evaluated.
	PulledOn time.Time `json:"pulledOn"`
}

// ImagePolicyOverride is an admin's waiver of a policy decision for one digest.
// It does not change the policy; it records a deliberate exception.
type ImagePolicyOverride struct {
	UID        string    `json:"uid"`
	PolicyUID  string    `json:"policyUid"`
	PolicyName string    `json:"policyName,omitempty"`
	Digest     string    `json:"digest"`
	Reason     string    `json:"reason,omitempty"`
	CreatedBy  string    `json:"createdBy,omitempty"`
	CreateTime time.Time `json:"createTime"`
}

// ---- Libraries policy types (org-scoped) ----

// LibraryEntitlement is an org's grant to pull one library ecosystem.
type LibraryEntitlement struct {
	UID       string `json:"uid"`
	Ecosystem string `json:"ecosystem"`
	// Access is which sources the org may pull: "chainguard" (internal builds
	// only) or "chainguard+upstream".
	Access string `json:"access"`
	Source string `json:"source,omitempty"` // trial | sfdc
	// CooldownDays is the entitlement's cooldown; 0 means no cooldown enforced.
	CooldownDays int32 `json:"cooldownDays"`
}

// LibraryPolicy is a Libraries gate configuration (cooldown, malware, licences,
// block/allow lists) available to an org. Type is "system" (Chainguard-authored,
// read-only) or "custom" (org-managed).
type LibraryPolicy struct {
	UID         string `json:"uid"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"`
	// CooldownDays is nil when the policy inherits the default (7 days); 0 means
	// the cooldown gate is disabled.
	CooldownDays    *int32              `json:"cooldownDays,omitempty"`
	BlockList       []string            `json:"blockList,omitempty"`
	AllowList       []LibraryAllowEntry `json:"allowList,omitempty"`
	BlockedLicenses []string            `json:"blockedLicenses,omitempty"`
	// Expression is the raw Rego of a system policy; empty for custom policies.
	Expression string    `json:"expression,omitempty"`
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

// LibraryAllowEntry exempts a PURL from specific gates.
type LibraryAllowEntry struct {
	Purl           string `json:"purl"`
	BypassCooldown bool   `json:"bypassCooldown,omitempty"`
	BypassMalware  bool   `json:"bypassMalware,omitempty"`
	Justification  string `json:"justification,omitempty"`
}

// LibraryPolicyBinding activates a policy for one (org, ecosystem) pair.
type LibraryPolicyBinding struct {
	UID       string `json:"uid"`
	PolicyUID string `json:"policyUid"`
	// PolicyName is resolved from the org's policy list; empty if the policy is
	// not visible to the caller.
	PolicyName string    `json:"policyName,omitempty"`
	Ecosystem  string    `json:"ecosystem"`
	Mode       string    `json:"mode"` // enforced | log
	CreateTime time.Time `json:"createTime"`
	UpdateTime time.Time `json:"updateTime"`
}

// LibraryBlockEvent records a package version that policy withheld from an org.
type LibraryBlockEvent struct {
	UID       string `json:"uid"`
	Purl      string `json:"purl"`
	Package   string `json:"package"`
	Version   string `json:"version,omitempty"`
	Ecosystem string `json:"ecosystem"`
	Mode      string `json:"mode"`
	Reason    string `json:"reason"` // cooldown | malware | policy
	PolicyUID string `json:"policyUid,omitempty"`
	// CooldownDays and UnblocksAt are set when Reason is cooldown.
	CooldownDays   int32     `json:"cooldownDays,omitempty"`
	PublishDate    time.Time `json:"publishDate,omitempty"`
	UnblocksAt     time.Time `json:"unblocksAt,omitempty"`
	FirstBlockedAt time.Time `json:"firstBlockedAt,omitempty"`
	LastBlockedAt  time.Time `json:"lastBlockedAt,omitempty"`
	AttemptCount   int32     `json:"attemptCount"`
}

// LibraryOrgPolicy is an org's whole Libraries posture: what it may pull, which
// policies exist, and which are active.
type LibraryOrgPolicy struct {
	Entitlements []LibraryEntitlement   `json:"entitlements"`
	Policies     []LibraryPolicy        `json:"policies"`
	Bindings     []LibraryPolicyBinding `json:"bindings"`
}

// EcosystemStatus is the org's posture for a single ecosystem, as shown on the
// libraries ecosystem picker.
type EcosystemStatus struct {
	Ecosystem   string                 `json:"ecosystem"`
	Entitlement *LibraryEntitlement    `json:"entitlement,omitempty"` // nil = not entitled
	Bindings    []LibraryPolicyBinding `json:"bindings,omitempty"`
}

// ---- Chart types ----

// Chart is a Helm chart repo in one of an org's chart catalog folders.
type Chart struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
	// Catalog is the chart folder the chart lives in (e.g. "charts",
	// "iamguarded-charts").
	Catalog     string    `json:"catalog"`
	Description string    `json:"description,omitempty"`
	CreateTime  time.Time `json:"createTime"`
	UpdateTime  time.Time `json:"updateTime"`
}

type LibraryArtifactVersion struct {
	UID              string    `json:"uid"`
	Name             string    `json:"name"`
	Version          string    `json:"version"`
	Description      string    `json:"description,omitempty"`
	License          string    `json:"license,omitempty"`
	SourceType       string    `json:"sourceType,omitempty"`
	SizeBytes        int64     `json:"sizeBytes,omitempty"`
	Provenance       string    `json:"provenance,omitempty"`
	MalwareScanned   bool      `json:"malwareScanned"`
	MalwareMalicious bool      `json:"malwareMalicious"`
	MalwareScannedAt time.Time `json:"malwareScannedAt,omitempty"`
	CreateTime       time.Time `json:"createTime"`
	UpdateTime       time.Time `json:"updateTime"`
}

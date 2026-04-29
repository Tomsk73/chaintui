package api

import "time"

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
	UID          string    `json:"uid"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
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

type AdvisoryEvent struct {
	UID                        string                           `json:"uid"`
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

package cookiemunch

// This file mirrors the request/response shapes of the Cookie Munch Developer
// API. Where the published TypeScript SDK's aspirational types differ from the
// server's authoritative OpenAPI schema, the wire shape (OpenAPI) wins so these
// structs decode real responses; such cases are noted inline.

// Identity is the response of GET /v1/me.
type Identity struct {
	OrgID     string `json:"orgId"`
	Plan      string `json:"plan"`
	KeyPrefix string `json:"keyPrefix"`
}

// Site is a registered property.
type Site struct {
	CBID         string `json:"cbid"`
	OrgID        string `json:"orgId"`
	Domain       string `json:"domain"`
	Verified     bool   `json:"verified"`
	VerifyToken  string `json:"verifyToken,omitempty"`
	VerifyMethod string `json:"verifyMethod,omitempty"` // dns | meta | file | embed
	VerifiedAt   int64  `json:"verifiedAt,omitempty"`
}

// SiteCreate is the body of POST /v1/sites. CBID is optional; the server
// auto-generates one when omitted.
type SiteCreate struct {
	Domain string `json:"domain"`
	CBID   string `json:"cbid,omitempty"`
}

// SiteConfig is the open, deeply-nested banner/blocking config owned by
// @cookiemunch/core. It is left untyped.
type SiteConfig map[string]any

// SnippetOptions are the query parameters for the install-snippet endpoint.
type SnippetOptions struct {
	// BlockingMode overrides the mode: "auto" | "manual" | "checklist".
	BlockingMode string
	// Culture is a language override, e.g. "en" or "fr".
	Culture string
}

// InstallSnippet is the response of GET /v1/sites/{cbid}/snippet.
type InstallSnippet struct {
	Snippet      string `json:"snippet"`
	Src          string `json:"src"`
	API          string `json:"api,omitempty"`
	CBID         string `json:"cbid"`
	BlockingMode string `json:"blockingMode"`
}

// VerifyResult is the response of POST /v1/sites/{cbid}/verify.
type VerifyResult struct {
	Verified bool   `json:"verified"`
	Method   string `json:"method,omitempty"` // dns | meta | file
	Reason   string `json:"reason,omitempty"`
}

// BrandSuggestion holds theme tokens extracted from a site homepage.
type BrandSuggestion struct {
	Background string   `json:"background,omitempty"`
	Text       string   `json:"text,omitempty"`
	Highlight  string   `json:"highlight,omitempty"`
	FontFamily string   `json:"fontFamily,omitempty"`
	FontURL    string   `json:"fontUrl,omitempty"`
	Palette    []string `json:"palette,omitempty"`
}

// BrandExtractionResult is the response of POST /v1/sites/{cbid}/brand.
type BrandExtractionResult struct {
	Suggestion BrandSuggestion `json:"suggestion"`
}

// CategorizedCookie is one classified cookie in a scan snapshot.
type CategorizedCookie struct {
	Name     string `json:"name"`
	Domain   string `json:"domain,omitempty"`
	Category string `json:"category"` // necessary | preferences | statistics | marketing | unclassified
	Purpose  string `json:"purpose,omitempty"`
	Provider string `json:"provider,omitempty"`
	Expiry   string `json:"expiry,omitempty"`
}

// CookieDeclaration is the response of GET /v1/sites/{cbid}/cookies.
type CookieDeclaration struct {
	UpdatedAt int64               `json:"updatedAt"`
	Cookies   []CategorizedCookie `json:"cookies"`
}

// ScanStatus is the response of the cookie-scan endpoints. Note: the wire shape
// (per OpenAPI) is { status, lastScannedAt }, not the richer ScanResult in the
// TypeScript SDK's types.
type ScanStatus struct {
	Status        string `json:"status"`        // idle | scanning
	LastScannedAt *int64 `json:"lastScannedAt"` // epoch-ms of last scan, or null
}

// AbResult is one A/B banner-experiment variant's results. Per the OpenAPI
// schema the wire fields are { variant, impressions, optIns, optInRate }.
type AbResult struct {
	Variant     string  `json:"variant"`
	Impressions int     `json:"impressions"`
	OptIns      int     `json:"optIns"`
	OptInRate   float64 `json:"optInRate"` // opt-in rate as a percentage
}

// FlowIssue is a flow validation/lint finding.
type FlowIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// SiteFlow is the v2 banner flow returned by GET /v1/sites/{cbid}/flow.
type SiteFlow struct {
	V          int            `json:"v"`
	Flow       map[string]any `json:"flow"`
	Categories map[string]any `json:"categories"`
	CustomCSS  string         `json:"customCss,omitempty"`
	Lint       []FlowIssue    `json:"lint,omitempty"`
}

// FlowOp is a single structured flow edit operation. The "op" field selects the
// kind; remaining args go in Args.
type FlowOp struct {
	Op   string
	Args map[string]any
}

// FlowWriteResult is the result of a flow edit/replace. On lint/validation
// failure OK is false and Issues explains why (HTTP is still 200 — always check
// OK).
type FlowWriteResult struct {
	OK       bool           `json:"ok"`
	Flow     map[string]any `json:"flow,omitempty"`
	FailedAt *int           `json:"failedAt,omitempty"`
	Op       any            `json:"op,omitempty"`
	Issues   []FlowIssue    `json:"issues,omitempty"`
}

// AdPersonalizationInput is the body of POST
// /v1/sites/{cbid}/elements/ad-personalization.
type AdPersonalizationInput struct {
	Enabled *bool  `json:"enabled,omitempty"`
	Default *bool  `json:"default,omitempty"`
	Label   string `json:"label,omitempty"`
}

// AdPersonalizationResult reports what the ad-personalization element endpoint did.
type AdPersonalizationResult struct {
	OK              bool   `json:"ok"`
	Form            string `json:"form"` // inline | flow
	Enabled         bool   `json:"enabled"`
	InjectedElement bool   `json:"injectedElement,omitempty"`
	Note            string `json:"note,omitempty"`
}

// ---- consent ----------------------------------------------------------------

// ConsentChoices is the standard cookie-category consent triple.
type ConsentChoices struct {
	Preferences bool `json:"preferences"`
	Statistics  bool `json:"statistics"`
	Marketing   bool `json:"marketing"`
}

// ConsentIngest is the payload of the public POST /api/v1/consent endpoint (the
// consent-log write the embed makes). No auth header is required — the cbid must
// be a registered site.
type ConsentIngest struct {
	CBID              string            `json:"cbid"`
	Stamp             string            `json:"stamp"`
	Choices           ConsentChoices    `json:"choices"`
	Method            string            `json:"method"` // "explicit" | "implied"
	Ver               int               `json:"ver"`
	UTC               int64             `json:"utc"`
	URL               string            `json:"url"`
	TCString          string            `json:"tcString,omitempty"`
	GPPString         string            `json:"gppString,omitempty"`
	Purposes          map[string]bool   `json:"purposes,omitempty"`
	SubjectPolicyHash string            `json:"subjectPolicyHash,omitempty"`
	PurposeRopa       map[string]string `json:"purposeRopa,omitempty"`
	Variant           string            `json:"variant,omitempty"`
	// SubjectID is an optional, app-supplied STABLE cross-surface subject id (e.g. a
	// logged-in account id). It lets an org correlate one subject's consent across all
	// its sites/surfaces. Opaque — stored and hashed server-side, never interpreted.
	SubjectID string `json:"subjectId,omitempty"`
}

// ConsentDay is one day of aggregated consent stats.
type ConsentDay struct {
	Date          string         `json:"Date"`
	OptIn         int            `json:"OptIn"`
	OptOut        int            `json:"OptOut"`
	OptInImplied  int            `json:"OptInImplied"`
	OptInStrict   int            `json:"OptInStrict"`
	TypeOptInPref int            `json:"TypeOptInPref"`
	TypeOptInStat int            `json:"TypeOptInStat"`
	TypeOptInMark int            `json:"TypeOptInMark"`
	Impressions   int            `json:"Impressions"`
	Countries     map[string]int `json:"Countries"`
}

// ConsentLogRow is one anonymised consent record from the log endpoint.
type ConsentLogRow struct {
	Stamp      string         `json:"stamp"`
	ReceivedAt int64          `json:"receivedAt"`
	Region     string         `json:"region"`
	Method     string         `json:"method"`
	Choices    ConsentChoices `json:"choices"`
	AnonIP     string         `json:"anonIp"`
	URL        string         `json:"url"`
}

// RangeQuery bounds a time window (epoch-ms). Zero fields are omitted.
type RangeQuery struct {
	From int64
	To   int64
}

// LogQuery is a RangeQuery plus an optional row limit.
type LogQuery struct {
	From  int64
	To    int64
	Limit int
}

// SignedReceipt is a signed ISO-27560 consent receipt (shape owned by
// @cookiemunch/receipts).
type SignedReceipt map[string]any

// EraseResult is the response of the subject crypto-erase endpoint.
type EraseResult struct {
	Erased int `json:"erased"`
}

// SubjectExport is the response of the subject data-export endpoint.
type SubjectExport struct {
	CBID    string `json:"cbid"`
	Stamp   string `json:"stamp"`
	Records []any  `json:"records"`
	Count   int    `json:"count"`
}

// ---- DSAR -------------------------------------------------------------------

// DsarRequest is a data-subject access request.
type DsarRequest struct {
	ID           string `json:"id"`
	Type         string `json:"type"` // access | deletion | rectification | portability | opt-out
	SubjectEmail string `json:"subjectEmail"`
	Regulation   string `json:"regulation"` // gdpr | ccpa
	Status       string `json:"status"`     // received | verifying | in_progress | completed | rejected
	CreatedAt    int64  `json:"createdAt"`
	DueAt        int64  `json:"dueAt"`
	Note         string `json:"note,omitempty"`
}

// DsarCreate is the body of POST /v1/dsar.
type DsarCreate struct {
	Type         string `json:"type"`
	SubjectEmail string `json:"subjectEmail"`
	Regulation   string `json:"regulation"`
	Note         string `json:"note,omitempty"`
}

// dsarWrap wraps DSAR responses ({ request: ... }).
type dsarWrap struct {
	Request DsarRequest `json:"request"`
}

// ---- vendors ----------------------------------------------------------------

// VendorInput is the body of POST /v1/vendors.
type VendorInput struct {
	Name           string   `json:"name"`
	Category       string   `json:"category"`
	DataShared     []string `json:"dataShared"`
	DpaSigned      bool     `json:"dpaSigned"`
	Subprocessors  int      `json:"subprocessors"`
	Certifications []string `json:"certifications"`
	Region         string   `json:"region"`
}

// RiskScore is a computed vendor-risk rating.
type RiskScore struct {
	Score int    `json:"score"`
	Band  string `json:"band"` // low | medium | high
}

// ScoredVendor is a vendor with its risk flattened in (list response).
type ScoredVendor struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Category       string    `json:"category"`
	DataShared     []string  `json:"dataShared"`
	DpaSigned      bool      `json:"dpaSigned"`
	Subprocessors  int       `json:"subprocessors"`
	Certifications []string  `json:"certifications"`
	Region         string    `json:"region"`
	Risk           RiskScore `json:"risk"`
}

// VendorCreateResult is the response of POST /v1/vendors.
type VendorCreateResult struct {
	Vendor map[string]any `json:"vendor"`
	Risk   RiskScore      `json:"risk"`
}

// ---- RoPA -------------------------------------------------------------------

// RopaInput is the body of POST /v1/ropa.
type RopaInput struct {
	Name                string   `json:"name"`
	Purpose             string   `json:"purpose"`
	LegalBasis          string   `json:"legalBasis"` // consent | contract | legal-obligation | vital-interests | public-task | legitimate-interests
	DataCategories      []string `json:"dataCategories"`
	Recipients          []string `json:"recipients"`
	RetentionDays       int      `json:"retentionDays"`
	CrossBorderTransfer bool     `json:"crossBorderTransfer"`
}

// RopaEntry is a stored RoPA record.
type RopaEntry struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Purpose             string   `json:"purpose"`
	LegalBasis          string   `json:"legalBasis"`
	DataCategories      []string `json:"dataCategories"`
	Recipients          []string `json:"recipients"`
	RetentionDays       int      `json:"retentionDays"`
	CrossBorderTransfer bool     `json:"crossBorderTransfer"`
}

type ropaWrap struct {
	Entry RopaEntry `json:"entry"`
}

// ---- brand kits -------------------------------------------------------------

// BrandKit is a reusable, org-level banner theme.
type BrandKit struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"`
	Name      string `json:"name"`
	Theme     any    `json:"theme,omitempty"`
	Content   any    `json:"content,omitempty"`
	LogoURL   string `json:"logoUrl,omitempty"`
	CustomCSS string `json:"customCss,omitempty"`
}

// BrandKitCreate is the body of POST /v1/brand-kits.
type BrandKitCreate struct {
	Name      string `json:"name"`
	Theme     any    `json:"theme"`
	Content   any    `json:"content,omitempty"`
	LogoURL   string `json:"logoUrl,omitempty"`
	CustomCSS string `json:"customCss,omitempty"`
}

type brandKitWrap struct {
	Kit BrandKit `json:"kit"`
}

// ---- preferences ------------------------------------------------------------

// PreferenceItem is a configurable consent preference/purpose record.
type PreferenceItem struct {
	ID          string `json:"id"`
	OrgID       string `json:"orgId"`
	CBID        string `json:"cbid,omitempty"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// ---- members ----------------------------------------------------------------

// Member is an organisation member. Per the server's OpenAPI schema the wire
// identifier is UserID (not "id").
type Member struct {
	UserID string `json:"userId"`
	Email  string `json:"email"`
	Role   string `json:"role"` // owner | admin | member | viewer
}

type memberWrap struct {
	Member Member `json:"member"`
}

// ---- keys -------------------------------------------------------------------

// ApiKeyPrefix is display metadata for an issued key (never the secret).
type ApiKeyPrefix struct {
	Prefix    string `json:"prefix"`
	CreatedAt int64  `json:"createdAt,omitempty"`
}

// ApiKeyIssued is the response of POST /v1/keys. The Key is returned only once.
type ApiKeyIssued struct {
	Key    string `json:"key"`
	Prefix string `json:"prefix"`
}

// ApiKeyIssueInput is the optional body of POST /v1/keys.
type ApiKeyIssueInput struct {
	Name string `json:"name,omitempty"`
}

// ---- usage ------------------------------------------------------------------

// Usage is the org's resource-usage summary.
type Usage struct {
	Domains       int `json:"domains,omitempty"`
	Seats         int `json:"seats,omitempty"`
	MonthlyEvents int `json:"monthlyEvents,omitempty"`
}

// ---- webhooks ---------------------------------------------------------------

// WebhookSubscription is a webhook subscription. Secret is present only on the
// create response.
type WebhookSubscription struct {
	ID        string   `json:"id"`
	OrgID     string   `json:"orgId"`
	URL       string   `json:"url"`
	Secret    string   `json:"secret,omitempty"`
	Events    []string `json:"events"`
	CBID      *string  `json:"cbid"` // null = all properties in the org
	Active    bool     `json:"active"`
	CreatedAt int64    `json:"createdAt"`
}

// WebhookCreate is the body of POST /v1/webhooks.
type WebhookCreate struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	CBID   string   `json:"cbid,omitempty"`
}

// ---- banners ----------------------------------------------------------------

// BannerSummary is the listing view of an account-level banner design.
type BannerSummary struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	UpdatedAt     int64    `json:"updatedAt"`
	AssignedCBIDs []string `json:"assignedCbids"`
}

// BannerRecord is a full account-level banner design.
type BannerRecord struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"orgId"`
	Name      string     `json:"name"`
	JSON      SiteConfig `json:"json"`
	CreatedAt int64      `json:"createdAt"`
	UpdatedAt int64      `json:"updatedAt"`
}

// BannerCreate is the body of POST /v1/banners.
type BannerCreate struct {
	Name string     `json:"name"`
	JSON SiteConfig `json:"json"`
}

// BannerUpdate is the body of PUT /v1/banners/{id}.
type BannerUpdate struct {
	Name string     `json:"name,omitempty"`
	JSON SiteConfig `json:"json,omitempty"`
}

// BannerAssignments wraps the cbids a design is assigned to.
type BannerAssignments struct {
	CBIDs []string `json:"cbids"`
}

// BannerPublishResult reports which sites a publish compiled the design into.
type BannerPublishResult struct {
	PublishedCBIDs []string `json:"publishedCbids"`
}

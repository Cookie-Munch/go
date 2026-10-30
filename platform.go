package cookiemunch

import (
	"context"
	"encoding/base64"
	"net/url"
	"strconv"
	"strings"
)

// The privacy platform beyond the consent banner, the reseller API, and the site, DSAR
// and RoPA calls that return text.
//
// Identity, vault and profile reads are POSTs on purpose: a person's identifiers travel
// in the request body, never in a URL where logs and proxies would keep them.

// Identifier is one of the ways a person is known, e.g. {Space: "email_sha256", Value: "<hex>"}.
type Identifier struct {
	Space string `json:"space"`
	Value string `json:"value"`
}

// Object is an open-ended JSON object returned by the platform endpoints.
type Object = map[string]any

func (c *Client) object(ctx context.Context, method, path string, query url.Values, body any) (Object, error) {
	var out Object
	if err := c.do(ctx, method, path, query, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) text(ctx context.Context, path string, query url.Values) (string, error) {
	var out string
	if err := c.do(ctx, "GET", path, query, nil, &out); err != nil {
		return "", err
	}
	return out, nil
}

func esc(s string) string { return url.PathEscape(s) }

// ---- sites, DSAR, RoPA: the missing calls -----------------------------------

// Banner reports which banner design a site uses — GET /v1/sites/{cbid}/banner.
// The result's "bannerId" is nil when none is assigned.
func (s *SitesService) Banner(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/banner"), nil, nil)
}

// PolicyOptions tunes a generated policy. Every field is optional.
type PolicyOptions struct {
	// ContactEmail shown in the policy. Falls back to the org controller's, then the owner's.
	ContactEmail string
	// EffectiveDate as YYYY-MM-DD. Defaults to today.
	EffectiveDate string
	// Jurisdictions the policy addresses, overriding the defaults.
	Jurisdictions []string
}

// Policy generates the site's privacy and cookie policy, as Markdown —
// GET /v1/sites/{cbid}/policy.
func (s *SitesService) Policy(ctx context.Context, cbid string, opts *PolicyOptions) (string, error) {
	q := url.Values{}
	if opts != nil {
		if opts.ContactEmail != "" {
			q.Set("contactEmail", opts.ContactEmail)
		}
		if opts.EffectiveDate != "" {
			q.Set("effectiveDate", opts.EffectiveDate)
		}
		if len(opts.Jurisdictions) > 0 {
			q.Set("jurisdictions", strings.Join(opts.Jurisdictions, ","))
		}
	}
	return s.c.text(ctx, sitePath(cbid, "/policy"), q)
}

// SessionAnalysisInput is a captured browsing session. Supply HAR or Requests.
type SessionAnalysisInput struct {
	HAR      any             `json:"har,omitempty"`
	Requests []any           `json:"requests,omitempty"`
	Consent  map[string]bool `json:"consent,omitempty"`
	GPC      *bool           `json:"gpc,omitempty"`
}

// AnalyzeSession reports which trackers fired after opt-out in a captured session, and
// what personal data left the page — POST /v1/sites/{cbid}/sentry.
func (s *SitesService) AnalyzeSession(ctx context.Context, cbid string, input SessionAnalysisInput) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/sentry"), nil, input)
}

// Response returns the subject-facing response notice for a request, as plain text —
// GET /v1/dsar/{id}/response.
func (s *DSARService) Response(ctx context.Context, id string) (string, error) {
	return s.c.text(ctx, "/v1/dsar/"+esc(id)+"/response", nil)
}

// ExportCSV returns the org's RoPA (GDPR Art. 30) as CSV — GET /v1/ropa/export.csv.
func (s *RoPAService) ExportCSV(ctx context.Context) (string, error) {
	return s.c.text(ctx, "/v1/ropa/export.csv", nil)
}

// ---- identity ---------------------------------------------------------------

// IdentityService groups the /v1/identity endpoints: a person as a cluster of identifiers.
type IdentityService struct{ c *Client }

// Resolve returns the subject id for these identifiers, or a nil "subjectId" if unknown.
func (s *IdentityService) Resolve(ctx context.Context, identifiers []Identifier) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/identity/resolve", nil, map[string]any{"identifiers": identifiers})
}

// Link stitches identifiers into one subject. A durable merge.
func (s *IdentityService) Link(ctx context.Context, identifiers []Identifier) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/identity/link", nil, map[string]any{"identifiers": identifiers})
}

// Cluster returns every identifier stitched to a subject.
func (s *IdentityService) Cluster(ctx context.Context, subjectID string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/identity/"+esc(subjectID), nil, nil)
}

// ---- vault ------------------------------------------------------------------

// VaultService groups the /v1/vault endpoints: a resolved person's consent.
type VaultService struct{ c *Client }

// ConsentDecision is one purpose decision recorded against a person.
type ConsentDecision struct {
	Purpose      string `json:"purpose"`
	Allowed      bool   `json:"allowed"`
	LegalBasis   string `json:"legalBasis,omitempty"`
	Jurisdiction string `json:"jurisdiction,omitempty"`
	Provenance   string `json:"provenance,omitempty"`
	CollectedAt  int64  `json:"collectedAt,omitempty"`
}

// Record stores purpose decisions against the resolved person.
func (s *VaultService) Record(ctx context.Context, identifiers []Identifier, decisions []ConsentDecision) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/vault/record", nil, map[string]any{"identifiers": identifiers, "decisions": decisions})
}

// Current returns allow/deny per purpose, across all of the person's identifiers.
func (s *VaultService) Current(ctx context.Context, identifiers []Identifier) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/vault/current", nil, map[string]any{"identifiers": identifiers})
}

// Permits returns the same decisions in full: legal basis, jurisdiction, provenance, time.
func (s *VaultService) Permits(ctx context.Context, identifiers []Identifier) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/vault/permits", nil, map[string]any{"identifiers": identifiers})
}

// ---- profile ----------------------------------------------------------------

// ProfileService groups the /v1/profile endpoints: attributes, with consent enforced
// when they are used.
type ProfileService struct{ c *Client }

// ProfileAttribute is one attribute value with its provenance.
type ProfileAttribute struct {
	Value       string `json:"value"`
	Purpose     string `json:"purpose,omitempty"`
	CollectedAt int64  `json:"collectedAt,omitempty"`
}

// Get returns a person's attributes and permits.
func (s *ProfileService) Get(ctx context.Context, identifiers []Identifier) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/profile/get", nil, map[string]any{"identifiers": identifiers})
}

// SetAttributes stores attributes against a person.
func (s *ProfileService) SetAttributes(ctx context.Context, identifiers []Identifier, attributes map[string]ProfileAttribute) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/profile/attributes", nil, map[string]any{"identifiers": identifiers, "attributes": attributes})
}

// Activate returns the attribute values usable for purpose — empty when the person has
// not consented to it.
func (s *ProfileService) Activate(ctx context.Context, identifiers []Identifier, purpose string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/profile/activate", nil, map[string]any{"identifiers": identifiers, "purpose": purpose})
}

// ---- subscriptions ----------------------------------------------------------

// SubscriptionsService groups the /v1/subscriptions endpoints: marketing preferences as
// topics x channels.
type SubscriptionsService struct{ c *Client }

// SubscriptionTopic is one entry in the org's topic catalog.
type SubscriptionTopic struct {
	Code       string            `json:"code"`
	Name       string            `json:"name,omitempty"`
	Channels   []string          `json:"channels"`
	Downstream map[string]string `json:"downstream,omitempty"`
}

// Topics returns the org's topic catalog.
func (s *SubscriptionsService) Topics(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/subscriptions/topics", nil, nil)
}

// SetTopics replaces the catalog. It is authored whole; anything omitted is removed.
func (s *SubscriptionsService) SetTopics(ctx context.Context, topics []SubscriptionTopic) (Object, error) {
	return s.c.object(ctx, "PUT", "/v1/subscriptions/topics", nil, map[string]any{"topics": topics})
}

// Get returns a subject's topic x channel state.
func (s *SubscriptionsService) Get(ctx context.Context, subjectID string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/subscriptions/"+esc(subjectID), nil, nil)
}

// Set opts a subject in or out of one topic on one channel.
func (s *SubscriptionsService) Set(ctx context.Context, subjectID, topic, channel string, optedIn bool) (Object, error) {
	return s.c.object(ctx, "PUT", "/v1/subscriptions/"+esc(subjectID), nil,
		map[string]any{"topic": topic, "channel": channel, "optedIn": optedIn})
}

// UnsubscribeAll suppresses every topic and channel, keeping the per-topic choices.
func (s *SubscriptionsService) UnsubscribeAll(ctx context.Context, subjectID string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/subscriptions/"+esc(subjectID)+"/unsubscribe-all", nil, nil)
}

// Resubscribe lifts a global unsubscribe, restoring the per-topic choices from before it.
func (s *SubscriptionsService) Resubscribe(ctx context.Context, subjectID string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/subscriptions/"+esc(subjectID)+"/resubscribe", nil, nil)
}

// Activation resolves what to send downstream for a subject across the given topics.
func (s *SubscriptionsService) Activation(ctx context.Context, subjectID string, topics []SubscriptionTopic) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/subscriptions/"+esc(subjectID)+"/activation", nil, map[string]any{"topics": topics})
}

// ---- assessments ------------------------------------------------------------

// AssessmentsService groups the /v1/assessments endpoints: DPIA, PIA, LIA, TIA,
// AI-impact and vendor assessments.
type AssessmentsService struct{ c *Client }

// Templates returns the available templates and their questions.
func (s *AssessmentsService) Templates(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/assessments/templates", nil, nil)
}

// List returns every assessment and its status.
func (s *AssessmentsService) List(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/assessments", nil, nil)
}

// Start begins an assessment from a template against a named subject.
func (s *AssessmentsService) Start(ctx context.Context, template, subject string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/assessments", nil, map[string]any{"template": template, "subject": subject})
}

// Get returns an assessment with its derived score, completeness and reviewer routing.
func (s *AssessmentsService) Get(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/assessments/"+esc(id), nil, nil)
}

// Answer answers one question.
func (s *AssessmentsService) Answer(ctx context.Context, id, questionID string, value any) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/assessments/"+esc(id)+"/answer", nil, map[string]any{"questionId": questionID, "value": value})
}

// AutoPopulateFromMap fills factual answers from the latest data map. It never
// overwrites a human answer.
func (s *AssessmentsService) AutoPopulateFromMap(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/assessments/"+esc(id)+"/autopopulate-from-map", nil, nil)
}

// AutoPopulate fills from evidence you supply (questionId → answer), stamped with
// source. It never overwrites a human answer. Pass "" for no source.
func (s *AssessmentsService) AutoPopulate(ctx context.Context, id string, evidence map[string]any, source string) (Object, error) {
	body := map[string]any{"evidence": evidence}
	if source != "" {
		body["source"] = source
	}
	return s.c.object(ctx, "POST", "/v1/assessments/"+esc(id)+"/autopopulate", nil, body)
}

// Submit submits an assessment for review.
func (s *AssessmentsService) Submit(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/assessments/"+esc(id)+"/submit", nil, nil)
}

// Approve records approval. by becomes the approval record — pass the person who approved.
func (s *AssessmentsService) Approve(ctx context.Context, id, by string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/assessments/"+esc(id)+"/approve", nil, map[string]any{"by": by})
}

// Reject returns an assessment to draft with the reviewer's reason.
func (s *AssessmentsService) Reject(ctx context.Context, id, by, reason string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/assessments/"+esc(id)+"/reject", nil, map[string]any{"by": by, "reason": reason})
}

// ---- discovery --------------------------------------------------------------

// DiscoveryService groups the /v1/discovery endpoints: the data map from an
// in-environment scan (metadata only).
type DiscoveryService struct{ c *Client }

// IngestMap uploads a data map produced by an in-environment scan.
func (s *DiscoveryService) IngestMap(ctx context.Context, dataMap map[string]any) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/discovery/map", nil, map[string]any{"map": dataMap})
}

// GetMap returns the latest data map.
func (s *DiscoveryService) GetMap(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/discovery/map", nil, nil)
}

// RopaDrafts returns processing records drafted from the map.
func (s *DiscoveryService) RopaDrafts(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/discovery/ropa-drafts", nil, nil)
}

// Evidence returns the assessment answers the map can genuinely support.
func (s *DiscoveryService) Evidence(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/discovery/evidence", nil, nil)
}

// Drift reports what changed since the last scan, and where the RoPA disagrees with reality.
func (s *DiscoveryService) Drift(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/discovery/drift", nil, nil)
}

// EnforcementOptions tunes a warehouse enforcement plan.
type EnforcementOptions struct {
	PermitsTable string `json:"permitsTable,omitempty"`
	PolicyPrefix string `json:"policyPrefix,omitempty"`
}

// PlanEnforcement plans masking / row-access policy for a warehouse ("postgres",
// "mysql" or "snowflake"). It applies nothing.
func (s *DiscoveryService) PlanEnforcement(ctx context.Context, dialect string, rules []map[string]any, opts *EnforcementOptions) (Object, error) {
	body := map[string]any{"dialect": dialect, "rules": rules}
	if opts != nil {
		if opts.PermitsTable != "" {
			body["permitsTable"] = opts.PermitsTable
		}
		if opts.PolicyPrefix != "" {
			body["policyPrefix"] = opts.PolicyPrefix
		}
	}
	return s.c.object(ctx, "POST", "/v1/discovery/enforcement", nil, body)
}

// ---- AI governance ----------------------------------------------------------

// AIService groups the /v1/ai endpoints: policy, the inline gateway, inventory and lineage.
type AIService struct{ c *Client }

// GetPolicy returns the AI gateway policy.
func (s *AIService) GetPolicy(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/ai/policy", nil, nil)
}

// SetPolicy replaces the AI gateway policy.
func (s *AIService) SetPolicy(ctx context.Context, policy map[string]any) (Object, error) {
	return s.c.object(ctx, "PUT", "/v1/ai/policy", nil, map[string]any{"policy": policy})
}

// AIInspectInput is a prompt or response to check against consent and policy.
type AIInspectInput struct {
	Prompt    string          `json:"prompt"`
	Purpose   string          `json:"purpose"`
	Model     string          `json:"model,omitempty"`
	Actor     string          `json:"actor,omitempty"`
	Direction string          `json:"direction,omitempty"`
	Consent   map[string]bool `json:"consent,omitempty"`
}

// Inspect enforces consent and policy on a prompt or response. Needs the ai:inspect scope.
func (s *AIService) Inspect(ctx context.Context, input AIInspectInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/ai/inspect", nil, input)
}

// Inventory returns registered systems plus shadow AI inferred from observed traffic.
func (s *AIService) Inventory(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/ai/inventory", nil, nil)
}

// Lineage returns which data categories have reached which model.
func (s *AIService) Lineage(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/ai/lineage", nil, nil)
}

// AISystem declares an AI system.
type AISystem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
	Purpose  string `json:"purpose,omitempty"`
}

// RegisterSystem declares a system so it is inventoried as registered, not shadow.
func (s *AIService) RegisterSystem(ctx context.Context, system AISystem) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/ai/systems", nil, system)
}

// Systems returns the declared systems.
func (s *AIService) Systems(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/ai/systems", nil, nil)
}

// Audit returns recent gateway decisions. limit <= 0 uses the server default.
func (s *AIService) Audit(ctx context.Context, limit int) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/ai/audit", limitQuery(limit), nil)
}

// ---- DSAR fulfilment --------------------------------------------------------

// FulfillmentService groups DSAR fulfilment: plan work per system, and the
// in-environment agent's protocol.
type FulfillmentService struct{ c *Client }

// FulfillmentSystem is one system a request must be carried out in. Operation is
// "locate", "export", "erase" or "optOut".
type FulfillmentSystem struct {
	System    string `json:"system"`
	Operation string `json:"operation"`
}

// SLA returns the rights queue: overdue, due today, due soon, on track.
func (s *FulfillmentService) SLA(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/sla", nil, nil)
}

// Plan breaks a request into one task per system.
func (s *FulfillmentService) Plan(ctx context.Context, requestID string, systems []FulfillmentSystem, includeHistorical bool) (Object, error) {
	body := map[string]any{"systems": systems}
	if includeHistorical {
		body["includeHistorical"] = true
	}
	return s.c.object(ctx, "POST", "/v1/dsar/"+esc(requestID)+"/plan", nil, body)
}

// Status returns per-system progress for a request.
func (s *FulfillmentService) Status(ctx context.Context, requestID string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/"+esc(requestID)+"/fulfillment", nil, nil)
}

// Executors lists the systems connected to run part of a request themselves.
func (s *FulfillmentService) Executors(ctx context.Context) ([]Object, error) {
	var out []Object
	if err := s.c.get(ctx, "/v1/dsar/executors", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Executor returns one connection with its full profile, never its credentials — GET
// /v1/dsar/executors/{id}. Read it before changing the profile: UpdateExecutor replaces the whole profile.
func (s *FulfillmentService) Executor(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/executors/"+esc(id), nil, nil)
}

// ExecutorInput connects a system that runs part of a rights request. Profile describes
// that system's API — paths, the words it uses for export and erase, its status vocabulary,
// how it signs webhooks — so connecting a new platform needs no code. SecretKey is stored
// encrypted and never returned; WebhookSecret is optional, and without it completions are
// picked up by polling.
type ExecutorInput struct {
	System        string `json:"system"`
	BaseURL       string `json:"baseUrl"`
	SecretKey     string `json:"secretKey"`
	Profile       Object `json:"profile"`
	WebhookSecret string `json:"webhookSecret,omitempty"`
	Auto          *bool  `json:"auto,omitempty"`
}

// ConnectExecutor registers a system. The response carries the webhook URL to configure in it.
// A WebhookSecret of the form whsec_<base64> verifies deliveries signed with its base64-decoded
// bytes or with the literal string; any other secret only with its UTF-8 bytes.
func (s *FulfillmentService) ConnectExecutor(ctx context.Context, input ExecutorInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors", nil, input)
}

// dryRunQuery is ?dryRun=1: validate in full, answer what would happen, store nothing.
func dryRunQuery() url.Values { return url.Values{"dryRun": {"1"}} }

// DryRunConnectExecutor validates a connection exactly as ConnectExecutor would and returns
// {dryRun, executor, diff} without storing anything — POST /v1/dsar/executors?dryRun=1.
func (s *FulfillmentService) DryRunConnectExecutor(ctx context.Context, input ExecutorInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors", dryRunQuery(), input)
}

// ExecutorPresetInput connects a system from a versioned preset instead of a hand-written
// profile: "atlas" (optional AtlasIDAppID), "forumulate" (OrgID), "cm-standard-v1" (BaseURL
// and System) or "chatly". A preset waiting on another platform's API is refused with 409
// preset_waiting_on_platform; see ExecutorPresets.
type ExecutorPresetInput struct {
	Preset string `json:"preset"`
	// SecretKey is the connection's key. Send it or MintFrom.
	SecretKey string `json:"secretKey,omitempty"`
	// MintFrom is a partner id (one that registered an executorKeyUrl) that mints the key for the
	// site under its partner link, in place of SecretKey. The connection is scoped to that site.
	MintFrom string `json:"mintFrom,omitempty"`
	// ExternalID (with MintFrom) is the partner account; default the one the site's live link names.
	ExternalID    string `json:"externalId,omitempty"`
	PresetVersion int    `json:"presetVersion,omitempty"`
	System        string `json:"system,omitempty"`
	BaseURL       string `json:"baseUrl,omitempty"`
	OrgID         string `json:"orgId,omitempty"`
	AtlasIDAppID  string `json:"atlasIdAppId,omitempty"`
	WebhookSecret string `json:"webhookSecret,omitempty"`
	Auto          *bool  `json:"auto,omitempty"`
	Cbid          string `json:"cbid,omitempty"`
	// RevokeAtlasIDGrant (atlas) adds the optional step revoking the person's app grant on erasure.
	RevokeAtlasIDGrant *bool `json:"revokeAtlasIdGrant,omitempty"`
	// Label is your name for the credential, shown beside keyPrefix.
	Label string `json:"label,omitempty"`
	// Orchestrator is 2 (the default) or 1, the v1 adapter.
	Orchestrator int `json:"orchestrator,omitempty"`
}

// ConnectExecutorPreset registers a system from a preset — POST /v1/dsar/executors {preset}.
// The response carries presetVersion and presetDiff.
func (s *FulfillmentService) ConnectExecutorPreset(ctx context.Context, input ExecutorPresetInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors", nil, input)
}

// DryRunConnectExecutorPreset is ConnectExecutorPreset with ?dryRun=1: nothing is stored.
func (s *FulfillmentService) DryRunConnectExecutorPreset(ctx context.Context, input ExecutorPresetInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors", dryRunQuery(), input)
}

// ExecutorPresets lists the versioned presets, whether each is usable now or waits on
// another platform, and the organisation's canary address — GET /v1/dsar/executors/presets.
func (s *FulfillmentService) ExecutorPresets(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/executors/presets", nil, nil)
}

// ExecutorPresetUpgrade is what UpgradeExecutorPreset sends: a version, a preset to adopt
// (on a hand-written connection), or params to add. All optional.
type ExecutorPresetUpgrade struct {
	Preset        string `json:"preset,omitempty"`
	PresetVersion int    `json:"presetVersion,omitempty"`
	OrgID         string `json:"orgId,omitempty"`
	AtlasIDAppID  string `json:"atlasIdAppId,omitempty"`
	// RevokeAtlasIDGrant (atlas) adds the optional grant step.
	RevokeAtlasIDGrant *bool `json:"revokeAtlasIdGrant,omitempty"`
	// AcceptPreset names overridden paths (profileOverrides) that take the preset's value again;
	// every other override is kept and listed in the diff as kept or conflict.
	AcceptPreset []string `json:"acceptPreset,omitempty"`
}

// UpgradeExecutorPreset rebuilds a connection from its preset's latest version, keeping its
// id and credential, and answers with a diff — POST /v1/dsar/executors/{id}/upgrade-preset.
func (s *FulfillmentService) UpgradeExecutorPreset(ctx context.Context, id string, input ExecutorPresetUpgrade) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors/"+esc(id)+"/upgrade-preset", nil, input)
}

// DryRunUpgradeExecutorPreset answers what UpgradeExecutorPreset would change, changing nothing.
func (s *FulfillmentService) DryRunUpgradeExecutorPreset(ctx context.Context, id string, input ExecutorPresetUpgrade) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors/"+esc(id)+"/upgrade-preset", dryRunQuery(), input)
}

// ExecutorUpdate changes a connection in place. Only non-nil fields are sent; the stored
// credential is kept unless SecretKey is set. WebhookSecret follows the three-way rule: nil
// leaves it, ClearExecutorWebhookSecret() removes it, ExecutorWebhookSecret(v) replaces it.
type ExecutorUpdate struct {
	Profile       Object
	BaseURL       *string
	SecretKey     *string
	WebhookSecret **string
	Auto          *bool
	// Label is your name for the credential; ClearExecutorLabel removes it.
	Label **string
	// PresetPinned true stops UpgradeExecutorPreset changing a preset-built connection.
	PresetPinned *bool
	// Orchestrator opts the connection in to (2) or out of (1) orchestrator v2.
	Orchestrator *int
}

// ClearExecutorLabel removes a connection's credential label.
func ClearExecutorLabel() **string { var none *string; return &none }

// ExecutorLabel sets a connection's credential label.
func ExecutorLabel(label string) **string { v := &label; return &v }

// ClearExecutorWebhookSecret removes a connection's webhook secret.
func ClearExecutorWebhookSecret() **string { var none *string; return &none }

// ExecutorWebhookSecret sets a connection's webhook secret.
func ExecutorWebhookSecret(secret string) **string { v := &secret; return &v }

// UpdateExecutor changes a connection in place — PATCH /v1/dsar/executors/{id}.
func (s *FulfillmentService) UpdateExecutor(ctx context.Context, id string, patch ExecutorUpdate) (Object, error) {
	return s.c.object(ctx, "PATCH", "/v1/dsar/executors/"+esc(id), nil, executorPatchBody(patch))
}

// DryRunUpdateExecutor returns the connection as UpdateExecutor would leave it, and a diff,
// changing nothing — PATCH /v1/dsar/executors/{id}?dryRun=1.
func (s *FulfillmentService) DryRunUpdateExecutor(ctx context.Context, id string, patch ExecutorUpdate) (Object, error) {
	return s.c.object(ctx, "PATCH", "/v1/dsar/executors/"+esc(id), dryRunQuery(), executorPatchBody(patch))
}

// WebhookDelivery is one webhook delivery to check: its headers and the raw body string,
// exactly the bytes that were signed.
type WebhookDelivery struct {
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

// TestExecutorWebhook runs the receiver's exact check on a delivery and acts on nothing —
// POST /v1/dsar/executors/{id}/webhook/test. The result is {verified, stage, reason}.
func (s *FulfillmentService) TestExecutorWebhook(ctx context.Context, id string, delivery WebhookDelivery) (Object, error) {
	if delivery.Headers == nil {
		delivery.Headers = map[string]string{}
	}
	return s.c.object(ctx, "POST", "/v1/dsar/executors/"+esc(id)+"/webhook/test", nil, delivery)
}

// ExecutorWebhookDeliveries lists the last 50 deliveries to a connection's webhook, newest
// first, each with its reason — GET /v1/dsar/executors/{id}/webhook/deliveries.
func (s *FulfillmentService) ExecutorWebhookDeliveries(ctx context.Context, id string) ([]Object, error) {
	var out []Object
	if err := s.c.get(ctx, "/v1/dsar/executors/"+esc(id)+"/webhook/deliveries", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RecheckExecutor re-checks a connection now: a canary test that records its health, for a
// deploy pipeline — POST /v1/dsar/executors/{id}/recheck.
func (s *FulfillmentService) RecheckExecutor(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors/"+esc(id)+"/recheck", nil, map[string]any{})
}

// ExecutorHealthHistory lists a connection's last health checks, newest first (limit 1 to 50;
// 0 for the default) — GET /v1/dsar/executors/{id}/health/history.
func (s *FulfillmentService) ExecutorHealthHistory(ctx context.Context, id string, limit int) ([]Object, error) {
	var out []Object
	if err := s.c.get(ctx, "/v1/dsar/executors/"+esc(id)+"/health/history", limitQuery(limit), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func executorPatchBody(patch ExecutorUpdate) map[string]any {
	body := map[string]any{}
	if patch.Profile != nil {
		body["profile"] = patch.Profile
	}
	if patch.BaseURL != nil {
		body["baseUrl"] = *patch.BaseURL
	}
	if patch.SecretKey != nil {
		body["secretKey"] = *patch.SecretKey
	}
	if patch.WebhookSecret != nil {
		if *patch.WebhookSecret == nil {
			body["webhookSecret"] = nil
		} else {
			body["webhookSecret"] = **patch.WebhookSecret
		}
	}
	if patch.Auto != nil {
		body["auto"] = *patch.Auto
	}
	if patch.Label != nil {
		if *patch.Label == nil {
			body["label"] = nil
		} else {
			body["label"] = **patch.Label
		}
	}
	if patch.PresetPinned != nil {
		body["presetPinned"] = *patch.PresetPinned
	}
	if patch.Orchestrator != nil {
		body["orchestrator"] = *patch.Orchestrator
	}
	return body
}

// ExecutorTest asks a connection to look one person up without filing a request.
// Operation is locate (default), export or erase; Open on an export opens and polls a real
// export once. An erasure is never sent by a test. Canary (instead of Email) uses the org
// canary address — no real person — and a canary locate updates the connection's health.
type ExecutorTest struct {
	Email     string `json:"email,omitempty"`
	Canary    bool   `json:"canary,omitempty"`
	Operation string `json:"operation,omitempty"`
	Open      bool   `json:"open,omitempty"`
}

// TestExecutor tries a connection — POST /v1/dsar/executors/{id}/test. The result carries
// the raw (redacted) upstream answers and a one-line verdict.
func (s *FulfillmentService) TestExecutor(ctx context.Context, id string, input ExecutorTest) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors/"+esc(id)+"/test", nil, input)
}

// ExecutorConformance runs the cm-standard-v2 conformance kit against a connection, on the
// canary subject only — POST /v1/dsar/executors/{id}/test {"conformance": true}.
func (s *FulfillmentService) ExecutorConformance(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors/"+esc(id)+"/test", nil, map[string]any{"conformance": true})
}

// RollExecutorCompletionSecret mints a new completion secret, shown once; the old one keeps
// verifying for 24 hours — POST /v1/dsar/executors/{id}/completion-secret/roll.
func (s *FulfillmentService) RollExecutorCompletionSecret(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors/"+esc(id)+"/completion-secret/roll", nil, nil)
}

// Task returns one fulfilment task: its status history (events), every request made to a
// connected system on its behalf (log, redacted, 30 days) and whether that system's version
// changed mid-request — GET /v1/dsar/{id}/tasks/{taskId}.
func (s *FulfillmentService) Task(ctx context.Context, requestID, taskID string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/"+esc(requestID)+"/tasks/"+esc(taskID), nil, nil)
}

// DisconnectExecutor removes a connection; its open sub-tasks stop being driven.
func (s *FulfillmentService) DisconnectExecutor(ctx context.Context, id string) error {
	return s.c.delete(ctx, "/v1/dsar/executors/"+esc(id))
}

// TaskExport returns the export bundle a connected system produced, fetched from it on demand.
func (s *FulfillmentService) TaskExport(ctx context.Context, requestID, taskID string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/"+esc(requestID)+"/tasks/"+esc(taskID)+"/export", nil, nil)
}

// PendingTasks is for the in-environment agent: tasks to execute inside your network.
func (s *FulfillmentService) PendingTasks(ctx context.Context, limit int) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/agent/tasks", limitQuery(limit), nil)
}

// ReportTask is for the in-environment agent: only the outcome crosses the boundary.
// Pass "" for no error.
func (s *FulfillmentService) ReportTask(ctx context.Context, taskID string, ok bool, errMsg string) (Object, error) {
	body := map[string]any{"ok": ok}
	if errMsg != "" {
		body["error"] = errMsg
	}
	return s.c.object(ctx, "POST", "/v1/dsar/agent/tasks/"+esc(taskID)+"/result", nil, body)
}

// ---- regulatory intelligence ------------------------------------------------

// RegulatoryService groups the /v1/regulatory endpoints: the curated privacy-law dataset.
type RegulatoryService struct{ c *Client }

// Feed returns the dataset, optionally narrowed to jurisdictions.
func (s *RegulatoryService) Feed(ctx context.Context, jurisdictions []string) (Object, error) {
	q := url.Values{}
	if len(jurisdictions) > 0 {
		q.Set("jurisdictions", strings.Join(jurisdictions, ","))
	}
	return s.c.object(ctx, "GET", "/v1/regulatory/feed", q, nil)
}

// Upcoming returns what takes effect within days (<= 0 uses the server default).
func (s *RegulatoryService) Upcoming(ctx context.Context, days int) (Object, error) {
	q := url.Values{}
	if days > 0 {
		q.Set("days", strconv.Itoa(days))
	}
	return s.c.object(ctx, "GET", "/v1/regulatory/upcoming", q, nil)
}

// ---- reseller ---------------------------------------------------------------

// ResellerService groups the /v1/reseller endpoints: provision and manage child orgs.
// It needs a key with the reseller:* scopes.
type ResellerService struct{ c *Client }

// ResellerChildCreate provisions a child org.
type ResellerChildCreate struct {
	Name            string         `json:"name"`
	OwnerEmail      string         `json:"ownerEmail,omitempty"`
	Controller      map[string]any `json:"controller,omitempty"`
	WhiteLabel      map[string]any `json:"whiteLabel,omitempty"`
	DelegatedAccess *bool          `json:"delegatedAccess,omitempty"`
	// MintKey also issues the child's first API key, returned once as "apiKey".
	MintKey   bool     `json:"mintKey,omitempty"`
	KeyScopes []string `json:"keyScopes,omitempty"`
}

// ResellerChildPatch updates a child org. Only non-nil fields are sent.
//
// DSARRouting is a pointer to a pointer so the three cases stay distinct: nil leaves the
// override alone, a pointer to nil clears it, and a pointer to a value pins it. Use
// ClearDSARRouting or PinDSARRouting to build it.
type ResellerChildPatch struct {
	Status          *string
	DelegatedAccess *bool
	DSARRouting     **string
	Controller      map[string]any
}

// ClearDSARRouting returns a DSARRouting value that clears a child's override.
func ClearDSARRouting() **string { var none *string; return &none }

// PinDSARRouting returns a DSARRouting value pinning a child to "reseller" or "child".
func PinDSARRouting(routing string) **string { v := &routing; return &v }

func (p ResellerChildPatch) body() map[string]any {
	b := map[string]any{}
	if p.Status != nil {
		b["status"] = *p.Status
	}
	if p.DelegatedAccess != nil {
		b["delegatedAccess"] = *p.DelegatedAccess
	}
	if p.DSARRouting != nil {
		if *p.DSARRouting == nil {
			b["dsarRouting"] = nil
		} else {
			b["dsarRouting"] = **p.DSARRouting
		}
	}
	if p.Controller != nil {
		b["controller"] = p.Controller
	}
	return b
}

// ChildKeyInput mints a key for a child org.
type ChildKeyInput struct {
	Name   string   `json:"name,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
	Cbids  []string `json:"cbids,omitempty"`
}

// List returns the child orgs, with pooled usage against the reseller plan's cap.
func (s *ResellerService) List(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/reseller/customers", nil, nil)
}

// Create provisions a child org.
func (s *ResellerService) Create(ctx context.Context, input ResellerChildCreate) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/reseller/customers", nil, input)
}

// Get returns one child org: controller details and usage.
func (s *ResellerService) Get(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/reseller/customers/"+esc(id), nil, nil)
}

// Update changes a child org's status, delegated access, DSAR routing or controller.
func (s *ResellerService) Update(ctx context.Context, id string, patch ResellerChildPatch) (Object, error) {
	return s.c.object(ctx, "PATCH", "/v1/reseller/customers/"+esc(id), nil, patch.body())
}

// Deprovision suspends a child org, which is reversible. With purge it deletes the org
// and its data instead — irreversibly.
func (s *ResellerService) Deprovision(ctx context.Context, id string, purge bool) error {
	q := url.Values{}
	if purge {
		q.Set("purge", "true")
	}
	return s.c.do(ctx, "DELETE", "/v1/reseller/customers/"+esc(id), q, nil, nil)
}

// ListKeys returns a child org's key prefixes — never the secrets.
func (s *ResellerService) ListKeys(ctx context.Context, id string) ([]ApiKeyPrefix, error) {
	var out []ApiKeyPrefix
	if err := s.c.get(ctx, "/v1/reseller/customers/"+esc(id)+"/keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MintKey issues an API key for a child org; the secret is returned once.
func (s *ResellerService) MintKey(ctx context.Context, id string, input ChildKeyInput) (*ApiKeyIssued, error) {
	var out ApiKeyIssued
	if err := s.c.post(ctx, "/v1/reseller/customers/"+esc(id)+"/keys", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RevokeKey revokes a child org's key by prefix. It takes effect immediately.
func (s *ResellerService) RevokeKey(ctx context.Context, id, prefix string) error {
	return s.c.delete(ctx, "/v1/reseller/customers/"+esc(id)+"/keys/"+esc(prefix))
}

func limitQuery(limit int) url.Values {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return q
}

// ---- the routes the spec was missing ----------------------------------------

// VerifyChallenge returns exactly what to publish to prove control of the domain, for
// each method — GET /v1/sites/{cbid}/verify/challenge.
func (s *SitesService) VerifyChallenge(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/verify/challenge"), nil, nil)
}

// PreviewTokenOptions asks for a banner preview. Every field is optional; zero values are
// not sent.
type PreviewTokenOptions struct {
	// Region to force: a country or subdivision code ("de", "us-ca") or "eu".
	Region string `json:"region,omitempty"`
	// Lang to force, as a BCP 47 tag ("de", "pt-BR").
	Lang string `json:"lang,omitempty"`
	// URL of the page to preview. Default: the site's registered domain.
	URL string `json:"url,omitempty"`
	// TTLSeconds is the token lifetime, 1..3600 (default 900).
	TTLSeconds int `json:"ttlSeconds,omitempty"`
}

// PreviewToken mints a short-lived signed token to see the banner as a visitor in a given
// region and language sees it — headless browsers included, nothing recorded —
// POST /v1/sites/{cbid}/preview-token. The result carries "token", "expiresAt" and
// "previewUrl".
func (s *SitesService) PreviewToken(ctx context.Context, cbid string, opts PreviewTokenOptions) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/preview-token"), nil, opts)
}

// BulkSite is one site to create in a bulk call.
type BulkSite struct {
	Domain   string `json:"domain"`
	Cbid     string `json:"cbid,omitempty"`
	Platform string `json:"platform,omitempty"`
}

// CreateBulk creates up to 100 sites — POST /v1/sites/bulk. Partial success: each item
// in "results" reports ok or its own error.
func (s *SitesService) CreateBulk(ctx context.Context, sites []BulkSite) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/sites/bulk", nil, map[string]any{"sites": sites})
}

// Revoke revokes a key by its prefix — DELETE /v1/keys/{prefix}. Immediate.
func (s *KeysService) Revoke(ctx context.Context, prefix string) error {
	return s.c.delete(ctx, "/v1/keys/"+esc(prefix))
}

// WebhookUpdate changes a subscription. Only non-nil fields are sent.
//
// Cbid follows the same three-way rule as ResellerChildPatch.DSARRouting: nil leaves the
// scope alone, a pointer to nil widens it to every property in the org
// (WebhookAllProperties), and a pointer to a value narrows it (WebhookProperty).
type WebhookUpdate struct {
	URL    *string
	Events []string
	Cbid   **string
	// Active false pauses delivery without deleting the subscription.
	Active *bool
}

// WebhookAllProperties widens a subscription to every property in the org.
func WebhookAllProperties() **string { var none *string; return &none }

// WebhookProperty narrows a subscription to one property.
func WebhookProperty(cbid string) **string { v := &cbid; return &v }

// Update changes or pauses a subscription — PATCH /v1/webhooks/{id}.
func (s *WebhooksService) Update(ctx context.Context, id string, patch WebhookUpdate) (Object, error) {
	body := map[string]any{}
	if patch.URL != nil {
		body["url"] = *patch.URL
	}
	if patch.Events != nil {
		body["events"] = patch.Events
	}
	if patch.Cbid != nil {
		if *patch.Cbid == nil {
			body["cbid"] = nil
		} else {
			body["cbid"] = **patch.Cbid
		}
	}
	if patch.Active != nil {
		body["active"] = *patch.Active
	}
	return s.c.object(ctx, "PATCH", "/v1/webhooks/"+esc(id), nil, body)
}

// SubjectsService reads one person's consent across every site in the org, by the
// subject id your apps attach.
type SubjectsService struct{ c *Client }

// Consent returns a subject's records across all sites — GET /v1/subjects/{id}/consent.
// It needs consent:read and is not available to property-locked keys.
func (s *SubjectsService) Consent(ctx context.Context, subjectID string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/subjects/"+esc(subjectID)+"/consent", nil, nil)
}

// RightsLinksService mints account-linked rights requests: single-use links to the hosted
// rights page for a person your backend knows is signed in.
type RightsLinksService struct{ c *Client }

// RightsLinkSubject is the signed-in person. Email must be an address you have verified;
// UserID (optional) is your own account id, recorded on the request.
type RightsLinkSubject struct {
	Email  string `json:"email"`
	UserID string `json:"userId,omitempty"`
}

// RightsLinkInput asks for a rights link. Rights limits the link to those rights (default:
// every right the page offers); TTLSeconds is 60..3600 (default 900). Zero values are not sent.
type RightsLinkInput struct {
	Subject    RightsLinkSubject `json:"subject"`
	Rights     []string          `json:"rights,omitempty"`
	TTLSeconds int               `json:"ttlSeconds,omitempty"`
}

// Create mints the link — POST /v1/sites/{cbid}/rights-links. The page locks the person's
// email and skips the code step; show "url" only to them. The result carries "url" and
// "expiresAt". Needs dsar:write.
func (s *RightsLinksService) Create(ctx context.Context, cbid string, in RightsLinkInput) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/rights-links"), nil, in)
}

// PreferenceCenterService is the hosted preference centre, opened for a visitor your server has
// signed in.
type PreferenceCenterService struct{ c *Client }

// CenterTokenInput names who a preference-centre token is for: a site of your organisation and
// exactly one of Email (resolved as the emailed preference link resolves it) or SubjectID.
// TTLSeconds is 1..86400 (default 3600). Zero values are not sent.
type CenterTokenInput struct {
	CBID       string `json:"cbid"`
	Email      string `json:"email,omitempty"`
	SubjectID  string `json:"subjectId,omitempty"`
	TTLSeconds int    `json:"ttlSeconds,omitempty"`
}

// MintToken mints a preference-centre token — POST /v1/preference-center/tokens — for the page
// to pass to CookieMunch.setSubjectToken. The result carries "token" and "expiresAt". The token
// is a credential: never put it in a shared URL or a log. Needs subscriptions:write and
// profile:write, plus identity:read by email.
func (s *PreferenceCenterService) MintToken(ctx context.Context, in CenterTokenInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/preference-center/tokens", nil, in)
}

// ConsentSyncSecretService is a site's consent-sync signing secret. Your server signs
// consent-sync tokens with it so a signed-in visitor's consent decision follows them across
// browsers (hosted consent sync). Reading needs sites:read; Rotate and Remove need sites:write
// and consent:write.
type ConsentSyncSecretService struct{ c *Client }

// Get says whether it is set up — GET /v1/sites/{cbid}/consent-sync-secret ("configured",
// "secretLast4", "createdAt"). Never the secret.
func (s *ConsentSyncSecretService) Get(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/consent-sync-secret"), nil, nil)
}

// Rotate mints the secret, or replaces it (the old one stops at once) — POST
// /v1/sites/{cbid}/consent-sync-secret. The result's "secret" is shown this once: keep it on
// your server, never in a page.
func (s *ConsentSyncSecretService) Rotate(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/consent-sync-secret"), nil, nil)
}

// Remove deletes it — DELETE /v1/sites/{cbid}/consent-sync-secret: hosted consent sync stops.
func (s *ConsentSyncSecretService) Remove(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/consent-sync-secret"), nil, nil)
}

// RightsCaptchaService is a site's rights-page captcha: your own Turnstile, hCaptcha or
// reCAPTCHA keys (recommended), or off. A site without its own setting uses the organisation
// default (Org.RightsCaptcha), else off. Cookie Munch's own captcha is never used on rights
// pages. Needs sites:read / sites:write.
type RightsCaptchaService struct{ c *Client }

// RightsCaptchaInput sets a rights-page captcha. Mode is "own" or "off". For "own": Provider
// (turnstile, hcaptcha, recaptcha_v2, recaptcha_v3), SiteKey, and Secret (write-only; leave it
// empty to keep the stored one) — the provider must accept the secret before it is saved.
// MinScore is for recaptcha_v3 (0.1-0.9; 0 = the default 0.5). "off" removes spam protection
// beyond the spam-trap field, the rate limits and the email code. Zero values are not sent.
type RightsCaptchaInput struct {
	Mode     string  `json:"mode"`
	Provider string  `json:"provider,omitempty"`
	SiteKey  string  `json:"siteKey,omitempty"`
	Secret   string  `json:"secret,omitempty"`
	MinScore float64 `json:"minScore,omitempty"`
}

// Get returns the setting in force — GET /v1/sites/{cbid}/rights-captcha ("source": site, org
// or default). Never the secret.
func (s *RightsCaptchaService) Get(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/rights-captcha"), nil, nil)
}

// Set saves the site's own setting — PUT /v1/sites/{cbid}/rights-captcha.
func (s *RightsCaptchaService) Set(ctx context.Context, cbid string, in RightsCaptchaInput) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/rights-captcha"), nil, in)
}

// Remove drops the site's own setting — DELETE /v1/sites/{cbid}/rights-captcha. The site
// inherits the organisation default again; the result is the setting now in force.
func (s *RightsCaptchaService) Remove(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/rights-captcha"), nil, nil)
}

// RightsIdentityService is a site's sign-in provider (Clerk, Atlas or any JWKS issuer): a
// visitor signed in to your site is verified by their session, with no code mail. A site
// without its own setting uses the organisation default (Org.RightsIdentity). Needs
// sites:read / sites:write.
type RightsIdentityService struct{ c *Client }

// RightsIdentityInput connects a sign-in provider. Type is "clerk", "atlas" or "oidc_jwt";
// Issuer is exactly the token's iss. JwksURL is required for oidc_jwt (clerk/atlas default to
// <issuer>/.well-known/jwks.json); AuthorizedParties (your own origins) is required for clerk
// and atlas; oidc_jwt needs an Audience or AuthorizedParties. The JWKS is fetched and checked
// before anything is saved. The token must carry the email and a verified-email claim set to
// true; TrustEmailClaim (clerk/atlas only) accepts a token without the verified claim — set it
// only if every address your provider puts in the email claim is verified. Zero values are not sent.
type RightsIdentityInput struct {
	Type               string   `json:"type"`
	Issuer             string   `json:"issuer"`
	JwksURL            string   `json:"jwksUrl,omitempty"`
	Audience           string   `json:"audience,omitempty"`
	AuthorizedParties  []string `json:"authorizedParties,omitempty"`
	EmailClaim         string   `json:"emailClaim,omitempty"`
	EmailVerifiedClaim string   `json:"emailVerifiedClaim,omitempty"`
	SessionCookieName  string   `json:"sessionCookieName,omitempty"`
	TrustEmailClaim    bool     `json:"trustEmailClaim,omitempty"`
}

// Get returns the setting in force — GET /v1/sites/{cbid}/rights-identity ("source": site, org
// or default).
func (s *RightsIdentityService) Get(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/rights-identity"), nil, nil)
}

// Set saves the site's own setting — PUT /v1/sites/{cbid}/rights-identity.
func (s *RightsIdentityService) Set(ctx context.Context, cbid string, in RightsIdentityInput) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/rights-identity"), nil, in)
}

// Remove drops the site's own setting — DELETE /v1/sites/{cbid}/rights-identity. The site
// inherits the organisation default again; the result is the setting now in force.
func (s *RightsIdentityService) Remove(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/rights-identity"), nil, nil)
}

// ---- the operations only the dashboard used to have --------------------------

// OrgService reads and changes the key's own organisation — /v1/org. It needs an
// unscoped key that is not property-locked. Deleting an org is not part of the API.
type OrgService struct{ c *Client }

// Get returns the organisation: id, name, plan and logo — GET /v1/org.
func (s *OrgService) Get(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/org", nil, nil)
}

// OrgUpdate renames the organisation or changes its logo. Only non-nil fields are sent.
//
// LogoURL follows the same three-way rule as WebhookUpdate.Cbid: nil leaves the logo
// alone, a pointer to nil removes it (ClearOrgLogo), and a pointer to a value sets it
// (OrgLogo). An omitted field and an explicit null mean different things to the server,
// so the distinction has to survive into the JSON body.
type OrgUpdate struct {
	Name    *string
	LogoURL **string
}

// ClearOrgLogo removes the organisation's logo.
func ClearOrgLogo() **string { var none *string; return &none }

// OrgLogo sets the organisation's logo to a URL — upload one with Assets.Upload.
func OrgLogo(url string) **string { v := &url; return &v }

// Update renames the organisation and/or sets its logo — PATCH /v1/org.
func (s *OrgService) Update(ctx context.Context, patch OrgUpdate) (Object, error) {
	body := map[string]any{}
	if patch.Name != nil {
		body["name"] = *patch.Name
	}
	if patch.LogoURL != nil {
		if *patch.LogoURL == nil {
			body["logoUrl"] = nil
		} else {
			body["logoUrl"] = **patch.LogoURL
		}
	}
	return s.c.object(ctx, "PATCH", "/v1/org", nil, body)
}

// MailProviderSES is the Amazon SES part of a MailProviderInput. The secret access key goes in Secret.
type MailProviderSES struct {
	Region      string `json:"region"`
	AccessKeyID string `json:"accessKeyId"`
}

// MailProviderSMTP is the SMTP part of a MailProviderInput. Port is 25, 465, 587 or 2525;
// Security is "tls" (implicit) or "starttls". The password goes in Secret.
type MailProviderSMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Security string `json:"security"`
	Username string `json:"username"`
}

// MailProviderInput is your own mail provider for rights-request mail. Provider is one of
// brevo, sendgrid, postmark, resend, ses, smtp. Secret is write-only (API key, server token,
// secret access key or SMTP password); leave it empty to keep the stored one (same provider only).
// The whole provider is replaced, so a field left empty is cleared.
type MailProviderInput struct {
	Provider  string            `json:"provider"`
	FromEmail string            `json:"fromEmail"`
	FromName  string            `json:"fromName,omitempty"`
	ReplyTo   string            `json:"replyTo,omitempty"`
	Secret    string            `json:"secret,omitempty"`
	SES       *MailProviderSES  `json:"ses,omitempty"`
	SMTP      *MailProviderSMTP `json:"smtp,omitempty"`
}

// OrgControllerInput is the organisation's own data-controller identity. LegalName is required;
// a field left empty is left out, and PUT replaces the stored identity, so it is removed.
type OrgControllerInput struct {
	LegalName    string `json:"legalName"`
	ContactEmail string `json:"contactEmail,omitempty"`
	Address      string `json:"address,omitempty"`
	DpoContact   string `json:"dpoContact,omitempty"`
}

// Controller returns your organisation's own data-controller identity — GET /v1/org/controller.
// Every rights notice names it, before any site config controller block. "set" is false when
// there is none.
func (s *OrgService) Controller(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/org/controller", nil, nil)
}

// SetController sets the organisation's data-controller identity — PUT /v1/org/controller. It
// replaces the stored one. Needs an unscoped key or rights:write.
func (s *OrgService) SetController(ctx context.Context, in OrgControllerInput) (Object, error) {
	return s.c.object(ctx, "PUT", "/v1/org/controller", nil, in)
}

// RemoveController removes it — DELETE /v1/org/controller. Notices then name the site config
// controller block.
func (s *OrgService) RemoveController(ctx context.Context) error {
	return s.c.delete(ctx, "/v1/org/controller")
}

// MailProvider returns your own mail provider for rights-request mail — GET /v1/org/mail-provider.
// The credential is never returned, only secretLast4; "sender" says whether verification codes
// go through it now ("own") or are not mailed at all ("none") — there is no shared sender.
func (s *OrgService) MailProvider(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/org/mail-provider", nil, nil)
}

// SetMailProvider saves your own mail provider (recommended) — PUT /v1/org/mail-provider. It
// carries rights mail only after TestMailProvider succeeds; every save clears that.
func (s *OrgService) SetMailProvider(ctx context.Context, input MailProviderInput) (Object, error) {
	return s.c.object(ctx, "PUT", "/v1/org/mail-provider", nil, input)
}

// TestMailProvider sends a test message through your provider — POST /v1/org/mail-provider/test.
// An empty to sends to the reply-to address, else the from address. A refusal is ok=false
// with the provider's reason, not an error.
func (s *OrgService) TestMailProvider(ctx context.Context, to string) (Object, error) {
	body := map[string]any{}
	if to != "" {
		body["to"] = to
	}
	return s.c.object(ctx, "POST", "/v1/org/mail-provider/test", nil, body)
}

// RemoveMailProvider removes your provider — DELETE /v1/org/mail-provider. No verification codes
// are mailed after this.
func (s *OrgService) RemoveMailProvider(ctx context.Context) error {
	return s.c.delete(ctx, "/v1/org/mail-provider")
}

// RightsCaptcha returns the default captcha for your rights pages (sites without their own) —
// GET /v1/org/rights-captcha. Never the secret.
func (s *OrgService) RightsCaptcha(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/org/rights-captcha", nil, nil)
}

// SetRightsCaptcha sets the default — PUT /v1/org/rights-captcha: own keys (recommended) or off.
func (s *OrgService) SetRightsCaptcha(ctx context.Context, in RightsCaptchaInput) (Object, error) {
	return s.c.object(ctx, "PUT", "/v1/org/rights-captcha", nil, in)
}

// RemoveRightsCaptcha removes the default — DELETE /v1/org/rights-captcha: sites without their
// own setting have no captcha. The result is the setting now in force.
func (s *OrgService) RemoveRightsCaptcha(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "DELETE", "/v1/org/rights-captcha", nil, nil)
}

// RightsIdentity returns the default sign-in provider rights requests are verified with —
// GET /v1/org/rights-identity.
func (s *OrgService) RightsIdentity(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/org/rights-identity", nil, nil)
}

// SetRightsIdentity sets the default — PUT /v1/org/rights-identity.
func (s *OrgService) SetRightsIdentity(ctx context.Context, in RightsIdentityInput) (Object, error) {
	return s.c.object(ctx, "PUT", "/v1/org/rights-identity", nil, in)
}

// RemoveRightsIdentity removes the default — DELETE /v1/org/rights-identity. The result is the
// setting now in force.
func (s *OrgService) RemoveRightsIdentity(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "DELETE", "/v1/org/rights-identity", nil, nil)
}

// AssetsService stores images used by banners — /v1/assets.
type AssetsService struct{ c *Client }

// Upload stores an image and returns its public URL — POST /v1/assets. data is the raw
// image; it is base64-encoded here. PNG, JPEG, WebP, GIF or SVG, up to 1,000,000 bytes.
// Requires sites:write.
func (s *AssetsService) Upload(ctx context.Context, data []byte, contentType string) (Object, error) {
	body := map[string]any{"data": base64.StdEncoding.EncodeToString(data), "contentType": contentType}
	return s.c.object(ctx, "POST", "/v1/assets", nil, body)
}

// Blocked lists pages where the embed could not load its banner renderer —
// GET /v1/sites/{cbid}/blocked. The host page's Content Security Policy or Trusted Types
// policy refused it, so nobody there can be asked for consent. Empty is the healthy answer.
func (s *SitesService) Blocked(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/blocked"), nil, nil)
}

// ImportDeclaration reads a cookie declaration exported from another CMP and translates
// its categories into ours — POST /v1/sites/{cbid}/import. Nothing is applied: their
// vocabulary is not ours, and a cookie in the wrong category is a tag firing against a
// refusal, so the result comes back for review.
func (s *SitesService) ImportDeclaration(ctx context.Context, cbid, data string) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/import"), nil, map[string]any{"data": data})
}

// ApplyImport imports a declaration exported from another CMP AND declares it —
// POST /v1/sites/{cbid}/import?apply=1. Every row whose category mapped to ours and that
// has a purpose is declared; the rest come back in "skipped". Cookies the site already
// declared are kept unless overwrite is true.
func (s *SitesService) ApplyImport(ctx context.Context, cbid, data string, overwrite bool) (Object, error) {
	q := url.Values{"apply": {"1"}}
	if overwrite {
		q.Set("overwrite", "1")
	}
	return s.c.object(ctx, "POST", sitePath(cbid, "/import"), q, map[string]any{"data": data})
}

// DeclaredCookieInput is what the site owner says a cookie is. Category is a standard
// category or one of the site's categories.suite or categories.custom ids (not "unclassified").
type DeclaredCookieInput struct {
	Category string `json:"category"`
	Purpose  string `json:"purpose"`
	// Purposes is the purpose in other languages, keyed by BCP-47 tag ("es", "pt-BR").
	// Purpose is the fallback for any language it lacks. Left empty, nothing is sent.
	Purposes map[string]string `json:"purposes,omitempty"`
	Provider string            `json:"provider,omitempty"`
	Expiry   string            `json:"expiry,omitempty"`
	Domain   string            `json:"domain,omitempty"`
	// Pattern treats the name as a prefix wildcard (a trailing "*" does the same).
	Pattern bool `json:"pattern,omitempty"`
	// Type declares browser storage instead of a cookie: "localStorage", "sessionStorage"
	// or "indexedDB". Left empty, it is a cookie.
	Type string `json:"type,omitempty"`
}

// CookiePresetParams are a vendor preset's parameters: Atlas FrontendAPIHost and
// CrossSubdomainCookies; Chatly PublicID, Dragging and Category; referral presets CookieDomain, CookieDays and Category (referalli LegacyName, impact CampaignID).
type CookiePresetParams struct {
	FrontendAPIHost       string `json:"frontendApiHost,omitempty"`
	CrossSubdomainCookies *bool  `json:"crossSubdomainCookies,omitempty"`
	PublicID              string `json:"publicId,omitempty"`
	Dragging              bool   `json:"dragging,omitempty"`
	Category              string `json:"category,omitempty"`
	CookieDomain          string `json:"cookieDomain,omitempty"`
	CookieDays            int    `json:"cookieDays,omitempty"`
	LegacyName            *bool  `json:"legacyName,omitempty"`
	CampaignID            string `json:"campaignId,omitempty"`
}

// PurgeObservedInput is the window of a purge: Since (and optionally Until), ISO-8601.
type PurgeObservedInput struct {
	Since string `json:"since"`
	Until string `json:"until,omitempty"`
}

// CookiePresets lists every declaration preset (atlas, forumulate, chatly, cookiemunch, referalli, firstpromoter, partnerstack, tapfiliate, impact) —
// GET /v1/sites/{cbid}/cookies/presets: parameters, what each declared here, and whether
// the vendor's manifest exists yet.
func (s *SitesService) CookiePresets(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/cookies/presets"), nil, nil)
}

// ApplyCookiePreset applies (or re-syncs) a vendor preset — PUT
// /v1/sites/{cbid}/cookies/presets/{vendor}: declares what that platform writes, marked
// "vendor". Names you declared yourself are kept ("skipped").
func (s *SitesService) ApplyCookiePreset(ctx context.Context, cbid, vendor string, in CookiePresetParams) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies/presets/"+esc(vendor)), nil, in)
}

// ApplyCookiePresetDryRun is ApplyCookiePreset with ?dryRun=1; nothing is stored.
func (s *SitesService) ApplyCookiePresetDryRun(ctx context.Context, cbid, vendor string, in CookiePresetParams) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies/presets/"+esc(vendor)), dryRun, in)
}

// ImportCookieManifest declares any vendor's cookies and storage from its embed manifest — PUT
// /v1/sites/{cbid}/cookies/presets/custom with {"url": …} or {"manifest": …} (and "params"). The
// vendor id is the manifest's "product".
func (s *SitesService) ImportCookieManifest(ctx context.Context, cbid string, in Object) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies/presets/custom"), nil, in)
}

// ImportCookieManifestDryRun is ImportCookieManifest with ?dryRun=1; nothing is stored.
func (s *SitesService) ImportCookieManifestDryRun(ctx context.Context, cbid string, in Object) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies/presets/custom"), dryRun, in)
}

// CookieSuggestions returns what to declare — GET /v1/sites/{cbid}/cookies/suggestions: detected
// vendors and one suggestion per undeclared cookie or key.
func (s *SitesService) CookieSuggestions(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/cookies/suggestions"), nil, nil)
}

// DeclareCookieSuggestions declares the suggestions — POST
// /v1/sites/{cbid}/cookies/suggestions/declare ({"names": […], "presets": bool}, both optional).
func (s *SitesService) DeclareCookieSuggestions(ctx context.Context, cbid string, in Object) (Object, error) {
	if in == nil {
		in = Object{}
	}
	return s.c.object(ctx, "POST", sitePath(cbid, "/cookies/suggestions/declare"), nil, in)
}

// DeclareCookieSuggestionsDryRun is DeclareCookieSuggestions with ?dryRun=1; nothing is stored.
func (s *SitesService) DeclareCookieSuggestionsDryRun(ctx context.Context, cbid string, in Object) (Object, error) {
	if in == nil {
		in = Object{}
	}
	return s.c.object(ctx, "POST", sitePath(cbid, "/cookies/suggestions/declare"), dryRun, in)
}

// RemoveCookiePreset removes every declaration a vendor preset made — DELETE
// /v1/sites/{cbid}/cookies/presets/{vendor}.
func (s *SitesService) RemoveCookiePreset(ctx context.Context, cbid, vendor string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/cookies/presets/"+esc(vendor)), nil, nil)
}

// DeleteObservedCookie removes every stored (scanned or observed) row of this name — DELETE
// /v1/sites/{cbid}/observed-cookies/{name}. A declaration of it stays.
func (s *SitesService) DeleteObservedCookie(ctx context.Context, cbid, name string) error {
	return s.c.do(ctx, "DELETE", sitePath(cbid, "/observed-cookies/"+esc(name)), nil, nil, nil)
}

// DeleteObservedCookieDryRun answers with what DeleteObservedCookie would remove (?dryRun=1).
func (s *SitesService) DeleteObservedCookieDryRun(ctx context.Context, cbid, name string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/observed-cookies/"+esc(name)), dryRun, nil)
}

// PurgeObservedCookies purges observed since T — POST /v1/sites/{cbid}/observed-cookies/purge:
// every row a real visit first reported at or after Since (and before Until). Crawled rows and
// declarations stay.
func (s *SitesService) PurgeObservedCookies(ctx context.Context, cbid string, in PurgeObservedInput) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/observed-cookies/purge"), nil, in)
}

// PurgeObservedCookiesDryRun is PurgeObservedCookies with ?dryRun=1; nothing is removed.
func (s *SitesService) PurgeObservedCookiesDryRun(ctx context.Context, cbid string, in PurgeObservedInput) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/observed-cookies/purge"), dryRun, in)
}

// DeclaredCookieItem is one entry of a bulk declaration.
type DeclaredCookieItem struct {
	Name string `json:"name"`
	DeclaredCookieInput
}

// DeclareCookie declares what one cookie is — PUT /v1/sites/{cbid}/cookies/{name}. It
// overrides scanned, observed and database classification everywhere (API, preference
// modal, data-cc="declaration"). A name ending in "*" declares a prefix wildcard.
func (s *SitesService) DeclareCookie(ctx context.Context, cbid, name string, in DeclaredCookieInput) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies/"+esc(name)), nil, in)
}

// DeclareCookies declares many cookies at once — PUT /v1/sites/{cbid}/cookies.
// All-or-nothing; replace also removes declarations the list does not name.
func (s *SitesService) DeclareCookies(ctx context.Context, cbid string, cookies []DeclaredCookieItem, replace bool) (Object, error) {
	body := map[string]any{"cookies": cookies}
	if replace {
		body["replace"] = true
	}
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies"), nil, body)
}

// UndeclareCookie removes a declaration — DELETE /v1/sites/{cbid}/cookies/{name}. The
// scanned classification applies again.
func (s *SitesService) UndeclareCookie(ctx context.Context, cbid, name string) error {
	return s.c.do(ctx, "DELETE", sitePath(cbid, "/cookies/"+esc(name)), nil, nil, nil)
}

var dryRun = url.Values{"dryRun": {"1"}}

// RetireCookie retires a cookie the site renamed or stopped setting — POST
// /v1/sites/{cbid}/cookies/{name}/retire. It leaves the banner's cookie list,
// data-cc="declaration" and the public feed, and stays listed for the owner with
// "retired". A name ending in * retires every cookie with that prefix. If a crawl or a
// visit sees it again afterwards it is listed again, flagged "retired_but_seen".
func (s *SitesService) RetireCookie(ctx context.Context, cbid, name string, in RetireCookieInput) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/cookies/"+esc(name)+"/retire"), nil, in)
}

// RetireCookieDryRun is RetireCookie with ?dryRun=1: the retirement as it would be
// stored and the would-be list ("retired"); nothing is.
func (s *SitesService) RetireCookieDryRun(ctx context.Context, cbid, name string, in RetireCookieInput) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/cookies/"+esc(name)+"/retire"), dryRun, in)
}

// UnretireCookie un-retires a cookie — DELETE /v1/sites/{cbid}/cookies/{name}/retire.
// It is listed again everywhere.
func (s *SitesService) UnretireCookie(ctx context.Context, cbid, name string) error {
	return s.c.do(ctx, "DELETE", sitePath(cbid, "/cookies/"+esc(name)+"/retire"), nil, nil, nil)
}

// UnretireCookieDryRun answers with what un-retiring would leave ("retired") and removes
// nothing (?dryRun=1).
func (s *SitesService) UnretireCookieDryRun(ctx context.Context, cbid, name string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/cookies/"+esc(name)+"/retire"), dryRun, nil)
}

// DeclareCookieDryRun validates DeclareCookie and answers with what it would store —
// "cookie", "declared" (the would-be list) and a "diff" — storing nothing (?dryRun=1).
func (s *SitesService) DeclareCookieDryRun(ctx context.Context, cbid, name string, in DeclaredCookieInput) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies/"+esc(name)), dryRun, in)
}

// DeclareCookiesDryRun is DeclareCookies with ?dryRun=1: what would be stored, plus a
// "diff"; nothing is.
func (s *SitesService) DeclareCookiesDryRun(ctx context.Context, cbid string, cookies []DeclaredCookieItem, replace bool) (Object, error) {
	body := map[string]any{"cookies": cookies}
	if replace {
		body["replace"] = true
	}
	return s.c.object(ctx, "PUT", sitePath(cbid, "/cookies"), dryRun, body)
}

// UndeclareCookieDryRun answers with what removing a declaration would leave —
// "declared" and a "diff" — and removes nothing (?dryRun=1).
func (s *SitesService) UndeclareCookieDryRun(ctx context.Context, cbid, name string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/cookies/"+esc(name)), dryRun, nil)
}

// Delete removes a stored image — DELETE /v1/assets/{fileName}. Pass the URL Upload
// returned, or just its file name. Only this org's images are reachable: the folder comes
// from the API key, not from the name sent.
func (s *AssetsService) Delete(ctx context.Context, urlOrFileName string) error {
	name := urlOrFileName
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return s.c.delete(ctx, "/v1/assets/"+esc(name))
}

// Roll rotates a key — POST /v1/keys/{prefix}/roll. The new secret is returned once and
// keeps the old key's name, scopes, property lock and expiry; the old secret stops
// working immediately.
func (s *KeysService) Roll(ctx context.Context, prefix string) (*ApiKeyIssued, error) {
	var out ApiKeyIssued
	if err := s.c.post(ctx, "/v1/keys/"+esc(prefix)+"/roll", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ApiKeyUpdate renames a key, or replaces its scopes or property lock. Only non-nil
// fields are sent; the secret is unchanged.
type ApiKeyUpdate struct {
	Name   *string
	Scopes []string
	Cbids  []string
	// ExpiresAt follows the three-way rule: nil leaves it, ClearKeyExpiry() removes it,
	// KeyExpiresAt(ms) sets it.
	ExpiresAt **int64
}

// ClearKeyExpiry removes a key's expiry.
func ClearKeyExpiry() **int64 { var none *int64; return &none }

// KeyExpiresAt sets a key's expiry (epoch ms).
func KeyExpiresAt(ms int64) **int64 { v := &ms; return &v }

// Report returns key hygiene — GET /v1/keys/report: keys unused for unusedDays (0 = default
// 90), keys with broad access, and keys expiring within expiringDays (0 = default 14).
func (s *KeysService) Report(ctx context.Context, unusedDays, expiringDays int) (Object, error) {
	q := url.Values{}
	if unusedDays > 0 {
		q.Set("unusedDays", strconv.Itoa(unusedDays))
	}
	if expiringDays > 0 {
		q.Set("expiringDays", strconv.Itoa(expiringDays))
	}
	return s.c.object(ctx, "GET", "/v1/keys/report", q, nil)
}

// Update changes a key's name, scopes or property lock — PATCH /v1/keys/{prefix}.
func (s *KeysService) Update(ctx context.Context, prefix string, patch ApiKeyUpdate) (Object, error) {
	body := map[string]any{}
	if patch.Name != nil {
		body["name"] = *patch.Name
	}
	if patch.Scopes != nil {
		body["scopes"] = patch.Scopes
	}
	if patch.Cbids != nil {
		body["cbids"] = patch.Cbids
	}
	if patch.ExpiresAt != nil {
		if *patch.ExpiresAt == nil {
			body["expiresAt"] = nil
		} else {
			body["expiresAt"] = **patch.ExpiresAt
		}
	}
	return s.c.object(ctx, "PATCH", "/v1/keys/"+esc(prefix), nil, body)
}

// RollSecret rotates a subscription's signing secret — POST /v1/webhooks/{id}/roll. The
// new secret is returned once; deliveries are signed with it from now on.
func (s *WebhooksService) RollSecret(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/webhooks/"+esc(id)+"/roll", nil, nil)
}

// Test sends a signed test event to a subscription now and reports what the endpoint
// answered — POST /v1/webhooks/{id}/test.
func (s *WebhooksService) Test(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/webhooks/"+esc(id)+"/test", nil, nil)
}

// DeadLetters returns deliveries that failed every retry, newest first —
// GET /v1/webhooks/dead-letters. A property-locked key sees only its own properties'.
func (s *WebhooksService) DeadLetters(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/webhooks/dead-letters", nil, nil)
}

// ReplayDeadLetter delivers a dead-lettered event again, to the subscription as it is
// now — POST /v1/webhooks/dead-letters/{id}/replay.
func (s *WebhooksService) ReplayDeadLetter(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/webhooks/dead-letters/"+esc(id)+"/replay", nil, nil)
}

// Replay sends what a failing or disabled endpoint missed since `since` (epoch ms or ISO
// date), oldest first, one attempt each, stopping at the first failure —
// POST /v1/webhooks/{id}/replay. limit 0 uses the server default (100).
func (s *WebhooksService) Replay(ctx context.Context, id, since string, limit int) (Object, error) {
	q := url.Values{"since": {since}}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	return s.c.object(ctx, "POST", "/v1/webhooks/"+esc(id)+"/replay", q, nil)
}

// Erase erases a subject's consent records on one site, for a deletion request past
// identity verification — POST /v1/dsar/{id}/erase. Irreversible, and noted on the
// request. Needs dsar:write and consent:write. When the response carries a "warning",
// no key was configured and nothing was cryptographically erased.
func (s *DSARService) Erase(ctx context.Context, id, cbid, stamp string) (Object, error) {
	body := map[string]any{"cbid": cbid, "stamp": stamp}
	return s.c.object(ctx, "POST", "/v1/dsar/"+esc(id)+"/erase", nil, body)
}

// Export returns a subject's consent records on one site, for an access or portability
// request past identity verification — POST /v1/dsar/{id}/export. Noted on the request.
// Needs dsar:write and consent:read.
func (s *DSARService) Export(ctx context.Context, id, cbid, stamp string) (Object, error) {
	body := map[string]any{"cbid": cbid, "stamp": stamp}
	return s.c.object(ctx, "POST", "/v1/dsar/"+esc(id)+"/export", nil, body)
}

// Get returns one subject's preference record — GET /v1/preferences/{subjectId}. A
// subject with none saved has empty purposes. Needs consent:read.
func (s *PreferencesService) Get(ctx context.Context, subjectID string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/preferences/"+esc(subjectID), nil, nil)
}

// ---- orchestrator v2: cases, grace setting, subject card --------------------

// SubjectIdentity is a signed-in identity from any issuer the site trusts; (Iss, Sub) is the key.
type SubjectIdentity struct {
	Iss       string `json:"iss"`
	Sub       string `json:"sub"`
	Ext       string `json:"ext,omitempty"`
	Principal string `json:"principal,omitempty"`
}

// CaseSubject is who a case is about.
type CaseSubject struct {
	Emails   []string         `json:"emails"`
	Identity *SubjectIdentity `json:"identity,omitempty"`
}

// CaseInput opens a case — POST /v1/dsar/requests. Type takes the canonical words (access,
// deletion, closure, restriction, rectification) or an alias (erasure, erase, export). Source
// "product" or "atlas" attests the person (needs dsar:write); "admin" files it received.
type CaseInput struct {
	Cbid         string       `json:"cbid"`
	Type         string       `json:"type"`
	Subject      *CaseSubject `json:"subject,omitempty"`
	SubjectToken string       `json:"subjectToken,omitempty"`
	Source       string       `json:"source,omitempty"`
	Reason       string       `json:"reason,omitempty"`
	SkipSystems  []string     `json:"skipSystems,omitempty"`
	Fulfill      *bool        `json:"fulfill,omitempty"`
	Test         bool         `json:"test,omitempty"`
}

// CaseOptions carries the headers of a case intake: an Idempotency-Key (16–128 characters) so a
// retry answers the same case, and an X-Trace-Id to keep.
type CaseOptions struct {
	IdempotencyKey string
	Trace          string
}

// OpenCase opens a case fanned out to every connected system — POST /v1/dsar/requests.
func (s *DSARService) OpenCase(ctx context.Context, in CaseInput, opts CaseOptions) (Object, error) {
	headers := map[string]string{}
	if opts.IdempotencyKey != "" {
		headers["Idempotency-Key"] = opts.IdempotencyKey
	}
	if opts.Trace != "" {
		headers["X-Trace-Id"] = opts.Trace
	}
	var out Object
	if err := s.c.doWithHeaders(ctx, "POST", "/v1/dsar/requests", nil, in, &out, headers); err != nil {
		return nil, err
	}
	return out, nil
}

// GetCase returns one case across every system — GET /v1/dsar/requests/{id}.
func (s *DSARService) GetCase(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/requests/"+esc(id), nil, nil)
}

// CancelCase cancels a case before executeAfter — POST /v1/dsar/requests/{id}/cancel. Every
// system keeps its data.
func (s *DSARService) CancelCase(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/requests/"+esc(id)+"/cancel", nil, nil)
}

// CaseReceipt returns the signed (Ed25519) receipt for a case — GET /v1/dsar/requests/{id}/receipt.
func (s *DSARService) CaseReceipt(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/dsar/requests/"+esc(id)+"/receipt", nil, nil)
}

// ListByTrace returns the request carrying one X-Trace-Id — GET /v1/dsar?trace=.
func (s *DSARService) ListByTrace(ctx context.Context, trace string) ([]Object, error) {
	var out []Object
	if err := s.c.get(ctx, "/v1/dsar", url.Values{"trace": {trace}}, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DsarSettingsService is a site's rights-request settings: GraceDays (0–30; 7 for a new site, 0 until set for an existing one), how long an
// erasure or account closure waits before any system acts. Needs dsar:read / dsar:write.
type DsarSettingsService struct{ c *Client }

// Get — GET /v1/sites/{cbid}/dsar-settings.
func (s *DsarSettingsService) Get(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/dsar-settings"), nil, nil)
}

// Set — PUT /v1/sites/{cbid}/dsar-settings. Applies to cases opened afterwards.
func (s *DsarSettingsService) Set(ctx context.Context, cbid string, graceDays int) (Object, error) {
	return s.c.object(ctx, "PUT", sitePath(cbid, "/dsar-settings"), nil, map[string]any{"graceDays": graceDays})
}

// IntegrationsService is the cards for support, CRM and helpdesk sidebars (dsar:read), and
// identity-provider connect (dsar:write).
type IntegrationsService struct{ c *Client }

// SubjectCard lists a person's open rights requests — GET /v1/integrations/{app}/card — by iss and
// sub (preferred) or email. Never matched across identities on a shared email.
func (s *IntegrationsService) SubjectCard(ctx context.Context, app, iss, sub, email string) (Object, error) {
	q := url.Values{}
	if iss != "" {
		q.Set("iss", iss)
	}
	if sub != "" {
		q.Set("sub", sub)
	}
	if email != "" {
		q.Set("email", email)
	}
	return s.c.object(ctx, "GET", "/v1/integrations/"+esc(app)+"/card", q, nil)
}

// Connect connects an identity provider's privacy orchestrator — POST /v1/integrations/{provider}/connect:
// it mints a dsar:write key locked to cbid and hands it to the provider, authorised with providerKey.
// The minted key is never returned, only its prefix. connector ({url, method, body}) is only for a
// provider with no built-in connector (nil otherwise). Needs dsar:write (no keys:write); dryRun mints
// and sends nothing.
func (s *IntegrationsService) Connect(ctx context.Context, provider, cbid, providerKey string, connector Object, dryRun bool) (Object, error) {
	var q url.Values
	if dryRun {
		q = dryRunQuery()
	}
	body := map[string]any{"cbid": cbid, "providerKey": providerKey}
	if connector != nil {
		body["connector"] = connector
	}
	return s.c.object(ctx, "POST", "/v1/integrations/"+esc(provider)+"/connect", q, body)
}

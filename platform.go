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
func (s *FulfillmentService) ConnectExecutor(ctx context.Context, input ExecutorInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/dsar/executors", nil, input)
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

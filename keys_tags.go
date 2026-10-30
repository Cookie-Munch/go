package cookiemunch

import (
	"context"
	"net/url"
	"strings"
)

// metaQuery is ?metadata[k]=v tag filters for list endpoints.
func metaQuery(metadata map[string]string) url.Values {
	if len(metadata) == 0 {
		return nil
	}
	q := url.Values{}
	for k, v := range metadata {
		q.Set("metadata["+k+"]", v)
	}
	return q
}

// ---- the key model (CMU-D09, S02, X35) ----

// ListStandard lists the keys in the suite-standard shape — GET /v1/keys?shape=standard.
func (s *KeysService) ListStandard(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/keys", url.Values{"shape": {"standard"}}, nil)
}

// Self is the calling key itself, with any key — GET /v1/keys/self: scopes, properties, expiry, owner.
func (s *KeysService) Self(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/keys/self", nil, nil)
}

// UpdateSelf re-pins the calling key to another dated API version (any key, for itself) —
// PATCH /v1/keys/self.
func (s *KeysService) UpdateSelf(ctx context.Context, apiVersion string) (Object, error) {
	return s.c.object(ctx, "PATCH", "/v1/keys/self", nil, map[string]any{"apiVersion": apiVersion})
}

// RollSelf rotates the calling key — POST /v1/keys/self/roll. The old secret stops at once; make the
// client WithHeader("Idempotency-Key", …) so a lost answer replays.
func (s *KeysService) RollSelf(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/keys/self/roll", nil, nil)
}

// VendInput is a short-lived key: Scopes (a subset of the caller's) and TTLSeconds (60 to 43200).
type VendInput struct {
	Scopes     []string `json:"scopes"`
	TTLSeconds int      `json:"ttlSeconds"`
	Cbids      []string `json:"cbids,omitempty"`
	Name       string   `json:"name,omitempty"`
}

// Vend mints a short-lived key (at most 12 hours) within this key's own scopes and properties —
// POST /v1/keys/vend. Needs keys:write or a full-access key.
func (s *KeysService) Vend(ctx context.Context, in VendInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/keys/vend", nil, in)
}

// OperatorTokenExchange swaps an identity provider's token for a short-lived, scoped key (RFC 8693).
type OperatorTokenExchange struct {
	SubjectToken string
	// SubjectTokenType is access_token (default), jwt or id_token.
	SubjectTokenType string
	Scope            []string
	// Resource is "org:<id>" or "site:<cbid>".
	Resource string
	// ExpiresIn is seconds, 60 to 3600; 0 means the default (3600).
	ExpiresIn int
}

// ExchangeOperatorToken posts POST /v1/oauth/token. It needs no API key: the route never reads one,
// so a client made with any placeholder key works. The key it returns lives at most an hour.
func (s *KeysService) ExchangeOperatorToken(ctx context.Context, in OperatorTokenExchange) (Object, error) {
	tt := in.SubjectTokenType
	if tt == "" {
		tt = "access_token"
	}
	body := map[string]any{
		"grant_type": "urn:ietf:params:oauth:grant-type:token-exchange", "subject_token": in.SubjectToken,
		"subject_token_type": "urn:ietf:params:oauth:token-type:" + tt, "scope": strings.Join(in.Scope, " "),
		"resource": in.Resource, "audience": "cookiemunch",
	}
	if in.ExpiresIn > 0 {
		body["expires_in"] = in.ExpiresIn
	}
	return s.c.object(ctx, "POST", "/v1/oauth/token", nil, body)
}

// RevokeLeaked revokes a published secret — POST /v1/keys/revoke_leaked. It needs no valid key and
// always answers {"received": n}. source ("" for none) is where the secret was found.
func (s *KeysService) RevokeLeaked(ctx context.Context, key, source string) (Object, error) {
	body := map[string]any{"key": key}
	if source != "" {
		body["source"] = source
	}
	return s.c.object(ctx, "POST", "/v1/keys/revoke_leaked", nil, body)
}

// ---- organisations ----

// CreateOrgOptions are POST /v1/orgs's optional fields.
type CreateOrgOptions struct {
	MintKey          bool
	KeyScopes        []string
	KeyExpiresInDays int
	KeyOwner         string
	ExternalRef      string
	Metadata         map[string]any
}

// CreateOrgWith creates a sibling organisation with its first key and tags — POST /v1/orgs. A key with
// the orgs:create scope must send KeyScopes; the minted key expires in KeyExpiresInDays (default 365).
func (c *Client) CreateOrgWith(ctx context.Context, name string, opts CreateOrgOptions) (Object, error) {
	body := map[string]any{"name": name}
	if opts.MintKey {
		body["mintKey"] = true
	}
	if opts.KeyScopes != nil {
		body["keyScopes"] = opts.KeyScopes
	}
	if opts.KeyExpiresInDays > 0 {
		body["keyExpiresInDays"] = opts.KeyExpiresInDays
	}
	if opts.KeyOwner != "" {
		body["keyOwner"] = opts.KeyOwner
	}
	if opts.ExternalRef != "" {
		body["externalRef"] = opts.ExternalRef
	}
	if opts.Metadata != nil {
		body["metadata"] = opts.Metadata
	}
	return c.object(ctx, "POST", "/v1/orgs", nil, body)
}

// ListOrgsTagged is ListOrgs keeping only organisations with these tags — GET /v1/orgs?metadata[k]=v.
func (c *Client) ListOrgsTagged(ctx context.Context, metadata map[string]string) (Object, error) {
	return c.object(ctx, "GET", "/v1/orgs", metaQuery(metadata), nil)
}

// DeleteSandboxOrg removes a SANDBOX organisation of this account — DELETE /v1/orgs/{id}?hard=1.
// Anything not created with metadata.stack_sandbox is 403 not_a_sandbox.
func (c *Client) DeleteSandboxOrg(ctx context.Context, id string) (Object, error) {
	return c.object(ctx, "DELETE", "/v1/orgs/"+esc(id), url.Values{"hard": {"1"}}, nil)
}

// ProvisionEnv is what env writes for the product provisioned with this stack ref —
// GET /v1/provision/env: names, non-secret values, and references for secrets.
func (c *Client) ProvisionEnv(ctx context.Context, ref string) (Object, error) {
	return c.object(ctx, "GET", "/v1/provision/env", url.Values{"ref": {ref}}, nil)
}

// ProvisionFrom is Provision resuming from a stable step id (fromStep; "" for none) — a version 2 body.
func (c *Client) ProvisionFrom(ctx context.Context, input Object, dryRun bool, fromStep string) (Object, error) {
	q := url.Values{}
	if dryRun {
		q.Set("dryRun", "1")
	}
	if fromStep != "" {
		q.Set("fromStep", fromStep)
	}
	return c.object(ctx, "POST", "/v1/provision", q, input)
}

// ---- sites ----

// ListTagged lists sites with these tags — GET /v1/sites?metadata[k]=v.
func (s *SitesService) ListTagged(ctx context.Context, metadata map[string]string) (Object, error) {
	var out []any
	if err := s.c.get(ctx, "/v1/sites", metaQuery(metadata), &out); err != nil {
		return nil, err
	}
	return Object{"sites": out}, nil
}

// DeleteSandbox purges a SANDBOX site — DELETE /v1/sites/{cbid}?hard=1: its consent ledger, declarations
// and test rights requests too. Anything not created as a sandbox is 403 not_a_sandbox.
func (s *SitesService) DeleteSandbox(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, ""), url.Values{"hard": {"1"}}, nil)
}

// ---- tags (CMU-X29) ----

// MetadataService is tags on created resources (type: org, site, webhook, executor, dsar, partner_link).
type MetadataService struct{ c *Client }

// Get returns a resource's tags, externalRef and sandbox flag — GET /v1/metadata/{type}/{id}.
func (s *MetadataService) Get(ctx context.Context, typ, id string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/metadata/"+esc(typ)+"/"+esc(id), nil, nil)
}

// Update merges tags (a nil value removes one) — PATCH /v1/metadata/{type}/{id}. stack_sandbox
// cannot be set here. dryRun answers with the diff.
func (s *MetadataService) Update(ctx context.Context, typ, id string, metadata map[string]any, dryRun bool) (Object, error) {
	var q url.Values
	if dryRun {
		q = dryRunQuery()
	}
	return s.c.object(ctx, "PATCH", "/v1/metadata/"+esc(typ)+"/"+esc(id), q, map[string]any{"metadata": metadata})
}

// ---- partner links (CMU-X34) ----

// PartnerLinksService is suite-standard partner links.
type PartnerLinksService struct{ c *Client }

// StandardPartnerLinkInput is POST /v1/partner_links: Resource is "site:<cbid>" or "org:<id>".
type StandardPartnerLinkInput struct {
	Partner    string   `json:"partner"`
	Resource   string   `json:"resource"`
	Scopes     []string `json:"scopes"`
	ExpiresAt  string   `json:"expires_at,omitempty"`
	ExternalID string   `json:"externalId,omitempty"`
	Label      string   `json:"label,omitempty"`
}

// List lists every partner link of the organisation — GET /v1/partner_links (nil metadata: all).
func (s *PartnerLinksService) List(ctx context.Context, metadata map[string]string) ([]Object, error) {
	var out []Object
	if err := s.c.get(ctx, "/v1/partner_links", metaQuery(metadata), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create makes a partner link — POST /v1/partner_links. It never grants more than this key holds.
func (s *PartnerLinksService) Create(ctx context.Context, in StandardPartnerLinkInput) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/partner_links", nil, in)
}

// Revoke revokes a partner link at once — DELETE /v1/partner_links/{id}.
func (s *PartnerLinksService) Revoke(ctx context.Context, id string) (Object, error) {
	return s.c.object(ctx, "DELETE", "/v1/partner_links/"+esc(id), nil, nil)
}

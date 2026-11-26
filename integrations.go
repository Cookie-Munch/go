package cookiemunch

import (
	"context"
	"net/url"
	"strconv"
)

// SiteUpdate renames or moves a site. Only non-nil fields are sent. Name follows the three-way
// rule: nil leaves it, ClearSiteName() removes it, SiteName(v) sets it. Aliases replaces the
// list when non-nil; an empty, non-nil slice clears it.
type SiteUpdate struct {
	Domain  *string
	Name    **string
	Aliases []string
	// KeepOldDomain, with a new Domain, keeps the old domain as an alias while both run.
	KeepOldDomain bool
}

// ClearSiteName removes a site's display name (the domain is shown again).
func ClearSiteName() **string { var none *string; return &none }

// SiteName sets a site's display name.
func SiteName(name string) **string { v := &name; return &v }

func siteUpdateBody(p SiteUpdate) map[string]any {
	body := map[string]any{}
	if p.Domain != nil {
		body["domain"] = *p.Domain
	}
	if p.Name != nil {
		if *p.Name == nil {
			body["name"] = nil
		} else {
			body["name"] = **p.Name
		}
	}
	if p.Aliases != nil {
		body["aliases"] = p.Aliases
	}
	if p.KeepOldDomain {
		body["keepOldDomain"] = true
	}
	return body
}

// Tenant is a site for one of your customers (a creator, a shop), the controller for its own visitors.
// Path or Domains; zero values are not sent.
type Tenant struct {
	ExternalID string         `json:"externalId"`
	Name       string         `json:"name,omitempty"`
	Path       string         `json:"path,omitempty"`
	Domains    []string       `json:"domains,omitempty"`
	Controller map[string]any `json:"controller,omitempty"`
	Inherit    []string       `json:"inherit,omitempty"`
}

func tenantsPath(parent string, rest string) string { return sitePath(parent, "/tenants"+rest) }

func maybeDryRun(dryRun bool) url.Values {
	if dryRun {
		return dryRunQuery()
	}
	return nil
}

// CreateTenant creates a tenant, idempotent by ExternalID — POST /v1/sites/{cbid}/tenants.
func (s *SitesService) CreateTenant(ctx context.Context, parentCbid string, in Tenant, dryRun bool) (Object, error) {
	return s.c.object(ctx, "POST", tenantsPath(parentCbid, ""), maybeDryRun(dryRun), in)
}

// CreateTenantsBulk creates up to 100 tenants; send the same list again to resume — POST /v1/sites/{cbid}/tenants/bulk.
func (s *SitesService) CreateTenantsBulk(ctx context.Context, parentCbid string, tenants []Tenant, dryRun bool) (Object, error) {
	return s.c.object(ctx, "POST", tenantsPath(parentCbid, "/bulk"), maybeDryRun(dryRun), map[string]any{"tenants": tenants})
}

// ListTenants returns one page {tenants, nextCursor, total} — GET /v1/sites/{cbid}/tenants. limit 0 is the default.
func (s *SitesService) ListTenants(ctx context.Context, parentCbid string, limit int, cursor string) (Object, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	return s.c.object(ctx, "GET", tenantsPath(parentCbid, ""), q, nil)
}

// GetTenant returns one tenant by your id for the customer — GET /v1/sites/{cbid}/tenants/{externalId}.
func (s *SitesService) GetTenant(ctx context.Context, parentCbid, externalID string) (Object, error) {
	return s.c.object(ctx, "GET", tenantsPath(parentCbid, "/"+esc(externalID)), nil, nil)
}

// UpdateTenant changes a tenant's name, path or controller — PATCH /v1/sites/{cbid}/tenants/{externalId}.
func (s *SitesService) UpdateTenant(ctx context.Context, parentCbid, externalID string, patch map[string]any, dryRun bool) (Object, error) {
	return s.c.object(ctx, "PATCH", tenantsPath(parentCbid, "/"+esc(externalID)), maybeDryRun(dryRun), patch)
}

// DeleteTenant deletes a tenant and everything its site holds — DELETE /v1/sites/{cbid}/tenants/{externalId}. Irreversible.
func (s *SitesService) DeleteTenant(ctx context.Context, parentCbid, externalID string, dryRun bool) error {
	return s.c.do(ctx, "DELETE", tenantsPath(parentCbid, "/"+esc(externalID)), maybeDryRun(dryRun), nil, nil)
}

// SiteMove is the body of Move. To is required; zero values of the rest are not sent.
type SiteMove struct {
	To           string `json:"to"`
	From         string `json:"from,omitempty"`
	KeepOld      *bool  `json:"keepOld,omitempty"`
	TrustOldDays *int   `json:"trustOldDays,omitempty"`
}

// Move moves a site to a new domain: every stored value naming the old host, rewritten —
// POST /v1/sites/{cbid}/move (dryRun: ?dryRun=1). After a refusal, send retry.body again.
func (s *SitesService) Move(ctx context.Context, cbid string, in SiteMove, dryRun bool) (Object, error) {
	var q url.Values
	if dryRun {
		q = dryRunQuery()
	}
	return s.c.object(ctx, "POST", sitePath(cbid, "/move"), q, in)
}

// Update renames or moves a site — PATCH /v1/sites/{cbid}. The cbid and the consent ledger
// stay; a domain move clears verification. The result is {site, diff, effects, warnings?}.
func (s *SitesService) Update(ctx context.Context, cbid string, patch SiteUpdate) (Object, error) {
	return s.c.object(ctx, "PATCH", sitePath(cbid, ""), nil, siteUpdateBody(patch))
}

// DryRunUpdate answers with the diff and effects Update would have, storing nothing —
// PATCH /v1/sites/{cbid}?dryRun=1.
func (s *SitesService) DryRunUpdate(ctx context.Context, cbid string, patch SiteUpdate) (Object, error) {
	return s.c.object(ctx, "PATCH", sitePath(cbid, ""), dryRunQuery(), siteUpdateBody(patch))
}

// PartnerLinkInput grants a partner platform ("forumulate", "atlas") a site's consent events.
type PartnerLinkInput struct {
	Partner    string   `json:"partner"`
	ExternalID string   `json:"externalId"`
	Label      string   `json:"label,omitempty"`
	Events     []string `json:"events,omitempty"`
}

// ListPartners returns the site's partner links and the partner catalogue —
// GET /v1/sites/{cbid}/partners.
func (s *SitesService) ListPartners(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/partners"), nil, nil)
}

// LinkPartner grants a partner the site's consent events; no secret is created and a repeat is
// a no-op — POST /v1/sites/{cbid}/partners. The result is {change, link}.
func (s *SitesService) LinkPartner(ctx context.Context, cbid string, input PartnerLinkInput) (Object, error) {
	return s.c.object(ctx, "POST", sitePath(cbid, "/partners"), nil, input)
}

// UnlinkPartner revokes a partner link; deliveries stop at once —
// DELETE /v1/sites/{cbid}/partners/{linkId}.
func (s *SitesService) UnlinkPartner(ctx context.Context, cbid, linkID string) (Object, error) {
	return s.c.object(ctx, "DELETE", sitePath(cbid, "/partners/"+esc(linkID)), nil, nil)
}

// UpdatePartner fixes a site link's externalId or label in place, keeping the grant —
// PATCH /v1/sites/{cbid}/partners/{linkId} (dryRun: ?dryRun=1). The result is {change, link, steps, nextSteps, warnings?}.
func (s *SitesService) UpdatePartner(ctx context.Context, cbid, linkID string, in PartnerLinkUpdate, dryRun bool) (Object, error) {
	var q url.Values
	if dryRun {
		q = dryRunQuery()
	}
	return s.c.object(ctx, "PATCH", sitePath(cbid, "/partners/"+esc(linkID)), q, in)
}

// IntegrationHealth returns one row per link a site's privacy wiring depends on, each with
// "ok" (null = could not check) and a "fixHint" — GET /v1/integration-health. cbid may be
// empty when the org has one site.
func (c *Client) IntegrationHealth(ctx context.Context, cbid string) (Object, error) {
	var q url.Values
	if cbid != "" {
		q = url.Values{"cbid": {cbid}}
	}
	return c.object(ctx, "GET", "/v1/integration-health", q, nil)
}

// HealthWaivers lists the checks the org chose not to meet — GET /v1/integration-health/waivers.
func (c *Client) HealthWaivers(ctx context.Context, cbid string) (Object, error) {
	var q url.Values
	if cbid != "" {
		q = url.Values{"cbid": {cbid}}
	}
	return c.object(ctx, "GET", "/v1/integration-health/waivers", q, nil)
}

// SetHealthWaiver waives a check — PUT /v1/integration-health/waivers {cbid?, check, subject?,
// reason, until?}. The row then reports ok: null with waived. Audited.
func (c *Client) SetHealthWaiver(ctx context.Context, input map[string]any) (Object, error) {
	return c.object(ctx, "PUT", "/v1/integration-health/waivers", nil, input)
}

// RemoveHealthWaiver removes a waiver — DELETE /v1/integration-health/waivers. Empty cbid or
// subject are omitted (subject defaults to every subject).
func (c *Client) RemoveHealthWaiver(ctx context.Context, check, cbid, subject string) error {
	q := url.Values{"check": {check}}
	if cbid != "" {
		q.Set("cbid", cbid)
	}
	if subject != "" {
		q.Set("subject", subject)
	}
	_, err := c.object(ctx, "DELETE", "/v1/integration-health/waivers", q, nil)
	return err
}

// EmbedManifest returns the site's embed manifest — GET /v1/embed?cbid=, the tenant variant of
// /.well-known/cookiemunch-embed.json: every origin by CSP directive and every browser key the embed
// writes for this site's config. Public: no key is needed.
func (c *Client) EmbedManifest(ctx context.Context, cbid string) (Object, error) {
	return c.object(ctx, "GET", "/v1/embed", url.Values{"cbid": {cbid}}, nil)
}

// Provision wires a product's privacy in one call (site, connected systems, Atlas, Forumulate)
// — POST /v1/provision. Idempotent; dryRun writes nothing anywhere. Secrets in input are used
// for this request only; none is returned.
func (c *Client) Provision(ctx context.Context, input Object, dryRun bool) (Object, error) {
	var q url.Values
	if dryRun {
		q = dryRunQuery()
	}
	return c.object(ctx, "POST", "/v1/provision", q, input)
}

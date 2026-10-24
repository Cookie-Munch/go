package cookiemunch

import (
	"context"
	"net/url"
)

// SiteUpdate renames or moves a site. Only non-nil fields are sent. Name follows the three-way
// rule: nil leaves it, ClearSiteName() removes it, SiteName(v) sets it. Aliases replaces the
// list when non-nil; an empty, non-nil slice clears it.
type SiteUpdate struct {
	Domain  *string
	Name    **string
	Aliases []string
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
	return body
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
// /.well-known/forgedeck-embed.json: every origin by CSP directive and every browser key the embed
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

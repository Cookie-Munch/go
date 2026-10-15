package cookiemunch

import (
	"context"
	"net/url"
)

// RightsPageService groups /v1/sites/{cbid}/rights-page — the hosted rights page document.
type RightsPageService struct{ c *Client }

func rightsPagePath(cbid, suffix string) string { return sitePath(cbid, "/rights-page"+suffix) }

// Get returns the draft (the default document until first saved) with publish warnings.
func (s *RightsPageService) Get(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", rightsPagePath(cbid, ""), nil, nil)
}

// Put replaces the draft. Nothing is live until Publish.
// Add "baseUpdatedAt" (the draft's updatedAt as last read; nil if it had none) to page to
// refuse overwriting a newer save: a mismatch is a 409 draft_conflict.
func (s *RightsPageService) Put(ctx context.Context, cbid string, page Object) (Object, error) {
	return s.c.object(ctx, "PUT", rightsPagePath(cbid, ""), nil, page)
}

// Patch merges into the draft: objects merge, arrays replace, nil removes.
func (s *RightsPageService) Patch(ctx context.Context, cbid string, page Object) (Object, error) {
	return s.c.object(ctx, "PATCH", rightsPagePath(cbid, ""), nil, page)
}

// DryRun validates a write without saving it; replace=true previews a PUT.
func (s *RightsPageService) DryRun(ctx context.Context, cbid string, page Object, replace bool) (Object, error) {
	method := "PATCH"
	if replace {
		method = "PUT"
	}
	return s.c.object(ctx, method, rightsPagePath(cbid, ""), url.Values{"dryRun": {"1"}}, page)
}

// GetPublished returns the live document.
func (s *RightsPageService) GetPublished(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", rightsPagePath(cbid, "/published"), nil, nil)
}

// PublishOption adjusts a Publish or Rollback request.
type PublishOption func(Object)

// WithBaseUpdatedAt sends baseUpdatedAt: the draft's updatedAt as you last read it (epoch ms
// or an ISO date; nil if it had none). If the draft changed since, the call is refused with
// a 409 draft_conflict and nothing goes live.
func WithBaseUpdatedAt(v any) PublishOption {
	return func(b Object) { b["baseUpdatedAt"] = v }
}

// Publish publishes the draft as the next version; it goes live at once. Pass
// WithBaseUpdatedAt so only the draft you read goes live.
func (s *RightsPageService) Publish(ctx context.Context, cbid string, opts ...PublishOption) (Object, error) {
	body := Object{}
	for _, o := range opts {
		o(body)
	}
	return s.c.object(ctx, "POST", rightsPagePath(cbid, "/publish"), nil, body)
}

// Versions lists published versions, newest first.
func (s *RightsPageService) Versions(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", rightsPagePath(cbid, "/versions"), nil, nil)
}

// Rollback publishes an earlier version again; it goes live at once and REPLACES the
// current draft (unpublished edits are lost). WithBaseUpdatedAt refuses with a 409
// draft_conflict if the draft changed since you read it.
func (s *RightsPageService) Rollback(ctx context.Context, cbid string, version int, opts ...PublishOption) (Object, error) {
	body := Object{"version": version}
	for _, o := range opts {
		o(body)
	}
	return s.c.object(ctx, "POST", rightsPagePath(cbid, "/rollback"), nil, body)
}

func rightsDomainPath(cbid, hostname string) string {
	p := sitePath(cbid, "/rights-page/domains")
	if hostname != "" {
		p += "/" + url.PathEscape(hostname)
	}
	return p
}

// ListDomains returns the custom hostnames serving the rights page, with the plan
// allowance: {"domains", "limit" (nil = unlimited), "used", "target"} —
// GET /v1/sites/{cbid}/rights-page/domains.
func (s *RightsPageService) ListDomains(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", rightsDomainPath(cbid, ""), nil, nil)
}

// AddDomain adds a custom hostname (e.g. privacy.example.com); the answer's "dns" holds
// the exact records to create — POST /v1/sites/{cbid}/rights-page/domains.
func (s *RightsPageService) AddDomain(ctx context.Context, cbid, hostname string) (Object, error) {
	return s.c.object(ctx, "POST", rightsDomainPath(cbid, ""), nil, map[string]string{"hostname": hostname})
}

// VerifyDomain checks a hostname now: DNS while pending, the HTTPS answer once routed —
// POST /v1/sites/{cbid}/rights-page/domains/{hostname}/verify.
func (s *RightsPageService) VerifyDomain(ctx context.Context, cbid, hostname string) (Object, error) {
	return s.c.object(ctx, "POST", rightsDomainPath(cbid, hostname)+"/verify", nil, nil)
}

// RemoveDomain removes a hostname: it stops being served and its certificate is not
// renewed — DELETE /v1/sites/{cbid}/rights-page/domains/{hostname}.
func (s *RightsPageService) RemoveDomain(ctx context.Context, cbid, hostname string) error {
	return s.c.delete(ctx, rightsDomainPath(cbid, hostname))
}

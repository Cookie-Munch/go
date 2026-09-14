package cookiemunch

import (
	"context"
	"net/url"
)

// ---- DSAR -------------------------------------------------------------------

// DSARService groups the /v1/dsar endpoints.
type DSARService struct{ c *Client }

// List returns all DSARs — GET /v1/dsar.
func (s *DSARService) List(ctx context.Context) ([]DsarRequest, error) {
	var out []DsarRequest
	if err := s.c.get(ctx, "/v1/dsar", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create files a new DSAR — POST /v1/dsar.
func (s *DSARService) Create(ctx context.Context, input DsarCreate) (*DsarRequest, error) {
	var out dsarWrap
	if err := s.c.post(ctx, "/v1/dsar", input, &out); err != nil {
		return nil, err
	}
	return &out.Request, nil
}

// Advance moves a DSAR to a new status — POST /v1/dsar/{id}/advance.
func (s *DSARService) Advance(ctx context.Context, id, toStatus string) (*DsarRequest, error) {
	var out dsarWrap
	path := "/v1/dsar/" + url.PathEscape(id) + "/advance"
	if err := s.c.post(ctx, path, map[string]string{"toStatus": toStatus}, &out); err != nil {
		return nil, err
	}
	return &out.Request, nil
}

// ---- vendors ----------------------------------------------------------------

// VendorsService groups the /v1/vendors endpoints.
type VendorsService struct{ c *Client }

// List returns vendors with their risk scores — GET /v1/vendors.
func (s *VendorsService) List(ctx context.Context) ([]ScoredVendor, error) {
	var out []ScoredVendor
	if err := s.c.get(ctx, "/v1/vendors", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create adds a vendor and returns it with a computed risk score —
// POST /v1/vendors.
func (s *VendorsService) Create(ctx context.Context, input VendorInput) (*VendorCreateResult, error) {
	var out VendorCreateResult
	if err := s.c.post(ctx, "/v1/vendors", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- RoPA -------------------------------------------------------------------

// RoPAService groups the /v1/ropa endpoints.
type RoPAService struct{ c *Client }

// List returns all RoPA entries — GET /v1/ropa.
func (s *RoPAService) List(ctx context.Context) ([]RopaEntry, error) {
	var out []RopaEntry
	if err := s.c.get(ctx, "/v1/ropa", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create adds a RoPA entry — POST /v1/ropa.
func (s *RoPAService) Create(ctx context.Context, input RopaInput) (*RopaEntry, error) {
	var out ropaWrap
	if err := s.c.post(ctx, "/v1/ropa", input, &out); err != nil {
		return nil, err
	}
	return &out.Entry, nil
}

// ---- brand kits -------------------------------------------------------------

// BrandKitsService groups the /v1/brand-kits endpoints.
type BrandKitsService struct{ c *Client }

// List returns the org's reusable banner themes — GET /v1/brand-kits.
func (s *BrandKitsService) List(ctx context.Context) ([]BrandKit, error) {
	var out []BrandKit
	if err := s.c.get(ctx, "/v1/brand-kits", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create adds a brand kit — POST /v1/brand-kits.
func (s *BrandKitsService) Create(ctx context.Context, input BrandKitCreate) (*BrandKit, error) {
	var out brandKitWrap
	if err := s.c.post(ctx, "/v1/brand-kits", input, &out); err != nil {
		return nil, err
	}
	return &out.Kit, nil
}

// Delete removes a brand kit — DELETE /v1/brand-kits/{id}.
func (s *BrandKitsService) Delete(ctx context.Context, id string) error {
	return s.c.delete(ctx, "/v1/brand-kits/"+url.PathEscape(id))
}

// ---- preferences ------------------------------------------------------------

// PreferencesService groups the /v1/preferences endpoints.
type PreferencesService struct{ c *Client }

// List returns the org's preference-center records — GET /v1/preferences.
func (s *PreferencesService) List(ctx context.Context) ([]PreferenceItem, error) {
	var out []PreferenceItem
	if err := s.c.get(ctx, "/v1/preferences", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Save records a subject's purpose choices — POST /v1/preferences. The response
// shape is open; it is returned as a generic map.
func (s *PreferencesService) Save(ctx context.Context, subjectID string, purposes map[string]bool) (map[string]any, error) {
	var out map[string]any
	body := map[string]any{"subjectId": subjectID, "purposes": purposes}
	if err := s.c.post(ctx, "/v1/preferences", body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ---- members ----------------------------------------------------------------

// MembersService groups the /v1/members endpoints.
type MembersService struct{ c *Client }

// List returns the org's members — GET /v1/members.
func (s *MembersService) List(ctx context.Context) ([]Member, error) {
	var out []Member
	if err := s.c.get(ctx, "/v1/members", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Invite invites a member by email with a role — POST /v1/members.
func (s *MembersService) Invite(ctx context.Context, email, role string) (*Member, error) {
	var out memberWrap
	if err := s.c.post(ctx, "/v1/members", map[string]string{"email": email, "role": role}, &out); err != nil {
		return nil, err
	}
	return &out.Member, nil
}

// SetRole changes a member's role — PATCH /v1/members/{userId}.
func (s *MembersService) SetRole(ctx context.Context, userID, role string) (*Member, error) {
	var out memberWrap
	path := "/v1/members/" + url.PathEscape(userID)
	if err := s.c.patch(ctx, path, map[string]string{"role": role}, &out); err != nil {
		return nil, err
	}
	return &out.Member, nil
}

// Remove removes a member — DELETE /v1/members/{userId}.
func (s *MembersService) Remove(ctx context.Context, userID string) error {
	return s.c.delete(ctx, "/v1/members/"+url.PathEscape(userID))
}

// ---- keys -------------------------------------------------------------------

// KeysService groups the /v1/keys endpoints.
type KeysService struct{ c *Client }

// List returns API-key prefixes (display metadata; never the secret) —
// GET /v1/keys.
func (s *KeysService) List(ctx context.Context) ([]ApiKeyPrefix, error) {
	var out []ApiKeyPrefix
	if err := s.c.get(ctx, "/v1/keys", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Issue creates a new API key. The returned Key is shown ONCE — POST /v1/keys.
func (s *KeysService) Issue(ctx context.Context, input *ApiKeyIssueInput) (*ApiKeyIssued, error) {
	var body any = map[string]any{}
	if input != nil {
		body = input
	}
	var out ApiKeyIssued
	if err := s.c.post(ctx, "/v1/keys", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ---- webhooks ---------------------------------------------------------------

// WebhooksService groups the /v1/webhooks endpoints.
type WebhooksService struct{ c *Client }

// List returns webhook subscriptions (secrets omitted) — GET /v1/webhooks.
func (s *WebhooksService) List(ctx context.Context) ([]WebhookSubscription, error) {
	var out []WebhookSubscription
	if err := s.c.get(ctx, "/v1/webhooks", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create adds a webhook subscription (the secret is returned ONCE) —
// POST /v1/webhooks.
func (s *WebhooksService) Create(ctx context.Context, input WebhookCreate) (*WebhookSubscription, error) {
	var out WebhookSubscription
	if err := s.c.post(ctx, "/v1/webhooks", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a webhook subscription — DELETE /v1/webhooks/{id}.
func (s *WebhooksService) Delete(ctx context.Context, id string) error {
	return s.c.delete(ctx, "/v1/webhooks/"+url.PathEscape(id))
}

// ---- banners ----------------------------------------------------------------

// BannersService groups the /v1/banners endpoints.
type BannersService struct{ c *Client }

// List returns the org's reusable banner designs — GET /v1/banners.
func (s *BannersService) List(ctx context.Context) ([]BannerSummary, error) {
	var out []BannerSummary
	if err := s.c.get(ctx, "/v1/banners", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create adds a reusable banner design — POST /v1/banners.
func (s *BannersService) Create(ctx context.Context, input BannerCreate) (*BannerRecord, error) {
	var out BannerRecord
	if err := s.c.post(ctx, "/v1/banners", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches one banner design — GET /v1/banners/{id}.
func (s *BannersService) Get(ctx context.Context, id string) (*BannerRecord, error) {
	var out BannerRecord
	if err := s.c.get(ctx, "/v1/banners/"+url.PathEscape(id), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Update changes a banner design's name and/or json — PUT /v1/banners/{id}.
func (s *BannersService) Update(ctx context.Context, id string, patch BannerUpdate) (*BannerRecord, error) {
	var out BannerRecord
	if err := s.c.put(ctx, "/v1/banners/"+url.PathEscape(id), patch, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a banner design (fails while it is still assigned) —
// DELETE /v1/banners/{id}.
func (s *BannersService) Delete(ctx context.Context, id string) error {
	return s.c.delete(ctx, "/v1/banners/"+url.PathEscape(id))
}

// Assignments lists the site cbids a design is assigned to —
// GET /v1/banners/{id}/assignments.
func (s *BannersService) Assignments(ctx context.Context, id string) (*BannerAssignments, error) {
	var out BannerAssignments
	if err := s.c.get(ctx, "/v1/banners/"+url.PathEscape(id)+"/assignments", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetAssignments sets the sites a design is assigned to —
// PUT /v1/banners/{id}/assignments.
func (s *BannersService) SetAssignments(ctx context.Context, id string, cbids []string) (*BannerAssignments, error) {
	var out BannerAssignments
	body := map[string][]string{"cbids": cbids}
	if err := s.c.put(ctx, "/v1/banners/"+url.PathEscape(id)+"/assignments", body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Publish compiles the design into every assigned site's config —
// POST /v1/banners/{id}/publish.
func (s *BannersService) Publish(ctx context.Context, id string) (*BannerPublishResult, error) {
	var out BannerPublishResult
	if err := s.c.post(ctx, "/v1/banners/"+url.PathEscape(id)+"/publish", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

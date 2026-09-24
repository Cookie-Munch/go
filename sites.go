package cookiemunch

import (
	"context"
	"net/url"
	"strconv"
)

// SitesService groups the /v1/sites endpoints.
type SitesService struct{ c *Client }

func sitePath(cbid string, suffix string) string {
	return "/v1/sites/" + url.PathEscape(cbid) + suffix
}

// List returns all sites for the key's org — GET /v1/sites.
func (s *SitesService) List(ctx context.Context) ([]Site, error) {
	var out []Site
	if err := s.c.get(ctx, "/v1/sites", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Create registers a new site — POST /v1/sites.
func (s *SitesService) Create(ctx context.Context, input SiteCreate) (*Site, error) {
	var out Site
	if err := s.c.post(ctx, "/v1/sites", input, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Get fetches one site — GET /v1/sites/{cbid}.
func (s *SitesService) Get(ctx context.Context, cbid string) (*Site, error) {
	var out Site
	if err := s.c.get(ctx, sitePath(cbid, ""), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete removes a site — DELETE /v1/sites/{cbid}.
func (s *SitesService) Delete(ctx context.Context, cbid string) error {
	return s.c.delete(ctx, sitePath(cbid, ""))
}

// GetConfig returns a site's config — GET /v1/sites/{cbid}/config.
func (s *SitesService) GetConfig(ctx context.Context, cbid string) (SiteConfig, error) {
	var out SiteConfig
	if err := s.c.get(ctx, sitePath(cbid, "/config"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PutConfig upserts a site's config — PUT /v1/sites/{cbid}/config.
func (s *SitesService) PutConfig(ctx context.Context, cbid string, config SiteConfig) (SiteConfig, error) {
	var out SiteConfig
	if err := s.c.put(ctx, sitePath(cbid, "/config"), config, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PatchConfig changes part of a site's config — PATCH /v1/sites/{cbid}/config.
// Omitted fields keep their stored value; PutConfig replaces the whole document.
func (s *SitesService) PatchConfig(ctx context.Context, cbid string, config SiteConfig) (SiteConfig, error) {
	var out SiteConfig
	if err := s.c.patch(ctx, sitePath(cbid, "/config"), config, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Cookies returns the latest categorized cookie declaration —
// GET /v1/sites/{cbid}/cookies.
func (s *SitesService) Cookies(ctx context.Context, cbid string) (*CookieDeclaration, error) {
	var out CookieDeclaration
	if err := s.c.get(ctx, sitePath(cbid, "/cookies"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Scan kicks off an async cookie crawl — POST /v1/sites/{cbid}/scan.
func (s *SitesService) Scan(ctx context.Context, cbid string) (*ScanStatus, error) {
	var out ScanStatus
	if err := s.c.post(ctx, sitePath(cbid, "/scan"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ScanStatus returns the current cookie-scan status —
// GET /v1/sites/{cbid}/scan.
func (s *SitesService) ScanStatus(ctx context.Context, cbid string) (*ScanStatus, error) {
	var out ScanStatus
	if err := s.c.get(ctx, sitePath(cbid, "/scan"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Ab returns A/B experiment results — GET /v1/sites/{cbid}/ab.
func (s *SitesService) Ab(ctx context.Context, cbid string) ([]AbResult, error) {
	var out []AbResult
	if err := s.c.get(ctx, sitePath(cbid, "/ab"), nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Snippet returns the install snippet — GET /v1/sites/{cbid}/snippet.
func (s *SitesService) Snippet(ctx context.Context, cbid string, opts *SnippetOptions) (*InstallSnippet, error) {
	q := url.Values{}
	if opts != nil {
		if opts.BlockingMode != "" {
			q.Set("blockingmode", opts.BlockingMode)
		}
		if opts.Culture != "" {
			q.Set("culture", opts.Culture)
		}
	}
	var out InstallSnippet
	if err := s.c.get(ctx, sitePath(cbid, "/snippet"), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Verify runs domain verification — POST /v1/sites/{cbid}/verify. method is one
// of "dns", "meta", "file".
func (s *SitesService) Verify(ctx context.Context, cbid, method string) (*VerifyResult, error) {
	var out VerifyResult
	if err := s.c.post(ctx, sitePath(cbid, "/verify"), map[string]string{"method": method}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Brand suggests theme tokens by extracting the site's brand —
// POST /v1/sites/{cbid}/brand.
func (s *SitesService) Brand(ctx context.Context, cbid string) (*BrandExtractionResult, error) {
	var out BrandExtractionResult
	if err := s.c.post(ctx, sitePath(cbid, "/brand"), map[string]any{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetFlow reads a site's v2 banner flow plus lint issues —
// GET /v1/sites/{cbid}/flow.
func (s *SitesService) GetFlow(ctx context.Context, cbid string) (*SiteFlow, error) {
	var out SiteFlow
	if err := s.c.get(ctx, sitePath(cbid, "/flow"), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EditFlow applies an ordered batch of structured edit ops —
// POST /v1/sites/{cbid}/flow/ops. Always check FlowWriteResult.OK.
func (s *SitesService) EditFlow(ctx context.Context, cbid string, operations []FlowOp) (*FlowWriteResult, error) {
	ops := make([]map[string]any, len(operations))
	for i, op := range operations {
		m := map[string]any{"op": op.Op}
		for k, v := range op.Args {
			m[k] = v
		}
		ops[i] = m
	}
	var out FlowWriteResult
	if err := s.c.post(ctx, sitePath(cbid, "/flow/ops"), map[string]any{"operations": ops}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SetFlow wholesale-replaces the flow with a full v2 config —
// PUT /v1/sites/{cbid}/flow. Always check FlowWriteResult.OK.
func (s *SitesService) SetFlow(ctx context.Context, cbid string, config map[string]any) (*FlowWriteResult, error) {
	var out FlowWriteResult
	if err := s.c.put(ctx, sitePath(cbid, "/flow"), config, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EnableAdPersonalization enables the personalized-ads split on the site's
// banner — POST /v1/sites/{cbid}/elements/ad-personalization.
func (s *SitesService) EnableAdPersonalization(ctx context.Context, cbid string, input *AdPersonalizationInput) (*AdPersonalizationResult, error) {
	var body any = map[string]any{}
	if input != nil {
		body = input
	}
	var out AdPersonalizationResult
	if err := s.c.post(ctx, sitePath(cbid, "/elements/ad-personalization"), body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// rangeValues turns a RangeQuery into query params (epoch-ms; zero omitted).
func rangeValues(r *RangeQuery) url.Values {
	q := url.Values{}
	if r != nil {
		if r.From != 0 {
			q.Set("from", strconv.FormatInt(r.From, 10))
		}
		if r.To != 0 {
			q.Set("to", strconv.FormatInt(r.To, 10))
		}
	}
	return q
}

// logValues turns a LogQuery into query params.
func logValues(r *LogQuery) url.Values {
	q := url.Values{}
	if r != nil {
		if r.From != 0 {
			q.Set("from", strconv.FormatInt(r.From, 10))
		}
		if r.To != 0 {
			q.Set("to", strconv.FormatInt(r.To, 10))
		}
		if r.Limit != 0 {
			q.Set("limit", strconv.Itoa(r.Limit))
		}
	}
	return q
}

package cookiemunch

import (
	"context"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
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

// PutConfigKeepingUnspecified replaces the fields config sends and keeps the stored
// value of every top-level field it omits — PUT /v1/sites/{cbid}/config?keep=unspecified.
// Each field sent is still replaced whole; one sent as nil reverts to its default.
func (s *SitesService) PutConfigKeepingUnspecified(ctx context.Context, cbid string, config SiteConfig) (SiteConfig, error) {
	var out SiteConfig
	if err := s.c.do(ctx, http.MethodPut, sitePath(cbid, "/config"), url.Values{"keep": {"unspecified"}}, config, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DryRunPutConfigKeepingUnspecified previews PutConfigKeepingUnspecified without saving —
// PUT /v1/sites/{cbid}/config?dryRun=1&keep=unspecified.
func (s *SitesService) DryRunPutConfigKeepingUnspecified(ctx context.Context, cbid string, config SiteConfig) (map[string]any, error) {
	var out map[string]any
	if err := s.c.do(ctx, http.MethodPut, sitePath(cbid, "/config"), url.Values{"dryRun": {"1"}, "keep": {"unspecified"}}, config, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PutConfig replaces a site's config — PUT /v1/sites/{cbid}/config. A top-level field
// config omits reverts to its default (see PutConfigKeepingUnspecified).
func (s *SitesService) PutConfig(ctx context.Context, cbid string, config SiteConfig) (SiteConfig, error) {
	var out SiteConfig
	if err := s.c.put(ctx, sitePath(cbid, "/config"), config, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// PatchConfig changes part of a site's config — PATCH /v1/sites/{cbid}/config, a JSON
// Merge Patch (RFC 7386): omitted fields keep their stored value, and a field set to nil
// is removed (it takes its default). PutConfig replaces the whole document.
func (s *SitesService) PatchConfig(ctx context.Context, cbid string, config SiteConfig) (SiteConfig, error) {
	var out SiteConfig
	if err := s.c.patch(ctx, sitePath(cbid, "/config"), config, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// DryRunConfig previews a config write without saving it — PATCH (or, with
// replace=true, PUT) /v1/sites/{cbid}/config?dryRun=1. The result carries the
// config as it would be saved ("config"), every leaf that would change ("diff")
// and any "warnings".
func (s *SitesService) DryRunConfig(ctx context.Context, cbid string, config SiteConfig, replace bool) (map[string]any, error) {
	method := "PATCH"
	if replace {
		method = "PUT"
	}
	var out map[string]any
	if err := s.c.do(ctx, method, sitePath(cbid, "/config"), url.Values{"dryRun": {"1"}}, config, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// LintConfig checks a proposed config for consent-button parity without saving it —
// POST /v1/sites/{cbid}/config/lint: is "Accept all" more prominent than "Reject all"
// anywhere (colour, size, placement, copy, custom CSS)? The result carries
// "designWarnings", "score" (0-100) and "suggestedReject". It checks exactly what the
// write would store: merged over the stored config as PatchConfig would, or with
// replace=true replaced as PutConfig would. Needs only sites:read.
func (s *SitesService) LintConfig(ctx context.Context, cbid string, config SiteConfig, replace bool) (map[string]any, error) {
	var q url.Values
	if replace {
		q = url.Values{"replace": {"1"}}
	}
	var out map[string]any
	if err := s.c.do(ctx, "POST", sitePath(cbid, "/config/lint"), q, config, &out); err != nil {
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

// ScanWait starts a cookie crawl and holds the request until it is done or failed, or
// until wait (at most 60 s) runs out — POST /v1/sites/{cbid}/scan?wait=.
func (s *SitesService) ScanWait(ctx context.Context, cbid string, wait time.Duration) (*ScanStatus, error) {
	var out ScanStatus
	if err := s.c.do(ctx, http.MethodPost, sitePath(cbid, "/scan"), waitQuery(wait), nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ScanStatusWait is ScanStatus holding the request until a running scan settles, or
// until wait (at most 60 s) runs out — GET /v1/sites/{cbid}/scan?wait=.
func (s *SitesService) ScanStatusWait(ctx context.Context, cbid string, wait time.Duration) (*ScanStatus, error) {
	var out ScanStatus
	if err := s.c.get(ctx, sitePath(cbid, "/scan"), waitQuery(wait), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// WaitForScan returns once the site's scan is done or failed (read State and Error),
// long-polling the server; at timeout it returns the still-running state. Each request
// asks the server to hold no longer than the HTTP client's own timeout allows.
func (s *SitesService) WaitForScan(ctx context.Context, cbid string, timeout time.Duration) (*ScanStatus, error) {
	until := time.Now().Add(timeout)
	for {
		asked := time.Until(until)
		if asked > 60*time.Second {
			asked = 60 * time.Second
		}
		if t := s.c.httpClient.Timeout; t > 0 && asked > t-5*time.Second {
			asked = t - 5*time.Second
		}
		if asked < 0 {
			asked = 0
		}
		started := time.Now()
		r, err := s.ScanStatusWait(ctx, cbid, asked)
		if err != nil || r.Status != "scanning" || !time.Now().Before(until) {
			return r, err
		}
		// A server that answered sooner than asked is not hammered.
		gap := time.Second
		if left := time.Until(until); left < gap {
			gap = left
		}
		if early := gap - time.Since(started); early > 0 {
			select {
			case <-ctx.Done():
				return r, ctx.Err()
			case <-time.After(early):
			}
		}
	}
}

// waitQuery is ?wait=<seconds> (30, 1.5).
func waitQuery(d time.Duration) url.Values {
	return url.Values{"wait": {strconv.FormatFloat(math.Round(d.Seconds()*1000)/1000, 'f', -1, 64)}}
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

// GetBrand reads back the brand.json last imported for the site — GET /v1/sites/{cbid}/brand:
// each role, the theme token it sets, its source ("brand.json" or "site"), ignored[] and warnings[].
func (s *SitesService) GetBrand(ctx context.Context, cbid string) (Object, error) {
	return s.c.object(ctx, "GET", sitePath(cbid, "/brand"), nil, nil)
}

// SetBrand imports a DTCG brand.json into the site's banner theme (the rights page uses it too)
// — PUT /v1/sites/{cbid}/brand. input is {"url": "https://…"} or the brand.json document. Values
// the site set itself are kept (brand.json < site banner theme < per-banner variant). dryRun
// answers the read-back and saves nothing.
func (s *SitesService) SetBrand(ctx context.Context, cbid string, input Object, dryRun bool) (Object, error) {
	var q url.Values
	if dryRun {
		q = url.Values{"dryRun": {"1"}}
	}
	return s.c.object(ctx, "PUT", sitePath(cbid, "/brand"), q, input)
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

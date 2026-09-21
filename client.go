// Package cookiemunch is a small, idiomatic Go client for the Cookie Munch
// Developer API (the /v1 surface) plus the public consent-ingest endpoint.
//
// Construct a client with an API key; the organisation is derived server-side
// from the key, so callers never pass an orgId:
//
//	cm := cookiemunch.New("fck_live_...")
//	sites, err := cm.Sites.List(ctx)
//
// The default base URL is https://api.cookiemunch.net. Override it (and the
// underlying *http.Client, auth style, etc.) with functional options passed to
// New.
package cookiemunch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBaseURL is the production API origin. The "/v1" (dev API) and
// "/api/v1" (public ingest) prefixes are appended by the client as needed.
const DefaultBaseURL = "https://api.cookiemunch.net"

// defaultUserAgent is sent on every request unless overridden with WithUserAgent.
const defaultUserAgent = "cookiemunch-go/1.0"

// authStyle selects how the API key is presented to the server.
type authStyle int

const (
	// authBearer sends "Authorization: Bearer <key>" (the default).
	authBearer authStyle = iota
	// authHeader sends "X-API-Key: <key>".
	authHeader
)

// Client is the Cookie Munch Developer API client. It is safe for concurrent
// use by multiple goroutines. Create one with New.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
	userAgent  string
	auth       authStyle

	// Resource groups. Each mirrors a section of the TypeScript SDK surface.
	Sites       *SitesService
	Consent     *ConsentService
	DSAR        *DSARService
	Vendors     *VendorsService
	RoPA        *RoPAService
	BrandKits   *BrandKitsService
	Preferences *PreferencesService
	Members     *MembersService
	Keys        *KeysService
	Webhooks    *WebhooksService
	Banners     *BannersService

	// The privacy platform beyond the banner, and the reseller API.
	Identity      *IdentityService
	Vault         *VaultService
	Profile       *ProfileService
	Subscriptions *SubscriptionsService
	Assessments   *AssessmentsService
	Discovery     *DiscoveryService
	AI            *AIService
	Fulfillment   *FulfillmentService
	Regulatory    *RegulatoryService
	Reseller      *ResellerService
	Subjects      *SubjectsService
	Org           *OrgService
	Assets        *AssetsService
}

// Option configures a Client. Pass options to New.
type Option func(*Client)

// WithBaseURL overrides the API origin (default DefaultBaseURL). Any trailing
// slashes are trimmed.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithHTTPClient sets the underlying *http.Client used for all requests. Use it
// to control timeouts, transports, or proxies. Passing nil is a no-op.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithUserAgent overrides the User-Agent header sent on every request.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithHeaderAuth sends the API key as the "X-API-Key" header instead of the
// default "Authorization: Bearer <key>". Both are accepted by the server.
func WithHeaderAuth() Option {
	return func(c *Client) { c.auth = authHeader }
}

// New creates a Client authenticating with apiKey (an "fck_..." developer key).
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		userAgent:  defaultUserAgent,
		auth:       authBearer,
	}
	for _, opt := range opts {
		opt(c)
	}
	c.Sites = &SitesService{c: c}
	c.Consent = &ConsentService{c: c}
	c.DSAR = &DSARService{c: c}
	c.Vendors = &VendorsService{c: c}
	c.RoPA = &RoPAService{c: c}
	c.BrandKits = &BrandKitsService{c: c}
	c.Preferences = &PreferencesService{c: c}
	c.Members = &MembersService{c: c}
	c.Keys = &KeysService{c: c}
	c.Webhooks = &WebhooksService{c: c}
	c.Banners = &BannersService{c: c}
	c.Identity = &IdentityService{c: c}
	c.Vault = &VaultService{c: c}
	c.Profile = &ProfileService{c: c}
	c.Subscriptions = &SubscriptionsService{c: c}
	c.Assessments = &AssessmentsService{c: c}
	c.Discovery = &DiscoveryService{c: c}
	c.AI = &AIService{c: c}
	c.Fulfillment = &FulfillmentService{c: c}
	c.Regulatory = &RegulatoryService{c: c}
	c.Reseller = &ResellerService{c: c}
	c.Subjects = &SubjectsService{c: c}
	c.Org = &OrgService{c: c}
	c.Assets = &AssetsService{c: c}
	return c
}

// APIError is returned for any non-2xx response. It carries the HTTP status
// code and the raw response body, plus the server's parsed "error"/"code"
// fields when the body is JSON.
type APIError struct {
	// StatusCode is the HTTP status of the failed response.
	StatusCode int
	// Body is the raw, undecoded response body.
	Body string
	// Message is the server's "error" field, when the body was JSON; otherwise
	// a generic fallback.
	Message string
	// Code is the server's optional "code" field (e.g. "banner_in_use").
	Code string
}

// Error implements the error interface.
func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("cookiemunch: %d %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf("cookiemunch: request failed with status %d", e.StatusCode)
}

func newAPIError(status int, body []byte) *APIError {
	e := &APIError{StatusCode: status, Body: string(body), Message: fmt.Sprintf("request failed with status %d", status)}
	var parsed struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Error != "" {
			e.Message = parsed.Error
		}
		if parsed.Code != "" {
			e.Code = parsed.Code
		}
	}
	return e
}

// me identifies the caller (org, plan, key prefix) — GET /v1/me.
//
// It lives on Client rather than in a sub-resource, mirroring the TypeScript
// SDK's top-level me().
func (c *Client) Me(ctx context.Context) (*Identity, error) {
	var out Identity
	if err := c.get(ctx, "/v1/me", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Languages returns the languages the banner already has copy for — GET /v1/languages.
// Diff it against your visitors' locales to find the ones you still have to write.
func (c *Client) Languages(ctx context.Context) ([]SupportedLanguage, error) {
	var out []SupportedLanguage
	if err := c.get(ctx, "/v1/languages", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Usage returns current resource usage for the org — GET /v1/usage.
func (c *Client) Usage(ctx context.Context) (*Usage, error) {
	var out Usage
	if err := c.get(ctx, "/v1/usage", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Audit returns the org's audit log, newest first — GET /v1/audit. Sign-ins aside, every
// administrative change made in the dashboard or through the API, with who made it; API
// actions are attributed to "apikey:<prefix>". limit <= 0 uses the server default (200).
// Requires an unscoped key that is not property-locked.
//
// It lives on Client rather than in a sub-resource, mirroring the TypeScript SDK's
// top-level audit().
func (c *Client) Audit(ctx context.Context, limit int) (Object, error) {
	return c.object(ctx, "GET", "/v1/audit", limitQuery(limit), nil)
}

// ---- internal request plumbing ----------------------------------------------

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

func (c *Client) put(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPut, path, nil, body, out)
}

func (c *Client) patch(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPatch, path, nil, body, out)
}

func (c *Client) delete(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

// do performs a request against c.baseURL+path. When body is non-nil it is
// JSON-encoded. When out is non-nil the (2xx) response body is decoded into it;
// out may be a *string to receive the raw body verbatim (e.g. CSV export).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("cookiemunch: encoding request body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("cookiemunch: building request: %w", err)
	}

	switch c.auth {
	case authHeader:
		req.Header.Set("X-API-Key", c.apiKey)
	default:
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("cookiemunch: request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("cookiemunch: reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp.StatusCode, data)
	}

	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	if s, ok := out.(*string); ok {
		*s = string(data)
		return nil
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("cookiemunch: decoding response: %w", err)
	}
	return nil
}

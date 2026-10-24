package cookiemunch

import "context"

// SandboxService is the organisation's sandbox: a separate test organisation with the same
// people and plan features, never billed. Its keys are test keys and its rights requests are
// all tests. Create and Delete need a full-access key or orgs:create.
type SandboxService struct{ c *Client }

// Get returns {sandbox}: the sandbox, or null — GET /v1/sandbox. From a sandbox key, the
// sandbox itself.
func (s *SandboxService) Get(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/sandbox", nil, nil)
}

// Create makes the sandbox (or reuses it, created: false) and a new test key for it, shown
// once — POST /v1/sandbox. keyName may be empty for the default name.
func (s *SandboxService) Create(ctx context.Context, keyName string) (Object, error) {
	body := map[string]any{}
	if keyName != "" {
		body["keyName"] = keyName
	}
	return s.c.object(ctx, "POST", "/v1/sandbox", nil, body)
}

// Delete erases the sandbox and everything in it — DELETE /v1/sandbox. It cannot be undone;
// the live organisation is not touched.
func (s *SandboxService) Delete(ctx context.Context) (Object, error) {
	return s.c.object(ctx, "DELETE", "/v1/sandbox", nil, nil)
}

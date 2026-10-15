package cookiemunch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

type recorded struct {
	method, url string
	body        map[string]any
}

func recorder(t *testing.T, contentType, payload string) (*Client, *[]recorded) {
	t.Helper()
	calls := &[]recorded{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		*calls = append(*calls, recorded{r.Method, r.URL.RequestURI(), body})
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(payload))
	}))
	t.Cleanup(srv.Close)
	return New("fck_test", WithBaseURL(srv.URL)), calls
}

func TestDeprovisionSuspendsByDefaultAndPurgesOnlyWhenAsked(t *testing.T) {
	c, calls := recorder(t, "application/json", "")
	ctx := context.Background()
	_ = c.Reseller.Deprovision(ctx, "c1", false)
	_ = c.Reseller.Deprovision(ctx, "c1", true)
	if got := (*calls)[0].url; got != "/v1/reseller/customers/c1" {
		t.Errorf("suspend url = %q", got)
	}
	if got := (*calls)[1].url; got != "/v1/reseller/customers/c1?purge=true" {
		t.Errorf("purge url = %q", got)
	}
}

// Leaving, clearing and pinning the DSAR routing override are three different requests.
func TestDSARRoutingTriState(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	suspended := "suspended"
	_, _ = c.Reseller.Update(ctx, "c1", ResellerChildPatch{Status: &suspended})
	_, _ = c.Reseller.Update(ctx, "c1", ResellerChildPatch{DSARRouting: ClearDSARRouting()})
	_, _ = c.Reseller.Update(ctx, "c1", ResellerChildPatch{DSARRouting: PinDSARRouting("child")})

	if _, present := (*calls)[0].body["dsarRouting"]; present {
		t.Error("leaving routing alone must not send dsarRouting")
	}
	if v, present := (*calls)[1].body["dsarRouting"]; !present || v != nil {
		t.Errorf("clearing must send an explicit null, got %v (present=%v)", v, present)
	}
	if v := (*calls)[2].body["dsarRouting"]; v != "child" {
		t.Errorf("pinning must send the value, got %v", v)
	}
}

func TestIssueSendsLeastPrivilegeFields(t *testing.T) {
	c, calls := recorder(t, "application/json", `{"key":"k","prefix":"p"}`)
	_, _ = c.Keys.Issue(context.Background(), &ApiKeyIssueInput{Name: "agency", Scopes: []string{"consent:read"}, Cbids: []string{"cb_shop"}, ExpiresInDays: 30})
	want := map[string]any{"name": "agency", "scopes": []any{"consent:read"}, "cbids": []any{"cb_shop"}, "expiresInDays": float64(30)}
	if !reflect.DeepEqual((*calls)[0].body, want) {
		t.Errorf("body = %v", (*calls)[0].body)
	}
}

func TestPolicyIsMarkdownWithOptionsInTheQuery(t *testing.T) {
	c, calls := recorder(t, "text/markdown", "# Privacy policy")
	md, err := c.Sites.Policy(context.Background(), "s1", &PolicyOptions{ContactEmail: "dpo@x.com", Jurisdictions: []string{"gdpr", "ccpa"}})
	if err != nil || md != "# Privacy policy" {
		t.Fatalf("policy = %q, %v", md, err)
	}
	if got := (*calls)[0].url; got != "/v1/sites/s1/policy?contactEmail=dpo%40x.com&jurisdictions=gdpr%2Cccpa" {
		t.Errorf("url = %q", got)
	}
}

func TestRopaExportAndDSARNoticeAreText(t *testing.T) {
	c, _ := recorder(t, "text/csv", "a,b\n")
	if csv, _ := c.RoPA.ExportCSV(context.Background()); csv != "a,b\n" {
		t.Errorf("csv = %q", csv)
	}
	c2, _ := recorder(t, "text/plain", "Dear subject")
	if n, _ := c2.DSAR.Response(context.Background(), "d1"); n != "Dear subject" {
		t.Errorf("notice = %q", n)
	}
}

func TestIdentifiersTravelInTheBodyNeverTheURL(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	ids := []Identifier{{Space: "email_sha256", Value: "abc"}}
	_, _ = c.Identity.Resolve(ctx, ids)
	_, _ = c.Vault.Current(ctx, ids)
	_, _ = c.Profile.Activate(ctx, ids, "marketing")
	for _, call := range *calls {
		if call.method != "POST" || strings.Contains(call.url, "abc") {
			t.Errorf("%s %s leaked or used the wrong verb", call.method, call.url)
		}
		if _, ok := call.body["identifiers"]; !ok {
			t.Errorf("%s: identifiers missing from body", call.url)
		}
	}
}

// The operations that used to exist only in the dashboard. Each pins the method, the
// path and the body, since a wrong path fails silently against a 404-tolerant caller.
func TestDashboardParityOperations(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	_, _ = c.Org.Get(ctx)
	_, _ = c.Keys.Roll(ctx, "fck_ab12")
	_, _ = c.Keys.Update(ctx, "fck_ab12", ApiKeyUpdate{Scopes: []string{"sites:read"}})
	_, _ = c.Webhooks.RollSecret(ctx, "w1")
	_, _ = c.Webhooks.Test(ctx, "w1")
	_, _ = c.Webhooks.DeadLetters(ctx)
	_, _ = c.Webhooks.ReplayDeadLetter(ctx, "dlq_1")
	_, _ = c.DSAR.Erase(ctx, "d1", "cb_shop", "st-1")
	_, _ = c.DSAR.Export(ctx, "d1", "cb_shop", "st-1")
	_, _ = c.Preferences.Get(ctx, "jane@example.com")
	_, _ = c.Audit(ctx, 50)
	_, _ = c.Audit(ctx, 0)

	want := []struct{ method, url string }{
		{"GET", "/v1/org"},
		{"POST", "/v1/keys/fck_ab12/roll"},
		{"PATCH", "/v1/keys/fck_ab12"},
		{"POST", "/v1/webhooks/w1/roll"},
		{"POST", "/v1/webhooks/w1/test"},
		{"GET", "/v1/webhooks/dead-letters"},
		{"POST", "/v1/webhooks/dead-letters/dlq_1/replay"},
		{"POST", "/v1/dsar/d1/erase"},
		{"POST", "/v1/dsar/d1/export"},
		{"GET", "/v1/preferences/jane@example.com"},
		{"GET", "/v1/audit?limit=50"},
		{"GET", "/v1/audit"},
	}
	if len(*calls) != len(want) {
		t.Fatalf("made %d calls, want %d", len(*calls), len(want))
	}
	for i, w := range want {
		got := (*calls)[i]
		if got.method != w.method || got.url != w.url {
			t.Errorf("call %d = %s %s, want %s %s", i, got.method, got.url, w.method, w.url)
		}
	}
	if body := (*calls)[7].body; body["cbid"] != "cb_shop" || body["stamp"] != "st-1" {
		t.Errorf("erase body = %v", body)
	}
}

// An omitted logo and a removed logo are different requests: "leave it alone" versus
// "take it down". A serializer that drops nulls turns the second into the first.
func TestOrgUpdateSendsLogoNullOnlyWhenClearing(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	name := "Acme Ltd"
	_, _ = c.Org.Update(ctx, OrgUpdate{Name: &name})
	_, _ = c.Org.Update(ctx, OrgUpdate{LogoURL: ClearOrgLogo()})
	_, _ = c.Org.Update(ctx, OrgUpdate{LogoURL: OrgLogo("https://cdn.example.com/x.png")})

	if _, present := (*calls)[0].body["logoUrl"]; present {
		t.Errorf("renaming sent logoUrl: %v", (*calls)[0].body)
	}
	if v, present := (*calls)[1].body["logoUrl"]; !present || v != nil {
		t.Errorf("clearing sent %v, want an explicit null", (*calls)[1].body)
	}
	if got := (*calls)[2].body["logoUrl"]; got != "https://cdn.example.com/x.png" {
		t.Errorf("setting sent %v", got)
	}
}

// The image is base64 in the body, not raw bytes.
func TestAssetUploadBase64EncodesTheImage(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	_, _ = c.Assets.Upload(context.Background(), []byte{0x89, 'P', 'N', 'G'}, "image/png")
	body := (*calls)[0].body
	if body["data"] != "iVBORw==" || body["contentType"] != "image/png" {
		t.Errorf("upload body = %v", body)
	}
	if got := (*calls)[0].url; got != "/v1/assets" {
		t.Errorf("url = %q", got)
	}
}

func TestRightsLinksCreateSendsTheSubjectAndOptions(t *testing.T) {
	c, calls := recorder(t, "application/json", `{"url":"https://cookiemunch.net/r/s1?t=a.b","expiresAt":1}`)
	ctx := context.Background()
	out, err := c.RightsLinks.Create(ctx, "s1", RightsLinkInput{Subject: RightsLinkSubject{Email: "p@x.com", UserID: "u-1"}, Rights: []string{"access"}, TTLSeconds: 600})
	if err != nil || out["url"] != "https://cookiemunch.net/r/s1?t=a.b" {
		t.Fatalf("out = %v, err = %v", out, err)
	}
	_, _ = c.RightsLinks.Create(ctx, "s1", RightsLinkInput{Subject: RightsLinkSubject{Email: "p@x.com"}})
	first, second := (*calls)[0], (*calls)[1]
	if first.method != "POST" || first.url != "/v1/sites/s1/rights-links" {
		t.Errorf("request = %s %s", first.method, first.url)
	}
	want := map[string]any{"subject": map[string]any{"email": "p@x.com", "userId": "u-1"}, "rights": []any{"access"}, "ttlSeconds": float64(600)}
	if !reflect.DeepEqual(first.body, want) {
		t.Errorf("body = %v", first.body)
	}
	if !reflect.DeepEqual(second.body, map[string]any{"subject": map[string]any{"email": "p@x.com"}}) {
		t.Errorf("minimal body = %v", second.body)
	}
}

// The org's own mail provider for rights mail: four operations, the secret only ever sent.
func TestOrgMailProvider(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	_, _ = c.Org.MailProvider(ctx)
	_, _ = c.Org.SetMailProvider(ctx, MailProviderInput{
		Provider: "smtp", FromEmail: "p@acme.com", Secret: "pw",
		SMTP: &MailProviderSMTP{Host: "smtp.acme.com", Port: 587, Security: "starttls", Username: "u"},
	})
	_, _ = c.Org.TestMailProvider(ctx, "me@acme.com")
	_, _ = c.Org.TestMailProvider(ctx, "")
	_ = c.Org.RemoveMailProvider(ctx)
	want := []struct{ method, url string }{
		{"GET", "/v1/org/mail-provider"},
		{"PUT", "/v1/org/mail-provider"},
		{"POST", "/v1/org/mail-provider/test"},
		{"POST", "/v1/org/mail-provider/test"},
		{"DELETE", "/v1/org/mail-provider"},
	}
	for i, w := range want {
		if (*calls)[i].method != w.method || (*calls)[i].url != w.url {
			t.Errorf("call %d = %s %s, want %s %s", i, (*calls)[i].method, (*calls)[i].url, w.method, w.url)
		}
	}
	put := map[string]any{"provider": "smtp", "fromEmail": "p@acme.com", "secret": "pw",
		"smtp": map[string]any{"host": "smtp.acme.com", "port": float64(587), "security": "starttls", "username": "u"}}
	if !reflect.DeepEqual((*calls)[1].body, put) {
		t.Errorf("put body = %v", (*calls)[1].body)
	}
	if v := (*calls)[2].body["to"]; v != "me@acme.com" {
		t.Errorf("test to = %v", v)
	}
	if _, present := (*calls)[3].body["to"]; present {
		t.Errorf("an empty recipient must be left out, got %v", (*calls)[3].body)
	}
}

func TestRightsPageDomains(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	_, _ = c.RightsPage.ListDomains(ctx, "s 1")
	_, _ = c.RightsPage.AddDomain(ctx, "s", "privacy.acme.com")
	_, _ = c.RightsPage.VerifyDomain(ctx, "s", "privacy.acme.com")
	_ = c.RightsPage.RemoveDomain(ctx, "s", "privacy.acme.com")
	want := []recorded{
		{"GET", "/v1/sites/s%201/rights-page/domains", nil},
		{"POST", "/v1/sites/s/rights-page/domains", map[string]any{"hostname": "privacy.acme.com"}},
		{"POST", "/v1/sites/s/rights-page/domains/privacy.acme.com/verify", nil},
		{"DELETE", "/v1/sites/s/rights-page/domains/privacy.acme.com", nil},
	}
	if len(*calls) != len(want) {
		t.Fatalf("calls = %v", *calls)
	}
	for i, w := range want {
		g := (*calls)[i]
		if g.method != w.method || g.url != w.url || (w.body != nil && !reflect.DeepEqual(g.body, w.body)) {
			t.Errorf("call %d = %+v, want %+v", i, g, w)
		}
	}
}

// The rights page's captcha: the site's own setting and the organisation default.
func TestRightsCaptcha(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	_, _ = c.RightsCaptcha.Get(ctx, "s 1")
	_, _ = c.RightsCaptcha.Set(ctx, "s1", RightsCaptchaInput{Mode: "own", Provider: "recaptcha_v3", SiteKey: "k-12345678", Secret: "sec", MinScore: 0.7})
	_, _ = c.RightsCaptcha.Remove(ctx, "s1")
	_, _ = c.Org.RightsCaptcha(ctx)
	_, _ = c.Org.SetRightsCaptcha(ctx, RightsCaptchaInput{Mode: "off"})
	_, _ = c.Org.RemoveRightsCaptcha(ctx)
	want := []struct{ method, url string }{
		{"GET", "/v1/sites/s%201/rights-captcha"},
		{"PUT", "/v1/sites/s1/rights-captcha"},
		{"DELETE", "/v1/sites/s1/rights-captcha"},
		{"GET", "/v1/org/rights-captcha"},
		{"PUT", "/v1/org/rights-captcha"},
		{"DELETE", "/v1/org/rights-captcha"},
	}
	for i, w := range want {
		if (*calls)[i].method != w.method || (*calls)[i].url != w.url {
			t.Errorf("call %d = %s %s, want %s %s", i, (*calls)[i].method, (*calls)[i].url, w.method, w.url)
		}
	}
	if !reflect.DeepEqual((*calls)[1].body, map[string]any{"mode": "own", "provider": "recaptcha_v3", "siteKey": "k-12345678", "secret": "sec", "minScore": 0.7}) {
		t.Errorf("put body = %v", (*calls)[1].body)
	}
	if !reflect.DeepEqual((*calls)[4].body, map[string]any{"mode": "off"}) {
		t.Errorf("off body = %v", (*calls)[4].body)
	}
}

func TestRightsPageBodiesAndQuery(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	_, _ = c.RightsPage.DryRun(ctx, "s1", Object{"seo": Object{"title": "x"}}, false)
	_, _ = c.RightsPage.DryRun(ctx, "s1", Object{"v": 1}, true)
	_, _ = c.RightsPage.Put(ctx, "s1", Object{"v": 1, "baseUpdatedAt": nil})
	_, _ = c.RightsPage.Publish(ctx, "s1")
	_, _ = c.RightsPage.Publish(ctx, "s1", WithBaseUpdatedAt(int64(7)))
	_, _ = c.RightsPage.Publish(ctx, "s1", WithBaseUpdatedAt(nil))
	_, _ = c.RightsPage.Rollback(ctx, "s1", 2)
	_, _ = c.RightsPage.Rollback(ctx, "s1", 2, WithBaseUpdatedAt("2026-10-14T00:00:00.000Z"))
	got := *calls
	want := []struct {
		method, url string
		body        map[string]any
	}{
		{"PATCH", "/v1/sites/s1/rights-page?dryRun=1", map[string]any{"seo": map[string]any{"title": "x"}}},
		{"PUT", "/v1/sites/s1/rights-page?dryRun=1", map[string]any{"v": float64(1)}},
		{"PUT", "/v1/sites/s1/rights-page", map[string]any{"v": float64(1), "baseUpdatedAt": nil}},
		{"POST", "/v1/sites/s1/rights-page/publish", map[string]any{}},
		{"POST", "/v1/sites/s1/rights-page/publish", map[string]any{"baseUpdatedAt": float64(7)}},
		{"POST", "/v1/sites/s1/rights-page/publish", map[string]any{"baseUpdatedAt": nil}},
		{"POST", "/v1/sites/s1/rights-page/rollback", map[string]any{"version": float64(2)}},
		{"POST", "/v1/sites/s1/rights-page/rollback", map[string]any{"version": float64(2), "baseUpdatedAt": "2026-10-14T00:00:00.000Z"}},
	}
	if len(got) != len(want) {
		t.Fatalf("calls = %d, want %d", len(got), len(want))
	}
	for i, w := range want {
		if got[i].method != w.method || got[i].url != w.url || !reflect.DeepEqual(got[i].body, w.body) {
			t.Errorf("call %d = %s %s %v, want %s %s %v", i, got[i].method, got[i].url, got[i].body, w.method, w.url, w.body)
		}
	}
	// baseUpdatedAt: nil must reach the server as null, not be dropped
	if _, ok := got[2].body["baseUpdatedAt"]; !ok {
		t.Error("baseUpdatedAt nil was dropped from PUT")
	}
	if _, ok := got[5].body["baseUpdatedAt"]; !ok {
		t.Error("baseUpdatedAt nil was dropped from publish")
	}
}

func TestLintConfigPostsAndReplaceAddsTheFlag(t *testing.T) {
	c, calls := recorder(t, "application/json", `{"mode":"merged","designWarnings":[],"score":100,"suggestedReject":[]}`)
	ctx := context.Background()
	r, err := c.Sites.LintConfig(ctx, "s1", SiteConfig{"banner": map[string]any{"type": "accept-only"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	if r["score"] != float64(100) {
		t.Errorf("score = %v", r["score"])
	}
	_, _ = c.Sites.LintConfig(ctx, "s1", SiteConfig{"v": 2}, true)
	got := *calls
	if got[0].method != "POST" || got[0].url != "/v1/sites/s1/config/lint" {
		t.Errorf("call 0 = %s %s", got[0].method, got[0].url)
	}
	if !reflect.DeepEqual(got[0].body, map[string]any{"banner": map[string]any{"type": "accept-only"}}) {
		t.Errorf("body = %v", got[0].body)
	}
	if got[1].url != "/v1/sites/s1/config/lint?replace=1" {
		t.Errorf("call 1 url = %s", got[1].url)
	}
}

// The sign-in provider rights requests are verified with: the site's own setting and the organisation default.
func TestRightsIdentity(t *testing.T) {
	c, calls := recorder(t, "application/json", "{}")
	ctx := context.Background()
	_, _ = c.RightsIdentity.Get(ctx, "s 1")
	_, _ = c.RightsIdentity.Set(ctx, "s1", RightsIdentityInput{Type: "clerk", Issuer: "https://clerk.acme.example", AuthorizedParties: []string{"https://www.acme.example"}, TrustEmailClaim: true})
	_, _ = c.RightsIdentity.Remove(ctx, "s1")
	_, _ = c.Org.RightsIdentity(ctx)
	_, _ = c.Org.SetRightsIdentity(ctx, RightsIdentityInput{Type: "oidc_jwt", Issuer: "https://id.acme.example", JwksURL: "https://id.acme.example/keys", Audience: "acme-rights", SessionCookieName: "sid"})
	_, _ = c.Org.RemoveRightsIdentity(ctx)
	want := []struct{ method, url string }{
		{"GET", "/v1/sites/s%201/rights-identity"},
		{"PUT", "/v1/sites/s1/rights-identity"},
		{"DELETE", "/v1/sites/s1/rights-identity"},
		{"GET", "/v1/org/rights-identity"},
		{"PUT", "/v1/org/rights-identity"},
		{"DELETE", "/v1/org/rights-identity"},
	}
	for i, w := range want {
		if (*calls)[i].method != w.method || (*calls)[i].url != w.url {
			t.Errorf("call %d = %s %s, want %s %s", i, (*calls)[i].method, (*calls)[i].url, w.method, w.url)
		}
	}
	if !reflect.DeepEqual((*calls)[1].body, map[string]any{"type": "clerk", "issuer": "https://clerk.acme.example", "authorizedParties": []any{"https://www.acme.example"}, "trustEmailClaim": true}) {
		t.Errorf("put body = %v", (*calls)[1].body)
	}
	if !reflect.DeepEqual((*calls)[4].body, map[string]any{"type": "oidc_jwt", "issuer": "https://id.acme.example", "jwksUrl": "https://id.acme.example/keys", "audience": "acme-rights", "sessionCookieName": "sid"}) {
		t.Errorf("org body = %v", (*calls)[4].body)
	}
}

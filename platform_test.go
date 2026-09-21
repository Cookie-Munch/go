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

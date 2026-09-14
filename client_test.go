package cookiemunch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient spins up an httptest.Server with the given handler and returns a
// Client pointed at it (plus a cleanup via t.Cleanup).
func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	base := append([]Option{WithBaseURL(srv.URL)}, opts...)
	return New("fck_test_key", base...)
}

func TestBearerAuthHeaderAndGet(t *testing.T) {
	var gotAuth, gotAccept, gotPath string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Identity{OrgID: "org_1", Plan: "pro", KeyPrefix: "fck_test"})
	})

	id, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: unexpected error: %v", err)
	}
	if gotAuth != "Bearer fck_test_key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer fck_test_key")
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want application/json", gotAccept)
	}
	if gotPath != "/v1/me" {
		t.Errorf("path = %q, want /v1/me", gotPath)
	}
	if id.OrgID != "org_1" || id.Plan != "pro" || id.KeyPrefix != "fck_test" {
		t.Errorf("decoded identity = %+v", id)
	}
}

func TestHeaderAuthOption(t *testing.T) {
	var gotAPIKey, gotAuth string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-API-Key")
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(Identity{OrgID: "org_1"})
	}, WithHeaderAuth())

	if _, err := c.Me(context.Background()); err != nil {
		t.Fatalf("Me: unexpected error: %v", err)
	}
	if gotAPIKey != "fck_test_key" {
		t.Errorf("X-API-Key = %q, want fck_test_key", gotAPIKey)
	}
	if gotAuth != "" {
		t.Errorf("Authorization should be empty with header auth, got %q", gotAuth)
	}
}

func TestSitesListGet(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/sites" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode([]Site{
			{CBID: "cb_1", OrgID: "org_1", Domain: "example.com", Verified: true},
		})
	})

	sites, err := c.Sites.List(context.Background())
	if err != nil {
		t.Fatalf("Sites.List: %v", err)
	}
	if len(sites) != 1 || sites[0].CBID != "cb_1" || !sites[0].Verified {
		t.Errorf("sites = %+v", sites)
	}
}

func TestConsentIngestPost(t *testing.T) {
	var gotBody ConsentIngest
	var gotMethod, gotPath, gotContentType string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusNoContent) // 204, no body — as the real endpoint does
	})

	payload := ConsentIngest{
		CBID:    "cb_1",
		Stamp:   "stamp-abc",
		Choices: ConsentChoices{Preferences: true, Statistics: false, Marketing: true},
		Method:  "explicit",
		Ver:     1,
		UTC:     1700000000000,
		URL:     "https://example.com/",
	}
	if err := c.Consent.Ingest(context.Background(), payload); err != nil {
		t.Fatalf("Consent.Ingest: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if gotPath != "/api/v1/consent" {
		t.Errorf("path = %q, want /api/v1/consent", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotBody.CBID != "cb_1" || gotBody.Stamp != "stamp-abc" || !gotBody.Choices.Marketing || gotBody.Method != "explicit" {
		t.Errorf("decoded ingest body = %+v", gotBody)
	}
}

func TestConsentIngestSubjectID(t *testing.T) {
	base := ConsentIngest{
		CBID:    "cb_1",
		Stamp:   "stamp-abc",
		Choices: ConsentChoices{Preferences: true},
		Method:  "explicit",
		Ver:     1,
		UTC:     1700000000000,
		URL:     "https://example.com/",
	}

	// Omitted when unset (omitempty keeps the wire body unchanged).
	var rawWithout string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rawWithout = string(body)
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Consent.Ingest(context.Background(), base); err != nil {
		t.Fatalf("Consent.Ingest: %v", err)
	}
	if strings.Contains(rawWithout, "subjectId") {
		t.Errorf("subjectId should be omitted when unset; body = %s", rawWithout)
	}

	// Included on the wire when set.
	var gotBody ConsentIngest
	c2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		w.WriteHeader(http.StatusNoContent)
	})
	withID := base
	withID.SubjectID = "user-42"
	if err := c2.Consent.Ingest(context.Background(), withID); err != nil {
		t.Fatalf("Consent.Ingest: %v", err)
	}
	if gotBody.SubjectID != "user-42" {
		t.Errorf("SubjectID = %q, want user-42", gotBody.SubjectID)
	}
}

func TestConsentStatsQueryParams(t *testing.T) {
	var gotQuery string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode([]ConsentDay{{Date: "2024-01-01", OptIn: 5}})
	})

	days, err := c.Consent.Stats(context.Background(), "cb_1", &RangeQuery{From: 100, To: 200})
	if err != nil {
		t.Fatalf("Consent.Stats: %v", err)
	}
	if gotQuery != "from=100&to=200" {
		t.Errorf("query = %q, want from=100&to=200", gotQuery)
	}
	if len(days) != 1 || days[0].OptIn != 5 {
		t.Errorf("days = %+v", days)
	}
}

func TestErrorMapping(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"error":"cbid already claimed","code":"cbid_taken"}`)
	})

	_, err := c.Sites.Create(context.Background(), SiteCreate{Domain: "dup.com"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T (%v)", err, err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode = %d, want 409", apiErr.StatusCode)
	}
	if apiErr.Message != "cbid already claimed" {
		t.Errorf("Message = %q, want %q", apiErr.Message, "cbid already claimed")
	}
	if apiErr.Code != "cbid_taken" {
		t.Errorf("Code = %q, want cbid_taken", apiErr.Code)
	}
	if apiErr.Body == "" {
		t.Error("Body should carry the raw response")
	}
}

func TestErrorMappingNonJSON(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = io.WriteString(w, "upstream boom")
	})

	err := c.Consent.Ingest(context.Background(), ConsentIngest{CBID: "cb_1"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Errorf("StatusCode = %d, want 502", apiErr.StatusCode)
	}
	if apiErr.Body != "upstream boom" {
		t.Errorf("Body = %q", apiErr.Body)
	}
}

func TestExportReturnsRawString(t *testing.T) {
	csv := "stamp,method\nabc,explicit\n"
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/csv")
		_, _ = io.WriteString(w, csv)
	})

	got, err := c.Consent.Export(context.Background(), "cb_1", nil)
	if err != nil {
		t.Fatalf("Consent.Export: %v", err)
	}
	if got != csv {
		t.Errorf("export = %q, want %q", got, csv)
	}
}

func TestDeleteSendsNoBodyAnd204(t *testing.T) {
	var gotMethod string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		w.WriteHeader(http.StatusNoContent)
	})
	if err := c.Sites.Delete(context.Background(), "cb_1"); err != nil {
		t.Fatalf("Sites.Delete: %v", err)
	}
	if gotMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", gotMethod)
	}
}

func TestEditFlowSerializesOps(t *testing.T) {
	var body struct {
		Operations []map[string]any `json:"operations"`
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &body)
		_ = json.NewEncoder(w).Encode(FlowWriteResult{OK: true})
	})

	res, err := c.Sites.EditFlow(context.Background(), "cb_1", []FlowOp{
		{Op: "addView", Args: map[string]any{"id": "v2", "surface": "bar"}},
	})
	if err != nil {
		t.Fatalf("EditFlow: %v", err)
	}
	if !res.OK {
		t.Errorf("res.OK = false")
	}
	if len(body.Operations) != 1 || body.Operations[0]["op"] != "addView" || body.Operations[0]["id"] != "v2" {
		t.Errorf("serialized operations = %+v", body.Operations)
	}
}

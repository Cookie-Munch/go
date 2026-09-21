# cookiemunch-go

A small, idiomatic Go client for the [Cookie Munch](https://cookiemunch.net) Developer API — the `/v1` surface (sites, consent, DSAR, vendors, RoPA, brand kits, preferences, members, keys, webhooks, banners) plus the public consent-log ingest endpoint.

- Go 1.21+, **standard library only** (`net/http`) — no third-party dependencies.
- Context-aware methods, typed structs, and a rich `*APIError` for non-2xx responses.
- The organisation is derived server-side from the API key, so you never pass an `orgId`.

## Install

```bash
go get github.com/Cookie-Munch/go
```

```go
import cookiemunch "github.com/Cookie-Munch/go"
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	cookiemunch "github.com/Cookie-Munch/go"
)

func main() {
	cm := cookiemunch.New("fck_live_your_key_here")
	ctx := context.Background()

	// Who am I?
	me, err := cm.Me(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("org=%s plan=%s\n", me.OrgID, me.Plan)

	// List sites.
	sites, err := cm.Sites.List(ctx)
	if err != nil {
		log.Fatal(err)
	}
	for _, s := range sites {
		fmt.Printf("%s -> %s (verified=%v)\n", s.CBID, s.Domain, s.Verified)
	}
}
```

## Configuration

`New` takes functional options:

```go
cm := cookiemunch.New(
	"fck_live_your_key_here",
	cookiemunch.WithBaseURL("https://cmp.example.com"), // default: https://api.cookiemunch.net
	cookiemunch.WithHTTPClient(&http.Client{Timeout: 10 * time.Second}),
	cookiemunch.WithUserAgent("my-app/1.2.3"),
	cookiemunch.WithHeaderAuth(), // send X-API-Key instead of Authorization: Bearer
)
```

By default the key is sent as `Authorization: Bearer fck_…`. `WithHeaderAuth()` switches to the `X-API-Key` header; both are accepted by the server.

## Logging consent (public ingest)

`Consent.Ingest` posts to the public `POST /api/v1/consent` endpoint — the same high-volume write the browser embed makes. No auth header is required (the `cbid` must be a registered site); a success returns HTTP 204.

```go
err := cm.Consent.Ingest(ctx, cookiemunch.ConsentIngest{
	CBID:    "cb_abc123",
	Stamp:   "9f2b…",                  // per-subject receipt stamp
	Choices: cookiemunch.ConsentChoices{Preferences: true, Statistics: true, Marketing: false},
	Method:  "explicit",              // or "implied"
	Ver:     1,
	UTC:     time.Now().UnixMilli(),
	URL:     "https://example.com/checkout",
})
```

## Reading consent data

```go
// Per-day aggregated stats over a time window (epoch-ms bounds).
stats, _ := cm.Consent.Stats(ctx, "cb_abc123", &cookiemunch.RangeQuery{From: from, To: to})

// Recent anonymised records.
rows, _ := cm.Consent.Log(ctx, "cb_abc123", &cookiemunch.LogQuery{Limit: 100})

// CSV audit export (returned as a raw string).
csv, _ := cm.Consent.Export(ctx, "cb_abc123", nil)

// A signed ISO-27560 receipt for one subject.
receipt, _ := cm.Consent.Receipt(ctx, "cb_abc123", "9f2b…")
```

## Error handling

Every non-2xx response is returned as `*APIError`, carrying the status code, the raw body, and the server's parsed `error`/`code` fields:

```go
_, err := cm.Sites.Create(ctx, cookiemunch.SiteCreate{Domain: "dup.example.com"})
var apiErr *cookiemunch.APIError
if errors.As(err, &apiErr) {
	if apiErr.StatusCode == http.StatusConflict {
		fmt.Println("already claimed:", apiErr.Message, apiErr.Code)
	}
}
```

## Resource groups

| Field | Endpoints |
|---|---|
| `cm.Me`, `cm.Usage` | `/v1/me`, `/v1/usage` |
| `cm.Sites` | list/create/get/delete, config, cookies, scan, A/B, snippet, verify, brand, flow (`GetFlow`/`EditFlow`/`SetFlow`), `EnableAdPersonalization`, `Banner`, `Policy` (Markdown), `AnalyzeSession` |
| `cm.Consent` | `Ingest` (public), `Stats`, `Log`, `Export`, `Receipt`, `EraseSubject`, `ExportSubject` |
| `cm.DSAR` | `List`, `Create`, `Advance`, `Response` (plain-text notice) |
| `cm.Vendors` | `List`, `Create` (with risk scoring) |
| `cm.RoPA` | `List`, `Create`, `ExportCSV` |
| `cm.BrandKits` | `List`, `Create`, `Delete` |
| `cm.Preferences` | `List`, `Save` |
| `cm.Members` | `List`, `Invite`, `SetRole`, `Remove` |
| `cm.Keys` | `List`, `Issue` — set `Scopes` and/or `Cbids` for a least-privilege key |
| `cm.Webhooks` | `List`, `Create`, `Delete` |
| `cm.Banners` | list/create/get/update/delete, `Assignments`, `SetAssignments`, `Publish` |
| `cm.Identity` | `Resolve`, `Link`, `Cluster` |
| `cm.Vault` | `Record`, `Current`, `Permits` |
| `cm.Profile` | `Get`, `SetAttributes`, `Activate` |
| `cm.Subscriptions` | `Topics`, `SetTopics`, `Get`, `Set`, `UnsubscribeAll`, `Resubscribe`, `Activation` |
| `cm.Assessments` | `Templates`, `List`, `Start`, `Get`, `Answer`, `AutoPopulateFromMap`, `AutoPopulate`, `Submit`, `Approve`, `Reject` |
| `cm.Discovery` | `IngestMap`, `GetMap`, `RopaDrafts`, `Evidence`, `Drift`, `PlanEnforcement` |
| `cm.AI` | `GetPolicy`, `SetPolicy`, `Inspect`, `Inventory`, `Lineage`, `RegisterSystem`, `Systems`, `Audit` |
| `cm.Fulfillment` | `SLA`, `Plan`, `Status`, and for the in-environment agent `PendingTasks`, `ReportTask` |
| `cm.Regulatory` | `Feed`, `Upcoming` |
| `cm.Reseller` | `List`, `Create`, `Get`, `Update`, `Deprovision` (suspends; `purge=true` deletes irreversibly), `ListKeys`, `MintKey`, `RevokeKey` — needs the `reseller:*` scopes |

Every operation of the `/v1` API is reachable, and `parity_test.go` keeps it that way
against `sdks/operations.json`, generated from the server's OpenAPI document. Identity,
vault and profile reads are `POST`s so a person's identifiers never appear in a URL.

`ResellerChildPatch.DSARRouting` distinguishes leaving the override alone (nil), clearing
it (`ClearDSARRouting()`), and pinning it (`PinDSARRouting("child")`).

## Testing

The client accepts any base URL, so tests can point it at an `httptest.Server` — no real network is used:

```bash
go build ./...
go vet ./...
go test ./...
```

## License

Part of the Cookie Munch monorepo.

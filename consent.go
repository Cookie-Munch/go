package cookiemunch

import (
	"context"
	"net/url"
)

// ConsentService groups the consent endpoints: the authenticated per-site
// stats/log/export/receipt reads (/v1/...) and the public consent-log ingest
// write (POST /api/v1/consent).
type ConsentService struct{ c *Client }

// Ingest records a consent decision via the PUBLIC POST /api/v1/consent
// endpoint. This is the high-volume write the browser embed makes: no auth
// header is required (the cbid must be a registered site), and a successful call
// returns HTTP 204 with no body. Use it to log consent server-side (e.g. for a
// non-browser subject flow).
//
// Note this endpoint lives under /api/v1, not the /v1 dev-API prefix.
func (s *ConsentService) Ingest(ctx context.Context, payload ConsentIngest) error {
	return s.c.post(ctx, "/api/v1/consent", payload, nil)
}

// Stats returns aggregated per-day consent stats —
// GET /v1/sites/{cbid}/consent/stats.
func (s *ConsentService) Stats(ctx context.Context, cbid string, query *RangeQuery) ([]ConsentDay, error) {
	var out []ConsentDay
	if err := s.c.get(ctx, sitePath(cbid, "/consent/stats"), rangeValues(query), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Log returns recent anonymised consent records —
// GET /v1/sites/{cbid}/consent/log.
func (s *ConsentService) Log(ctx context.Context, cbid string, query *LogQuery) ([]ConsentLogRow, error) {
	var out []ConsentLogRow
	if err := s.c.get(ctx, sitePath(cbid, "/consent/log"), logValues(query), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Export returns the CSV audit export as a raw string —
// GET /v1/sites/{cbid}/consent/export.
func (s *ConsentService) Export(ctx context.Context, cbid string, query *RangeQuery) (string, error) {
	var out string
	if err := s.c.get(ctx, sitePath(cbid, "/consent/export"), rangeValues(query), &out); err != nil {
		return "", err
	}
	return out, nil
}

// Receipt returns a signed ISO-27560 consent receipt (JSON) —
// GET /v1/sites/{cbid}/receipt/{stamp}.
func (s *ConsentService) Receipt(ctx context.Context, cbid, stamp string) (SignedReceipt, error) {
	var out SignedReceipt
	path := sitePath(cbid, "/receipt/"+url.PathEscape(stamp))
	if err := s.c.get(ctx, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// EraseSubject crypto-erases a subject's consent records by their consent-receipt
// stamp — POST /v1/sites/{cbid}/erase-consent. Irreversible.
func (s *ConsentService) EraseSubject(ctx context.Context, cbid, stamp string) (*EraseResult, error) {
	var out EraseResult
	if err := s.c.post(ctx, sitePath(cbid, "/erase-consent"), map[string]string{"stamp": stamp}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ExportSubject exports a data subject's consent records by their receipt stamp
// (GDPR access/portability) — GET /v1/sites/{cbid}/subject-export?stamp=...
func (s *ConsentService) ExportSubject(ctx context.Context, cbid, stamp string) (*SubjectExport, error) {
	q := url.Values{}
	q.Set("stamp", stamp)
	var out SubjectExport
	if err := s.c.get(ctx, sitePath(cbid, "/subject-export"), q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

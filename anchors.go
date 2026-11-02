package cookiemunch

import (
	"context"
	"net/url"
	"strconv"
)

// AnchorsService is audit anchoring: post the head of your own hash-chained log and keep the
// signed receipt. seq must strictly increase per stream; the same seq and head again returns
// the original receipt, a different head for an anchored seq is a 409 (anchor_head_mismatch).
// Create needs anchors:write; List and Get need anchors:read.
type AnchorsService struct{ c *Client }

// Create anchors head at seq on stream — POST /v1/anchors. The receipt is signed with the
// key in /v1/partner/jwks.json.
func (s *AnchorsService) Create(ctx context.Context, stream, head string, seq int64) (Object, error) {
	return s.c.object(ctx, "POST", "/v1/anchors", nil, map[string]any{"stream": stream, "head": head, "seq": seq})
}

// List returns {stream, anchors, nextBefore}, newest first — GET /v1/anchors/{stream}. limit
// 0 means the default; before < 0 means the first page (pass nextBefore for the next one).
func (s *AnchorsService) List(ctx context.Context, stream string, limit int, before int64) (Object, error) {
	q := limitQuery(limit)
	if before >= 0 {
		q.Set("before", strconv.FormatInt(before, 10))
	}
	return s.c.object(ctx, "GET", "/v1/anchors/"+url.PathEscape(stream), q, nil)
}

// Get returns one receipt — GET /v1/anchors/{stream}/{id}.
func (s *AnchorsService) Get(ctx context.Context, stream, id string) (Object, error) {
	return s.c.object(ctx, "GET", "/v1/anchors/"+url.PathEscape(stream)+"/"+url.PathEscape(id), nil, nil)
}

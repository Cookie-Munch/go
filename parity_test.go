package cookiemunch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
)

// TestParity checks every operation the Developer API documents is reachable from this
// SDK. The list lives in ../operations.json, generated from the server's OpenAPI
// document and shared by all six server-side SDKs. Every exported method of the client
// and of each *Service is called against a recording server, with arguments built from
// their reflected types, and what reached the wire is compared both ways.
func TestParity(t *testing.T) {
	// ../operations.json in the product repo; ./operations.json in the published SDK repo.
	raw, err := os.ReadFile("../operations.json")
	if err != nil {
		raw, err = os.ReadFile("operations.json")
	}
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Operations []string `json:"operations"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}

	const placeholder = "x1"
	var mu sync.Mutex
	seen := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The public consent ingest (/api/v1/consent) is not part of the Developer API.
		if strings.HasPrefix(r.URL.Path, "/v1/") {
			segs := strings.Split(r.URL.EscapedPath(), "/")
			for i, s := range segs {
				if s == placeholder {
					segs[i] = "{}"
				}
			}
			mu.Lock()
			seen[r.Method+" "+strings.Join(segs, "/")] = true
			mu.Unlock()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	c := New("fck_test", WithBaseURL(srv.URL))
	targets := []reflect.Value{reflect.ValueOf(c)}
	cv := reflect.ValueOf(c).Elem()
	for i := 0; i < cv.NumField(); i++ {
		if f := cv.Field(i); f.CanInterface() && f.Kind() == reflect.Ptr && strings.HasSuffix(f.Type().Elem().Name(), "Service") {
			targets = append(targets, f)
		}
	}

	for _, target := range targets {
		for i := 0; i < target.NumMethod(); i++ {
			m := target.Method(i)
			mt := m.Type()
			args := make([]reflect.Value, mt.NumIn())
			for j := 0; j < mt.NumIn(); j++ {
				args[j] = placeholderValue(mt.In(j), placeholder)
			}
			if mt.IsVariadic() {
				continue // options-style signatures are not operations
			}
			func() {
				defer func() { _ = recover() }() // only what reached the wire matters here
				m.Call(args)
			}()
		}
	}

	var missing, extra []string
	documented := map[string]bool{}
	for _, op := range fixture.Operations {
		documented[op] = true
		if !seen[op] {
			missing = append(missing, op)
		}
	}
	for op := range seen {
		if !documented[op] {
			extra = append(extra, op)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) > 0 {
		t.Errorf("%d documented operations are unreachable:\n  %s", len(missing), strings.Join(missing, "\n  "))
	}
	if len(extra) > 0 {
		t.Errorf("calls the API does not document:\n  %s", strings.Join(extra, "\n  "))
	}
}

var ctxType = reflect.TypeOf((*context.Context)(nil)).Elem()

func placeholderValue(t reflect.Type, placeholder string) reflect.Value {
	if t == ctxType {
		return reflect.ValueOf(context.Background())
	}
	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf(placeholder).Convert(t)
	case reflect.Bool:
		return reflect.ValueOf(true)
	case reflect.Int, reflect.Int64:
		return reflect.ValueOf(1).Convert(t)
	case reflect.Slice:
		s := reflect.MakeSlice(t, 1, 1)
		s.Index(0).Set(placeholderValue(t.Elem(), placeholder))
		return s
	case reflect.Map:
		m := reflect.MakeMap(t)
		if t.Key().Kind() == reflect.String {
			m.SetMapIndex(reflect.ValueOf("a").Convert(t.Key()), placeholderValue(t.Elem(), placeholder))
		}
		return m
	case reflect.Ptr:
		return reflect.New(t.Elem())
	case reflect.Interface:
		return reflect.ValueOf(map[string]any{"a": placeholder})
	default:
		return reflect.Zero(t)
	}
}

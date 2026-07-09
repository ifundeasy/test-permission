package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifundeasy/test-permission/internal/core/domain"
)

type fakeRouter struct {
	gotEngine string
	gotReq    domain.Request
	res       domain.Result
	err       error
}

func (f *fakeRouter) Check(_ context.Context, engine string, req domain.Request) (domain.Result, error) {
	f.gotEngine, f.gotReq = engine, req
	return f.res, f.err
}

func (f *fakeRouter) Engines() []string { return []string{"cedar"} }

func post(h *Handler, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.Authorize(rec, httptest.NewRequest(http.MethodPost, "/v1/authorize", strings.NewReader(body)))
	return rec
}

func TestAuthorize_MissingFields(t *testing.T) {
	h := NewHandler(&fakeRouter{})
	if rec := post(h, `{"engine":"cedar"}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAuthorize_InvalidJSON(t *testing.T) {
	h := NewHandler(&fakeRouter{})
	if rec := post(h, `not json`); rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAuthorize_OK_ContextNormalized(t *testing.T) {
	f := &fakeRouter{res: domain.Result{Decision: domain.Allow, EntitiesLoaded: 7}}
	h := NewHandler(f)
	rec := post(h, `{
		"engine":"cedar","model":"pbac","principal":"psn-1","action":"po.approve",
		"resource_type":"PurchaseOrder","resource":"po-1",
		"context":{"amount":25000000,"region":"jakarta","tags":["a","b"],"flag":true}
	}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body)
	}
	var resp authorizeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Decision != "allow" || resp.EntitiesLoaded != 7 {
		t.Errorf("resp = %+v", resp)
	}
	// JSON numbers must arrive as int64, string arrays as []string
	if v, ok := f.gotReq.Context["amount"].(int64); !ok || v != 25_000_000 {
		t.Errorf("amount = %#v, want int64(25000000)", f.gotReq.Context["amount"])
	}
	if v, ok := f.gotReq.Context["tags"].([]string); !ok || len(v) != 2 {
		t.Errorf("tags = %#v, want []string{a,b}", f.gotReq.Context["tags"])
	}
}

func TestAuthorize_SpiceDBAlias(t *testing.T) {
	f := &fakeRouter{res: domain.Result{Decision: domain.Deny}}
	h := NewHandler(f)
	post(h, `{"engine":"spicedb","model":"acl","principal":"p","action":"acl.view",
		"resource_type":"AclDocument","resource":"d"}`)
	if f.gotEngine != "spicedb-minimize_latency" {
		t.Errorf("engine = %q, want spicedb-minimize_latency", f.gotEngine)
	}
}

func TestAuthorize_BackendFaultIs502Generic(t *testing.T) {
	f := &fakeRouter{err: errors.New("cedar check: load entities: connect refused 10.0.0.1:5432")}
	h := NewHandler(f)
	rec := post(h, `{"engine":"cedar","model":"acl","principal":"p","action":"acl.view",
		"resource_type":"AclDocument","resource":"d"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.1") {
		t.Errorf("internal detail leaked to client: %s", rec.Body.String())
	}
}

func TestAuthorize_FractionalNumberRejected(t *testing.T) {
	h := NewHandler(&fakeRouter{})
	rec := post(h, `{"engine":"cedar","model":"pbac","principal":"p","action":"po.approve",
		"resource_type":"PurchaseOrder","resource":"po","context":{"amount":1.5}}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

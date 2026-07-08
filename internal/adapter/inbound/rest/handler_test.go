package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ifundeasy/test-permission/internal/core/domain"
)

type fakeAuthorizer struct {
	res domain.Result
	err error
}

func (f fakeAuthorizer) Authorize(context.Context, domain.Request) (domain.Result, error) {
	return f.res, f.err
}

func TestAuthorize_MissingFields(t *testing.T) {
	h := NewHandler(fakeAuthorizer{})
	rec := httptest.NewRecorder()
	h.Authorize(rec, httptest.NewRequest(http.MethodPost, "/authorize",
		strings.NewReader(`{"principal":"bob"}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAuthorize_InvalidJSON(t *testing.T) {
	h := NewHandler(fakeAuthorizer{})
	rec := httptest.NewRecorder()
	h.Authorize(rec, httptest.NewRequest(http.MethodPost, "/authorize",
		strings.NewReader(`not json`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestAuthorize_OK(t *testing.T) {
	h := NewHandler(fakeAuthorizer{res: domain.Result{Decision: domain.Allow, EntitiesLoaded: 4}})
	rec := httptest.NewRecorder()
	h.Authorize(rec, httptest.NewRequest(http.MethodPost, "/authorize",
		strings.NewReader(`{"principal":"bob","action":"view","resource":"design"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp authorizeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Decision != "allow" || resp.EntitiesLoaded != 4 {
		t.Errorf("resp = %+v, want {allow 4}", resp)
	}
}

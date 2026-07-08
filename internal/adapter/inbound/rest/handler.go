// Package rest is the inbound (driving) adapter: it turns HTTP requests into
// calls on the Authorizer port and serializes the result. It holds no business
// logic — parsing and shaping only.
package rest

import (
	"encoding/json"
	"net/http"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/port"
)

// Handler exposes the authorization use case over HTTP.
type Handler struct {
	authz port.Authorizer
}

func NewHandler(authz port.Authorizer) *Handler {
	return &Handler{authz: authz}
}

type authorizeRequest struct {
	Principal string `json:"principal"`
	Action    string `json:"action"`
	Resource  string `json:"resource"`
}

type authorizeResponse struct {
	Decision       string `json:"decision"`
	EntitiesLoaded int    `json:"entities_loaded"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Authorize handles POST /authorize.
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	var req authorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if req.Principal == "" || req.Action == "" || req.Resource == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "need {principal, action, resource}"})
		return
	}

	res, err := h.authz.Authorize(r.Context(), domain.Request{
		Principal: req.Principal,
		Action:    req.Action,
		Resource:  req.Resource,
	})
	if err != nil {
		// Do not leak internal/driver detail to the client (see gotcha G8).
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "authorization failed"})
		return
	}

	writeJSON(w, http.StatusOK, authorizeResponse{
		Decision:       string(res.Decision),
		EntitiesLoaded: res.EntitiesLoaded,
	})
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

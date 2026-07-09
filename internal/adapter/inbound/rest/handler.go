// Package rest is the inbound (driving) adapter: the benchmark facade. It turns
// HTTP requests into engine checks via the core Router and serializes results.
// No business logic lives here — parsing and shaping only.
package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/service"
)

// checker is the slice of the core Router the handler needs.
type checker interface {
	Check(ctx context.Context, engine string, req domain.Request) (domain.Result, error)
	Engines() []string
}

type Handler struct {
	router checker
}

func NewHandler(router checker) *Handler {
	return &Handler{router: router}
}

type authorizeRequest struct {
	Engine       string          `json:"engine"`
	Model        string          `json:"model"`
	Principal    string          `json:"principal"`
	Action       string          `json:"action"`
	ResourceType string          `json:"resource_type"`
	Resource     string          `json:"resource"`
	Context      json.RawMessage `json:"context"`
}

type authorizeResponse struct {
	Engine         string `json:"engine"`
	Model          string `json:"model"`
	Decision       string `json:"decision"`
	EntitiesLoaded int    `json:"entities_loaded"`
	DurationMS     int64  `json:"duration_ms"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Authorize handles POST /v1/authorize.
func (h *Handler) Authorize(w http.ResponseWriter, r *http.Request) {
	var req authorizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON body"})
		return
	}
	if req.Engine == "" || req.Model == "" || req.Principal == "" ||
		req.Action == "" || req.ResourceType == "" || req.Resource == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{
			Error: "need {engine, model, principal, action, resource_type, resource}",
		})
		return
	}
	reqCtx, err := decodeContext(req.Context)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	// "spicedb" without a mode suffix means SpiceDB's production default.
	engine := req.Engine
	if engine == "spicedb" {
		engine = "spicedb-minimize_latency"
	}

	start := time.Now()
	res, err := h.router.Check(r.Context(), engine, domain.Request{
		Model:        domain.Model(req.Model),
		Principal:    req.Principal,
		Action:       req.Action,
		ResourceType: req.ResourceType,
		Resource:     req.Resource,
		Context:      reqCtx,
	})
	if err != nil {
		if errors.Is(err, service.ErrBadRequest) {
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
			return
		}
		// Engine/backend fault (Postgres down, SpiceDB unreachable, …):
		// 502 with a generic body — internals go to the server log only.
		log.Printf("authorize failed: engine=%s model=%s: %v", engine, req.Model, err)
		writeJSON(w, http.StatusBadGateway, errorResponse{Error: "authorization backend failed"})
		return
	}
	writeJSON(w, http.StatusOK, authorizeResponse{
		Engine:         engine,
		Model:          req.Model,
		Decision:       string(res.Decision),
		EntitiesLoaded: res.EntitiesLoaded,
		DurationMS:     time.Since(start).Milliseconds(),
	})
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "engines": h.router.Engines()})
}

// decodeContext parses the request context, normalizing JSON numbers to int64
// (the domain's integer type) and homogeneous string arrays to []string.
func decodeContext(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid context: %w", err)
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch t := v.(type) {
		case float64:
			if t != float64(int64(t)) {
				return nil, fmt.Errorf("context %q: only integer numbers are supported", k)
			}
			out[k] = int64(t)
		case []any:
			strs := make([]string, 0, len(t))
			for _, e := range t {
				s, ok := e.(string)
				if !ok {
					return nil, fmt.Errorf("context %q: only string arrays are supported", k)
				}
				strs = append(strs, s)
			}
			out[k] = strs
		case bool, string:
			out[k] = t
		default:
			return nil, fmt.Errorf("context %q: unsupported value type", k)
		}
	}
	return out, nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

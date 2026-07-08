package rest

import "net/http"

// NewRouter wires the routes using the stdlib ServeMux method-based patterns
// (Go 1.22+). No third-party router is needed for two endpoints.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /authorize", h.Authorize)
	mux.HandleFunc("GET /health", h.Health)
	return mux
}

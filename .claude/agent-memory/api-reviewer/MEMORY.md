# api-reviewer memory (maintained by the agent; reseeded 2026-07-09 after the benchmark pivot)

## Endpoints (facade — cmd/authz-service, internal/adapter/inbound/rest)
- `POST /v1/authorize` — body `{ engine, model, principal, action, resource_type, resource,
  context? }`; all except context required → else `400 { error }`. Response
  `{ engine, model, decision, entities_loaded, duration_ms }`.
- `GET /health` — `{ ok, engines: [cedar, spicedb-minimize_latency, spicedb-fully_consistent] }`.

## Contract details
- `engine: "spicedb"` is aliased to `spicedb-minimize_latency` (SpiceDB's production default).
- Context normalization is part of the contract: JSON numbers must be integral → int64 (fractional
  → 400); arrays must be homogeneous string arrays → []string; bool/string pass through.
- Models: rbac | rebac | abac | pbac | acl. Actions per model: execute/view/render · doc.view ·
  doc.read · po.approve · acl.view/acl.edit. Resource types: Endpoint/Page/Component,
  RebacDocument, AbacDocument, PurchaseOrder, AclDocument.
- Errors return the router's message with 400; internals (SQL/gRPC details) stay server-side.

## Testing
- `internal/adapter/inbound/rest/handler_test.go` covers validation, context normalization, and the
  spicedb alias. The http/ directory has 10 `.http` files with real seeded IDs (allow + deny).

# api-reviewer memory (seeded from initial scan — the agent maintains this going forward)

## Endpoints
- `POST /authorize` — body `{ principal, action, resource }` (all required strings). Response
  `{ decision: "allow"|"deny", entities_loaded: number }`. Missing field or invalid JSON → `400 { error }`.
- `GET /health` — `{ ok: true }`.

## Contract / behavior (Go rewrite, hexagonal)
- Inbound adapter: `internal/adapter/inbound/rest` (stdlib net/http, method-based ServeMux). It only
  decodes/validates, calls the `Authorizer` port, and encodes the result.
- Cedar decision comes from the outbound cedar adapter (`cedar.Authorize`), mapped to
  `domain.Decision` ("allow"/"deny"); `entities_loaded` = number of entities the repository returned.

## History / notes
- Migrated from Node/Express to Go on 2026-07-09. Two prior gotchas are now FIXED in the Go handler:
  entity data is no longer logged, and 500s return a generic message (no raw error leaked).
- Handler validation is covered by `internal/adapter/inbound/rest/handler_test.go`.

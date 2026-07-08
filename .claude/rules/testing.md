---
paths:
  - "**/*_test.go"
---
# Testing conventions
- Go's built-in testing (`go test ./...`, run via `make test`). No third-party assertion library —
  use table-driven tests and `t.Run` subtests.
- The 8 canonical decision scenarios are pinned in
  `internal/adapter/outbound/cedar/engine_test.go` (`TestDecisionMatrix`) against the real
  `policies/policy.cedar` with hand-built seed entities mirroring `db/init.sql`. HTTP validation is
  covered in `internal/adapter/inbound/rest/handler_test.go` with a fake `Authorizer`.
- **Any change to `policies/policy.cedar` or an outbound adapter must keep `make test` green** (or
  intentionally update the matrix with justification). This is the guard against silent authz drift.
- The Postgres adapter is currently covered only end-to-end (via the running stack), not by a unit
  test. TODO: add an integration test against a disposable Postgres seeded from `db/init.sql`.
- Name tests `*_test.go` beside the code; keep them hermetic (no shared global state, no real network
  unless explicitly an integration test).

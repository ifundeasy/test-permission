---
paths:
  - "**/*_test.go"
---
# Testing conventions
- Go's built-in testing via `make test`; table-driven tests with `t.Run` subtests; no third-party
  assertion library.
- Per-model Cedar policy tests live in `internal/adapter/outbound/cedar/engine_test.go` with
  hand-built fixtures that mirror what the Postgres loader emits — including the PBAC
  chained-deref case (`resource.policy.max_amount`), which is a load-bearing cedar-go behavior.
- HTTP facade validation tests: `internal/adapter/inbound/rest/handler_test.go` (fake router;
  JSON-number → int64 normalization is part of the contract).
- The REAL correctness check is the **equivalence gate**: `make seed-test` then `make verify`
  (miniature dataset, both engines, ground-truth tuples). Run it after ANY change to policies,
  schema.zed, loaders, adapters, generator, or sampler.
- Keep unit tests hermetic (no DB/network); the gate covers integration.

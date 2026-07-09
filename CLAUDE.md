# cedar-vs-spicedb-benchmark — Claude Code project context

## Summary
An **apple-to-apple benchmark of two authorization engines** on one Postgres server:
**Cedar** (embedded `cedar-go` — the PEP fetches entities from Postgres and evaluates in-process)
vs **SpiceDB** (standalone server owning its own datastore, checked over gRPC). Five access models —
**RBAC, ReBAC, ABAC, PBAC, ACL** — each with **≥1M rows per engine**, seeded from ONE deterministic
generator so both engines hold the identical logical dataset. An equivalence gate (Cedar == SpiceDB
== expected on every ground-truth tuple) must pass before any timing runs. Domain: imagined
enterprise SaaS ERP "Nusantara ERP" (orgs w/ subsidiaries, divisions/roles incl. custom, personas,
and an app-permission registry of 5 microservices as metadata — endpoints/pages/components).

## Stack
- Go 1.26 (module `github.com/ifundeasy/test-permission`) · stdlib `net/http` (method-based ServeMux)
- `cedar-policy/cedar-go` 1.8.0 (embedded engine) · `authzed/authzed-go` v1.10.0 (SpiceDB client)
- `jackc/pgx/v5` · Postgres 18.4 · SpiceDB v1.54.0 (`authzed/spicedb`) · Docker Compose · Makefile

## Commands (see `make help`)
- Stack: `make up` (postgres + spicedb-migrate + spicedb + facade) · `make down` · `make reset` (DESTRUCTIVE)
- Seed: `make seed` (full, both engines + tuples, batch 1000, resumable) · `make seed-test` (miniature)
  · seeder flags: `-engine cedar|spicedb|tuples|all -scale full|test -wipe -resume`
- Gate: `make verify` — equivalence gate; MUST pass before benchmarking
- Bench: `make bench` → console + `bench/results/<ts>.{csv,json}`
- Dev: `make test` · `make vet` · `make fmt` · `make build` · `make run`

## Architecture — hexagonal, two engines behind one port
```
cmd/authz-service      facade: POST /v1/authorize {engine, model, principal, action, resource_type, resource, context}
cmd/authz-seed         deterministic dual-engine seeder (canonical stream → sinks)
cmd/authz-bench        -mode verify (equivalence gate) | -mode run (timing cells)
internal/core          domain (Request+Context, Entity, Model) · port (Decider, EntityLoader) · service (Router)
internal/adapter/outbound/cedar     Decider: load entities (via EntityLoader) + in-process evaluate
internal/adapter/outbound/postgres  EntityLoader: per-model SQL loaders (schema "cedar")
internal/adapter/outbound/spicedb   Decider: gRPC CheckPermission (consistency mode per instance)
internal/seed          generator (fixed-seed PCG; per-phase rng streams) · writers · tuple sampler
internal/bench         measurement harness (warmup, concurrency, percentiles)
internal/catalog       loads catalog/services.json (the ERP registry metadata)
policies/*.cedar       ONE file per model; data-driven generic rules (PBAC = policy-parameter entities)
schema/spicedb/schema.zed  definitions + caveats (doc_attrs, po_limits)
db/bootstrap.sh        initdb: roles cedar/spicedb + schemas + per-role search_path — NEVER data
```

## Critical invariants (breaking any = broken benchmark)
1. **Determinism**: same `-seed` ⇒ byte-identical dataset on both engines. Never iterate Go maps in
   the generator's emit paths; per-phase PRNG streams (`phaseTags`) must not be reordered.
2. **Equivalence**: `policies/*.cedar` and `schema.zed` are twins — any semantic change on one side
   MUST be mirrored on the other AND `make verify` re-run. Tuples' `expected` comes from the
   generator's ground truth.
3. **Batching**: 1000 records/write on BOTH engines (multi-row INSERT ON CONFLICT DO NOTHING;
   SpiceDB WriteRelationships OPERATION_TOUCH — server cap is exactly 1000).
4. **Isolation**: Cedar app data only in schema `cedar`; SpiceDB tables only in schema `spicedb`
   (via role search_path). Nothing in `public`.
5. **Scale changes need `-wipe`** — IDs overlap between scales; ON CONFLICT DO NOTHING would keep
   stale rows and silently diverge the engines.

## Known environment quirks (verified, see README "Findings")
- SpiceDB `ImportBulkRelationships` (binary COPY) breaks vs Postgres 18 (`08P01`); use TOUCH writes.
- SpiceDB object IDs forbid `:` → registry IDs use `/` (`ep/finance/invoice-approve`).
- SpiceDB schema-level isolation on shared Postgres is a Postgres-side mechanism (search_path), not
  an official SpiceDB feature.

## Security notes
- Credentials in `.env` (never read/echo it); `.env.example` documents vars incl. per-engine DB
  roles and the SpiceDB preshared key.
- `policies/`, `schema/spicedb/`, and `internal/adapter/outbound/` remain security-critical: a bug
  is an authorization-decision error. See `.claude/rules/authz.md`.

## Working agreements
- Human documentation lives in `docs/` (01 use case, 02 architecture, 03 benchmark results) with
  README as the index — keep them in sync with code changes (Mermaid allowed there, never here).
- Respect hexagonal boundaries (core imports no adapter). Both engines implement `port.Decider`.
- After changes: `make test` + `make vet`; after policy/schema/loader/generator changes ALSO
  `make seed-test` + `make verify` (miniature) and report results. Never commit unless asked.

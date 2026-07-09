# security-auditor memory (maintained by the agent; reseeded 2026-07-09 after the benchmark pivot)

## What this repo is now
An apple-to-apple benchmark: Cedar (embedded cedar-go 1.8.0) vs SpiceDB (v1.54.0 server) on one
Postgres 18.4 server — schema `cedar` (app data, role cedar) vs schema `spicedb` (SpiceDB datastore,
role spicedb, isolated via per-role search_path). 5 access models, ≥3M rows/model/engine
(~35M combined; 1.2M personas, 40k org nodes depth ≤6).

## Authorization model (twin schemas — MUST stay in sync)
- `policies/*.cedar` (one per model) ⟷ `schema/spicedb/schema.zed` (definitions + caveats).
- RBAC: persona→role (Parents / role#assignee) + registry allowed_roles (1–4 roles/persona).
- ReBAC: doc→folder→org_unit→ancestors; membership at ancestor sees descendant docs; ~5% folders
  SHARED with a second same-root unit (fan-in, union semantics both sides).
- ABAC: clearance/division/status/region — data residency: region match OR clearance-4 override;
  Cedar forbid archived ⟷ caveat `doc_attrs`.
- PBAC: policy-parameter entities (Cedar; no templates in cedar-go) ⟷ caveat `po_limits`
  (static: active/min_amount/max_amount/regions; check-time: amount/region). Amount must sit
  INSIDE the window (below floor = petty cash → deny).
- ACL: viewers/editors sets ⟷ viewer/editor relations; editors can view on BOTH sides.
- Equivalence gate: `make verify` — must show 0 mismatch/0 error before any timing run.

## Known risks (tracked in .issues/)
- Weak local-dev defaults: cedar/spicedb role passwords default to role names; SpiceDB preshared
  key defaults to `benchkey` (G9).
- SpiceDB schema isolation is a Postgres search_path mechanism, not an official feature — re-smoke
  after every SpiceDB upgrade (G3).
- SpiceDB ImportBulk (binary COPY) breaks vs Postgres 18 → seeder uses WriteRelationships TOUCH (G1).
- ABAC principal attrs travel as check-time context to SpiceDB — fairness asymmetry, documented (G6).
- Host-run tools (seed/verify/bench) need SPICEDB_ENDPOINT=localhost:50051; the compose-internal
  default spicedb:50051 only resolves inside Docker (G12) — Makefile forces the host override.
- SpiceDB `CONDITIONAL` response = under-supplied caveat context = treated as failure everywhere.

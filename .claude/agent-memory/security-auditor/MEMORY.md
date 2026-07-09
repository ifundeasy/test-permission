# security-auditor memory (maintained by the agent; reseeded 2026-07-09 after the benchmark pivot)

## What this repo is now
An apple-to-apple benchmark: Cedar (embedded cedar-go 1.8.0) vs SpiceDB (v1.54.0 server) on one
Postgres 18.4 server — schema `cedar` (app data, role cedar) vs schema `spicedb` (SpiceDB datastore,
role spicedb, isolated via per-role search_path). 5 access models, ≥1M rows/model/engine.

## Authorization model (twin schemas — MUST stay in sync)
- `policies/*.cedar` (one per model) ⟷ `schema/spicedb/schema.zed` (definitions + caveats).
- RBAC: persona→role (Parents / role#assignee) + registry allowed_roles.
- ReBAC: doc→folder→org_unit→ancestors; membership at ancestor sees descendant docs.
- ABAC: clearance/division/status; Cedar forbid archived ⟷ caveat `doc_attrs` (`status != "archived"`).
- PBAC: policy-parameter entities (Cedar; no templates in cedar-go) ⟷ caveat `po_limits`
  (static: active/max_amount/regions; check-time: amount/region).
- ACL: viewers/editors sets ⟷ viewer/editor relations; editors can view on BOTH sides.
- Equivalence gate: `make verify` (42,836 ground-truth tuples, 0 mismatch on 2026-07-09).

## Known risks (tracked in .issues/)
- Weak local-dev defaults: cedar/spicedb role passwords default to role names; SpiceDB preshared
  key defaults to `benchkey` (G9).
- SpiceDB schema isolation is a Postgres search_path mechanism, not an official feature — re-smoke
  after every SpiceDB upgrade (G14).
- SpiceDB ImportBulk (binary COPY) breaks vs Postgres 18 → seeder uses WriteRelationships TOUCH (G11).
- ABAC principal attrs travel as check-time context to SpiceDB — fairness asymmetry, documented (G15).
- SpiceDB `CONDITIONAL` response = under-supplied caveat context = treated as failure everywhere.

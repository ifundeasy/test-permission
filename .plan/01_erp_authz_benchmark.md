# Plan 01 — ERP Authorization Benchmark: Cedar vs SpiceDB on one Postgres

Status: **implemented & verified** (2026-07-09)

> Outcome: all acceptance criteria met — search_path isolation smoke-tested (9 SpiceDB tables in
> schema `spicedb` only); full-scale seed ≥1M rows/model on BOTH engines (Cedar 6.76M rows,
> SpiceDB 5.49M live relationships); **equivalence gate PASSED at full scale** (42,836 ground-truth
> tuples × 2 engines, 0 mismatch / 0 error); facade + 10 `.http` files e2e-verified; benchmark
> cells measured to `bench/results/`. Deviations from plan, discovered by evidence:
> (1) SpiceDB ImportBulk (binary COPY) is incompatible with Postgres 18 → seeder uses
> WriteRelationships+TOUCH (still batch=1000, per requirement); (2) SpiceDB object IDs forbid `:`
> → registry IDs use `/`; (3) in-batch duplicate relationships rejected → writer dedupes per batch.
> See `.issues/02_gotcha_20260709.md` (G11–G15).

## Why (context & goal)
This repo pivots from a Cedar PEP demo into an **apple-to-apple benchmark of two authorization
engines** in an enterprise ERP context:

- **Cedar** — an embedded library (`cedar-go`). The PEP queries Postgres itself, shapes rows into
  entities, and calls the engine **in-process**. Data-fetch cost is on the application.
- **SpiceDB** — a standalone server (`authzed/spicedb`) that **owns its datastore** (its own schema
  on the same Postgres server) and answers `CheckPermission` over gRPC. Data traversal is inside
  the engine, with caching.

Both engines persist to the **same Postgres server**, isolated by **separate db schema + db user**
(`cedar` / `spicedb`) so the comparison shares one storage substrate.

Measured: decision latency/throughput for the five access-control models — **RBAC, ABAC, ReBAC,
PBAC, ACL** — each backed by **≥1,000,000 queryable rows per engine**.

## The imagined domain — "Nusantara ERP" (SaaS, B2B+B2C, low→enterprise market)
- **Platform level:** the SaaS owner (app admins) above all tenants.
- **Tenants:** ~2,000 root organizations → subsidiaries tree (depth ≥3), ~20k org nodes total.
- **Divisions:** 5 defaults per org node (finance, procurement, hr, sales, operations) + per-org
  custom (e.g. halal-compliance, export-desk) → ~140k rows.
- **Roles:** 5 default types (owner, admin, manager, staff, auditor) instantiated per root org +
  custom per org → ~16k role definitions (+5 platform roles).
- **Identity:** 300k root **accounts** → 480k **personas** (children of accounts); each persona
  belongs to exactly **one** org node, one division, and carries ABAC attributes (clearance 1–4,
  employment_type, region).
- **Application registry (metadata, not running servers):** 5 microservices — identity, finance,
  procurement, hr, sales — with **54 endpoints, 42 UI pages, ~420 UI components**, every one a
  first-class permission target, defined in `catalog/services.json`.

## Access-model mapping (the apple-to-apple core)
| Model | Benchmark question | Cedar realization | SpiceDB realization | ≥1M countable rows |
|---|---|---|---|---|
| RBAC | May persona P execute endpoint / view page / render component X? | persona's roles as entity Parents; resource attr `allowed_roles` (set); generic `principal in resource.allowed_roles` rule | `role#assignee@persona`; `endpoint/page/component#allowed_role@role#assignee` | 1.008M persona_roles + 128k role_grants |
| ReBAC | May persona P view document D through folder→org-unit→ancestor chain? | Parents graph doc→folder→unit→ancestors; `resource in principal.member_of` | `org_unit#parent/member/manager`, `folder#parent/unit`, `rebac_document#folder`, arrow permissions | 600k docs + 120k folders + 520k memberships + 18k unit edges |
| ABAC | May persona P (clearance, division) read doc D (classification, division, status)? | attribute comparison + `forbid` on archived; both sides' attrs loaded from Postgres | caveat `doc_attrs` (CEL) on wildcard `#reader@persona:*`; resource attrs stored as **static caveat context**, principal attrs sent at check time | 1.0M attribute-bearing docs / caveated rels |
| PBAC | May persona P approve PO N (amount A, region R) under the governing org policy? | **policy-parameter entities** (max_amount, regions, active) + `principal in resource.policy` + `context.amount/region` (cedar-go has no templates) | `pbac_policy#assignee`; `purchase_order#governed_by@policy` with caveat `po_limits` (static params) + check-time amount/region | 700k assignments + 300k PO links + 40k policies |
| ACL | May persona P view/edit doc D via direct grant? | `resource.viewers/editors` entity sets; `.contains(principal)` | `acl_document#viewer/#editor@persona` | 1.05M direct entries |

Totals ≈ 5.5M countable rows per engine (Cedar schema ≈6.5M rows incl. basis data; SpiceDB ≈5.6M
relationships).

## Key verified facts (evidence-based, from official docs)
- SpiceDB **v1.54.0**, image `authzed/spicedb:v1.54.0`; datastore migration `spicedb datastore migrate head`.
- authzed-go **v1.10.0**; `WriteRelationships` server cap default **1000/call** (matches the
  required batch size); bulk import via `RetryableBulkImportRelationships(..., Touch)` = idempotent.
- Caveats: typed CEL params; **relationship-static context wins over check-time context**; request
  context cap 4096B. Consistency: `minimize_latency` (cached, ~5s quantization) vs `fully_consistent`.
- cedar-go v1.8.0: **no policy templates** (hence PBAC-as-entities), value types incl. Long/Set/
  Record/EntityUID; `in` accepts entity sets; request `context` supported.
- ⚠️ **Risk:** SpiceDB does not document schema selection on Postgres. Mitigation: Postgres-side
  `ALTER ROLE spicedb IN DATABASE authz SET search_path = spicedb` + smoke test; **fallback** = two
  databases on the same server. Rest of design unaffected.

## Fairness rules (documented in README when shipped)
1. Same Postgres server, same host, warmed pools/buffers for both engines.
2. SpiceDB measured in **both** `fully_consistent` (no cache advantage — fairest vs Cedar's live
   fetch) and `minimize_latency` (its production posture); reported separately.
3. Cedar measured **end-to-end** (Postgres fetch + eval) and **eval-only** (engine cost in isolation).
4. ABAC/PBAC note: SpiceDB receives principal attrs / amount+region via check context (client
   supplies), Cedar fetches everything from Postgres — inherent architectural difference, stated.
5. **Equivalence gate before timing:** 10k tuples/model (~50/50 allow/deny, ground truth from the
   generator); Cedar decision == SpiceDB decision == expected, else the benchmark refuses to run.

## Architecture / repo layout (target)
```
cmd/authz-service/    HTTP facade: POST /v1/authorize {engine, model, principal, action, resource, context}
cmd/authz-seed/       seeder CLI: -engine cedar|spicedb|both -seed 42 -batch 1000 [-resume]
cmd/authz-bench/      -verify (equivalence gate) | -run (timing cells)
internal/core/        domain (Request+Context, Entity attrs incl Long/Set), ports (Decider), service
internal/adapter/     inbound/rest · outbound/cedar · outbound/postgres (per-model loaders) · outbound/spicedb
internal/seed/        generator (deterministic PCG, ground truth) · writer_postgres · writer_spicedb · progress
internal/bench/       sampler · runner · report
catalog/services.json 5 services → endpoints/pages/components (+permission keys)
policies/*.cedar      one file per model (rbac, rebac, abac, pbac, acl) — all data-driven
schema/spicedb/schema.zed
db/bootstrap.sh       initdb: ONLY roles/schemas/search_path/grants — never data
http/                 10 files: {cedar,spicedb}.{rbac,rebac,abac,pbac,acl}.http
bench/results/        <timestamp>.csv / .json
```
Removed: old demo (`db/init.sql`, `policies/policy.cedar`, `demo/cedar.http`, old loaders/tests).

## Seeding design (locked requirements)
- Trigger: **Makefile → Go seeder** (`make seed`, `seed-cedar`, `seed-spicedb`) — never at Postgres
  container init.
- **Batch = 1000** per engine: Postgres multi-row INSERT `ON CONFLICT DO NOTHING`; SpiceDB
  `RetryableBulkImportRelationships` (Touch).
- Deterministic: one canonical generator (seed 42) → both writers; same logical dataset both engines.
- Progress: `phase=abac 412,000/1,000,000 rows 18,400 rows/s ETA 32s` every 25 batches + summary.
- Resumable: `seed_checkpoints(engine, phase, last_batch)` + idempotent writes.
- Side output: `bench/tuples/<model>.json` — sampled ground-truth tuples for verify/bench.

## Benchmark design
- Cells: 5 models × {cedar-e2e, cedar-eval-only, spicedb-fully-consistent, spicedb-minimize-latency}
  × concurrency {1, 8, 32}; warmup 5k checks; 30s or `-n` per cell.
- Metrics: p50/p95/p99/max, mean, throughput; output console table + CSV/JSON in `bench/results/`.

## Implementation order
1. Infra: bootstrap.sh + compose (spicedb + migrate job, fresh named volume) + **search_path smoke test**.
2. Core + adapters + engine schemas (5 cedar files, schema.zed); remove old demo.
3. Catalog JSON + deterministic generator.
4. Writers + seeder CLI + Makefile targets.
5. Equivalence gate (`-verify`).
6. Bench runner + reports.
7. Facade endpoint + 10 `.http` files.
8. Housekeeping: README, CLAUDE.md/rules updates, `.issues/` reconcile, unit tests, adversarial review.

## Acceptance criteria
- `make up` → postgres + spicedb healthy; SpiceDB tables land in schema `spicedb` only.
- `make seed` → each model ≥1M countable rows on both engines (SQL-verified), batch=1000, visible progress.
- `make verify` → 50k-tuple equivalence gate passes.
- `make bench` → CSV/JSON with all cells; `make test` / `make vet` green.

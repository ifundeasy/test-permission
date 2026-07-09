# Plan 02 — 3M rows per use case, added complexity, resource monitoring, report.html

Status: **approved, in progress** (2026-07-09). Supersedes the scale defined in
`01_erp_authz_benchmark.md`; the benchmark's canonical dataset is now the one described here.

## Goal
Raise the benchmark dataset to **≥3,000,000 countable rows per use case per engine** with added
complexity, monitor CPU/memory per benchmark cell, publish `docs/report.html` (summary + key
insights of docs/01–03, self-contained, EN/ID), and sweep the entire project so this is the only
scale described anywhere. `bench/` is committed (no longer gitignored).

## Dataset (FullScale — internal/seed/types.go)
- Orgs: 2,500 roots → 40,000 nodes, subsidiary depth ≤ 6 (levels 0–5)
- Identity: 750k accounts → 1.2M personas (1 persona ↔ 1 org node)
- RBAC: roles/persona 1–4 weighted (avg ~2.5) → ~3.0M persona_roles + ~250k role_grants (12/role)
- ReBAC: 9 folders/org (≤9 nesting, ~5% SHARED with a second same-root unit — graph fan-in),
  5 docs/folder → 1.8M docs; memberships ~1.3M (100k manager edges)
- ABAC: 3.0M documents — now with **data-residency** rule (region match OR clearance-4 override)
- PBAC: 100k policies (40/root) with **amount floor** (min ≤ amount ≤ max), ~2.26M assignments,
  800k POs
- ACL: 1M docs × 2–4 entries (avg 3) → 3.0M
- Totals expected ≈ Cedar ~19M rows · SpiceDB ~16M live rels ≈ ~35M combined

## Semantics added (twin-schema, equivalence-gated)
1. ABAC residency: `(principal.region == resource.region || principal.clearance == 4)` — caveat
   `doc_attrs` gains `region`/`principal_region`.
2. PBAC floor: `min_amount ≤ amount ≤ max_amount` — caveat `po_limits` gains `min_amount`;
   sampler adds a below-floor deny variant.
3. ReBAC shared folders: ~5% folders carry one extra org-unit edge (same root; union semantics
   both sides).
Miniature gate result: PASSED (all 5 models, 0 mismatch/0 error) before the full burn.

## Resource monitoring (user requirement)
`internal/bench/resources.go`: host-level CPU% (/proc/stat deltas) + used memory (/proc/meminfo)
sampled every 500ms inside each cell's timed window; avg/max attached to every Cell, exported in
CSV/JSON, shown in the console table, and reported in docs/03 (plus total system RAM in the
environment section).

## Deliverables checklist
- [x] Code: scale + complexity + resource monitor (build/vet/tests green; miniature gate PASSED)
- [ ] Full seed 3M `-wipe` → SQL verify ≥3M per model per engine + actual max nesting depth
- [ ] Full equivalence gate (0 mismatch/0 error) → benchmark run (60 cells + cpu/mem columns)
- [x] Gotcha register consolidated (`.issues/01_gotcha_20260709.md`; stale findings removed)
- [ ] Project sweep: docs/01–03 (incl. nested-path depth note), README, CLAUDE.md, agent memory,
      http/ refreshed from new tuples, stale bench artifacts removed (bench/ now committed)
- [ ] `docs/report.html` — summary + key insights of the three MDs (self-contained, EN/ID toggle)

## Verification
Counts ≥3M per model per engine (SpiceDB live-row predicate) · gate 0 mismatch · bench 0 errors ·
docs numbers == CSV (jq spot-checks) · report.html offline + toggle works · link check · grep sweep
(no stale scale claims) · `make test`/`make vet` green.

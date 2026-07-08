# security-auditor memory (seeded from initial scan — the agent maintains this going forward)

## Authorization model (policies/policy.cedar)
- RBAC: `permit(principal in Role::"admin", action, resource)`.
- ReBAC: owner rule (`principal == resource.owner`); group→folder→document via `in`
  (`Group::"engineering"` may `view` resources in `Folder::"eng"`).
- ABAC: `forbid` when `resource.confidential == true` unless `principal == resource.owner`.
- Default DENY; `forbid` overrides all `permit` (admin `carol` is denied the confidential `secret`,
  but owner `alice` is allowed). Verified by `TestDecisionMatrix` (all 8 cases).

## Data / entities (Go, hexagonal)
- `internal/adapter/outbound/postgres` builds entities per request: User + Group/Role parents;
  Document with `owner` (entity ref) + `confidential`; full Folder ancestry via a recursive CTE.
- `internal/adapter/outbound/cedar` maps domain entities → Cedar (`owner` = EntityUID,
  `confidential` = Boolean). The core (`internal/core`) is Cedar/DB-agnostic.
- Tables: users, groups, roles, folders(self-ref), documents(owner_id, confidential, folder_id),
  memberships(user_id, parent_type, parent_id).

## Known gotchas / risks (statuses tracked in .issues/01_gotcha_20260709.md)
- OPEN — weak default DB credential `"app"` in `internal/config/config.go` (G9).
- OPEN — no migration tool; `init.sql` applies only on a fresh volume (G10).
- OPEN — no API contract/spec (G6); moving base-image tags in Dockerfile (G4).
- RESOLVED by the Go rewrite (2026-07-09): entity data no longer logged (G7); 500s no longer leak raw
  error text (G8); README file references and Node/Postgres version drift corrected (G5, G3).
- MITIGATED — the 8-scenario decision matrix is now an automated test (G1); Go has `gofmt`/`go vet`
  wired via the Makefile and the format hook (G2). The Postgres adapter still lacks a unit test.

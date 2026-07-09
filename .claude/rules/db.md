---
paths:
  - "db/**"
  - "internal/adapter/outbound/postgres/**"
  - "internal/seed/**"
---
# Database (Postgres, one server — two engine schemas)
- `db/bootstrap.sh` runs ONCE at initdb: creates roles `cedar`/`spicedb`, their schemas, and
  per-role `search_path`. It must NEVER contain data — all seeding is `make seed` (Go seeder).
- Schema `cedar` = the app data Cedar's PEP queries (DDL lives in
  `internal/seed/writer_postgres.go`, applied by the seeder; no migration tool by design — the
  dataset is regenerable). Schema `spicedb` = SpiceDB's internal tables; NEVER write to it directly.
- All queries parameterized (`$1`, …); drain and close `pgx.Rows`, check `rows.Err()`.
- Loader lookup paths must stay indexed (see the `CREATE INDEX` list in the DDL): persona_roles,
  role_grants, unit_memberships, folders.parent_id, rebac_documents.folder_id, pbac_assignments,
  acl_entries, organizations.parent_id.
- Reseeding at a different `-scale` REQUIRES `-wipe` (IDs overlap; ON CONFLICT DO NOTHING keeps
  stale rows and the engines silently diverge).
- Fresh volume = `make reset` (DESTRUCTIVE, denied to the assistant — run it yourself).

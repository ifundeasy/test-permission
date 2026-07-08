---
paths:
  - "db/**"
  - "internal/adapter/outbound/postgres/**"
---
# Database (Postgres via pgx)
- Data access lives only in `internal/adapter/outbound/postgres` (implements `port.EntityRepository`
  using `github.com/jackc/pgx/v5`). The core and other adapters never touch SQL.
- Schema + seed live in `db/init.sql`, applied once via Postgres `docker-entrypoint-initdb.d` on a
  fresh volume. There is **no migration tool** — editing `init.sql` only affects a fresh database.
  TODO: adopt a migration tool (e.g. golang-migrate) before schema changes ship.
- Applying an `init.sql` change requires recreating the volume (`make reset` / `docker compose down
  -v`, which is DENIED for the assistant by policy — run it yourself intentionally; data is lost).
- All queries must stay parameterized (`$1`, ...). Never build SQL by string concatenation.
- Always drain and close `pgx.Rows` and check `rows.Err()`. Index `folders.parent_folder_id`,
  `memberships.user_id`, and `documents.folder_id` if the dataset grows (see the `query-optimizer`
  agent); the folder ancestry is a recursive CTE.

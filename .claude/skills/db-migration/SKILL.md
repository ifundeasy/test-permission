---
description: Generate a database migration in the repo's detected migration tool (or bootstrap one if none exists)
argument-hint: <migration-name>
---
Create a migration for `$ARGUMENTS`.

1. Detect the migration tool (node-pg-migrate, Flyway, Prisma, Atlas, golang-migrate, Alembic, …)
   from the repo. If **none exists**, propose adopting one (pin its version) before proceeding — do
   not silently hand-edit a seed/init file that only applies to a fresh database.
2. Write both up and down migrations; keep them idempotent where the tool supports it.
3. Reference the schema conventions in `.claude/rules/` (naming, constraints, indexes).
4. State how to apply it locally and how it reaches other environments. Do not run destructive
   operations (drops/volume resets) without explicit confirmation. Do not commit.

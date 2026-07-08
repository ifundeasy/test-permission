---
description: Produce a concrete rollback procedure for a specific change (deploy, migration, config)
disable-model-invocation: true
argument-hint: <change description>
---
Produce a rollback plan for `$ARGUMENTS`.

1. Identify what the change touches (code, schema/data, config, infra) and whether each part is
   reversible. Database and data changes need special care — note irreversibility explicitly.
2. Give exact, ordered rollback steps with commands and verification after each.
3. State preconditions (backups, snapshots, feature flags) that must exist for rollback to be safe,
   and the point of no return.
4. Note data-loss risk and required confirmations. Author only — do not execute. Do not commit.

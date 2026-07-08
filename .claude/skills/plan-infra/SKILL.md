---
description: Dry-run infrastructure changes (terraform plan / helm diff / kubectl diff / compose config) and summarize impact — never apply
disable-model-invocation: true
argument-hint: [target dir or stack]
---
Produce a **read-only** preview of infra changes for `$ARGUMENTS`.

1. Detect the tool and run only its dry-run form: `terraform plan`, `helm diff upgrade`,
   `kubectl diff`, or `docker compose config` / `docker compose build` (no `up`/`apply`).
2. Summarize what would change: created / updated / **destroyed** resources, and any that force
   replacement or data loss — call those out prominently.
3. Flag risky diffs (public exposure, deleted stateful resources, secret changes).
4. **Never** run apply/destroy/upgrade/delete — those are denied and guarded. Tell the user the exact
   command to run themselves if they intend to apply.

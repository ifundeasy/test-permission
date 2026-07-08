---
description: Assess delivery/technical risk of a change or plan and propose mitigations
argument-hint: <change, plan, or area>
---
Assess risk for `$ARGUMENTS`.

1. Enumerate risks across dimensions: correctness, security/authorization, data integrity,
   availability/rollback, performance, dependencies, and delivery/timeline.
2. For each: likelihood × impact, the leading indicator to watch, and a concrete mitigation or
   contingency.
3. Highlight the top 3 risks and any blockers that should gate the change.
4. Output a ranked risk table. Read-only — assessment, not implementation.

---
description: Derive a structured test plan from a spec, diff, or feature description
argument-hint: <feature | diff | spec>
---
Produce a test plan for `$ARGUMENTS`.

1. Identify the behavior under test and its acceptance criteria; enumerate inputs, states, and
   boundaries (happy path, edge cases, error paths, security/authorization cases).
2. Organize as a table: scenario | preconditions | steps | expected result | type (unit/integration/e2e).
3. Prioritize by risk and call out what must be automated vs exploratory.
4. Note test data and environment needs. Output the plan only — do not write the tests here.

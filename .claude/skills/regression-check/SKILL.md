---
description: Map a change to its blast radius and the tests/areas that must be re-verified
argument-hint: [diff, commit range, or file]
---
Assess regression risk for `$ARGUMENTS` (default: the working diff).

1. Determine what changed and trace its dependents (callers, shared modules, data/schema, config).
2. List impacted behaviors and the existing tests that cover them; flag impacted areas with **no**
   test coverage as the highest risk.
3. Recommend the minimal set of tests to run/add before shipping, ordered by risk.
4. For an authorization service, always re-verify the full allow/deny decision matrix. Read-only.

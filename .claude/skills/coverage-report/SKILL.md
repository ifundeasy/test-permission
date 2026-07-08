---
description: Run the test suite with coverage and report gaps ranked by risk
argument-hint: [path]
---
Generate and interpret coverage for `$ARGUMENTS` (or the whole suite).

1. Run the repo's test command with its coverage flag (detect the tool). If coverage isn't set up,
   propose the minimal config to enable it.
2. Report overall and per-area coverage, but prioritize by **risk**, not raw percentage: highlight
   uncovered security/authorization, error-handling, and boundary code.
3. Recommend the specific tests that would most reduce risk. Do not chase 100% for its own sake.
4. Read-only — report numbers and gaps; do not weaken existing tests.

---
description: Review database queries and schema for correctness, indexing, parameterization, and N+1/recursion cost
argument-hint: [file or query]
---
Review the queries in `$ARGUMENTS` (or the repo's data-access layer).

1. Correctness: joins, filters, NULL handling, transaction boundaries, and recursive CTE termination.
2. Safety: all queries parameterized — never string-concatenate user input.
3. Performance: indexes for every WHERE/JOIN/ORDER-BY column and recursive-CTE anchor; flag N+1
   loops and unbounded result sets; suggest `EXPLAIN (ANALYZE)` where useful.
4. Report each finding with location, impact, and a concrete index/rewrite. Do not modify data.

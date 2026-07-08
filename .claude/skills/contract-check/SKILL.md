---
description: Check API implementation against its contract (OpenAPI / proto / GraphQL SDL); report drift both ways
argument-hint: [spec path]
---
Check the implementation against the API contract.

1. Locate the contract (OpenAPI/Swagger, gRPC `.proto`, GraphQL SDL). If **none exists**, say so and
   offer to generate a spec from the current routes (do not invent behavior).
2. Compare both directions: routes/fields/status codes in the code vs the spec — report endpoints or
   fields present in one but not the other, and type/required mismatches.
3. Reference any path-scoped contract rule in `.claude/rules/` and read specs from `.claude/docs/`
   on demand rather than inlining them.
4. Output a drift table (contract ↔ code) with concrete fixes. Read-only.

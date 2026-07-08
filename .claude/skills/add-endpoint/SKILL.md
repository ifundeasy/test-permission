---
description: Scaffold a new API endpoint with input validation, handler, and a test, matching the repo's stack and conventions
argument-hint: <method> <path> [purpose]
---
Scaffold an endpoint for `$ARGUMENTS`, conforming to the repo's detected framework, router, and
conventions (read `CLAUDE.md` and `.claude/rules/` first).

1. Locate the routing/handler layer and match its existing style (framework, error shape, module system).
2. Add the route with **input validation first** (reject malformed input with the repo's standard
   error response), then the handler body, then wiring/registration.
3. Keep business/data logic out of the handler — delegate to the existing service/data layer.
4. Add or update a test covering: happy path, validation failure, and one error path.
5. Run the repo's test/lint commands and report results. Do not commit.

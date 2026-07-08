---
description: Write API/integration tests for endpoints in the repo's detected test framework; bootstrap the test setup if none exists
argument-hint: [endpoint or file]
---
Write tests for `$ARGUMENTS` (or the most recently changed endpoints).

1. Detect the test framework and runner from the repo. If **none exists**, bootstrap one (pin the
   version), add a `test` script/target, and note the choice in `CLAUDE.md` via `/update-context`.
2. Cover per endpoint: happy path, each validation failure, and error/failure paths. For an
   authorization service, cover the full decision matrix (every principal/action/resource → expected
   allow/deny) — treat existing demo scenarios as the spec.
3. Use a disposable database/fixtures for integration tests; seed from the repo's init/seed source.
4. Run the tests and report pass/fail with output. Do not weaken assertions to make them pass. Do
   not commit.

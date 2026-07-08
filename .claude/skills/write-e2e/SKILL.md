---
description: Author end-to-end tests in the repo's detected e2e framework; bootstrap one if none exists
argument-hint: <flow or scenario>
---
Write e2e tests for `$ARGUMENTS`.

1. Detect the e2e framework (Playwright, Cypress, or an HTTP/API-level harness for a backend). If
   **none exists**, propose and pin one; for an API service, prefer black-box HTTP tests against a
   running stack (compose) over a browser tool.
2. Model real user/consumer flows end-to-end; assert on observable outputs, not internals.
3. Make tests deterministic: control test data, wait on real readiness (healthchecks), avoid sleeps.
4. Run against a disposable environment and report results. Do not commit.

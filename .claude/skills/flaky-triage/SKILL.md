---
description: Triage flaky tests — identify nondeterminism sources and propose stabilizations
argument-hint: [test name or suite]
---
Triage flakiness in `$ARGUMENTS`.

1. Reproduce by running the test(s) repeatedly; capture pass/fail ratio and any ordering dependence.
2. Classify the root cause: timing/races, shared/leaked state, real time or randomness, network/IO,
   test-order coupling, or under-specified assertions.
3. Propose a concrete stabilization (proper awaits/readiness checks, isolation/cleanup, injected
   clock/seed, deterministic fixtures) — never a blanket retry or `sleep` as the fix.
4. Report cause + fix per test. Do not quarantine silently; record the decision.

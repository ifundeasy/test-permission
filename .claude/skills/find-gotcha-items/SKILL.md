---
description: High-level audit that finds non-obvious gotchas/risks and logs them to .issues/
disable-model-invocation: true
argument-hint: [optional scope: path or subsystem]
---
First, if not already on the most capable model with extended thinking enabled, ask the user to
switch (`/model` → Opus; enable thinking) and wait. Read existing `.issues/NN_gotcha_*.md` to avoid
duplicates. Then run a HIGH-LEVEL analysis (in English) of the repo (or the $ARGUMENTS scope) for
gotchas — non-obvious traps, not style nits:
- hidden coupling, implicit ordering/timing assumptions, race conditions;
- silent failure modes, swallowed errors, missing validation at boundaries;
- config/build/deploy footguns (env-specific behavior, floating versions, silent default fallbacks);
- security/perf landmines and fragile invariants.

Write ONE file `.issues/NN_gotcha_YYYYMMDD.md` (next sequence number, today's date via `date +%Y%m%d`)
containing only NEW findings; reference still-open prior findings instead of repeating them. For EACH
new finding:
- **Title** | **Severity** (critical/high/medium/low) | **Status: open**
- **Location** (file/area + evidence) | **Why it's a gotcha** (the non-obvious part)
- **Impact** | **Suggested mitigation**
Do not fix anything — analysis only. Never commit. `/update-context` keeps these statuses current.

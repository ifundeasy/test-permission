---
description: Produce or update a project plan in .plan/ using maximum reasoning
disable-model-invocation: true
argument-hint: <plan-name>
---
First, if the session is not already on the most capable model with extended thinking enabled, ask
the user to switch (`/model` → Opus; enable thinking / Plan Mode) and wait. Then think step by step
and write a plan (in English) at `.plan/NN_$ARGUMENTS.md` (next sequence number, snake_case) covering:
goal, scope / non-goals, phased steps with owners and impact, risks, and acceptance criteria.
Planning only — do not implement.

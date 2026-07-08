---
description: Generate or update an operational runbook (deploy, on-call, recovery) for a service or procedure
disable-model-invocation: true
argument-hint: <service or procedure>
---
Write/update a runbook for `$ARGUMENTS` in English (store under the repo's docs location, or
`.claude/docs/` if it is reference-only).

Include: purpose and scope; prerequisites/access; step-by-step procedure with exact commands;
verification/health checks; rollback procedure; failure modes and their responses; escalation and
owners. Mark any destructive step explicitly and require confirmation. Do not run the procedure —
author the document only. Do not commit.

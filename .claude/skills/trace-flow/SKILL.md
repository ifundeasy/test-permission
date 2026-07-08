---
description: Trace a request end-to-end through the codebase (entrypoint → handler → data access → response)
argument-hint: <endpoint or scenario>
---
Trace `$ARGUMENTS` through the code and produce a concise, ordered call flow:

1. Start at the HTTP/entry layer; follow each hop (middleware, handler, service, data access,
   external calls) to the response, citing `file:line` at each step.
2. Note where inputs are validated, where errors are caught/mapped, and every external dependency
   (DB queries, engine calls, network) touched.
3. Call out anything surprising: hidden coupling, implicit ordering, swallowed errors, missing checks.
4. Output a numbered flow plus a short list of risks. Read-only — do not modify code.

---
description: Generate release notes from the commit/PR history since the last release
disable-model-invocation: true
argument-hint: [since tag or range]
---
!`git log --pretty=format:'%h %s' $ARGUMENTS 2>/dev/null | head -200 || true`

From the history above, produce release notes in English grouped as: Features, Fixes, Performance,
Security, Breaking changes, and Internal/Chore. Write user-facing, plain-language entries (not raw
commit subjects); call out breaking changes and required migration steps prominently. Omit noise
(merge/formatting commits). Output the notes only. Do not commit or tag.

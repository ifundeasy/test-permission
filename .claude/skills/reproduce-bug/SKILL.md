---
description: Reproduce a reported bug as a minimal failing test, starting from an issue or description
disable-model-invocation: true
argument-hint: <issue number or description>
---
!`gh issue view $ARGUMENTS 2>/dev/null || echo "No issue fetched — use the description in the prompt."`

Reproduce the bug for `$ARGUMENTS`:
1. Establish expected vs actual behavior from the issue/description above.
2. Build the smallest reproduction (a failing test or a minimal script in `.cache/`) that triggers it.
3. Capture the exact failing output and the conditions required (inputs, state, env).
4. Localize the likely cause with `file:line` evidence, but do not fix it unless asked — hand back
   the failing test + root-cause hypothesis. Do not commit.

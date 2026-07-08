---
description: Investigate a question with evidence (official docs or a real experiment), never assumption
disable-model-invocation: true
argument-hint: <question>
---
Answer "$ARGUMENTS" with EVIDENCE, not assumption:
1. State the hypothesis and what would confirm or refute it.
2. Gather evidence — official docs (fetch + cite source/version) and/or a real experiment (run a
   command or minimal repro in `.cache/`, capture output; check installed versions/lockfile/source).
3. Conclude only from what the evidence shows; cite each claim's source.
4. Label anything unverified as an assumption; if evidence is unobtainable, say so explicitly.
Do not edit project code while investigating — scratch goes in `.cache/`.

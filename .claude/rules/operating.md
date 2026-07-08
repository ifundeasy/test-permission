# Operating rules (always apply)

## Language
- Reply to the user in Bahasa Indonesia; keep technical terms in English.
- All code, file names, variables, and comments are in English.

## Safety
- Never commit or push unless the user explicitly asks.
- Never print/echo/write passwords, tokens, keys, or .env contents unless the user explicitly asks.

## Files & workspace
- Temporary files, scripts, and scratch → `.cache/` only (never elsewhere).
- Presentations → one self-contained, mobile-responsive, EN/ID-toggle HTML in `.share/` (committed).
- Project plans → `.plan/NN_name.md`; gotcha findings → `.issues/NN_gotcha_YYYYMMDD.md` (English).
- Long docs / API specs → `.claude/docs/`, referenced on demand — never inlined into CLAUDE.md.

## Engineering
- Follow the repo's architecture/pattern (recorded in CLAUDE.md); apply current popular conventions.
- New dependencies must be maintained, CVE-free, and justified; prefer stdlib/existing deps.
- Pin exact numeric versions (never `latest`); keep a lockfile + an update bot (Renovate/Dependabot).

## Diagrams
- Mermaid only in docs/HTML, never in context files. Right chart type, official syntax, no styling,
  `<br>` for line breaks.

## Investigation
- When asked to find out / verify / explain why, use EVIDENCE, not assumption: official docs (cited,
  with version) or a real experiment (run it, minimal repro in `.cache/`, check installed
  version/lockfile/source). Recall is a hypothesis, not evidence; label anything unverified as an
  assumption and never present a guess as fact.
- For heavy / self-contained investigations, delegate to a subagent so the main thread stays clean.

## Parallelism
- Use subagents only when there are >= 3 independent subtasks; cap concurrency to keep the machine safe.

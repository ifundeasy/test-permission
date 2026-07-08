---
description: Structured review of a diff or PR — correctness, security, design, tests, and conventions
argument-hint: [base..head | PR number]
---
!`git diff $ARGUMENTS 2>/dev/null | head -400 || true`

Review the diff above (or fetch the PR if a number was given). Produce a structured review:
1. **Correctness** — logic bugs, edge cases, error handling.
2. **Security** — authz/authn, input validation, secret handling, injection.
3. **Design** — fit with the repo's architecture (`CLAUDE.md`), coupling, simpler alternatives.
4. **Tests** — are changes covered? call out untested risk.
5. **Conventions** — `.claude/rules/` adherence.
Rank findings by severity; each includes `file:line` and a concrete fix. End with an
approve / request-changes recommendation. Read-only.

---
description: Update project context (CLAUDE.md and .claude/) from the current session and repo state
disable-model-invocation: true
argument-hint: [optional focus area]
---
Refresh the project's Claude context **recursively** from what changed this session and the current
repo. Walk the full tree — never touch only the root.

1. Re-scan for drift: new/changed commands, conventions, dependencies, architecture, API specs,
   directory structure.
2. **CLAUDE.md (recursive):** update the root `CLAUDE.md` (summary, tech stack, commands) **and**
   every nested per-package/subdir `CLAUDE.md`, each scoped to its own subtree. Extend, don't
   rewrite; keep each <= 200 lines.
3. **rules/ (recursive):** traverse `.claude/rules/` including subdirectories; update/add rules and
   their `paths:` scoping. Refresh `.claude/docs/` and the rules that point to it if specs changed.
4. **agents + memory (every one):** for each `.claude/agents/*.md`, confirm it still fits the repo;
   for each `.claude/agent-memory/<agent>/MEMORY.md`, update durable facts learned this session.
5. **skills/ (every one):** scan each `.claude/skills/*/SKILL.md` for stale references (changed
   commands, paths, tools) and correct them.
6. **Reconcile gotcha findings:** revisit `.issues/NN_gotcha_*.md` and update the **status** of each
   related finding (`open` → `mitigated`/`resolved` if fixed this session; keep `open` if still
   present; append a dated note). Never delete findings — record the change instead.
7. **settings.json:** flag permission/command drift as a *proposal* — never silently change enforcement.
8. Show a diff summary of every change and **wait for approval** before writing. Never commit.

Scope: recurse within the single project `.claude/` and across nested `CLAUDE.md` files only. Do NOT
create per-package `.claude/` folders. Personal `~/.claude/` is out of scope unless I ask.

---
description: Audit dependencies for health, maintenance, license, and CVE risk; flag unpinned versions
argument-hint: [manifest or lockfile]
---
Audit the dependencies in `$ARGUMENTS` (or the repo's manifests/lockfile).

1. For each significant dependency: maintenance status (recent releases/activity), known
   high/critical CVEs (run the ecosystem auditor — evidence, not recall), and license.
2. Flag anything unmaintained, CVE-affected, redundant (duplicate/overlapping), or **not pinned to an
   exact version** (violates the pinning rule).
3. Recommend removals, replacements, or safe upgrades, noting the update bot's role.
4. Output a table (dependency, version, status, CVE, license, action). Do not upgrade without asking.

---
description: Lint and security-review infra manifests (Dockerfile, compose, k8s/Helm, Terraform) for misconfig and hardening gaps
argument-hint: [file or dir]
---
Review the manifests in `$ARGUMENTS` (or the repo's infra files).

1. Correctness/lint: schema validity, pinned image tags (prefer digests), resource limits,
   healthchecks, restart policy, least-privilege users, minimal build context.
2. Security: no secrets in manifests/images, no `:latest`, no privileged/hostNetwork unless
   justified, sane network exposure, read-only root FS where possible.
3. Policy: honor `.claude/rules/` infra safety (destructive ops stay manual).
4. Output a findings table (location, severity, fix). Read-only — do not apply anything.

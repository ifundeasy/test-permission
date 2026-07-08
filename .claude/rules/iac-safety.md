---
paths:
  - "Dockerfile"
  - "docker-compose.yml"
  - ".dockerignore"
  - "Makefile"
---
# Infra & delivery safety
- **Never run destructive infra/data ops without explicit confirmation** — `docker compose down`
  (especially `-v` / `make reset`), `docker volume rm/prune`, `docker system prune`, `docker rm/rmi`,
  `rm -rf`, and any `terraform apply/destroy`, `kubectl apply/delete`, `helm upgrade/uninstall`.
  These are denied in `settings.json` and blocked by `hooks/guard-destructive.sh`. Use `make down`
  or `docker compose stop` (non-destructive) to stop the stack.
- **Image build is multi-stage:** `golang:1.26-bookworm` builds a static (`CGO_ENABLED=0`) binary;
  runtime is `gcr.io/distroless/static-debian12:nonroot`. Keep it non-root; the policy file is copied
  world-readable (source may be `0600`) so the non-root user can read it.
- **Pin images to digests.** `golang:1.26-bookworm` and the distroless tag still move — prefer
  `@sha256:<digest>`. Renovate `pinDigests` pins Docker digests on update PRs.
- Keep the build context minimal (`.dockerignore` excludes db/, demo/, .git, .env, .claude, workspace
  dirs, and Node leftovers). Only `policies/` is needed in the runtime image besides the binary.
- `docker-compose.yml` uses `postgres:18.4`; keep the README's stated Postgres version aligned with it.
- Secrets come from `.env` via `env_file`; never bake secrets into the image or compose file.

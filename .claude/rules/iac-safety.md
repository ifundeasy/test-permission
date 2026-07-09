---
paths:
  - "Dockerfile"
  - "docker-compose.yml"
  - ".dockerignore"
  - "Makefile"
  - "db/bootstrap.sh"
---
# Infra & delivery safety
- **Never run destructive infra/data ops without explicit confirmation** — `docker compose down -v`
  / `make reset`, `docker volume rm/prune`, `docker system prune`, `docker rm/rmi`, `rm -rf`,
  `terraform apply/destroy`, `kubectl apply/delete`, `helm upgrade/uninstall`. Denied in
  `settings.json` + blocked by `hooks/guard-destructive.sh`. `make down` / `docker compose stop`
  are the non-destructive stops. The seeder's `-wipe` flag is also destructive to benchmark data.
- Compose stack order matters: postgres (healthcheck) → `spicedb-migrate` (one-shot,
  `service_completed_successfully`) → `spicedb` (grpc_health_probe) → facade.
- Postgres 18 note: the named volume mounts at `/var/lib/postgresql` (PGDATA moved in the 18 image).
  `db/bootstrap.sh` only runs on a FRESH volume.
- Pin images to numeric tags (`postgres:18.4`, `authzed/spicedb:v1.54.0`, `golang:1.26-bookworm`),
  prefer digests; Renovate `pinDigests` manages them. Never `:latest`.
- Secrets via `.env` (`env_file`) — never bake into images/compose; SpiceDB preshared key is a
  secret too.

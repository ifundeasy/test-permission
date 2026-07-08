# cedar-authz-service — Claude Code project context

## Summary
A demo Policy Enforcement Point (PEP): an HTTP service that fetches authorization data from
Postgres, shapes it into Cedar `entities`, and decides allow/deny by calling the embedded
`github.com/cedar-policy/cedar-go` engine **in-process**. Cedar is a library, not a network service,
and never touches the database — this service owns all data access (the layer SpiceDB would give you
for free).

## Stack
- Go 1.26 (module `github.com/ifundeasy/test-permission`)
- HTTP: stdlib `net/http` (Go 1.22+ method-based `ServeMux`) — no third-party router
- `github.com/cedar-policy/cedar-go` 1.8.0 (policy engine) · `github.com/jackc/pgx/v5` (Postgres)
- Postgres (schema + seed via `db/init.sql`) · Docker + Docker Compose · Makefile task runner

## Commands (see `make help`)
- Run (local): `make run` (loads `.env`; → `go run ./cmd/authz-service`)
- Run (full stack): `make up` (→ `docker compose up --build`) · Stop: `make down` · Reset volume: `make reset`
- Build binary: `make build` (static, `CGO_ENABLED=0`, into `bin/`)
- Test: `make test` (→ `go test ./...`) · Coverage: `make cover`
- Format: `make fmt` (`gofmt -w .`) · Static analysis: `make vet` (`go vet ./...`)
- Deps: `make tidy` (`go mod tidy`) · Image: `make docker-build`

## Architecture — hexagonal (ports & adapters)
The core is pure and imports no Cedar / Postgres / HTTP. Adapters implement the ports; `cmd/` wires them.
```
cmd/authz-service/main.go        composition root — builds adapters, injects into core, serves HTTP
internal/
├── core/                        PURE domain (no Cedar/DB/HTTP imports)
│   ├── domain/                  Request, Result, Decision, Entity, EntityUID
│   ├── port/                    Authorizer (inbound) · EntityRepository, PolicyEngine (outbound)
│   └── service/                 Authorizer use case: load entities → decide
└── adapter/
    ├── inbound/rest/            driving adapter: HTTP → Authorizer port (net/http ServeMux)
    └── outbound/
        ├── cedar/               driven adapter: PolicyEngine via cedar-go (in-process engine)
        └── postgres/            driven adapter: EntityRepository via pgx (the "glue" you build)
internal/config/                 env-based configuration
```
Request flow of one `POST /authorize`: `rest` validates the body → `service.Authorizer` calls
`postgres.LoadEntities` (memberships, document attrs, recursive folder CTE) → `cedar` maps to Cedar
entities and calls `cedar.Authorize` in-process → decision returned.

- Policy model (`policies/policy.cedar`): RBAC (admin role) + ReBAC (owner + group→folder→doc) +
  ABAC (`confidential` docs). Default DENY; a single `forbid` overrides any `permit`.
- Data model (`db/init.sql`): users, groups, roles, folders (self-referential), documents,
  memberships. **No migration tool** — `init.sql` runs once via docker-entrypoint-initdb.d.
- Endpoints: `POST /authorize` → `{ decision, entities_loaded }` · `GET /health` → `{ ok }`.
- Tests: the 8 canonical decision scenarios are pinned in
  `internal/adapter/outbound/cedar/engine_test.go`; HTTP validation in `internal/adapter/inbound/rest`.
- No API spec file (OpenAPI/proto/GraphQL) in the repo; no CI workflows.

## Security notes
- Credentials live in `.env` (DB creds); never read or echo it. `.env.example` documents the vars.
- This is an authorization service — treat `policies/` and `internal/adapter/outbound/` as
  security-critical: a bug there is an authorization bypass. See `.claude/rules/authz.md`.

## Working agreements
- Prefer editing existing files over adding new ones unless asked.
- Respect the hexagonal boundaries: keep the core free of adapter imports; add behavior by adding or
  changing an adapter behind a port. See `.claude/rules/`.
- Run `make test` / `make vet` after changes and report the result. Never commit unless asked.

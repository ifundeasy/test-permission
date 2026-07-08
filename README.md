# Cedar Docker Demo — how it works without a database

A runnable answer to three questions:

1. If Cedar has no database, how does it work?
2. Does my service "talk to" Cedar (like it talks to SpiceDB)?
3. Is it like SpiceDB, where SpiceDB talks to Postgres?

Everything here was tested end-to-end: **Cedar-Go 1.8.0 + Postgres 18 + Go 1.26 (net/http + pgx v5)**.

> This service was originally a Node/Express demo; it has been rewritten in **Go using a
> hexagonal (ports & adapters) architecture**. The Cedar/SpiceDB mental model below is unchanged —
> only the implementation language and structure changed.

---

## The mental model (this is the key part)

SpiceDB and Cedar are **not the same shape**. Don't carry the SpiceDB model over.

### SpiceDB = a SERVICE (server). Three tiers.

```
  your app  ──gRPC/HTTP──▶  SpiceDB (a running server)  ──SQL──▶  Postgres
                             (owns the data, does the                (SpiceDB's
                              graph traversal, caching)               datastore)
```

You **talk to SpiceDB over the network**. SpiceDB **talks to Postgres itself**.
Postgres is SpiceDB's datastore. You never touch it directly.

### Cedar = a LIBRARY (engine). It runs *inside* your service.

```
  your app / authz-service
  ┌───────────────────────────────────────────────┐
  │  1. YOU query Postgres for the data you need    │──SQL──▶ Postgres (YOUR db)
  │  2. cedar.Authorize(policies, entities, req)    │
  │     ← in-process FUNCTION CALL                  │   (no network, no db here)
  │  3. get allow / deny                            │
  └───────────────────────────────────────────────┘
```

So, directly answering the questions:

| Question | Answer |
| --- | --- |
| How does it work with no DB? | You **hand Cedar the data** (`entities`) on every call. Cedar evaluates policies against that data in memory and returns allow/deny. It stores nothing. |
| Does my service talk to Cedar? | **No network call.** You import Cedar as a library and call a function. (You *can* wrap Cedar in your own HTTP server — this demo does, only so you can `curl` it — but the Cedar call inside is still an in-process function.) |
| Is it like SpiceDB → Postgres? | **No. Cedar never touches Postgres.** If you use Postgres, *your* code queries it and shapes the rows into Cedar `entities`. That glue is your job — it's exactly what SpiceDB does for you automatically. |

---

## Architecture (hexagonal / ports & adapters)

The core is pure and knows nothing about Cedar, Postgres, or HTTP. Adapters plug into ports:

```
cmd/authz-service/main.go            composition root — wires everything, starts HTTP
internal/
├── core/                            PURE domain — no Cedar / DB / HTTP imports
│   ├── domain/                      Request, Result, Decision, Entity, EntityUID
│   ├── port/                        Authorizer (inbound) · EntityRepository, PolicyEngine (outbound)
│   └── service/                     Authorizer use case: load entities → decide
└── adapter/
    ├── inbound/rest/                driving adapter: HTTP → Authorizer port (net/http)
    └── outbound/
        ├── cedar/                   driven adapter: PolicyEngine via cedar-go (the in-process engine)
        └── postgres/                driven adapter: EntityRepository via pgx (the "glue" you build)
```

Swap the engine or the datastore by writing a new adapter — the core does not change.

---

## Run it (Postgres + service)

The data lives in Postgres. The service queries it, builds Cedar entities, and decides.

```bash
make up            # docker compose up --build   (postgres + authz-service)
```

Wait for `authz-service listening on :8080`, then in another terminal:

```bash
# all 8 scenarios live in demo/cedar.http (VS Code REST Client); or one request via curl:
curl -s -X POST localhost:8080/authorize \
  -H 'Content-Type: application/json' \
  -d '{"principal":"bob","action":"view","resource":"design"}'
# -> {"decision":"allow","entities_loaded":4}
```

`entities_loaded` shows how many rows Cedar had to be handed for that one decision — i.e. the
data-fetch work that is on *you*, not on Cedar.

### Common tasks (`make help`)

```
make build         build a static binary into bin/
make run           run locally (loads .env)
make test          run all tests (incl. the 8-scenario decision matrix)
make cover         tests with coverage
make fmt / vet     format / static analysis
make up / logs     start the stack / tail service logs
make down          stop the stack (keeps data)
make reset         stop AND delete the data volume (DESTRUCTIVE)
```

### What actually happens on one `/authorize` call

1. `internal/adapter/inbound/rest` receives the HTTP request (this is your PEP) and validates the body.
2. `internal/adapter/outbound/postgres` queries Postgres — the user's group/role memberships, the
   document's owner + `confidential` flag, and the folder chain (a recursive CTE) — and shapes the
   rows into domain entities.
3. `internal/adapter/outbound/cedar` maps those to Cedar entities and calls `cedar.Authorize(...)`
   **in-process**, returning allow/deny.

Note in `docker-compose.yml`: there is **no `cedar` container**. Cedar is a Go dependency inside
`authz-service` (`github.com/cedar-policy/cedar-go`).

---

## The policy (`policies/policy.cedar`)

One file, three models:

- **RBAC** — `permit ( principal in Role::"admin", action, resource );`
- **ReBAC** — owner rule + group→folder→document inheritance via the `in` operator
- **ABAC** — `forbid ... when { resource.confidential == true } unless { principal == resource.owner }`

`forbid` beats `permit`, which is why `carol` (admin) is still denied the confidential `secret` doc,
but `alice` (its owner) is allowed. The 8 canonical cases are pinned as a Go test
(`internal/adapter/outbound/cedar/engine_test.go`) so a policy or entity change can't silently flip a
decision.

---

## Files

```
cedar-authz-service/
├── README.md                 (this file)
├── Makefile                  build / run / test / docker tasks
├── go.mod / go.sum           Go module (cedar-go 1.8.0, pgx v5) — pinned, reproducible
├── docker-compose.yml        postgres + authz-service
├── Dockerfile                multi-stage: build static Go binary → distroless runtime
├── .env.example              documented env vars (no real secrets)
├── cmd/authz-service/        main.go — composition root
├── internal/                 hexagonal core + adapters (see Architecture above)
├── policies/
│   └── policy.cedar          RBAC + ReBAC + ABAC
├── db/
│   └── init.sql              your application data (schema + seed)
└── demo/
    └── cedar.http            the 8 scenarios for the VS Code REST Client (humao.rest-client)
```

---

## The takeaway for your decision

The Postgres adapter (`internal/adapter/outbound/postgres`) is the point. With SpiceDB that layer
does not exist — SpiceDB owns the data, the graph traversal, the reverse-lookups
(`LookupResources`), and consistency. With Cedar you build and operate that layer yourself.

For a scan-by-pattern need ("what can user X see?"), this demo also shows the gap: the repository
fetches the entities for **one** known resource. To *list* every resource a user can access, you'd
loop `Authorize` over candidates or add partial-evaluation → SQL translation. That is not in this
demo because Cedar has no turnkey answer for it — which is exactly why the recommendation was to keep
Cedar as a stateless ABAC PDP and let SpiceDB own data + ReBAC + scans.

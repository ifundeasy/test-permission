---
paths:
  - "policies/**"
  - "internal/core/**"
  - "internal/adapter/outbound/cedar/**"
  - "internal/adapter/outbound/postgres/**"
---
# Authorization (Cedar) rules — security-critical
This service is a PEP; the policy plus the entity-building code IS the security boundary. A mistake
here is an authorization bypass, not a cosmetic bug.

- **Default is DENY.** Cedar denies unless a `permit` matches. Never add a broad `permit` to "make a
  case pass" — narrow it to the exact principal / action / resource.
- **`forbid` overrides `permit`.** The ABAC `forbid` on `confidential` documents must keep beating
  the admin `permit`. Preserve this when editing `policies/policy.cedar`.
- **Entities must be complete.** The Postgres adapter (`internal/adapter/outbound/postgres`) must
  load every ancestor and attribute a policy references (group/role memberships, `owner`,
  `confidential`, the full folder chain). Missing entities silently change decisions.
- **Keep the mapping faithful.** The Cedar adapter (`internal/adapter/outbound/cedar`) maps domain
  entities to Cedar; `owner` must stay an entity reference (EntityUID) and `confidential` a Boolean,
  or `principal == resource.owner` and the ABAC rule break.
- **Validate inputs.** `POST /authorize` must reject a missing `{ principal, action, resource }`
  (returns 400) and must not leak internal error detail (returns a generic 500 message).
- **Any policy/entity change requires re-running the decision matrix** — `make test` runs the 8
  canonical scenarios in `internal/adapter/outbound/cedar/engine_test.go`; confirm each expected
  ALLOW/DENY still holds (and add a case for any new rule).

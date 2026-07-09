---
paths:
  - "policies/**"
  - "schema/spicedb/**"
  - "internal/core/**"
  - "internal/adapter/outbound/**"
  - "internal/seed/**"
---
# Authorization & benchmark-equivalence rules — security-critical
This repo benchmarks two engines that MUST agree. A one-sided change is worse than a bug — it
silently invalidates the benchmark.

- **Twin schemas.** `policies/*.cedar` (one file per model) and `schema/spicedb/schema.zed` encode
  the same semantics. Any change to one side must be mirrored on the other, and the sampler's
  ground-truth logic (internal/seed/sampler.go) updated to match.
- **Default DENY** on both engines. Never widen a rule to "make a tuple pass" — fix the data or the
  ground truth instead.
- Cedar specifics: PBAC uses policy-parameter ENTITIES (`resource.policy.max_amount`) because
  cedar-go 1.8.0 has no templates; ABAC's `forbid` on archived must keep overriding permits; ACL
  editors also satisfy `acl.view` (matches SpiceDB `view = viewer + editor`).
- SpiceDB specifics: ABAC/PBAC use caveats — resource/policy params are STATIC caveat context
  (written at seed time, wins over check-time context); principal attrs / amount / region arrive at
  check time. A `CONDITIONAL` response means under-supplied context = caller bug, treated as failure.
- **Entity loaders must stay complete AND minimal** (`internal/adapter/outbound/postgres/loader.go`):
  load every ancestor/attribute a policy references, nothing more — over-fetching skews the
  benchmark, under-fetching flips decisions.
- **Any change here requires:** `make test && make vet`, then `make seed-test` + `make verify`
  (miniature equivalence gate) — and a full re-seed if generator output changed (`-wipe`).

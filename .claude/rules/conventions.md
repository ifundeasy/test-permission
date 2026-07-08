# Coding conventions
- Language: Go 1.26, module `github.com/ifundeasy/test-permission`. Standard Go layout:
  `cmd/` for binaries, `internal/` for private packages.
- Formatting is non-negotiable: `gofmt` (run `make fmt`; the PostToolUse `format.sh` hook also
  gofmts `.go` files on save). Keep `go vet` clean (`make vet`).
- Idioms: return errors, don't panic in library code; wrap with `fmt.Errorf("...: %w", err)` for
  context; accept `context.Context` as the first parameter on I/O calls; program to the interfaces
  in `internal/core/port`, not to concrete adapters.
- Prefer the standard library and existing dependencies over adding new ones (net/http is enough for
  this service — no third-party router). Justify any new dependency.
- Config via environment variables (`internal/config`); never hardcode secrets. Provide safe,
  non-secret defaults only for local dev.
- Pin dependencies to exact versions in `go.mod`; commit `go.sum`. Renovate (`renovate.json`) opens
  grouped update PRs and pins Docker digests.

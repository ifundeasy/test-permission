// Package port declares the hexagonal ports. Both engines implement Decider;
// the embedded engine additionally depends on EntityLoader to fetch its slice
// of data per check. Adapters implement these; the core never imports an adapter.
package port

import (
	"context"

	"github.com/ifundeasy/test-permission/internal/core/domain"
)

// Decider answers one authorization question. Implemented by:
//   - the Cedar adapter (load entities from Postgres, evaluate in-process), and
//   - the SpiceDB adapter (gRPC CheckPermission against the SpiceDB server).
type Decider interface {
	// Name identifies the engine variant (e.g. "cedar", "spicedb").
	Name() string
	Check(ctx context.Context, req domain.Request) (domain.Result, error)
}

// EntityLoader is the outbound port the embedded (Cedar) decider uses: given a
// check request, return exactly the entity slice needed to decide it.
type EntityLoader interface {
	LoadForCheck(ctx context.Context, req domain.Request) ([]domain.Entity, error)
}

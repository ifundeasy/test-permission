// Package port declares the hexagonal ports: the interfaces the core exposes
// (inbound / driving) and depends on (outbound / driven). Adapters implement
// these; the core never imports an adapter.
package port

import (
	"context"

	"github.com/ifundeasy/test-permission/internal/core/domain"
)

// Authorizer is the inbound (driving) port — the application use case invoked by
// a driving adapter such as the HTTP handler.
type Authorizer interface {
	Authorize(ctx context.Context, req domain.Request) (domain.Result, error)
}

// EntityRepository is an outbound (driven) port: given the principal and resource
// ids, it returns exactly the slice of entities needed to decide one request.
type EntityRepository interface {
	LoadEntities(ctx context.Context, principal, resource string) ([]domain.Entity, error)
}

// PolicyEngine is an outbound (driven) port: it evaluates a request against the
// loaded policies using the supplied entities and returns a decision.
type PolicyEngine interface {
	IsAuthorized(req domain.Request, entities []domain.Entity) (domain.Decision, error)
}

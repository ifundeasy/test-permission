// Package service implements the core use cases on top of the ports.
package service

import (
	"context"
	"fmt"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/port"
)

// authorizer is the concrete Authorizer use case. It orchestrates the two driven
// ports: load the entities for the request, then ask the policy engine to decide.
type authorizer struct {
	repo   port.EntityRepository
	engine port.PolicyEngine
}

// NewAuthorizer wires the use case to its driven ports.
func NewAuthorizer(repo port.EntityRepository, engine port.PolicyEngine) port.Authorizer {
	return &authorizer{repo: repo, engine: engine}
}

func (a *authorizer) Authorize(ctx context.Context, req domain.Request) (domain.Result, error) {
	entities, err := a.repo.LoadEntities(ctx, req.Principal, req.Resource)
	if err != nil {
		return domain.Result{}, fmt.Errorf("load entities: %w", err)
	}
	decision, err := a.engine.IsAuthorized(req, entities)
	if err != nil {
		return domain.Result{}, fmt.Errorf("evaluate policies: %w", err)
	}
	return domain.Result{Decision: decision, EntitiesLoaded: len(entities)}, nil
}

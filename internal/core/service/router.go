// Package service implements the engine-agnostic core use case: validate a
// check request and route it to the requested engine's Decider.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/port"
)

// ErrBadRequest marks caller mistakes (unknown engine/model, missing fields) so
// the transport can map them to 4xx; anything else is an engine/backend fault.
var ErrBadRequest = errors.New("bad request")

// Router dispatches authorization checks to registered engine deciders.
type Router struct {
	deciders map[string]port.Decider
}

func NewRouter(deciders ...port.Decider) *Router {
	m := make(map[string]port.Decider, len(deciders))
	for _, d := range deciders {
		m[d.Name()] = d
	}
	return &Router{deciders: m}
}

// Engines lists the registered engine names.
func (r *Router) Engines() []string {
	names := make([]string, 0, len(r.deciders))
	for n := range r.deciders {
		names = append(names, n)
	}
	return names
}

// Check validates the request and delegates to the named engine.
func (r *Router) Check(ctx context.Context, engine string, req domain.Request) (domain.Result, error) {
	d, ok := r.deciders[engine]
	if !ok {
		return domain.Result{}, fmt.Errorf("%w: unknown engine %q", ErrBadRequest, engine)
	}
	if req.Principal == "" || req.Action == "" || req.Resource == "" || req.ResourceType == "" {
		return domain.Result{}, fmt.Errorf("%w: principal, action, resource_type and resource are required", ErrBadRequest)
	}
	valid := false
	for _, m := range domain.Models {
		if req.Model == m {
			valid = true
			break
		}
	}
	if !valid {
		return domain.Result{}, fmt.Errorf("%w: unknown model %q", ErrBadRequest, req.Model)
	}
	res, err := d.Check(ctx, req)
	if err != nil {
		return domain.Result{}, fmt.Errorf("%s check: %w", d.Name(), err)
	}
	return res, nil
}

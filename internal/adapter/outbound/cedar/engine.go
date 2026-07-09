// Package cedar is the embedded-engine adapter: it implements port.Decider by
// loading the entity slice for a check (via port.EntityLoader) and evaluating
// the Cedar policy set IN-PROCESS with github.com/cedar-policy/cedar-go.
// There is no Cedar server, and Cedar never touches the database itself.
package cedar

import (
	"bytes"
	"context"
	"fmt"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/port"
)

// principalType is the entity type used for all principals.
const principalType = "Persona"

// Decider evaluates checks with the embedded Cedar engine.
type Decider struct {
	policies *cedar.PolicySet
	loader   port.EntityLoader
}

// NewDecider parses the concatenated policy documents once and returns a
// reusable decider. docs maps a file name (diagnostics only) to its content.
func NewDecider(docs map[string][]byte, loader port.EntityLoader) (*Decider, error) {
	var buf bytes.Buffer
	for name, doc := range docs {
		fmt.Fprintf(&buf, "// ---- %s ----\n", name)
		buf.Write(doc)
		buf.WriteByte('\n')
	}
	ps, err := cedar.NewPolicySetFromBytes("policies.cedar", buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("parse cedar policies: %w", err)
	}
	return &Decider{policies: ps, loader: loader}, nil
}

func (d *Decider) Name() string { return "cedar" }

// Check implements port.Decider: fetch the entity slice, then evaluate.
func (d *Decider) Check(ctx context.Context, req domain.Request) (domain.Result, error) {
	entities, err := d.loader.LoadForCheck(ctx, req)
	if err != nil {
		return domain.Result{}, fmt.Errorf("load entities: %w", err)
	}
	decision, err := d.Evaluate(req, entities)
	if err != nil {
		return domain.Result{}, err
	}
	return domain.Result{Decision: decision, EntitiesLoaded: len(entities)}, nil
}

// PreparedCheck is a check pre-converted to the engine's native form: the
// entity store and the request. Preparing once and evaluating repeatedly lets
// the benchmark's eval-only cell time PURELY cedar.Authorize — no domain→engine
// conversion, no allocation, inside the timed window.
type PreparedCheck struct {
	store types.EntityMap
	req   cedar.Request
}

// Prepare converts a check + its entity slice into engine-native form.
func (d *Decider) Prepare(req domain.Request, entities []domain.Entity) (*PreparedCheck, error) {
	store := make(types.EntityMap, len(entities))
	for _, de := range entities {
		attrs := make(types.RecordMap, len(de.Attributes))
		for k, raw := range de.Attributes {
			val, err := toValue(raw)
			if err != nil {
				return nil, fmt.Errorf("entity %s::%q attr %q: %w", de.UID.Type, de.UID.ID, k, err)
			}
			attrs[types.String(k)] = val
		}
		parents := make([]types.EntityUID, 0, len(de.Parents))
		for _, p := range de.Parents {
			parents = append(parents, toUID(p))
		}
		uid := toUID(de.UID)
		store[uid] = types.Entity{
			UID:        uid,
			Parents:    types.NewEntityUIDSet(parents...),
			Attributes: types.NewRecord(attrs),
		}
	}

	reqCtx := make(types.RecordMap, len(req.Context))
	for k, raw := range req.Context {
		val, err := toValue(raw)
		if err != nil {
			return nil, fmt.Errorf("request context %q: %w", k, err)
		}
		reqCtx[types.String(k)] = val
	}

	return &PreparedCheck{
		store: store,
		req: cedar.Request{
			Principal: types.NewEntityUID(principalType, types.String(req.Principal)),
			Action:    types.NewEntityUID("Action", types.String(req.Action)),
			Resource:  types.NewEntityUID(types.EntityType(req.ResourceType), types.String(req.Resource)),
			Context:   types.NewRecord(reqCtx),
		},
	}, nil
}

// EvaluatePrepared runs the engine on a prepared check.
//
// The Diagnostic is intentionally discarded: cedar-go skips erroring policies
// (deny-by-default), which matches SpiceDB's treatment of absent rows/entities
// (NO_PERMISSION) — failing loudly here would make the engines DIVERGE on
// unknown-resource checks. Decision-level divergence from data bugs is caught
// by the equivalence gate instead.
func (d *Decider) EvaluatePrepared(p *PreparedCheck) domain.Decision {
	decision, _ := cedar.Authorize(d.policies, p.store, p.req)
	if decision == cedar.Allow {
		return domain.Allow
	}
	return domain.Deny
}

// Evaluate runs the engine against pre-fetched entities (prepare + evaluate).
func (d *Decider) Evaluate(req domain.Request, entities []domain.Entity) (domain.Decision, error) {
	p, err := d.Prepare(req, entities)
	if err != nil {
		return domain.Deny, err
	}
	return d.EvaluatePrepared(p), nil
}

func toUID(u domain.EntityUID) types.EntityUID {
	return types.NewEntityUID(types.EntityType(u.Type), types.String(u.ID))
}

// toValue maps a domain-neutral attribute/context value to a Cedar value.
func toValue(raw any) (types.Value, error) {
	switch v := raw.(type) {
	case bool:
		return types.Boolean(v), nil
	case string:
		return types.String(v), nil
	case int:
		return types.Long(v), nil
	case int64:
		return types.Long(v), nil
	case domain.EntityUID:
		return toUID(v), nil
	case []string:
		elems := make([]types.Value, 0, len(v))
		for _, s := range v {
			elems = append(elems, types.String(s))
		}
		return types.NewSet(elems...), nil
	case []domain.EntityUID:
		elems := make([]types.Value, 0, len(v))
		for _, u := range v {
			elems = append(elems, toUID(u))
		}
		return types.NewSet(elems...), nil
	default:
		return nil, fmt.Errorf("unsupported attribute type %T", raw)
	}
}

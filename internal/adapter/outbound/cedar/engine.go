// Package cedar is the outbound adapter that implements port.PolicyEngine using
// the official Cedar Go engine (github.com/cedar-policy/cedar-go). Cedar runs
// in-process: there is no Cedar server and Cedar never touches the database.
package cedar

import (
	"fmt"

	cedar "github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/port"
)

// Well-known entity types used to phrase the authorization request.
const (
	typeUser     = "User"
	typeAction   = "Action"
	typeDocument = "Document"
)

type engine struct {
	policies *cedar.PolicySet
}

// NewEngine parses the Cedar policy document once and returns a reusable engine.
// fileName is used only for diagnostics/error messages.
func NewEngine(fileName string, policyDocument []byte) (port.PolicyEngine, error) {
	ps, err := cedar.NewPolicySetFromBytes(fileName, policyDocument)
	if err != nil {
		return nil, fmt.Errorf("parse cedar policies: %w", err)
	}
	return &engine{policies: ps}, nil
}

func (e *engine) IsAuthorized(req domain.Request, entities []domain.Entity) (domain.Decision, error) {
	store := make(types.EntityMap, len(entities))
	for _, de := range entities {
		attrs := make(types.RecordMap, len(de.Attributes))
		for k, raw := range de.Attributes {
			val, err := toValue(raw)
			if err != nil {
				return domain.Deny, fmt.Errorf("entity %s::%q attr %q: %w", de.UID.Type, de.UID.ID, k, err)
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

	decision, _ := cedar.Authorize(e.policies, store, cedar.Request{
		Principal: toUID(domain.EntityUID{Type: typeUser, ID: req.Principal}),
		Action:    toUID(domain.EntityUID{Type: typeAction, ID: req.Action}),
		Resource:  toUID(domain.EntityUID{Type: typeDocument, ID: req.Resource}),
		Context:   cedar.NewRecord(cedar.RecordMap{}),
	})

	if decision == cedar.Allow {
		return domain.Allow, nil
	}
	return domain.Deny, nil
}

func toUID(u domain.EntityUID) types.EntityUID {
	return types.NewEntityUID(types.EntityType(u.Type), types.String(u.ID))
}

// toValue maps a domain-neutral attribute value to a Cedar value. Allowed inputs
// are bool, string, and domain.EntityUID (an entity reference).
func toValue(raw any) (types.Value, error) {
	switch v := raw.(type) {
	case bool:
		return types.Boolean(v), nil
	case string:
		return types.String(v), nil
	case domain.EntityUID:
		return toUID(v), nil
	default:
		return nil, fmt.Errorf("unsupported attribute type %T", raw)
	}
}

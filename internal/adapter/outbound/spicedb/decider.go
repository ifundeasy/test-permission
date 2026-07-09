// Package spicedb is the server-engine adapter: it implements port.Decider by
// calling SpiceDB's CheckPermission over gRPC. SpiceDB owns its own datastore
// (schema "spicedb" on the shared Postgres) — the PEP does no data fetching.
package spicedb

import (
	"context"
	"fmt"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/ifundeasy/test-permission/internal/core/domain"
)

// ConsistencyMode selects SpiceDB's read consistency for checks.
type ConsistencyMode string

const (
	// MinimizeLatency serves from SpiceDB's cache (its production default).
	MinimizeLatency ConsistencyMode = "minimize_latency"
	// FullyConsistent bypasses caching — the fairest comparison against an
	// embedded engine that always reads live data.
	FullyConsistent ConsistencyMode = "fully_consistent"
)

// resourceTypes maps domain resource types to SpiceDB definitions.
var resourceTypes = map[string]string{
	"Endpoint":      "endpoint",
	"Page":          "page",
	"Component":     "component",
	"RebacDocument": "rebac_document",
	"AbacDocument":  "abac_document",
	"PurchaseOrder": "purchase_order",
	"AclDocument":   "acl_document",
}

// permissions maps (model, action) to the SpiceDB permission name.
var permissions = map[domain.Model]map[string]string{
	domain.ModelRBAC:  {"execute": "execute", "view": "view", "render": "render"},
	domain.ModelReBAC: {"doc.view": "view"},
	domain.ModelABAC:  {"doc.read": "read"},
	domain.ModelPBAC:  {"po.approve": "approve"},
	domain.ModelACL:   {"acl.view": "view", "acl.edit": "edit"},
}

// Decider checks permissions against a SpiceDB server.
type Decider struct {
	client *authzed.Client
	mode   ConsistencyMode
	name   string
}

// NewDecider dials SpiceDB (insecure transport — benchmark/local use) with the
// given preshared key and consistency mode.
func NewDecider(endpoint, presharedKey string, mode ConsistencyMode) (*Decider, error) {
	client, err := authzed.NewClient(endpoint,
		grpcutil.WithInsecureBearerToken(presharedKey),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial spicedb: %w", err)
	}
	return &Decider{client: client, mode: mode, name: "spicedb-" + string(mode)}, nil
}

func (d *Decider) Name() string { return d.name }

func (d *Decider) Check(ctx context.Context, req domain.Request) (domain.Result, error) {
	defType, ok := resourceTypes[req.ResourceType]
	if !ok {
		return domain.Result{}, fmt.Errorf("unsupported resource type %q", req.ResourceType)
	}
	perm, ok := permissions[req.Model][req.Action]
	if !ok {
		return domain.Result{}, fmt.Errorf("unsupported action %q for model %q", req.Action, req.Model)
	}

	var reqCtx *structpb.Struct
	if len(req.Context) > 0 {
		norm, err := normalizeContext(req.Context)
		if err != nil {
			return domain.Result{}, err
		}
		s, err := structpb.NewStruct(norm)
		if err != nil {
			return domain.Result{}, fmt.Errorf("build check context: %w", err)
		}
		reqCtx = s
	}

	resp, err := d.client.CheckPermission(ctx, &v1.CheckPermissionRequest{
		Consistency: d.consistency(),
		Resource:    &v1.ObjectReference{ObjectType: defType, ObjectId: req.Resource},
		Permission:  perm,
		Subject: &v1.SubjectReference{
			Object: &v1.ObjectReference{ObjectType: "persona", ObjectId: req.Principal},
		},
		Context: reqCtx,
	})
	if err != nil {
		return domain.Result{}, fmt.Errorf("check permission: %w", err)
	}

	switch resp.Permissionship {
	case v1.CheckPermissionResponse_PERMISSIONSHIP_HAS_PERMISSION:
		return domain.Result{Decision: domain.Allow}, nil
	case v1.CheckPermissionResponse_PERMISSIONSHIP_NO_PERMISSION:
		return domain.Result{Decision: domain.Deny}, nil
	case v1.CheckPermissionResponse_PERMISSIONSHIP_CONDITIONAL_PERMISSION:
		// Context was under-supplied for a caveat — a caller bug, not a decision.
		missing := resp.GetPartialCaveatInfo().GetMissingRequiredContext()
		return domain.Result{}, fmt.Errorf("conditional permission: missing caveat context %v", missing)
	default:
		return domain.Result{}, fmt.Errorf("unexpected permissionship %v", resp.Permissionship)
	}
}

func (d *Decider) consistency() *v1.Consistency {
	if d.mode == FullyConsistent {
		return &v1.Consistency{Requirement: &v1.Consistency_FullyConsistent{FullyConsistent: true}}
	}
	return &v1.Consistency{Requirement: &v1.Consistency_MinimizeLatency{MinimizeLatency: true}}
}

// maxExactInt is the largest integer float64 represents exactly (2^53).
// structpb carries numbers as float64; above this bound an int64 amount would
// silently lose precision and could flip a caveat comparison vs Cedar's exact
// Long — reject instead.
const maxExactInt = int64(1) << 53

// normalizeContext converts values structpb cannot represent directly.
func normalizeContext(in map[string]any) (map[string]any, error) {
	out := make(map[string]any, len(in))
	for k, v := range in {
		switch t := v.(type) {
		case int64:
			if t > maxExactInt || t < -maxExactInt {
				return nil, fmt.Errorf("context %q: integer %d exceeds float64 exact range (2^53)", k, t)
			}
			out[k] = float64(t) // structpb numbers are float64 (CEL int params accept them)
		case int:
			out[k] = float64(t)
		case []string:
			elems := make([]any, len(t))
			for i, s := range t {
				elems[i] = s
			}
			out[k] = elems
		default:
			out[k] = v
		}
	}
	return out, nil
}

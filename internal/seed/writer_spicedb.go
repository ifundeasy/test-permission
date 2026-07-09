package seed

import (
	"context"
	"fmt"

	v1 "github.com/authzed/authzed-go/proto/authzed/api/v1"
	"github.com/authzed/authzed-go/v1"
	"github.com/authzed/grpcutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/structpb"
)

// SpiceDBWriter is the SpiceDB-side sink: it converts the canonical stream to
// relationships and writes them in 1000-relationship batches via
// WriteRelationships with OPERATION_TOUCH (idempotent re-runs; the server's
// default max-updates-per-call is exactly 1000).
//
// NOTE: the faster ImportBulkRelationships path is NOT used — its binary COPY
// stream breaks against Postgres 18 ("protocol synchronization was lost",
// SQLSTATE 08P01, observed with SpiceDB v1.54.0 + postgres:18.4). TOUCH-writes
// use the normal extended protocol and are unaffected.
//
// Only relationship-bearing records are written; identity basis records that
// SpiceDB has no relation for (accounts, divisions, registry rows) are no-ops —
// their authorization-relevant facts arrive via grants/memberships/caveats,
// which is exactly the data SpiceDB would hold in production.
type SpiceDBWriter struct {
	NopSink
	client   *authzed.RetryableClient
	batch    int
	buf      []*v1.Relationship
	progress *Progress
	ctx      context.Context
}

func NewSpiceDBWriter(ctx context.Context, endpoint, presharedKey string, batch int, progress *Progress) (*SpiceDBWriter, error) {
	client, err := authzed.NewRetryableClient(endpoint,
		grpcutil.WithInsecureBearerToken(presharedKey),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dial spicedb: %w", err)
	}
	return &SpiceDBWriter{client: client, batch: batch, progress: progress, ctx: ctx}, nil
}

// resourceDefinitions lists every definition that appears as a relationship
// resource (used by WipeRelationships).
var resourceDefinitions = []string{
	"role", "endpoint", "page", "component",
	"org_unit", "folder", "rebac_document",
	"abac_document", "pbac_policy", "purchase_order", "acl_document",
}

// WipeRelationships deletes ALL relationships of every benchmark definition —
// used before reseeding at a different scale (IDs overlap across scales).
func WipeRelationships(ctx context.Context, endpoint, presharedKey string) error {
	client, err := authzed.NewClient(endpoint,
		grpcutil.WithInsecureBearerToken(presharedKey),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial spicedb: %w", err)
	}
	for _, def := range resourceDefinitions {
		if _, err := client.DeleteRelationships(ctx, &v1.DeleteRelationshipsRequest{
			RelationshipFilter: &v1.RelationshipFilter{ResourceType: def},
		}); err != nil {
			return fmt.Errorf("delete %s relationships: %w", def, err)
		}
	}
	return nil
}

// WriteSchema uploads the .zed schema (idempotent).
func WriteSchema(ctx context.Context, endpoint, presharedKey string, schema []byte) error {
	client, err := authzed.NewClient(endpoint,
		grpcutil.WithInsecureBearerToken(presharedKey),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("dial spicedb: %w", err)
	}
	_, err = client.WriteSchema(ctx, &v1.WriteSchemaRequest{Schema: string(schema)})
	if err != nil {
		return fmt.Errorf("write schema: %w", err)
	}
	return nil
}

func rel(resType, resID, relation, subjType, subjID, subjRel string, caveat *v1.ContextualizedCaveat) *v1.Relationship {
	return &v1.Relationship{
		Resource: &v1.ObjectReference{ObjectType: resType, ObjectId: resID},
		Relation: relation,
		Subject: &v1.SubjectReference{
			Object:           &v1.ObjectReference{ObjectType: subjType, ObjectId: subjID},
			OptionalRelation: subjRel,
		},
		OptionalCaveat: caveat,
	}
}

func (w *SpiceDBWriter) add(r *v1.Relationship) error {
	w.buf = append(w.buf, r)
	w.progress.Add(1)
	if len(w.buf) >= w.batch {
		return w.flush()
	}
	return nil
}

func (w *SpiceDBWriter) flush() error {
	if len(w.buf) == 0 {
		return nil
	}
	batch := w.buf
	w.buf = w.buf[:0]
	// WriteRelationships rejects the SAME relationship twice within ONE request
	// (the generator may draw duplicate edges, e.g. manager memberships) —
	// dedupe per batch; cross-batch duplicates are fine (TOUCH is idempotent).
	seen := make(map[string]struct{}, len(batch))
	updates := make([]*v1.RelationshipUpdate, 0, len(batch))
	for _, r := range batch {
		key := r.Resource.ObjectType + "\x00" + r.Resource.ObjectId + "\x00" + r.Relation +
			"\x00" + r.Subject.Object.ObjectType + "\x00" + r.Subject.Object.ObjectId +
			"\x00" + r.Subject.OptionalRelation
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		updates = append(updates, &v1.RelationshipUpdate{
			Operation:    v1.RelationshipUpdate_OPERATION_TOUCH,
			Relationship: r,
		})
	}
	if _, err := w.client.WriteRelationships(w.ctx, &v1.WriteRelationshipsRequest{Updates: updates}); err != nil {
		return fmt.Errorf("write relationships (%d rels): %w", len(updates), err)
	}
	return nil
}

func (w *SpiceDBWriter) BeginPhase(phase string, total int) error {
	w.progress.Begin(phase, total)
	return nil
}

func (w *SpiceDBWriter) EndPhase(string) error {
	if err := w.flush(); err != nil {
		return err
	}
	w.progress.End()
	return nil
}

// ---- record conversions ----

// Org: the subsidiary tree — org_unit#parent (ReBAC backbone).
func (w *SpiceDBWriter) Org(o Org) error {
	if o.ParentID == "" {
		w.progress.Add(1) // roots have no edge; keep progress aligned
		return nil
	}
	return w.add(rel("org_unit", o.ID, "parent", "org_unit", o.ParentID, "", nil))
}

func (w *SpiceDBWriter) PersonaRole(pr PersonaRole) error {
	return w.add(rel("role", pr.RoleID, "assignee", "persona", pr.PersonaID, "", nil))
}

// RoleGrant: subject is the role's assignee set (role:<id>#assignee).
func (w *SpiceDBWriter) RoleGrant(rg RoleGrant) error {
	return w.add(rel(rg.ResourceType, rg.ResourceID, "allowed_role", "role", rg.RoleID, "assignee", nil))
}

func (w *SpiceDBWriter) Folder(f Folder) error {
	if err := w.add(rel("folder", f.ID, "unit", "org_unit", f.OrgID, "", nil)); err != nil {
		return err
	}
	if f.SharedOrgID != "" { // graph fan-in: one extra unit granted visibility
		if err := w.add(rel("folder", f.ID, "unit", "org_unit", f.SharedOrgID, "", nil)); err != nil {
			return err
		}
	}
	if f.ParentID != "" {
		return w.add(rel("folder", f.ID, "parent", "folder", f.ParentID, "", nil))
	}
	return nil
}

func (w *SpiceDBWriter) RebacDoc(d RebacDoc) error {
	return w.add(rel("rebac_document", d.ID, "folder", "folder", d.FolderID, "", nil))
}

func (w *SpiceDBWriter) Membership(m UnitMembership) error {
	return w.add(rel("org_unit", m.OrgID, m.Relation, "persona", m.PersonaID, "", nil))
}

// AbacDoc: one caveated wildcard relationship; the document's attributes are
// STATIC caveat context (they win over check-time context, per SpiceDB docs).
func (w *SpiceDBWriter) AbacDoc(d AbacDoc) error {
	ctx, err := structpb.NewStruct(map[string]any{
		"classification": d.Classification,
		"division":       d.DivisionKey,
		"status":         d.Status,
		"region":         d.Region,
	})
	if err != nil {
		return fmt.Errorf("abac caveat context: %w", err)
	}
	return w.add(rel("abac_document", d.ID, "reader", "persona", "*", "",
		&v1.ContextualizedCaveat{CaveatName: "doc_attrs", Context: ctx}))
}

func (w *SpiceDBWriter) PbacAssignment(a PbacAssignment) error {
	return w.add(rel("pbac_policy", a.PolicyID, "assignee", "persona", a.PersonaID, "", nil))
}

// PurchaseOrder: governed_by edge carries the policy parameters as static
// caveat context (amount/region arrive at check time).
func (w *SpiceDBWriter) PurchaseOrder(po PurchaseOrder) error {
	regions := make([]any, len(po.PolicyRegions))
	for i, r := range po.PolicyRegions {
		regions[i] = r
	}
	ctx, err := structpb.NewStruct(map[string]any{
		"active":     po.PolicyActive,
		"min_amount": po.PolicyMinAmount,
		"max_amount": po.PolicyMaxAmount,
		"regions":    regions,
	})
	if err != nil {
		return fmt.Errorf("pbac caveat context: %w", err)
	}
	return w.add(rel("purchase_order", po.ID, "governed_by", "pbac_policy", po.PolicyID, "",
		&v1.ContextualizedCaveat{CaveatName: "po_limits", Context: ctx}))
}

func (w *SpiceDBWriter) AclEntry(e AclEntry) error {
	relation := "viewer"
	if e.Action == "edit" {
		relation = "editor"
	}
	return w.add(rel("acl_document", e.ResourceID, relation, "persona", e.PersonaID, "", nil))
}

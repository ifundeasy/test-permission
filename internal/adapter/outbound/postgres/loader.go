// Package postgres implements port.EntityLoader against the `cedar` schema:
// per access model, it fetches exactly the rows needed to decide one check and
// shapes them into domain entities. This is the data-fetch work an embedded
// engine puts on the PEP (a server engine like SpiceDB owns it internally).
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ifundeasy/test-permission/internal/core/domain"
	"github.com/ifundeasy/test-permission/internal/core/port"
)

// Entity type names shared with the Cedar policies.
const (
	typePersona    = "Persona"
	typeRole       = "Role"
	typeOrgUnit    = "OrgUnit"
	typeFolder     = "Folder"
	typePbacPolicy = "PbacPolicy"
)

// registryTypes maps Cedar resource types to the registry type key stored in
// role_grants.resource_type.
var registryTypes = map[string]string{
	"Endpoint":  "endpoint",
	"Page":      "page",
	"Component": "component",
}

type Loader struct {
	pool *pgxpool.Pool
}

func NewLoader(pool *pgxpool.Pool) port.EntityLoader {
	return &Loader{pool: pool}
}

func (l *Loader) LoadForCheck(ctx context.Context, req domain.Request) ([]domain.Entity, error) {
	switch req.Model {
	case domain.ModelRBAC:
		return l.loadRBAC(ctx, req)
	case domain.ModelReBAC:
		return l.loadReBAC(ctx, req)
	case domain.ModelABAC:
		return l.loadABAC(ctx, req)
	case domain.ModelPBAC:
		return l.loadPBAC(ctx, req)
	case domain.ModelACL:
		return l.loadACL(ctx, req)
	default:
		return nil, fmt.Errorf("unknown model %q", req.Model)
	}
}

// loadRBAC: persona's roles become its Parents; the registry resource carries
// the set of roles granted the requested action.
func (l *Loader) loadRBAC(ctx context.Context, req domain.Request) ([]domain.Entity, error) {
	regType, ok := registryTypes[req.ResourceType]
	if !ok {
		return nil, fmt.Errorf("rbac: unsupported resource type %q", req.ResourceType)
	}

	roleIDs, err := l.stringColumn(ctx,
		`SELECT role_id FROM persona_roles WHERE persona_id = $1`, req.Principal)
	if err != nil {
		return nil, fmt.Errorf("query persona_roles: %w", err)
	}
	grantIDs, err := l.stringColumn(ctx,
		`SELECT role_id FROM role_grants
		 WHERE resource_type = $1 AND resource_id = $2 AND action = $3`,
		regType, req.Resource, req.Action)
	if err != nil {
		return nil, fmt.Errorf("query role_grants: %w", err)
	}

	ents := make([]domain.Entity, 0, len(roleIDs)+2)
	personaParents := make([]domain.EntityUID, 0, len(roleIDs))
	for _, id := range roleIDs {
		uid := domain.EntityUID{Type: typeRole, ID: id}
		personaParents = append(personaParents, uid)
		ents = append(ents, domain.Entity{UID: uid})
	}
	allowed := make([]domain.EntityUID, 0, len(grantIDs))
	for _, id := range grantIDs {
		allowed = append(allowed, domain.EntityUID{Type: typeRole, ID: id})
	}
	ents = append(ents,
		domain.Entity{UID: domain.EntityUID{Type: typePersona, ID: req.Principal}, Parents: personaParents},
		domain.Entity{
			UID:        domain.EntityUID{Type: req.ResourceType, ID: req.Resource},
			Attributes: map[string]any{"allowed_roles": allowed},
		},
	)
	return ents, nil
}

// loadReBAC: document → folder chain → owning org units → ancestor units, plus
// the persona's unit memberships (member or manager) as `member_of`.
func (l *Loader) loadReBAC(ctx context.Context, req domain.Request) ([]domain.Entity, error) {
	ents := make([]domain.Entity, 0, 16)

	memberOrgIDs, err := l.stringColumn(ctx,
		`SELECT DISTINCT org_id FROM unit_memberships WHERE persona_id = $1`, req.Principal)
	if err != nil {
		return nil, fmt.Errorf("query unit_memberships: %w", err)
	}
	memberOf := make([]domain.EntityUID, 0, len(memberOrgIDs))
	for _, id := range memberOrgIDs {
		memberOf = append(memberOf, domain.EntityUID{Type: typeOrgUnit, ID: id})
	}
	ents = append(ents, domain.Entity{
		UID:        domain.EntityUID{Type: typePersona, ID: req.Principal},
		Attributes: map[string]any{"member_of": memberOf},
	})

	var folderID *string
	err = l.pool.QueryRow(ctx,
		`SELECT folder_id FROM rebac_documents WHERE id = $1`, req.Resource).Scan(&folderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ents, nil // unknown document → deny falls out naturally
	}
	if err != nil {
		return nil, fmt.Errorf("query rebac_documents: %w", err)
	}
	var docParents []domain.EntityUID
	if folderID != nil {
		docParents = append(docParents, domain.EntityUID{Type: typeFolder, ID: *folderID})
	}
	ents = append(ents, domain.Entity{
		UID:     domain.EntityUID{Type: req.ResourceType, ID: req.Resource},
		Parents: docParents,
	})
	if folderID == nil {
		return ents, nil
	}

	// Folder chain upward; each folder is parented by its parent folder AND its
	// owning org unit (mirrors SpiceDB folder.unit + folder.parent).
	frows, err := l.pool.Query(ctx, `
		WITH RECURSIVE fchain AS (
		  SELECT id, parent_id, org_id FROM folders WHERE id = $1
		  UNION ALL
		  SELECT f.id, f.parent_id, f.org_id
		  FROM folders f JOIN fchain c ON f.id = c.parent_id
		)
		SELECT id, parent_id, org_id FROM fchain`, *folderID)
	if err != nil {
		return nil, fmt.Errorf("query folder chain: %w", err)
	}
	unitIDs := make([]string, 0, 4)
	for frows.Next() {
		var id, orgID string
		var parent *string
		if err := frows.Scan(&id, &parent, &orgID); err != nil {
			frows.Close()
			return nil, fmt.Errorf("scan folder: %w", err)
		}
		parents := []domain.EntityUID{{Type: typeOrgUnit, ID: orgID}}
		if parent != nil {
			parents = append(parents, domain.EntityUID{Type: typeFolder, ID: *parent})
		}
		ents = append(ents, domain.Entity{UID: domain.EntityUID{Type: typeFolder, ID: id}, Parents: parents})
		unitIDs = append(unitIDs, orgID)
	}
	frows.Close()
	if err := frows.Err(); err != nil {
		return nil, fmt.Errorf("iterate folder chain: %w", err)
	}

	// Org-unit ancestor chain (subsidiary → parent → root).
	urows, err := l.pool.Query(ctx, `
		WITH RECURSIVE uchain AS (
		  SELECT id, parent_id FROM organizations WHERE id = ANY($1)
		  UNION ALL
		  SELECT o.id, o.parent_id
		  FROM organizations o JOIN uchain c ON o.id = c.parent_id
		)
		SELECT DISTINCT id, parent_id FROM uchain`, unitIDs)
	if err != nil {
		return nil, fmt.Errorf("query org chain: %w", err)
	}
	for urows.Next() {
		var id string
		var parent *string
		if err := urows.Scan(&id, &parent); err != nil {
			urows.Close()
			return nil, fmt.Errorf("scan org unit: %w", err)
		}
		var parents []domain.EntityUID
		if parent != nil {
			parents = append(parents, domain.EntityUID{Type: typeOrgUnit, ID: *parent})
		}
		ents = append(ents, domain.Entity{UID: domain.EntityUID{Type: typeOrgUnit, ID: id}, Parents: parents})
	}
	urows.Close()
	if err := urows.Err(); err != nil {
		return nil, fmt.Errorf("iterate org chain: %w", err)
	}
	return ents, nil
}

// loadABAC: both sides' attributes — the whole model is attribute comparison.
func (l *Loader) loadABAC(ctx context.Context, req domain.Request) ([]domain.Entity, error) {
	ents := make([]domain.Entity, 0, 2)

	var clearance int64
	var division string
	err := l.pool.QueryRow(ctx, `
		SELECT p.clearance, d.key
		FROM personas p JOIN divisions d ON d.id = p.division_id
		WHERE p.id = $1`, req.Principal).Scan(&clearance, &division)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("query persona attrs: %w", err)
	}
	if err == nil {
		ents = append(ents, domain.Entity{
			UID:        domain.EntityUID{Type: typePersona, ID: req.Principal},
			Attributes: map[string]any{"clearance": clearance, "division": division},
		})
	}

	var classification int64
	var docDivision, status string
	err = l.pool.QueryRow(ctx,
		`SELECT classification, division_key, status FROM abac_documents WHERE id = $1`,
		req.Resource).Scan(&classification, &docDivision, &status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("query abac document: %w", err)
	}
	if err == nil {
		ents = append(ents, domain.Entity{
			UID: domain.EntityUID{Type: req.ResourceType, ID: req.Resource},
			Attributes: map[string]any{
				"classification": classification,
				"division":       docDivision,
				"status":         status,
			},
		})
	}
	return ents, nil
}

// loadPBAC: the PO references its governing policy entity (parameters as
// attributes); the persona's assigned policies become its Parents.
func (l *Loader) loadPBAC(ctx context.Context, req domain.Request) ([]domain.Entity, error) {
	ents := make([]domain.Entity, 0, 4)

	assigned, err := l.stringColumn(ctx,
		`SELECT policy_id FROM pbac_assignments WHERE persona_id = $1`, req.Principal)
	if err != nil {
		return nil, fmt.Errorf("query pbac_assignments: %w", err)
	}
	personaParents := make([]domain.EntityUID, 0, len(assigned))
	for _, id := range assigned {
		personaParents = append(personaParents, domain.EntityUID{Type: typePbacPolicy, ID: id})
	}
	ents = append(ents, domain.Entity{
		UID:     domain.EntityUID{Type: typePersona, ID: req.Principal},
		Parents: personaParents,
	})

	var policyID string
	err = l.pool.QueryRow(ctx,
		`SELECT policy_id FROM purchase_orders WHERE id = $1`, req.Resource).Scan(&policyID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ents, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query purchase_orders: %w", err)
	}
	ents = append(ents, domain.Entity{
		UID:        domain.EntityUID{Type: req.ResourceType, ID: req.Resource},
		Attributes: map[string]any{"policy": domain.EntityUID{Type: typePbacPolicy, ID: policyID}},
	})

	var active bool
	var maxAmount int64
	var regions []string
	err = l.pool.QueryRow(ctx,
		`SELECT active, max_amount, regions FROM pbac_policies WHERE id = $1`, policyID).
		Scan(&active, &maxAmount, &regions)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("query pbac_policies: %w", err)
	}
	if err == nil {
		ents = append(ents, domain.Entity{
			UID: domain.EntityUID{Type: typePbacPolicy, ID: policyID},
			Attributes: map[string]any{
				"active":     active,
				"max_amount": maxAmount,
				"regions":    regions,
			},
		})
	}
	return ents, nil
}

// loadACL: the document's direct grants become viewers/editors entity sets.
func (l *Loader) loadACL(ctx context.Context, req domain.Request) ([]domain.Entity, error) {
	rows, err := l.pool.Query(ctx,
		`SELECT persona_id, action FROM acl_entries WHERE resource_id = $1`, req.Resource)
	if err != nil {
		return nil, fmt.Errorf("query acl_entries: %w", err)
	}
	var viewers, editors []domain.EntityUID
	for rows.Next() {
		var personaID, action string
		if err := rows.Scan(&personaID, &action); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan acl entry: %w", err)
		}
		uid := domain.EntityUID{Type: typePersona, ID: personaID}
		switch action {
		case "view":
			viewers = append(viewers, uid)
		case "edit":
			editors = append(editors, uid)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate acl entries: %w", err)
	}
	return []domain.Entity{
		{UID: domain.EntityUID{Type: typePersona, ID: req.Principal}},
		{
			UID: domain.EntityUID{Type: req.ResourceType, ID: req.Resource},
			Attributes: map[string]any{
				"viewers": viewers,
				"editors": editors,
			},
		},
	}, nil
}

// stringColumn runs a single-column query and drains it into a slice.
func (l *Loader) stringColumn(ctx context.Context, sql string, args ...any) ([]string, error) {
	rows, err := l.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

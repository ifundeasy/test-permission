// Package postgres is the outbound adapter that implements port.EntityRepository
// against Postgres. It queries the application's own tables and shapes the rows
// into the domain-neutral entity slice the policy engine needs. This is the layer
// Cedar makes you build yourself (SpiceDB would own it, plus the graph traversal).
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

type repository struct {
	pool *pgxpool.Pool
}

// NewRepository wraps a pgx pool as an EntityRepository.
func NewRepository(pool *pgxpool.Pool) port.EntityRepository {
	return &repository{pool: pool}
}

// LoadEntities builds just the slice of entities needed to decide one request:
// the principal plus its group/role parents, and the document plus its owner,
// confidential flag, and full folder ancestry (walked with a recursive CTE).
func (r *repository) LoadEntities(ctx context.Context, userID, docID string) ([]domain.Entity, error) {
	ents := make(map[domain.EntityUID]domain.Entity)
	put := func(e domain.Entity) { ents[e.UID] = e }

	// ---- principal: the user + the groups/roles it belongs to ----
	rows, err := r.pool.Query(ctx,
		`SELECT parent_type, parent_id FROM memberships WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("query memberships: %w", err)
	}
	var userParents []domain.EntityUID
	for rows.Next() {
		var ptype, pid string
		if err := rows.Scan(&ptype, &pid); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan membership: %w", err)
		}
		userParents = append(userParents, domain.EntityUID{Type: ptype, ID: pid})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate memberships: %w", err)
	}
	put(domain.Entity{UID: domain.EntityUID{Type: "User", ID: userID}, Parents: userParents})
	for _, p := range userParents {
		put(domain.Entity{UID: p}) // stub Group/Role entities (no attrs, no parents)
	}

	// ---- resource: the document + its attributes + its folder ancestry ----
	var ownerID string
	var confidential bool
	var folderID *string
	err = r.pool.QueryRow(ctx,
		`SELECT owner_id, confidential, folder_id FROM documents WHERE id = $1`, docID).
		Scan(&ownerID, &confidential, &folderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return toSlice(ents), nil // unknown document: no resource entities (matches original behavior)
	}
	if err != nil {
		return nil, fmt.Errorf("query document: %w", err)
	}

	var docParents []domain.EntityUID
	if folderID != nil {
		docParents = append(docParents, domain.EntityUID{Type: "Folder", ID: *folderID})
	}
	put(domain.Entity{
		UID: domain.EntityUID{Type: "Document", ID: docID},
		Attributes: map[string]any{
			"owner":        domain.EntityUID{Type: "User", ID: ownerID}, // entity reference
			"confidential": confidential,
		},
		Parents: docParents,
	})

	// Walk the folder hierarchy upward (the recursive CTE you'd never need with
	// SpiceDB — it traverses the graph internally).
	if folderID != nil {
		frows, err := r.pool.Query(ctx, `
			WITH RECURSIVE chain AS (
			  SELECT id, parent_folder_id FROM folders WHERE id = $1
			  UNION ALL
			  SELECT fo.id, fo.parent_folder_id
			  FROM folders fo JOIN chain c ON fo.id = c.parent_folder_id
			)
			SELECT id, parent_folder_id FROM chain`, *folderID)
		if err != nil {
			return nil, fmt.Errorf("query folder chain: %w", err)
		}
		for frows.Next() {
			var id string
			var parent *string
			if err := frows.Scan(&id, &parent); err != nil {
				frows.Close()
				return nil, fmt.Errorf("scan folder: %w", err)
			}
			var fparents []domain.EntityUID
			if parent != nil {
				fparents = append(fparents, domain.EntityUID{Type: "Folder", ID: *parent})
			}
			put(domain.Entity{UID: domain.EntityUID{Type: "Folder", ID: id}, Parents: fparents})
		}
		frows.Close()
		if err := frows.Err(); err != nil {
			return nil, fmt.Errorf("iterate folder chain: %w", err)
		}
	}

	return toSlice(ents), nil
}

func toSlice(m map[domain.EntityUID]domain.Entity) []domain.Entity {
	out := make([]domain.Entity, 0, len(m))
	for _, e := range m {
		out = append(out, e)
	}
	return out
}

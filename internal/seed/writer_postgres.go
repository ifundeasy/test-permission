package seed

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ddl creates the benchmark tables in schema "cedar" (the connecting role's
// search_path routes unqualified names there). No FK constraints: integrity is
// guaranteed by the generator and constraints would only slow bulk seeding.
// Indexes cover every PEP lookup path used by the entity loaders.
const ddl = `
CREATE TABLE IF NOT EXISTS organizations (id text PRIMARY KEY, parent_id text, root_id text NOT NULL, name text NOT NULL, depth int NOT NULL, region text NOT NULL);
CREATE TABLE IF NOT EXISTS divisions (id text PRIMARY KEY, org_id text NOT NULL, key text NOT NULL, name text NOT NULL, is_default boolean NOT NULL);
CREATE TABLE IF NOT EXISTS roles (id text PRIMARY KEY, org_id text NOT NULL, key text NOT NULL, name text NOT NULL, is_default boolean NOT NULL);
CREATE TABLE IF NOT EXISTS accounts (id text PRIMARY KEY, email text NOT NULL, name text NOT NULL);
CREATE TABLE IF NOT EXISTS personas (id text PRIMARY KEY, account_id text NOT NULL, org_id text NOT NULL, division_id text NOT NULL, clearance smallint NOT NULL, employment_type text NOT NULL, region text NOT NULL);
CREATE TABLE IF NOT EXISTS endpoints (id text PRIMARY KEY, service_key text NOT NULL, method text NOT NULL, path text NOT NULL);
CREATE TABLE IF NOT EXISTS pages (id text PRIMARY KEY, service_key text NOT NULL, route text NOT NULL);
CREATE TABLE IF NOT EXISTS components (id text PRIMARY KEY, service_key text NOT NULL, page_id text NOT NULL);
CREATE TABLE IF NOT EXISTS persona_roles (persona_id text NOT NULL, role_id text NOT NULL, org_id text NOT NULL, PRIMARY KEY (persona_id, role_id));
CREATE TABLE IF NOT EXISTS role_grants (role_id text NOT NULL, resource_type text NOT NULL, resource_id text NOT NULL, action text NOT NULL, PRIMARY KEY (role_id, resource_type, resource_id, action));
CREATE TABLE IF NOT EXISTS folders (id text PRIMARY KEY, org_id text NOT NULL, parent_id text);
CREATE TABLE IF NOT EXISTS rebac_documents (id text PRIMARY KEY, folder_id text NOT NULL, owner_persona_id text NOT NULL);
CREATE TABLE IF NOT EXISTS unit_memberships (persona_id text NOT NULL, org_id text NOT NULL, relation text NOT NULL, PRIMARY KEY (persona_id, org_id, relation));
CREATE TABLE IF NOT EXISTS abac_documents (id text PRIMARY KEY, org_id text NOT NULL, division_key text NOT NULL, classification smallint NOT NULL, status text NOT NULL, region text NOT NULL);
CREATE TABLE IF NOT EXISTS pbac_policies (id text PRIMARY KEY, org_id text NOT NULL, name text NOT NULL, division_key text NOT NULL, max_amount bigint NOT NULL, regions text[] NOT NULL, active boolean NOT NULL);
CREATE TABLE IF NOT EXISTS pbac_assignments (persona_id text NOT NULL, policy_id text NOT NULL, PRIMARY KEY (persona_id, policy_id));
CREATE TABLE IF NOT EXISTS purchase_orders (id text PRIMARY KEY, org_id text NOT NULL, policy_id text NOT NULL, division_key text NOT NULL, region text NOT NULL);
CREATE TABLE IF NOT EXISTS acl_documents (id text PRIMARY KEY, org_id text NOT NULL);
CREATE TABLE IF NOT EXISTS acl_entries (resource_id text NOT NULL, persona_id text NOT NULL, action text NOT NULL, PRIMARY KEY (resource_id, persona_id, action));
CREATE TABLE IF NOT EXISTS seed_checkpoints (engine text NOT NULL, phase text NOT NULL, completed_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY (engine, phase));
ALTER TABLE seed_checkpoints ADD COLUMN IF NOT EXISTS params text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_persona_roles_persona ON persona_roles (persona_id);
CREATE INDEX IF NOT EXISTS idx_role_grants_resource ON role_grants (resource_type, resource_id, action);
CREATE INDEX IF NOT EXISTS idx_memberships_persona ON unit_memberships (persona_id);
CREATE INDEX IF NOT EXISTS idx_folders_parent ON folders (parent_id);
CREATE INDEX IF NOT EXISTS idx_rebac_documents_folder ON rebac_documents (folder_id);
CREATE INDEX IF NOT EXISTS idx_pbac_assignments_persona ON pbac_assignments (persona_id);
CREATE INDEX IF NOT EXISTS idx_acl_entries_resource ON acl_entries (resource_id);
CREATE INDEX IF NOT EXISTS idx_organizations_parent ON organizations (parent_id);
`

// table column orders (must match the buffered row construction below).
var tableCols = map[string][]string{
	"organizations":    {"id", "parent_id", "root_id", "name", "depth", "region"},
	"divisions":        {"id", "org_id", "key", "name", "is_default"},
	"roles":            {"id", "org_id", "key", "name", "is_default"},
	"accounts":         {"id", "email", "name"},
	"personas":         {"id", "account_id", "org_id", "division_id", "clearance", "employment_type", "region"},
	"endpoints":        {"id", "service_key", "method", "path"},
	"pages":            {"id", "service_key", "route"},
	"components":       {"id", "service_key", "page_id"},
	"persona_roles":    {"persona_id", "role_id", "org_id"},
	"role_grants":      {"role_id", "resource_type", "resource_id", "action"},
	"folders":          {"id", "org_id", "parent_id"},
	"rebac_documents":  {"id", "folder_id", "owner_persona_id"},
	"unit_memberships": {"persona_id", "org_id", "relation"},
	"abac_documents":   {"id", "org_id", "division_key", "classification", "status", "region"},
	"pbac_policies":    {"id", "org_id", "name", "division_key", "max_amount", "regions", "active"},
	"pbac_assignments": {"persona_id", "policy_id"},
	"purchase_orders":  {"id", "org_id", "policy_id", "division_key", "region"},
	"acl_documents":    {"id", "org_id"},
	"acl_entries":      {"resource_id", "persona_id", "action"},
}

// PostgresWriter is the Cedar-side sink: batched multi-row INSERTs
// (ON CONFLICT DO NOTHING → idempotent, resumable) into schema "cedar".
type PostgresWriter struct {
	pool     *pgxpool.Pool
	batch    int
	buf      map[string][][]any
	progress *Progress
	ctx      context.Context
	params   string // seed/scale fingerprint stored with checkpoints
}

func NewPostgresWriter(ctx context.Context, pool *pgxpool.Pool, batch int, progress *Progress, params string) (*PostgresWriter, error) {
	if _, err := pool.Exec(ctx, ddl); err != nil {
		return nil, fmt.Errorf("apply DDL: %w", err)
	}
	return &PostgresWriter{
		pool: pool, batch: batch, buf: map[string][][]any{}, progress: progress, ctx: ctx, params: params,
	}, nil
}

func (w *PostgresWriter) add(table string, row []any) error {
	w.buf[table] = append(w.buf[table], row)
	w.progress.Add(1)
	if len(w.buf[table]) >= w.batch {
		return w.flush(table)
	}
	return nil
}

// flush writes one multi-row INSERT (≤ batch rows) for the table.
func (w *PostgresWriter) flush(table string) error {
	rows := w.buf[table]
	if len(rows) == 0 {
		return nil
	}
	w.buf[table] = w.buf[table][:0]

	cols := tableCols[table]
	var sb strings.Builder
	fmt.Fprintf(&sb, "INSERT INTO %s (%s) VALUES ", table, strings.Join(cols, ","))
	args := make([]any, 0, len(rows)*len(cols))
	for i, row := range rows {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('(')
		for j := range cols {
			if j > 0 {
				sb.WriteByte(',')
			}
			fmt.Fprintf(&sb, "$%d", len(args)+1)
			args = append(args, row[j])
		}
		sb.WriteByte(')')
	}
	sb.WriteString(" ON CONFLICT DO NOTHING")
	if _, err := w.pool.Exec(w.ctx, sb.String(), args...); err != nil {
		return fmt.Errorf("insert %s: %w", table, err)
	}
	return nil
}

func (w *PostgresWriter) flushAll() error {
	for table := range w.buf {
		if err := w.flush(table); err != nil {
			return err
		}
	}
	return nil
}

func (w *PostgresWriter) BeginPhase(phase string, total int) error {
	w.progress.Begin(phase, total)
	return nil
}

func (w *PostgresWriter) EndPhase(phase string) error {
	if err := w.flushAll(); err != nil {
		return err
	}
	if _, err := w.pool.Exec(w.ctx,
		`INSERT INTO seed_checkpoints (engine, phase, params) VALUES ('cedar', $1, $2) ON CONFLICT DO NOTHING`,
		phase, w.params); err != nil {
		return fmt.Errorf("checkpoint: %w", err)
	}
	w.progress.End()
	return nil
}

func (w *PostgresWriter) Org(o Org) error {
	return w.add("organizations", []any{o.ID, nullable(o.ParentID), o.RootID, o.Name, o.Depth, o.Region})
}
func (w *PostgresWriter) Division(d Division) error {
	return w.add("divisions", []any{d.ID, d.OrgID, d.Key, d.Name, d.IsDefault})
}
func (w *PostgresWriter) Role(r Role) error {
	return w.add("roles", []any{r.ID, r.OrgID, r.Key, r.Name, r.IsDefault})
}
func (w *PostgresWriter) Account(a Account) error {
	return w.add("accounts", []any{a.ID, a.Email, a.Name})
}
func (w *PostgresWriter) Persona(p Persona) error {
	return w.add("personas", []any{p.ID, p.AccountID, p.OrgID, p.DivisionID, p.Clearance, p.EmploymentType, p.Region})
}
func (w *PostgresWriter) Resource(r RegistryResource) error {
	switch r.Kind {
	case "endpoint":
		return w.add("endpoints", []any{r.ID, r.ServiceKey, r.Method, r.Path})
	case "page":
		return w.add("pages", []any{r.ID, r.ServiceKey, r.Route})
	case "component":
		return w.add("components", []any{r.ID, r.ServiceKey, r.PageID})
	}
	return fmt.Errorf("unknown resource kind %q", r.Kind)
}
func (w *PostgresWriter) PersonaRole(pr PersonaRole) error {
	return w.add("persona_roles", []any{pr.PersonaID, pr.RoleID, pr.OrgID})
}
func (w *PostgresWriter) RoleGrant(rg RoleGrant) error {
	return w.add("role_grants", []any{rg.RoleID, rg.ResourceType, rg.ResourceID, rg.Action})
}
func (w *PostgresWriter) Folder(f Folder) error {
	return w.add("folders", []any{f.ID, f.OrgID, nullable(f.ParentID)})
}
func (w *PostgresWriter) RebacDoc(d RebacDoc) error {
	return w.add("rebac_documents", []any{d.ID, d.FolderID, d.OwnerPersonaID})
}
func (w *PostgresWriter) Membership(m UnitMembership) error {
	return w.add("unit_memberships", []any{m.PersonaID, m.OrgID, m.Relation})
}
func (w *PostgresWriter) AbacDoc(d AbacDoc) error {
	return w.add("abac_documents", []any{d.ID, d.OrgID, d.DivisionKey, d.Classification, d.Status, d.Region})
}
func (w *PostgresWriter) PbacPolicy(p PbacPolicy) error {
	return w.add("pbac_policies", []any{p.ID, p.OrgID, p.Name, p.DivisionKey, p.MaxAmount, p.Regions, p.Active})
}
func (w *PostgresWriter) PbacAssignment(a PbacAssignment) error {
	return w.add("pbac_assignments", []any{a.PersonaID, a.PolicyID})
}
func (w *PostgresWriter) PurchaseOrder(po PurchaseOrder) error {
	return w.add("purchase_orders", []any{po.ID, po.OrgID, po.PolicyID, po.DivisionKey, po.Region})
}
func (w *PostgresWriter) AclDoc(d AclDoc) error {
	return w.add("acl_documents", []any{d.ID, d.OrgID})
}
func (w *PostgresWriter) AclEntry(e AclEntry) error {
	return w.add("acl_entries", []any{e.ResourceID, e.PersonaID, e.Action})
}

// nullable maps "" to NULL.
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// DonePhases returns the phases already checkpointed for an engine under the
// SAME seed/scale params. A checkpoint from DIFFERENT params is returned as
// conflict — resuming across seeds/scales would silently mix datasets (IDs
// overlap), so the caller must demand -wipe. Legacy rows with empty params are
// treated as matching.
func DonePhases(ctx context.Context, pool *pgxpool.Pool, engine, params string) (done map[string]bool, conflict string, err error) {
	if _, err := pool.Exec(ctx, ddl); err != nil { // ensure checkpoint table exists
		return nil, "", err
	}
	rows, err := pool.Query(ctx, `SELECT phase, params FROM seed_checkpoints WHERE engine = $1`, engine)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	done = map[string]bool{}
	for rows.Next() {
		var p, rowParams string
		if err := rows.Scan(&p, &rowParams); err != nil {
			return nil, "", err
		}
		if rowParams != "" && rowParams != params {
			conflict = rowParams
			continue
		}
		done[p] = true
	}
	return done, conflict, rows.Err()
}

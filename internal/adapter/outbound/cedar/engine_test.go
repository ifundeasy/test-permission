package cedar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ifundeasy/test-permission/internal/core/domain"
)

// newTestDecider loads the real policy files (no loader — Evaluate is called
// directly with hand-built fixtures mirroring what the Postgres loader emits).
func newTestDecider(t *testing.T) *Decider {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "..", "policies")
	paths, err := filepath.Glob(filepath.Join(dir, "*.cedar"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("glob policies: %v (found %d)", err, len(paths))
	}
	docs := make(map[string][]byte, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		docs[filepath.Base(p)] = b
	}
	d, err := NewDecider(docs, nil)
	if err != nil {
		t.Fatalf("new decider: %v", err)
	}
	return d
}

func uid(typ, id string) domain.EntityUID { return domain.EntityUID{Type: typ, ID: id} }

func TestRBAC(t *testing.T) {
	d := newTestDecider(t)
	ents := []domain.Entity{
		{UID: uid("Role", "role-mgr")},
		{UID: uid("Role", "role-staff")},
		{UID: uid("Persona", "p1"), Parents: []domain.EntityUID{uid("Role", "role-mgr")}},
		{UID: uid("Persona", "p2"), Parents: []domain.EntityUID{uid("Role", "role-staff")}},
		{UID: uid("Endpoint", "ep1"), Attributes: map[string]any{
			"allowed_roles": []domain.EntityUID{uid("Role", "role-mgr")},
		}},
	}
	cases := []struct {
		principal string
		want      domain.Decision
	}{
		{"p1", domain.Allow}, // has role-mgr
		{"p2", domain.Deny},  // staff not granted
	}
	for _, tc := range cases {
		got, err := d.Evaluate(domain.Request{
			Model: domain.ModelRBAC, Principal: tc.principal,
			Action: "execute", ResourceType: "Endpoint", Resource: "ep1",
		}, ents)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("rbac %s = %q, want %q", tc.principal, got, tc.want)
		}
	}
}

func TestReBAC(t *testing.T) {
	d := newTestDecider(t)
	// doc → folder-b → folder-a → unit "org-sub" → parent "org-root";
	// folder-b is also SHARED with "org-shared" (graph fan-in)
	ents := []domain.Entity{
		{UID: uid("RebacDocument", "doc1"), Parents: []domain.EntityUID{uid("Folder", "f-b")}},
		{UID: uid("Folder", "f-b"), Parents: []domain.EntityUID{
			uid("OrgUnit", "org-sub"), uid("OrgUnit", "org-shared"), uid("Folder", "f-a")}},
		{UID: uid("Folder", "f-a"), Parents: []domain.EntityUID{uid("OrgUnit", "org-sub")}},
		{UID: uid("OrgUnit", "org-sub"), Parents: []domain.EntityUID{uid("OrgUnit", "org-root")}},
		{UID: uid("OrgUnit", "org-root")},
		{UID: uid("OrgUnit", "org-shared")},
		// p1 is member at the ROOT (ancestor) — must see subsidiary docs
		{UID: uid("Persona", "p1"), Attributes: map[string]any{
			"member_of": []domain.EntityUID{uid("OrgUnit", "org-root")},
		}},
		// p2 is member of an unrelated unit
		{UID: uid("Persona", "p2"), Attributes: map[string]any{
			"member_of": []domain.EntityUID{uid("OrgUnit", "org-other")},
		}},
		// p3 reaches the doc only via the SHARED unit edge
		{UID: uid("Persona", "p3"), Attributes: map[string]any{
			"member_of": []domain.EntityUID{uid("OrgUnit", "org-shared")},
		}},
	}
	cases := []struct {
		principal string
		want      domain.Decision
	}{
		{"p1", domain.Allow},
		{"p2", domain.Deny},
		{"p3", domain.Allow}, // shared-folder fan-in
	}
	for _, tc := range cases {
		got, err := d.Evaluate(domain.Request{
			Model: domain.ModelReBAC, Principal: tc.principal,
			Action: "doc.view", ResourceType: "RebacDocument", Resource: "doc1",
		}, ents)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("rebac %s = %q, want %q", tc.principal, got, tc.want)
		}
	}
}

func TestABAC(t *testing.T) {
	d := newTestDecider(t)
	docs := map[string]domain.Entity{
		"active": {UID: uid("AbacDocument", "d-active"), Attributes: map[string]any{
			"classification": int64(3), "division": "finance", "status": "active", "region": "jakarta",
		}},
		"archived": {UID: uid("AbacDocument", "d-archived"), Attributes: map[string]any{
			"classification": int64(1), "division": "finance", "status": "archived", "region": "jakarta",
		}},
	}
	personas := map[string]domain.Entity{
		"cleared": {UID: uid("Persona", "p-cleared"), Attributes: map[string]any{
			"clearance": int64(3), "division": "finance", "region": "jakarta",
		}},
		"low": {UID: uid("Persona", "p-low"), Attributes: map[string]any{
			"clearance": int64(2), "division": "finance", "region": "jakarta",
		}},
		"wrongdiv": {UID: uid("Persona", "p-wrongdiv"), Attributes: map[string]any{
			"clearance": int64(4), "division": "sales", "region": "jakarta",
		}},
		"wrongregion": {UID: uid("Persona", "p-wrongregion"), Attributes: map[string]any{
			"clearance": int64(3), "division": "finance", "region": "medan",
		}},
		"auditor": {UID: uid("Persona", "p-auditor"), Attributes: map[string]any{
			"clearance": int64(4), "division": "finance", "region": "medan",
		}},
	}
	cases := []struct {
		persona, doc string
		want         domain.Decision
	}{
		{"cleared", "active", domain.Allow},    // clearance 3 >= 3, division + region match
		{"low", "active", domain.Deny},         // clearance too low
		{"wrongdiv", "active", domain.Deny},    // division mismatch
		{"wrongregion", "active", domain.Deny}, // data residency: region mismatch, clearance < 4
		{"auditor", "active", domain.Allow},    // clearance-4 override beats region mismatch
		{"cleared", "archived", domain.Deny},   // forbid overrides (archived)
	}
	for _, tc := range cases {
		p, doc := personas[tc.persona], docs[tc.doc]
		got, err := d.Evaluate(domain.Request{
			Model: domain.ModelABAC, Principal: p.UID.ID,
			Action: "doc.read", ResourceType: "AbacDocument", Resource: doc.UID.ID,
		}, []domain.Entity{p, doc})
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("abac %s/%s = %q, want %q", tc.persona, tc.doc, got, tc.want)
		}
	}
}

// TestPBAC pins the load-bearing assumption: chained attribute deref through an
// entity-typed attribute (resource.policy.max_amount) plus `principal in
// resource.policy` with the policy entity as a Parent of the persona.
func TestPBAC(t *testing.T) {
	d := newTestDecider(t)
	ents := []domain.Entity{
		{UID: uid("PbacPolicy", "pol1"), Attributes: map[string]any{
			"active": true, "min_amount": int64(2_000_000), "max_amount": int64(50_000_000),
			"regions": []string{"jakarta", "surabaya"},
		}},
		{UID: uid("PbacPolicy", "pol-inactive"), Attributes: map[string]any{
			"active": false, "min_amount": int64(2_000_000), "max_amount": int64(50_000_000),
			"regions": []string{"jakarta"},
		}},
		{UID: uid("Persona", "p1"), Parents: []domain.EntityUID{uid("PbacPolicy", "pol1")}},
		{UID: uid("Persona", "p2")}, // not assigned to any policy
		{UID: uid("Persona", "p3"), Parents: []domain.EntityUID{uid("PbacPolicy", "pol-inactive")}},
		{UID: uid("PurchaseOrder", "po1"), Attributes: map[string]any{
			"policy": uid("PbacPolicy", "pol1"),
		}},
		{UID: uid("PurchaseOrder", "po-inactive"), Attributes: map[string]any{
			"policy": uid("PbacPolicy", "pol-inactive"),
		}},
	}
	cases := []struct {
		name      string
		principal string
		po        string
		amount    int64
		region    string
		want      domain.Decision
	}{
		{"within window", "p1", "po1", 10_000_000, "jakarta", domain.Allow},
		{"over the ceiling", "p1", "po1", 60_000_000, "jakarta", domain.Deny},
		{"below the floor", "p1", "po1", 1_000_000, "jakarta", domain.Deny},
		{"wrong region", "p1", "po1", 10_000_000, "medan", domain.Deny},
		{"not assignee", "p2", "po1", 10_000_000, "jakarta", domain.Deny},
		{"inactive policy", "p3", "po-inactive", 10_000_000, "jakarta", domain.Deny},
	}
	for _, tc := range cases {
		got, err := d.Evaluate(domain.Request{
			Model: domain.ModelPBAC, Principal: tc.principal,
			Action: "po.approve", ResourceType: "PurchaseOrder", Resource: tc.po,
			Context: map[string]any{"amount": tc.amount, "region": tc.region},
		}, ents)
		if err != nil {
			t.Fatalf("%s: evaluate: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("pbac %s = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestACL(t *testing.T) {
	d := newTestDecider(t)
	ents := []domain.Entity{
		{UID: uid("Persona", "p-viewer")},
		{UID: uid("Persona", "p-editor")},
		{UID: uid("Persona", "p-none")},
		{UID: uid("AclDocument", "doc1"), Attributes: map[string]any{
			"viewers": []domain.EntityUID{uid("Persona", "p-viewer")},
			"editors": []domain.EntityUID{uid("Persona", "p-editor")},
		}},
	}
	cases := []struct {
		principal, action string
		want              domain.Decision
	}{
		{"p-viewer", "acl.view", domain.Allow},
		{"p-viewer", "acl.edit", domain.Deny},
		{"p-editor", "acl.view", domain.Allow}, // editors can also view
		{"p-editor", "acl.edit", domain.Allow},
		{"p-none", "acl.view", domain.Deny},
	}
	for _, tc := range cases {
		got, err := d.Evaluate(domain.Request{
			Model: domain.ModelACL, Principal: tc.principal,
			Action: tc.action, ResourceType: "AclDocument", Resource: "doc1",
		}, ents)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if got != tc.want {
			t.Errorf("acl %s %s = %q, want %q", tc.principal, tc.action, got, tc.want)
		}
	}
}

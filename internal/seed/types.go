// Package seed generates the deterministic "Nusantara ERP" benchmark dataset
// and writes it to both engines. One canonical generator (fixed-seed PRNG)
// feeds interchangeable sinks: the Postgres writer (schema "cedar"), the
// SpiceDB writer (relationships), and the ground-truth tuple sampler — so both
// engines hold the identical logical dataset, apple-to-apple.
package seed

// Scale holds every row-count budget. Defaults meet the ≥1M-rows-per-model
// requirement; TestScale is a miniature for fast integration tests.
type Scale struct {
	Roots    int // root customer organizations
	OrgNodes int // total org nodes incl. roots (subsidiary trees, depth ≤ MaxDepth)
	MaxDepth int

	DivCustomMax  int // 0..N custom divisions per org node (plus the 5 defaults)
	RoleCustomMax int // 0..N custom roles per root org (plus the 5 default types)

	Accounts int
	Personas int

	GrantsPerRole int // registry resources granted per role         (RBAC)
	FoldersPerOrg int // folders per org node                        (ReBAC)
	DocsPerFolder int // rebac documents per folder                  (ReBAC)
	ManagerEdges  int // extra manager memberships                   (ReBAC)

	AbacDocs int // attribute-bearing documents                      (ABAC)

	PoliciesPerRoot int // org-defined approval policies             (PBAC)
	PurchaseOrders  int // purchase orders governed by a policy      (PBAC)

	AclDocs          int // directly-shared documents                (ACL)
	AclEntriesPerDoc int // direct grants per document               (ACL)

	TuplesPerModel int // sampled ground-truth tuples per model (half allow, half deny)
}

// FullScale reaches ≥1M countable rows per access model per engine:
//
//	RBAC  ≈ 480k personas × ~2.1 roles + 16k roles × 8 grants ≈ 1.14M
//	ReBAC ≈ 600k doc edges + 120k folders + 520k memberships + 18k org edges ≈ 1.26M
//	ABAC  = 1.00M attribute rows / caveated relationships
//	PBAC  ≈ 700k assignments + 300k PO links + 40k policies ≈ 1.04M
//	ACL   = 350k docs × 3 entries = 1.05M
func FullScale() Scale {
	return Scale{
		Roots:    2_000,
		OrgNodes: 20_000,
		MaxDepth: 4,

		DivCustomMax:  4,
		RoleCustomMax: 5,

		Accounts: 300_000,
		Personas: 480_000,

		GrantsPerRole: 8,
		FoldersPerOrg: 6,
		DocsPerFolder: 5,
		ManagerEdges:  40_000,

		AbacDocs: 1_000_000,

		PoliciesPerRoot: 20,
		PurchaseOrders:  300_000,

		AclDocs:          350_000,
		AclEntriesPerDoc: 3,

		TuplesPerModel: 10_000,
	}
}

// TestScale is a miniature dataset for fast end-to-end verification.
func TestScale() Scale {
	return Scale{
		Roots:    5,
		OrgNodes: 40,
		MaxDepth: 4,

		DivCustomMax:  4,
		RoleCustomMax: 5,

		Accounts: 120,
		Personas: 200,

		GrantsPerRole: 8,
		FoldersPerOrg: 3,
		DocsPerFolder: 3,
		ManagerEdges:  20,

		AbacDocs: 2_000,

		PoliciesPerRoot: 5,
		PurchaseOrders:  800,

		AclDocs:          500,
		AclEntriesPerDoc: 3,

		TuplesPerModel: 200,
	}
}

// ---- generated record types (the canonical stream) ----

type Org struct {
	ID       string
	ParentID string // "" for roots
	RootID   string
	Name     string
	Depth    int
	Region   string
}

type Division struct {
	ID        string
	OrgID     string
	Key       string
	Name      string
	IsDefault bool
}

type Role struct {
	ID        string
	OrgID     string // root org owning the role definition ("" = platform)
	Key       string
	Name      string
	IsDefault bool
}

type Account struct {
	ID    string
	Email string
	Name  string
}

type Persona struct {
	ID             string
	AccountID      string
	OrgID          string
	DivisionID     string
	Clearance      int // 1..4
	EmploymentType string
	Region         string
}

// RegistryResource is a flattened catalog entry (endpoint / page / component).
type RegistryResource struct {
	ID         string // ep:<svc>:<key> | pg:<svc>:<key> | cmp:<svc>:<page>:<name>
	Kind       string // endpoint | page | component
	Action     string // execute | view | render
	ServiceKey string
	Method     string // endpoints only
	Path       string // endpoints only
	Route      string // pages only
	PageID     string // components only
}

type PersonaRole struct {
	PersonaID string
	RoleID    string
	OrgID     string // root org
}

type RoleGrant struct {
	RoleID       string
	ResourceType string // endpoint | page | component
	ResourceID   string
	Action       string // execute | view | render
}

type Folder struct {
	ID       string
	OrgID    string
	ParentID string // "" for org-level roots
}

type RebacDoc struct {
	ID             string
	FolderID       string
	OwnerPersonaID string
}

type UnitMembership struct {
	PersonaID string
	OrgID     string
	Relation  string // member | manager
}

type AbacDoc struct {
	ID             string
	OrgID          string
	DivisionKey    string
	Classification int    // 1..4
	Status         string // draft | active | archived
	Region         string
}

type PbacPolicy struct {
	ID          string
	OrgID       string // root org
	Name        string
	DivisionKey string
	MaxAmount   int64
	Regions     []string
	Active      bool
}

type PbacAssignment struct {
	PersonaID string
	PolicyID  string
}

type PurchaseOrder struct {
	ID          string
	OrgID       string // root org
	PolicyID    string
	DivisionKey string
	Region      string
	// Denormalized policy parameters so the SpiceDB writer can attach the
	// po_limits caveat context statelessly (identical facts both engines).
	PolicyActive    bool
	PolicyMaxAmount int64
	PolicyRegions   []string
}

type AclDoc struct {
	ID    string
	OrgID string
}

type AclEntry struct {
	ResourceID string
	PersonaID  string
	Action     string // view | edit
}

// Sink consumes the canonical stream. Implementations: Postgres writer,
// SpiceDB writer, tuple sampler, and a fan-out combining them.
type Sink interface {
	BeginPhase(phase string, total int) error
	EndPhase(phase string) error

	Org(Org) error
	Division(Division) error
	Role(Role) error
	Account(Account) error
	Persona(Persona) error
	Resource(RegistryResource) error

	PersonaRole(PersonaRole) error
	RoleGrant(RoleGrant) error

	Folder(Folder) error
	RebacDoc(RebacDoc) error
	Membership(UnitMembership) error

	AbacDoc(AbacDoc) error

	PbacPolicy(PbacPolicy) error
	PbacAssignment(PbacAssignment) error
	PurchaseOrder(PurchaseOrder) error

	AclDoc(AclDoc) error
	AclEntry(AclEntry) error
}

// NopSink implements Sink with no-ops; embed it to implement only what you need.
type NopSink struct{}

func (NopSink) BeginPhase(string, int) error        { return nil }
func (NopSink) EndPhase(string) error               { return nil }
func (NopSink) Org(Org) error                       { return nil }
func (NopSink) Division(Division) error             { return nil }
func (NopSink) Role(Role) error                     { return nil }
func (NopSink) Account(Account) error               { return nil }
func (NopSink) Persona(Persona) error               { return nil }
func (NopSink) Resource(RegistryResource) error     { return nil }
func (NopSink) PersonaRole(PersonaRole) error       { return nil }
func (NopSink) RoleGrant(RoleGrant) error           { return nil }
func (NopSink) Folder(Folder) error                 { return nil }
func (NopSink) RebacDoc(RebacDoc) error             { return nil }
func (NopSink) Membership(UnitMembership) error     { return nil }
func (NopSink) AbacDoc(AbacDoc) error               { return nil }
func (NopSink) PbacPolicy(PbacPolicy) error         { return nil }
func (NopSink) PbacAssignment(PbacAssignment) error { return nil }
func (NopSink) PurchaseOrder(PurchaseOrder) error   { return nil }
func (NopSink) AclDoc(AclDoc) error                 { return nil }
func (NopSink) AclEntry(AclEntry) error             { return nil }

package seed

import (
	"fmt"
	"math/rand/v2"

	"github.com/ifundeasy/test-permission/internal/catalog"
)

// Phase names (progress + checkpoints). Order matters: basis first, then one
// phase per access model.
const (
	PhaseBasis = "basis"
	PhaseRBAC  = "rbac"
	PhaseReBAC = "rebac"
	PhaseABAC  = "abac"
	PhasePBAC  = "pbac"
	PhaseACL   = "acl"
)

// AllPhases in canonical order.
var AllPhases = []string{PhaseBasis, PhaseRBAC, PhaseReBAC, PhaseABAC, PhasePBAC, PhaseACL}

// Per-phase PRNG streams: same seed + same tag → identical stream on every run,
// regardless of which sink consumes it. This is what makes the two engines'
// datasets provably identical.
var phaseTags = map[string]uint64{
	PhaseBasis: 0xb0a5,
	PhaseRBAC:  0x4bac,
	PhaseReBAC: 0x4eba,
	PhaseABAC:  0xabac,
	PhasePBAC:  0x9bac,
	PhaseACL:   0x0ac1,
}

var (
	defaultDivisions = []string{"finance", "procurement", "hr", "sales", "operations"}
	customDivisions  = []string{"halal-compliance", "export-desk", "r-and-d", "legal", "it-security", "csr", "quality-assurance", "fleet"}
	defaultRoles     = []string{"owner", "admin", "manager", "staff", "auditor"}
	customRoles      = []string{"tax-specialist", "warehouse-lead", "payroll-officer", "account-executive", "compliance-officer", "branch-head", "procurement-officer", "hr-partner"}
	regions          = []string{"jakarta", "surabaya", "bandung", "medan", "makassar", "semarang", "denpasar", "balikpapan"}
	employmentTypes  = []string{"full-time", "contract", "intern", "outsourced"}

	orgWords    = []string{"Samudra", "Nusantara", "Garuda", "Merapi", "Sriwijaya", "Borneo", "Celebes", "Andalas", "Krakatau", "Mahakam", "Batavia", "Rinjani", "Toba", "Flores", "Komodo", "Halmahera"}
	orgSuffixes = []string{"Group", "Sejahtera", "Makmur", "Abadi", "Perkasa", "Digital", "Logistik", "Industri", "Retail", "Energi"}
	branchKinds = []string{"Cabang", "Distribusi", "Pabrik", "Regional", "Unit"}
	firstNames  = []string{"Budi", "Siti", "Agus", "Dewi", "Rizky", "Putri", "Andi", "Ratna", "Joko", "Maya", "Hendra", "Lestari", "Bayu", "Intan", "Fajar", "Sari"}
	lastNames   = []string{"Santoso", "Wijaya", "Pratama", "Utami", "Saputra", "Handayani", "Kurniawan", "Rahayu", "Hidayat", "Anggraini", "Nugroho", "Puspita", "Setiawan", "Melati", "Gunawan", "Safitri"}
)

// Generator holds the retained "basis" dataset (identity + org structure) and
// derives every model-specific fact stream from it deterministically.
type Generator struct {
	Seed  uint64
	Scale Scale
	Cat   *catalog.Catalog

	// basis (built once by BuildBasis, deterministic)
	Orgs          []Org
	OrgIdx        map[string]int // org id → index
	RootOf        []int          // org index → root org index
	ParentOf      []int          // org index → parent org index (-1 for roots)
	Divisions     []Division
	DivsByOrg     [][]int // org index → division indexes
	Roles         []Role
	RolesByRoot   [][]int // root ordinal (0..Roots-1) → role indexes
	RootOrdinal   []int   // org index → root ordinal (index into RolesByRoot)
	Accounts      []Account
	Personas      []Persona
	PersonaOrg    []int   // persona index → org index
	PersonasByOrg [][]int // org index → persona indexes
	Resources     []RegistryResource
}

func NewGenerator(seed uint64, scale Scale, cat *catalog.Catalog) *Generator {
	return &Generator{Seed: seed, Scale: scale, Cat: cat}
}

func (g *Generator) rng(phase string) *rand.Rand {
	return rand.New(rand.NewPCG(g.Seed, phaseTags[phase]))
}

// ---------------------------------------------------------------------------
// Basis: orgs (trees), divisions, roles, accounts, personas, catalog registry.
// ---------------------------------------------------------------------------

// BuildBasis constructs the retained identity/org dataset in memory.
func (g *Generator) BuildBasis() {
	r := g.rng(PhaseBasis)
	s := g.Scale

	// Organizations: roots first, then subsidiaries attached to random existing
	// nodes of the same root (depth-capped) — guarantees forest structure.
	g.Orgs = make([]Org, 0, s.OrgNodes)
	g.RootOf = make([]int, 0, s.OrgNodes)
	g.ParentOf = make([]int, 0, s.OrgNodes)
	g.RootOrdinal = make([]int, 0, s.OrgNodes)
	nodesByRoot := make([][]int, s.Roots)
	for i := 0; i < s.Roots; i++ {
		name := fmt.Sprintf("PT %s %s", orgWords[r.IntN(len(orgWords))], orgSuffixes[r.IntN(len(orgSuffixes))])
		g.Orgs = append(g.Orgs, Org{
			ID:     fmt.Sprintf("org-%05d", i),
			RootID: fmt.Sprintf("org-%05d", i),
			Name:   fmt.Sprintf("%s %04d", name, i),
			Depth:  0,
			Region: regions[r.IntN(len(regions))],
		})
		g.RootOf = append(g.RootOf, i)
		g.ParentOf = append(g.ParentOf, -1)
		g.RootOrdinal = append(g.RootOrdinal, i)
		nodesByRoot[i] = []int{i}
	}
	for i := s.Roots; i < s.OrgNodes; i++ {
		rootOrd := r.IntN(s.Roots)
		// pick a parent within this root's subtree that still has depth room
		var parentIdx int
		for {
			cands := nodesByRoot[rootOrd]
			parentIdx = cands[r.IntN(len(cands))]
			if g.Orgs[parentIdx].Depth < s.MaxDepth-1 {
				break
			}
		}
		parent := g.Orgs[parentIdx]
		g.Orgs = append(g.Orgs, Org{
			ID:       fmt.Sprintf("org-%05d", i),
			ParentID: parent.ID,
			RootID:   parent.RootID,
			Name:     fmt.Sprintf("%s %s %04d", parent.Name, branchKinds[r.IntN(len(branchKinds))], i),
			Depth:    parent.Depth + 1,
			Region:   regions[r.IntN(len(regions))],
		})
		g.RootOf = append(g.RootOf, g.RootOf[parentIdx])
		g.ParentOf = append(g.ParentOf, parentIdx)
		g.RootOrdinal = append(g.RootOrdinal, rootOrd)
		nodesByRoot[rootOrd] = append(nodesByRoot[rootOrd], i)
	}
	g.OrgIdx = make(map[string]int, len(g.Orgs))
	for i, o := range g.Orgs {
		g.OrgIdx[o.ID] = i
	}

	// Divisions: 5 defaults + 0..DivCustomMax custom per org node.
	g.Divisions = g.Divisions[:0]
	g.DivsByOrg = make([][]int, len(g.Orgs))
	divSeq := 0
	for oi, o := range g.Orgs {
		for _, key := range defaultDivisions {
			g.DivsByOrg[oi] = append(g.DivsByOrg[oi], len(g.Divisions))
			g.Divisions = append(g.Divisions, Division{
				ID: fmt.Sprintf("div-%06d", divSeq), OrgID: o.ID, Key: key, Name: key, IsDefault: true,
			})
			divSeq++
		}
		for n := r.IntN(s.DivCustomMax + 1); n > 0; n-- {
			key := customDivisions[r.IntN(len(customDivisions))]
			g.DivsByOrg[oi] = append(g.DivsByOrg[oi], len(g.Divisions))
			g.Divisions = append(g.Divisions, Division{
				ID: fmt.Sprintf("div-%06d", divSeq), OrgID: o.ID, Key: key, Name: key, IsDefault: false,
			})
			divSeq++
		}
	}

	// Roles: 5 default types + 0..RoleCustomMax custom per ROOT org.
	g.Roles = g.Roles[:0]
	g.RolesByRoot = make([][]int, s.Roots)
	roleSeq := 0
	for rootOrd := 0; rootOrd < s.Roots; rootOrd++ {
		rootID := g.Orgs[rootOrd].ID
		for _, key := range defaultRoles {
			g.RolesByRoot[rootOrd] = append(g.RolesByRoot[rootOrd], len(g.Roles))
			g.Roles = append(g.Roles, Role{
				ID: fmt.Sprintf("role-%06d", roleSeq), OrgID: rootID, Key: key, Name: key, IsDefault: true,
			})
			roleSeq++
		}
		for n := r.IntN(s.RoleCustomMax + 1); n > 0; n-- {
			key := customRoles[r.IntN(len(customRoles))]
			g.RolesByRoot[rootOrd] = append(g.RolesByRoot[rootOrd], len(g.Roles))
			g.Roles = append(g.Roles, Role{
				ID: fmt.Sprintf("role-%06d", roleSeq), OrgID: rootID, Key: key, Name: key, IsDefault: false,
			})
			roleSeq++
		}
	}

	// Accounts → personas (1 persona ↔ 1 org node).
	g.Accounts = make([]Account, s.Accounts)
	for i := range g.Accounts {
		fn, ln := firstNames[r.IntN(len(firstNames))], lastNames[r.IntN(len(lastNames))]
		g.Accounts[i] = Account{
			ID:    fmt.Sprintf("acc-%06d", i),
			Email: fmt.Sprintf("%s.%s.%06d@example.co.id", fn, ln, i),
			Name:  fmt.Sprintf("%s %s", fn, ln),
		}
	}
	g.Personas = make([]Persona, s.Personas)
	g.PersonaOrg = make([]int, s.Personas)
	g.PersonasByOrg = make([][]int, len(g.Orgs))
	for i := range g.Personas {
		oi := r.IntN(len(g.Orgs))
		divs := g.DivsByOrg[oi]
		div := g.Divisions[divs[r.IntN(len(divs))]]
		g.Personas[i] = Persona{
			ID:             fmt.Sprintf("psn-%06d", i),
			AccountID:      g.Accounts[i%s.Accounts].ID,
			OrgID:          g.Orgs[oi].ID,
			DivisionID:     div.ID,
			Clearance:      1 + r.IntN(4),
			EmploymentType: employmentTypes[r.IntN(len(employmentTypes))],
			Region:         g.Orgs[oi].Region,
		}
		g.PersonaOrg[i] = oi
		g.PersonasByOrg[oi] = append(g.PersonasByOrg[oi], i)
	}

	// Application registry from the catalog (endpoints, pages, components).
	g.Resources = flattenCatalog(g.Cat)
}

// flattenCatalog derives registry resources + their IDs from the catalog JSON.
func flattenCatalog(c *catalog.Catalog) []RegistryResource {
	var out []RegistryResource
	for _, svc := range c.Services {
		for _, ep := range svc.Endpoints {
			// SpiceDB object IDs allow [a-zA-Z0-9/_|=+-] — '/' as separator, never ':'
			out = append(out, RegistryResource{
				ID:         fmt.Sprintf("ep/%s/%s", svc.Key, ep.Key),
				Kind:       "endpoint",
				Action:     "execute",
				ServiceKey: svc.Key,
				Method:     ep.Method,
				Path:       ep.Path,
			})
		}
		for _, pg := range svc.Pages {
			pageID := fmt.Sprintf("pg/%s/%s", svc.Key, pg.Key)
			out = append(out, RegistryResource{
				ID:         pageID,
				Kind:       "page",
				Action:     "view",
				ServiceKey: svc.Key,
				Route:      pg.Route,
			})
			for _, cmp := range pg.Components {
				out = append(out, RegistryResource{
					ID:         fmt.Sprintf("cmp/%s/%s/%s", svc.Key, pg.Key, cmp),
					Kind:       "component",
					Action:     "render",
					ServiceKey: svc.Key,
					PageID:     pageID,
				})
			}
		}
	}
	return out
}

// OrgAncestors returns the org index chain from oi up to its root (inclusive).
func (g *Generator) OrgAncestors(oi int) []int {
	var chain []int
	for i := oi; i != -1; i = g.ParentOf[i] {
		chain = append(chain, i)
	}
	return chain
}

// ---------------------------------------------------------------------------
// Streams: one per phase; identical for every sink given the same seed.
// ---------------------------------------------------------------------------

// StreamBasis emits the retained basis records (Postgres needs them as rows;
// SpiceDB only consumes org parent edges via the ReBAC phase).
func (g *Generator) StreamBasis(sink Sink) error {
	total := len(g.Orgs) + len(g.Divisions) + len(g.Roles) + len(g.Accounts) + len(g.Personas) + len(g.Resources)
	if err := sink.BeginPhase(PhaseBasis, total); err != nil {
		return err
	}
	for _, o := range g.Orgs {
		if err := sink.Org(o); err != nil {
			return err
		}
	}
	for _, d := range g.Divisions {
		if err := sink.Division(d); err != nil {
			return err
		}
	}
	for _, ro := range g.Roles {
		if err := sink.Role(ro); err != nil {
			return err
		}
	}
	for _, a := range g.Accounts {
		if err := sink.Account(a); err != nil {
			return err
		}
	}
	for _, p := range g.Personas {
		if err := sink.Persona(p); err != nil {
			return err
		}
	}
	for _, res := range g.Resources {
		if err := sink.Resource(res); err != nil {
			return err
		}
	}
	return sink.EndPhase(PhaseBasis)
}

// StreamRBAC emits role grants (role → registry resources) first, then persona
// role assignments (1–3 roles from the persona's ROOT org; avg ≈ 2.1).
func (g *Generator) StreamRBAC(sink Sink) error {
	r := g.rng(PhaseRBAC)
	s := g.Scale
	total := len(g.Roles)*s.GrantsPerRole + len(g.Personas)*2 // approximation for progress
	if err := sink.BeginPhase(PhaseRBAC, total); err != nil {
		return err
	}

	// Grants: each role may perform GrantsPerRole distinct registry resources.
	for _, role := range g.Roles {
		seen := make(map[int]struct{}, s.GrantsPerRole)
		for len(seen) < s.GrantsPerRole {
			ri := r.IntN(len(g.Resources))
			if _, dup := seen[ri]; dup {
				continue
			}
			seen[ri] = struct{}{}
			res := g.Resources[ri]
			if err := sink.RoleGrant(RoleGrant{
				RoleID: role.ID, ResourceType: res.Kind, ResourceID: res.ID, Action: res.Action,
			}); err != nil {
				return err
			}
		}
	}

	// Assignments: weighted 1/2/3 roles (25%/40%/35% → avg 2.1).
	for pi := range g.Personas {
		rootOrd := g.RootOrdinal[g.PersonaOrg[pi]]
		roles := g.RolesByRoot[rootOrd]
		var n int
		switch x := r.IntN(100); {
		case x < 25:
			n = 1
		case x < 65:
			n = 2
		default:
			n = 3
		}
		if n > len(roles) {
			n = len(roles)
		}
		seen := make(map[int]struct{}, n)
		for len(seen) < n {
			roleIdx := roles[r.IntN(len(roles))]
			if _, dup := seen[roleIdx]; dup {
				continue
			}
			seen[roleIdx] = struct{}{}
			if err := sink.PersonaRole(PersonaRole{
				PersonaID: g.Personas[pi].ID,
				RoleID:    g.Roles[roleIdx].ID,
				OrgID:     g.Roles[roleIdx].OrgID,
			}); err != nil {
				return err
			}
		}
	}
	return sink.EndPhase(PhaseRBAC)
}

// StreamReBAC emits folders (with parent chains), documents, and memberships
// (every persona is a member of its own org node; ManagerEdges extra managers
// at a random node within the persona's root tree).
func (g *Generator) StreamReBAC(sink Sink) error {
	r := g.rng(PhaseReBAC)
	s := g.Scale
	nFolders := len(g.Orgs) * s.FoldersPerOrg
	total := nFolders + nFolders*s.DocsPerFolder + len(g.Personas) + s.ManagerEdges
	if err := sink.BeginPhase(PhaseReBAC, total); err != nil {
		return err
	}

	// Folders: FoldersPerOrg per org node; ~half nest under a previous folder
	// of the same org (chain depth grows naturally, capped by count).
	folderOrg := make([]int, 0, nFolders)
	folderSeq := 0
	firstFolderOfOrg := make([]int, len(g.Orgs))
	for oi := range g.Orgs {
		firstFolderOfOrg[oi] = folderSeq
		for k := 0; k < s.FoldersPerOrg; k++ {
			parent := ""
			if k > 0 && r.IntN(2) == 0 {
				parent = fmt.Sprintf("fld-%06d", firstFolderOfOrg[oi]+r.IntN(k))
			}
			if err := sink.Folder(Folder{
				ID: fmt.Sprintf("fld-%06d", folderSeq), OrgID: g.Orgs[oi].ID, ParentID: parent,
			}); err != nil {
				return err
			}
			folderOrg = append(folderOrg, oi)
			folderSeq++
		}
	}

	// Documents: DocsPerFolder per folder, owner sampled from the folder's org
	// subtree root population (any persona — owner is not used by the policy).
	docSeq := 0
	for fi := 0; fi < nFolders; fi++ {
		for k := 0; k < s.DocsPerFolder; k++ {
			owner := g.Personas[r.IntN(len(g.Personas))].ID
			if err := sink.RebacDoc(RebacDoc{
				ID:             fmt.Sprintf("rdoc-%06d", docSeq),
				FolderID:       fmt.Sprintf("fld-%06d", fi),
				OwnerPersonaID: owner,
			}); err != nil {
				return err
			}
			docSeq++
		}
	}

	// Memberships: everyone is a member of their own org node…
	for pi := range g.Personas {
		if err := sink.Membership(UnitMembership{
			PersonaID: g.Personas[pi].ID, OrgID: g.Personas[pi].OrgID, Relation: "member",
		}); err != nil {
			return err
		}
	}
	// …plus ManagerEdges managers at a random node within their OWN root tree
	// (never crosses roots — keeps deny sampling sound).
	for k := 0; k < s.ManagerEdges; k++ {
		pi := r.IntN(len(g.Personas))
		rootOrd := g.RootOrdinal[g.PersonaOrg[pi]]
		// walk to a random org of the same root: pick among ancestors of a
		// random persona of that root (cheap deterministic approximation)
		cand := g.PersonaOrg[pi]
		anc := g.OrgAncestors(cand)
		target := anc[r.IntN(len(anc))]
		_ = rootOrd
		if err := sink.Membership(UnitMembership{
			PersonaID: g.Personas[pi].ID, OrgID: g.Orgs[target].ID, Relation: "manager",
		}); err != nil {
			return err
		}
	}
	return sink.EndPhase(PhaseReBAC)
}

// StreamABAC emits attribute-bearing documents (~10% archived).
func (g *Generator) StreamABAC(sink Sink) error {
	r := g.rng(PhaseABAC)
	s := g.Scale
	if err := sink.BeginPhase(PhaseABAC, s.AbacDocs); err != nil {
		return err
	}
	for i := 0; i < s.AbacDocs; i++ {
		oi := r.IntN(len(g.Orgs))
		status := "active"
		switch x := r.IntN(10); {
		case x == 0:
			status = "archived"
		case x <= 2:
			status = "draft"
		}
		if err := sink.AbacDoc(AbacDoc{
			ID:             fmt.Sprintf("adoc-%07d", i),
			OrgID:          g.Orgs[oi].ID,
			DivisionKey:    defaultDivisions[r.IntN(len(defaultDivisions))],
			Classification: 1 + r.IntN(4),
			Status:         status,
			Region:         g.Orgs[oi].Region,
		}); err != nil {
			return err
		}
	}
	return sink.EndPhase(PhaseABAC)
}

// StreamPBAC emits policies (per root org), persona→policy assignments
// (avg ≈ 1.46/persona), and purchase orders governed by a policy.
func (g *Generator) StreamPBAC(sink Sink) error {
	r := g.rng(PhasePBAC)
	s := g.Scale
	nPolicies := s.Roots * s.PoliciesPerRoot
	total := nPolicies + len(g.Personas)*3/2 + s.PurchaseOrders
	if err := sink.BeginPhase(PhasePBAC, total); err != nil {
		return err
	}

	policies := make([]PbacPolicy, 0, nPolicies)
	for rootOrd := 0; rootOrd < s.Roots; rootOrd++ {
		for k := 0; k < s.PoliciesPerRoot; k++ {
			nr := 1 + r.IntN(3)
			regs := make([]string, 0, nr)
			seen := map[int]struct{}{}
			for len(regs) < nr {
				ri := r.IntN(len(regions))
				if _, dup := seen[ri]; dup {
					continue
				}
				seen[ri] = struct{}{}
				regs = append(regs, regions[ri])
			}
			p := PbacPolicy{
				ID:          fmt.Sprintf("pol-%06d", len(policies)),
				OrgID:       g.Orgs[rootOrd].ID,
				Name:        fmt.Sprintf("approval-policy-%02d", k),
				DivisionKey: defaultDivisions[r.IntN(len(defaultDivisions))],
				MaxAmount:   int64(10+r.IntN(490)) * 1_000_000, // Rp 10jt .. 500jt
				Regions:     regs,
				Active:      r.IntN(10) != 0, // 90% active
			}
			policies = append(policies, p)
			if err := sink.PbacPolicy(p); err != nil {
				return err
			}
		}
	}

	// Assignments: 1 policy each + 46% a second (policies of the SAME root).
	for pi := range g.Personas {
		rootOrd := g.RootOrdinal[g.PersonaOrg[pi]]
		base := rootOrd * s.PoliciesPerRoot
		n := 1
		if r.IntN(100) < 46 {
			n = 2
		}
		seen := map[int]struct{}{}
		for len(seen) < n {
			k := r.IntN(s.PoliciesPerRoot)
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			if err := sink.PbacAssignment(PbacAssignment{
				PersonaID: g.Personas[pi].ID, PolicyID: policies[base+k].ID,
			}); err != nil {
				return err
			}
		}
	}

	// Purchase orders: governed by a random policy of a random root.
	for i := 0; i < s.PurchaseOrders; i++ {
		p := policies[r.IntN(len(policies))]
		if err := sink.PurchaseOrder(PurchaseOrder{
			ID:              fmt.Sprintf("po-%06d", i),
			OrgID:           p.OrgID,
			PolicyID:        p.ID,
			DivisionKey:     p.DivisionKey,
			Region:          p.Regions[r.IntN(len(p.Regions))],
			PolicyActive:    p.Active,
			PolicyMaxAmount: p.MaxAmount,
			PolicyRegions:   p.Regions,
		}); err != nil {
			return err
		}
	}
	return sink.EndPhase(PhasePBAC)
}

// StreamACL emits directly-shared documents with AclEntriesPerDoc grants each
// (personas of the document's own org subtree population; 70% view, 30% edit).
func (g *Generator) StreamACL(sink Sink) error {
	r := g.rng(PhaseACL)
	s := g.Scale
	total := s.AclDocs * (1 + s.AclEntriesPerDoc)
	if err := sink.BeginPhase(PhaseACL, total); err != nil {
		return err
	}
	for i := 0; i < s.AclDocs; i++ {
		oi := r.IntN(len(g.Orgs))
		docID := fmt.Sprintf("acldoc-%06d", i)
		if err := sink.AclDoc(AclDoc{ID: docID, OrgID: g.Orgs[oi].ID}); err != nil {
			return err
		}
		seen := map[int]struct{}{}
		for len(seen) < s.AclEntriesPerDoc {
			pi := r.IntN(len(g.Personas))
			if _, dup := seen[pi]; dup {
				continue
			}
			seen[pi] = struct{}{}
			action := "view"
			if r.IntN(10) < 3 {
				action = "edit"
			}
			if err := sink.AclEntry(AclEntry{
				ResourceID: docID, PersonaID: g.Personas[pi].ID, Action: action,
			}); err != nil {
				return err
			}
		}
	}
	return sink.EndPhase(PhaseACL)
}

// Run streams every phase into the sink (basis first).
func (g *Generator) Run(sink Sink, phases []string) error {
	for _, ph := range phases {
		var err error
		switch ph {
		case PhaseBasis:
			err = g.StreamBasis(sink)
		case PhaseRBAC:
			err = g.StreamRBAC(sink)
		case PhaseReBAC:
			err = g.StreamReBAC(sink)
		case PhaseABAC:
			err = g.StreamABAC(sink)
		case PhasePBAC:
			err = g.StreamPBAC(sink)
		case PhaseACL:
			err = g.StreamACL(sink)
		default:
			err = fmt.Errorf("unknown phase %q", ph)
		}
		if err != nil {
			return fmt.Errorf("phase %s: %w", ph, err)
		}
	}
	return nil
}

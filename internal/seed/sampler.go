package seed

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/ifundeasy/test-permission/internal/core/domain"
)

// Tuple is one ground-truth check: the generator KNOWS the expected decision
// because it created the underlying facts. Tuples feed the equivalence gate
// (Cedar == SpiceDB == expected) and the benchmark's query mix.
type Tuple struct {
	Model        string         `json:"model"`
	Principal    string         `json:"principal"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	Resource     string         `json:"resource"`
	Context      map[string]any `json:"context,omitempty"`
	Expected     string         `json:"expected"` // allow | deny
}

// Request converts the tuple to a domain check request.
func (t Tuple) Request() domain.Request {
	ctx := make(map[string]any, len(t.Context))
	for k, v := range t.Context {
		switch x := v.(type) {
		case float64: // JSON round-trip turns int64 into float64
			ctx[k] = int64(x)
		case []any:
			ss := make([]string, 0, len(x))
			for _, e := range x {
				ss = append(ss, e.(string))
			}
			ctx[k] = ss
		default:
			ctx[k] = v
		}
	}
	return domain.Request{
		Model:        domain.Model(t.Model),
		Principal:    t.Principal,
		Action:       t.Action,
		ResourceType: t.ResourceType,
		Resource:     t.Resource,
		Context:      ctx,
	}
}

// Sampler is a Sink that derives ground-truth tuples from the canonical stream.
// It never touches a database; determinism comes from its own PRNG stream.
type Sampler struct {
	NopSink
	gen *Generator
	rng *rand.Rand
	per int // tuples per model (half allow, half deny)

	Tuples map[string][]Tuple

	// cross-record state
	grantsByRole map[string][]RoleGrant // RBAC: role → granted resources
	rolesOfCur   []string               // RBAC: roles of the persona being streamed
	curPersona   string
	folderOrg    map[string]int // ReBAC: folder id → org index
	docsSeen     int
	aclCur       AclDoc     // ACL: doc being streamed
	aclEntries   []AclEntry // ACL: entries of current doc
	posSeen      int
	assignByPol  map[string][]string // PBAC: policy → assigned personas (sampled subset)
}

func NewSampler(gen *Generator, tuplesPerModel int) *Sampler {
	return &Sampler{
		gen:          gen,
		rng:          rand.New(rand.NewPCG(gen.Seed, 0x7a91e5)),
		per:          tuplesPerModel,
		Tuples:       map[string][]Tuple{},
		grantsByRole: map[string][]RoleGrant{},
		folderOrg:    map[string]int{},
		assignByPol:  map[string][]string{},
	}
}

func (s *Sampler) need(model string) bool { return len(s.Tuples[model]) < s.per }

func (s *Sampler) add(t Tuple) { s.Tuples[t.Model] = append(s.Tuples[t.Model], t) }

// randomPersonaOtherRoot picks a persona whose root org differs from rootOf.
func (s *Sampler) randomPersonaOtherRoot(rootIdx int) string {
	for {
		pi := s.rng.IntN(len(s.gen.Personas))
		if s.gen.RootOf[s.gen.PersonaOrg[pi]] != rootIdx {
			return s.gen.Personas[pi].ID
		}
	}
}

// ---- RBAC ------------------------------------------------------------------

func (s *Sampler) RoleGrant(g RoleGrant) error {
	s.grantsByRole[g.RoleID] = append(s.grantsByRole[g.RoleID], g)
	return nil
}

var kindToType = map[string]string{"endpoint": "Endpoint", "page": "Page", "component": "Component"}

func (s *Sampler) PersonaRole(pr PersonaRole) error {
	if pr.PersonaID != s.curPersona {
		s.flushRBACPersona()
		s.curPersona = pr.PersonaID
		s.rolesOfCur = s.rolesOfCur[:0]
	}
	s.rolesOfCur = append(s.rolesOfCur, pr.RoleID)
	return nil
}

// flushRBACPersona samples at most one allow and one deny from the finished
// persona's role set (sampling rate keeps totals near the target).
func (s *Sampler) flushRBACPersona() {
	if s.curPersona == "" || len(s.rolesOfCur) == 0 || !s.need("rbac") {
		return
	}
	if s.rng.IntN(len(s.gen.Personas)/(s.per/2)+1) != 0 {
		return
	}
	// allow: a resource granted to one of the persona's roles
	role := s.rolesOfCur[s.rng.IntN(len(s.rolesOfCur))]
	grants := s.grantsByRole[role]
	if len(grants) == 0 {
		return
	}
	gr := grants[s.rng.IntN(len(grants))]
	s.add(Tuple{
		Model: "rbac", Principal: s.curPersona, Action: gr.Action,
		ResourceType: kindToType[gr.ResourceType], Resource: gr.ResourceID, Expected: "allow",
	})
	// deny: a resource granted to NO role of this persona (rejection sample)
	mine := map[string]struct{}{}
	for _, r := range s.rolesOfCur {
		mine[r] = struct{}{}
	}
	for tries := 0; tries < 200; tries++ {
		ri := s.rng.IntN(len(s.gen.Resources))
		res := s.gen.Resources[ri]
		granted := false
		// a resource is allowed if ANY of the persona's roles grants it
		for _, r := range s.rolesOfCur {
			for _, g := range s.grantsByRole[r] {
				if g.ResourceID == res.ID {
					granted = true
					break
				}
			}
			if granted {
				break
			}
		}
		if !granted {
			s.add(Tuple{
				Model: "rbac", Principal: s.curPersona, Action: res.Action,
				ResourceType: kindToType[res.Kind], Resource: res.ID, Expected: "deny",
			})
			break
		}
	}
}

// ---- ReBAC -----------------------------------------------------------------

func (s *Sampler) Folder(f Folder) error {
	s.folderOrg[f.ID] = s.gen.OrgIdx[f.OrgID]
	return nil
}

func (s *Sampler) RebacDoc(d RebacDoc) error {
	s.docsSeen++
	if !s.need("rebac") {
		return nil
	}
	nDocs := len(s.gen.Orgs) * s.gen.Scale.FoldersPerOrg * s.gen.Scale.DocsPerFolder
	if s.rng.IntN(nDocs/(s.per/2)+1) != 0 {
		return nil
	}
	oi := s.folderOrg[d.FolderID]
	// allow: persona at the doc's org or any ancestor (members see downward)
	anc := s.gen.OrgAncestors(oi)
	for tries := 0; tries < 50; tries++ {
		ai := anc[s.rng.IntN(len(anc))]
		personas := s.gen.PersonasByOrg[ai]
		if len(personas) == 0 {
			continue
		}
		p := s.gen.Personas[personas[s.rng.IntN(len(personas))]]
		s.add(Tuple{
			Model: "rebac", Principal: p.ID, Action: "doc.view",
			ResourceType: "RebacDocument", Resource: d.ID, Expected: "allow",
		})
		break
	}
	// deny: persona from a different root — no cross-root membership exists
	s.add(Tuple{
		Model: "rebac", Principal: s.randomPersonaOtherRoot(s.gen.RootOf[oi]), Action: "doc.view",
		ResourceType: "RebacDocument", Resource: d.ID, Expected: "deny",
	})
	return nil
}

// ---- ABAC ------------------------------------------------------------------

func (s *Sampler) AbacDoc(d AbacDoc) error {
	if !s.need("abac") {
		return nil
	}
	if s.rng.IntN(s.gen.Scale.AbacDocs/(s.per/2)+1) != 0 {
		return nil
	}
	// find a persona whose TRUE attributes decide the outcome (context always
	// mirrors the true attributes — SpiceDB must judge the same facts as Cedar)
	for tries := 0; tries < 400; tries++ {
		pi := s.rng.IntN(len(s.gen.Personas))
		p := s.gen.Personas[pi]
		divKey := s.personaDivisionKey(pi)
		ctx := map[string]any{"clearance": int64(p.Clearance), "principal_division": divKey}
		match := p.Clearance >= d.Classification && divKey == d.DivisionKey
		expected := "deny"
		if match && d.Status != "archived" {
			expected = "allow"
		}
		// keep the allow/deny mix roughly even
		if expected == "allow" && s.countExpected("abac", "allow") > s.per/2 {
			continue
		}
		if expected == "deny" && s.countExpected("abac", "deny") > s.per/2 {
			continue
		}
		s.add(Tuple{
			Model: "abac", Principal: p.ID, Action: "doc.read",
			ResourceType: "AbacDocument", Resource: d.ID, Context: ctx, Expected: expected,
		})
		return nil
	}
	return nil
}

func (s *Sampler) personaDivisionKey(pi int) string {
	p := s.gen.Personas[pi]
	for _, di := range s.gen.DivsByOrg[s.gen.PersonaOrg[pi]] {
		if s.gen.Divisions[di].ID == p.DivisionID {
			return s.gen.Divisions[di].Key
		}
	}
	return ""
}

func (s *Sampler) countExpected(model, expected string) int {
	n := 0
	for _, t := range s.Tuples[model] {
		if t.Expected == expected {
			n++
		}
	}
	return n
}

// ---- PBAC ------------------------------------------------------------------

func (s *Sampler) PbacAssignment(a PbacAssignment) error {
	// retain a bounded sample of assignees per policy for allow tuples
	if lst := s.assignByPol[a.PolicyID]; len(lst) < 4 {
		s.assignByPol[a.PolicyID] = append(lst, a.PersonaID)
	}
	return nil
}

func (s *Sampler) PurchaseOrder(po PurchaseOrder) error {
	s.posSeen++
	if !s.need("pbac") {
		return nil
	}
	if s.rng.IntN(s.gen.Scale.PurchaseOrders/(s.per/2)+1) != 0 {
		return nil
	}
	assignees := s.assignByPol[po.PolicyID]
	if len(assignees) == 0 {
		return nil
	}
	assignee := assignees[s.rng.IntN(len(assignees))]
	inAmount := po.PolicyMaxAmount - int64(s.rng.IntN(1_000_000)+1)
	region := po.PolicyRegions[s.rng.IntN(len(po.PolicyRegions))]

	if po.PolicyActive {
		s.add(Tuple{
			Model: "pbac", Principal: assignee, Action: "po.approve",
			ResourceType: "PurchaseOrder", Resource: po.ID,
			Context:  map[string]any{"amount": inAmount, "region": region},
			Expected: "allow",
		})
	}
	// deny variants (rotate deterministically): over-amount, wrong region,
	// unassigned persona, inactive policy
	switch s.posSeen % 4 {
	case 0: // over amount
		s.add(Tuple{
			Model: "pbac", Principal: assignee, Action: "po.approve",
			ResourceType: "PurchaseOrder", Resource: po.ID,
			Context:  map[string]any{"amount": po.PolicyMaxAmount + 1_000_000, "region": region},
			Expected: "deny",
		})
	case 1: // region outside the policy
		s.add(Tuple{
			Model: "pbac", Principal: assignee, Action: "po.approve",
			ResourceType: "PurchaseOrder", Resource: po.ID,
			Context:  map[string]any{"amount": inAmount, "region": s.regionOutside(po.PolicyRegions)},
			Expected: "deny",
		})
	case 2: // persona from another root (never assigned to this policy)
		rootIdx := s.gen.OrgIdx[po.OrgID]
		s.add(Tuple{
			Model: "pbac", Principal: s.randomPersonaOtherRoot(s.gen.RootOf[rootIdx]), Action: "po.approve",
			ResourceType: "PurchaseOrder", Resource: po.ID,
			Context:  map[string]any{"amount": inAmount, "region": region},
			Expected: "deny",
		})
	case 3: // inactive policy denies even its assignee
		if !po.PolicyActive {
			s.add(Tuple{
				Model: "pbac", Principal: assignee, Action: "po.approve",
				ResourceType: "PurchaseOrder", Resource: po.ID,
				Context:  map[string]any{"amount": inAmount, "region": region},
				Expected: "deny",
			})
		}
	}
	return nil
}

func (s *Sampler) regionOutside(in []string) string {
	for {
		r := regions[s.rng.IntN(len(regions))]
		found := false
		for _, x := range in {
			if x == r {
				found = true
				break
			}
		}
		if !found {
			return r
		}
	}
}

// ---- ACL -------------------------------------------------------------------

func (s *Sampler) AclDoc(d AclDoc) error {
	s.flushACLDoc()
	s.aclCur = d
	s.aclEntries = s.aclEntries[:0]
	return nil
}

func (s *Sampler) AclEntry(e AclEntry) error {
	s.aclEntries = append(s.aclEntries, e)
	return nil
}

func (s *Sampler) flushACLDoc() {
	if s.aclCur.ID == "" || len(s.aclEntries) == 0 || !s.need("acl") {
		return
	}
	if s.rng.IntN(s.gen.Scale.AclDocs/(s.per/2)+1) != 0 {
		return
	}
	e := s.aclEntries[s.rng.IntN(len(s.aclEntries))]
	// allow: the entry's own action (editors also get acl.view — both valid)
	action := "acl." + e.Action
	s.add(Tuple{
		Model: "acl", Principal: e.PersonaID, Action: action,
		ResourceType: "AclDocument", Resource: s.aclCur.ID, Expected: "allow",
	})
	// deny 1: a viewer trying to edit
	if e.Action == "view" {
		s.add(Tuple{
			Model: "acl", Principal: e.PersonaID, Action: "acl.edit",
			ResourceType: "AclDocument", Resource: s.aclCur.ID, Expected: "deny",
		})
	} else {
		// deny 2: a persona with no entry at all
		inDoc := map[string]struct{}{}
		for _, en := range s.aclEntries {
			inDoc[en.PersonaID] = struct{}{}
		}
		for tries := 0; tries < 50; tries++ {
			p := s.gen.Personas[s.rng.IntN(len(s.gen.Personas))]
			if _, ok := inDoc[p.ID]; !ok {
				s.add(Tuple{
					Model: "acl", Principal: p.ID, Action: "acl.view",
					ResourceType: "AclDocument", Resource: s.aclCur.ID, Expected: "deny",
				})
				break
			}
		}
	}
}

// EndPhase flushes per-phase buffers.
func (s *Sampler) EndPhase(phase string) error {
	switch phase {
	case PhaseRBAC:
		s.flushRBACPersona()
	case PhaseACL:
		s.flushACLDoc()
	}
	return nil
}

// WriteFiles persists one JSON per model under dir.
func (s *Sampler) WriteFiles(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, model := range []string{"rbac", "rebac", "abac", "pbac", "acl"} {
		ts := s.Tuples[model]
		if len(ts) == 0 {
			return fmt.Errorf("no tuples sampled for model %s", model)
		}
		b, err := json.MarshalIndent(ts, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, model+".json"), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// LoadTuples reads back the sampled tuples for verify/bench.
func LoadTuples(dir, model string) ([]Tuple, error) {
	b, err := os.ReadFile(filepath.Join(dir, model+".json"))
	if err != nil {
		return nil, err
	}
	var ts []Tuple
	if err := json.Unmarshal(b, &ts); err != nil {
		return nil, err
	}
	return ts, nil
}

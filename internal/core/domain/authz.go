// Package domain holds the pure authorization domain model for the benchmark.
// It has no knowledge of Cedar, SpiceDB, Postgres, or HTTP — those live in
// adapters, so engines can be swapped/compared behind the same ports.
package domain

// Decision is the outcome of an authorization check.
type Decision string

const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
)

// Model is one of the five benchmarked access-control models.
type Model string

const (
	ModelRBAC  Model = "rbac"
	ModelReBAC Model = "rebac"
	ModelABAC  Model = "abac"
	ModelPBAC  Model = "pbac"
	ModelACL   Model = "acl"
)

// Models lists every benchmarked model (stable order).
var Models = []Model{ModelRBAC, ModelReBAC, ModelABAC, ModelPBAC, ModelACL}

// Request is one authorization question: may this principal (a persona) perform
// this action on this resource? Context carries request-time attributes that are
// not stored on entities (e.g. PBAC amount/region; principal attrs for SpiceDB
// caveats, which cannot look up arbitrary subject attributes server-side).
type Request struct {
	Model        Model
	Principal    string // persona id
	Action       string // e.g. execute, view, render, doc.view, doc.read, po.approve, acl.view, acl.edit
	ResourceType string // e.g. Endpoint, Page, Component, RebacDocument, AbacDocument, PurchaseOrder, AclDocument
	Resource     string // resource id
	Context      map[string]any
}

// Result is the decision plus how many entities had to be loaded to reach it.
// EntitiesLoaded is meaningful for Cedar (the data-fetch work is on the PEP);
// it is always 0 for SpiceDB (the server owns its data).
type Result struct {
	Decision       Decision
	EntitiesLoaded int
}

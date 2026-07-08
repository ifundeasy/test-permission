// Package domain holds the pure authorization domain model. It has no knowledge
// of Cedar, Postgres, or HTTP — those live in adapters. This keeps the core
// independent so engines and data sources can be swapped without touching it.
package domain

// Decision is the outcome of an authorization check.
type Decision string

const (
	Allow Decision = "allow"
	Deny  Decision = "deny"
)

// Request is one authorization question: may this principal perform this action
// on this resource? Ids are opaque strings scoped by their well-known types
// (User / Action / Document) at the engine boundary.
type Request struct {
	Principal string
	Action    string
	Resource  string
}

// Result is the decision plus how many entities had to be loaded to reach it —
// the data-fetch work that is on the service, not on the policy engine.
type Result struct {
	Decision       Decision
	EntitiesLoaded int
}

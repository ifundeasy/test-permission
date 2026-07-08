package domain

// EntityUID identifies an entity by its type and id (e.g. {User, "alice"}).
type EntityUID struct {
	Type string
	ID   string
}

// Entity is a domain-neutral representation of the data a policy engine needs to
// decide a request: an entity, its attributes, and its parents (for hierarchy /
// group / role membership). This is the "glue" Cedar makes you build — the layer
// SpiceDB would own for you.
//
// Attribute values must be one of: bool, string, or EntityUID (an entity
// reference, e.g. a document's owner). The engine adapter maps these to the
// engine's own value types.
type Entity struct {
	UID        EntityUID
	Attributes map[string]any
	Parents    []EntityUID
}

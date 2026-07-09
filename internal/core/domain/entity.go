package domain

// EntityUID identifies an entity by its type and id (e.g. {Persona, "psn-000042"}).
type EntityUID struct {
	Type string
	ID   string
}

// Entity is a domain-neutral representation of the data an embedded policy
// engine needs to decide a request: an entity, its attributes, and its parents
// (hierarchy / role / policy membership). This is the "glue" an embedded engine
// makes the PEP build — the layer a server engine (SpiceDB) owns internally.
//
// Attribute values must be one of:
//
//	bool · string · int64 · EntityUID (entity reference) ·
//	[]string (set of strings) · []EntityUID (set of entity references)
//
// The engine adapter maps these to the engine's own value types.
type Entity struct {
	UID        EntityUID
	Attributes map[string]any
	Parents    []EntityUID
}

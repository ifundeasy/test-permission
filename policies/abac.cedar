// ===========================================================================
// ABAC — pure attribute comparison, no relationships.
//
// Principal attrs (clearance, division, region) and resource attrs
// (classification, division, status, region) are loaded from Postgres.
// Data-residency rule: the reader must be in the document's region, UNLESS
// they hold top clearance (4 — "national auditor" override). Archived
// documents are hard-forbidden (forbid overrides permit).
// Mirrors the SpiceDB `doc_attrs` caveat exactly.
// ===========================================================================

permit (
  principal,
  action == Action::"doc.read",
  resource is AbacDocument
) when {
  principal.clearance >= resource.classification &&
  principal.division == resource.division &&
  (principal.region == resource.region || principal.clearance == 4)
};

forbid (
  principal,
  action == Action::"doc.read",
  resource is AbacDocument
) when { resource.status == "archived" };

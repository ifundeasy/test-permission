// ===========================================================================
// ABAC — pure attribute comparison, no relationships.
//
// Principal attrs (clearance, division) and resource attrs (classification,
// division, status) are loaded from Postgres. Archived documents are hard-
// forbidden (forbid overrides permit). Mirrors the SpiceDB `doc_attrs` caveat.
// ===========================================================================

permit (
  principal,
  action == Action::"doc.read",
  resource is AbacDocument
) when {
  principal.clearance >= resource.classification &&
  principal.division == resource.division
};

forbid (
  principal,
  action == Action::"doc.read",
  resource is AbacDocument
) when { resource.status == "archived" };

// ===========================================================================
// ReBAC — relationship graph: document → folder → org unit → ancestor units.
//
// The document's Parents chain encodes the graph (doc→folder, folder→parent
// folder + owning OrgUnit, unit→parent unit). The persona's `member_of` is the
// set of OrgUnits where it is member or manager (unit_memberships rows).
// `resource in principal.member_of` is true when any of those units is an
// ancestor of the document — membership at a parent company grants visibility
// into subsidiaries' documents (same shape as SpiceDB's arrow inheritance).
// ===========================================================================

permit (
  principal,
  action == Action::"doc.view",
  resource is RebacDocument
) when { resource in principal.member_of };

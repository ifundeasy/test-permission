// ===========================================================================
// ACL — direct per-resource grants, no hierarchy, no attributes.
//
// The document's acl_entries rows are loaded as `viewers` / `editors` entity
// sets. Editors can also view (same as SpiceDB's `view = viewer + editor`).
// ===========================================================================

permit (
  principal,
  action == Action::"acl.view",
  resource is AclDocument
) when {
  (resource has viewers && resource.viewers.contains(principal)) ||
  (resource has editors && resource.editors.contains(principal))
};

permit (
  principal,
  action == Action::"acl.edit",
  resource is AclDocument
) when { resource has editors && resource.editors.contains(principal) };

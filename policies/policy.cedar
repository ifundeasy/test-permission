// ===========================================================================
// One Cedar policy file. Three access models, all in one syntax.
// Rules are evaluated together; default is DENY; a single `forbid` wins over
// any number of `permit`s.
// ===========================================================================

// 1) RBAC  — anyone with the "admin" role can do anything.
permit ( principal in Role::"admin", action, resource );

// 2) ReBAC — the owner of a resource can do anything to it.
permit ( principal, action, resource )
when { principal == resource.owner };

// 3) ReBAC + group — members of the "engineering" group may VIEW any document
//    that lives inside Folder "eng" (permission inherited via the folder graph).
permit (
  principal in Group::"engineering",
  action == Action::"view",
  resource in Folder::"eng"
);

// 4) ABAC  — confidential documents are blocked, UNLESS you are the owner.
//    Because forbid overrides permit, this beats even the admin rule above.
forbid ( principal, action, resource )
when   { resource.confidential == true }
unless { principal == resource.owner };

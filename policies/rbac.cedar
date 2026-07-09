// ===========================================================================
// RBAC — application-registry permissions (endpoints, UI pages, UI components).
//
// Data-driven: the persona's roles are its entity Parents (persona_roles rows);
// each registry resource carries `allowed_roles` (set of Role entity refs,
// from role_grants rows). Row growth never changes this policy text.
// `principal in resource.allowed_roles` is true when the principal descends
// from ANY role in the set.
// ===========================================================================

permit (
  principal,
  action == Action::"execute",
  resource is Endpoint
) when { principal in resource.allowed_roles };

permit (
  principal,
  action == Action::"view",
  resource is Page
) when { principal in resource.allowed_roles };

permit (
  principal,
  action == Action::"render",
  resource is Component
) when { principal in resource.allowed_roles };

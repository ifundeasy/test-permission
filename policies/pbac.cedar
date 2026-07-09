// ===========================================================================
// PBAC — org-defined policies as PARAMETER ENTITIES (cedar-go has no policy
// templates, and compiling 1M policy rows into policy text would be
// pathological). Each PbacPolicy row becomes an entity with attributes
// {active, max_amount, regions}; assignees carry the policy as a Parent;
// the purchase order references its governing policy via `resource.policy`.
// Request-time facts (amount, region) arrive in `context` — mirroring the
// SpiceDB `po_limits` caveat exactly (static params + check-time context).
// ===========================================================================

permit (
  principal,
  action == Action::"po.approve",
  resource is PurchaseOrder
) when {
  principal in resource.policy &&
  resource.policy.active &&
  context.amount <= resource.policy.max_amount &&
  resource.policy.regions.contains(context.region)
};

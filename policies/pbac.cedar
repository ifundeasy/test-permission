// ===========================================================================
// PBAC — org-defined policies as PARAMETER ENTITIES (cedar-go has no policy
// templates, and compiling 100k policy rows into policy text would be
// pathological). Each PbacPolicy row becomes an entity with attributes
// {active, min_amount, max_amount, regions}; assignees carry the policy as a
// Parent; the purchase order references its governing policy via
// `resource.policy`. Request-time facts (amount, region) arrive in `context`.
//
// The amount must sit INSIDE the policy's window: below min_amount is petty
// cash (no approval flow), above max_amount exceeds the approver's mandate.
// Mirrors the SpiceDB `po_limits` caveat exactly.
// ===========================================================================

permit (
  principal,
  action == Action::"po.approve",
  resource is PurchaseOrder
) when {
  principal in resource.policy &&
  resource.policy.active &&
  context.amount >= resource.policy.min_amount &&
  context.amount <= resource.policy.max_amount &&
  resource.policy.regions.contains(context.region)
};

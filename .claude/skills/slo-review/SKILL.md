---
description: Assess SLO/alerting/observability configuration for coverage and sane thresholds
argument-hint: [config path or service]
---
Review SLO/alerting/observability for `$ARGUMENTS`.

1. Locate SLO definitions, alert rules, dashboards, and instrumentation (metrics/traces/logs). If
   **none exist**, propose a minimal starting set (availability + latency SLIs, error-budget alert).
2. Check: SLIs map to user-facing behavior; alert thresholds are actionable (not noisy/flappy);
   critical paths are instrumented; runbooks are linked from alerts.
3. Flag gaps (unmonitored dependencies, missing saturation signals, no error budget).
4. Output findings with concrete config suggestions. Read-only.

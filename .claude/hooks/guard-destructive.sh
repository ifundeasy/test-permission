#!/usr/bin/env bash
# Hard-stop destructive infra/data commands. Speed bump, not a boundary (see block-secrets.sh) —
# pair with permissions.deny for real enforcement. Register under PreToolUse matcher "Bash".
set -euo pipefail
command -v jq >/dev/null || { echo "jq required for this hook" >&2; exit 0; }
cmd="$(jq -r '.tool_input.command // empty')"
if printf '%s' "$cmd" | grep -Eiq '(terraform[[:space:]]+(apply|destroy)|kubectl[[:space:]]+(delete|apply)|helm[[:space:]]+(upgrade|uninstall|delete)|(gcloud|aws)[[:space:]].*(delete|destroy)|docker[[:space:]]+(compose[[:space:]]+down|rm|rmi|volume[[:space:]]+(rm|prune)|system[[:space:]]+prune)|dropdb|rm[[:space:]]+-rf)'; then
  echo "Blocked: destructive command. Run it yourself outside Claude if intended (data loss possible)." >&2
  exit 2
fi
exit 0

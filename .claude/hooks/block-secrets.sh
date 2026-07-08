#!/usr/bin/env bash
# Speed bump, not a security boundary — grep matching is bypassable (vars, base64, alt syntax).
# The real boundary is permissions.deny in settings.json. Register under PreToolUse matcher "Bash".
set -euo pipefail
command -v jq >/dev/null || { echo "jq required for this hook" >&2; exit 0; }
cmd="$(jq -r '.tool_input.command // empty')"
if printf '%s' "$cmd" | grep -Eiq '(\.env([^a-z.]|$)|\.pem|\.key|id_rsa|credentials\.json|kubeconfig)'; then
  echo "Blocked: command appears to access a credential file." >&2
  exit 2
fi
exit 0

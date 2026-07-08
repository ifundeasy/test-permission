#!/usr/bin/env bash
# Auto-format edited files if a formatter is installed. No-op otherwise (safe). Register under
# PostToolUse matcher "Edit|Write". Add Prettier to the repo to make the JS/JSON/MD arm active.
set -euo pipefail
command -v jq >/dev/null || exit 0
file="$(jq -r '.tool_input.file_path // empty')"; [ -z "$file" ] && exit 0
case "$file" in
  *.go)
    command -v gofmt >/dev/null && gofmt -w "$file" || true ;;
  *.ts|*.tsx|*.js|*.jsx|*.json|*.md)
    command -v prettier >/dev/null && prettier --write "$file" || true ;;
esac
exit 0

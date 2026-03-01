#!/usr/bin/env bash
# L4 verification runner — checks that visual test evidence exists for a task
set -euo pipefail

TASK_FILE="${1:-}"
if [[ -z "$TASK_FILE" ]]; then
  echo "Usage: $0 <task-file>"
  exit 1
fi

ERRORS=()

# 1. Check L4 scripts exist
SCRIPTS=$(grep -oP '\.claude/automations/test_dootsabha_\w+\.py' "$TASK_FILE" 2>/dev/null || true)
for script in $SCRIPTS; do
  [[ -f "$script" ]] || ERRORS+=("L4 script missing: $script")
done

# 2. Check screenshots exist (match by prefix)
SCREENSHOTS=$(grep -oP 'dootsabha_\w+\.png' "$TASK_FILE" 2>/dev/null || true)
for shot in $SCREENSHOTS; do
  prefix="${shot%.png}"
  found=$(find .claude/screenshots -name "${prefix}*" 2>/dev/null | head -1)
  [[ -n "$found" ]] || ERRORS+=("Screenshot missing: $shot")
done

# 3. Check Visual Test Results section exists with content
if ! grep -q '^## Visual Test Results' "$TASK_FILE"; then
  ERRORS+=("Missing '## Visual Test Results' section in task file")
elif [[ $(sed -n '/^## Visual Test Results/,/^## /p' "$TASK_FILE" | wc -l) -lt 5 ]]; then
  ERRORS+=("Visual Test Results section is too thin (needs actual findings)")
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo "❌ L4 GATE FAILED for $(basename "$TASK_FILE"):"
  for err in "${ERRORS[@]}"; do echo "  • $err"; done
  exit 2
fi
echo "✓ L4 gate passed for $(basename "$TASK_FILE")"

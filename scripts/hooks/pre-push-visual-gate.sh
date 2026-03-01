#!/usr/bin/env bash
# Claude Code PreToolUse hook: blocks git push if any IN PROGRESS task fails L4
ERRORS=()
for task in docs/tasks/*.md; do
  [[ -f "$task" ]] || continue
  if grep -q 'Status:.*IN PROGRESS' "$task"; then
    if ! bash scripts/verify-visual-tests.sh "$task" 2>/dev/null; then
      ERRORS+=("$task")
    fi
  fi
done
if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo "❌ PRE-PUSH GATE: L4 requirements unmet for:"
  for task in "${ERRORS[@]}"; do echo "  • $task"; done
  exit 2
fi

#!/usr/bin/env bash
# Claude Code PreToolUse hook: blocks DONE status on task files without L4 evidence
# Arguments: $1=tool (unused) $2=file $3=new-content-snippet
FILE="${2:-}"

# Only intercept task file edits
[[ "$FILE" == *docs/tasks/*.md ]] || exit 0

# Check if edit changes status to DONE
if grep -qi 'Status:.*DONE' <<< "${3:-}" 2>/dev/null; then
  bash scripts/verify-visual-tests.sh "$FILE"
fi

#!/usr/bin/env bash
# L4 gate — a task may not reach DONE without real visual evidence.
#
# L4 is shux (CLAUDE.md): a committed capture script under .shux/scripts/ that
# rasterises a real PTY headless. Frames live in .shux/out/, which is gitignored,
# so this gate verifies the SCRIPT — the reproducible part — not the PNGs.
#
# Uses grep -oE, not -oP: BSD grep on macOS has no -P, and the previous version
# paired -oP with `|| true`, so every check it performed was silently skipped.
set -euo pipefail

TASK_FILE="${1:-}"
if [[ -z "$TASK_FILE" ]]; then
  echo "Usage: $0 <task-file>"
  exit 1
fi

ERRORS=()

# The section itself must exist and carry content.
if ! grep -q '^## Visual Test Results' "$TASK_FILE"; then
  ERRORS+=("Missing '## Visual Test Results' section")
else
  SECTION=$(sed -n '/^## Visual Test Results/,$p' "$TASK_FILE")
  if [[ $(wc -l <<<"$SECTION") -lt 5 ]]; then
    ERRORS+=("Visual Test Results section is too thin (needs actual findings)")
  fi

  # A cloud session has no provider CLIs, so L4 cannot be captured there. Saying
  # so is allowed — inventing evidence is not — but it must name a reason.
  if grep -qiE '^\s*_?N/A\b|not applicable|no provider CLIs' <<<"$SECTION"; then
    if [[ $(wc -w <<<"$SECTION") -lt 12 ]]; then
      ERRORS+=("L4 marked N/A without a reason — say why it could not be captured")
    fi
  else
    # Otherwise the section must name a capture script, and it must exist.
    SCRIPTS=$(grep -oE '\.shux/scripts/[A-Za-z0-9._-]+\.sh' <<<"$SECTION" | sort -u || true)
    if [[ -z "$SCRIPTS" ]]; then
      ERRORS+=("No .shux/scripts/*.sh capture script referenced — L4 is shux (CLAUDE.md)")
    fi
    for script in $SCRIPTS; do
      [[ -f "$script" ]] || ERRORS+=("Capture script missing: $script")
    done
  fi
fi

if [[ ${#ERRORS[@]} -gt 0 ]]; then
  echo "❌ L4 GATE FAILED for $(basename "$TASK_FILE"):"
  for err in "${ERRORS[@]}"; do echo "  • $err"; done
  exit 2
fi
echo "✓ L4 gate passed for $(basename "$TASK_FILE")"

#!/usr/bin/env bash
# Demonstrates branch-guard.sh in a throwaway repo, for L4 frame 02.
#
# A standalone script, not an inline snippet: the first version of this lived
# inside a `bash -lc "..."` inside a shux frame, and the nested quoting silently
# broke the comparison so every row printed "allowed" — including the case that
# demonstrably blocks. A frame that states the opposite of the truth is worse than
# no frame.
set -euo pipefail

GUARD="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/.claude/hooks/branch-guard.sh"
T=$(mktemp -d)
trap 'rm -rf "$T"' EXIT
cp "$GUARD" "$T/guard.sh"
cd "$T"

git init -q -b main .
echo tracked > tracked.md
git add tracked.md
git -c user.email=t@t -c user.name=t commit -qm init
echo untracked > untracked.md

# verdict <file> — run the guard exactly as Claude Code would, print the outcome.
verdict() {
  local rc=0
  python3 -c 'import json,sys; print(json.dumps({"tool_name":"Edit","tool_input":{"file_path":sys.argv[1]}}))' "$1" \
    | sh ./guard.sh >/dev/null 2>&1 || rc=$?
  [[ "$rc" -eq 2 ]] && echo "BLOCKED" || echo "allowed"
}

printf '%-10s %-16s %s\n' branch file verdict
printf '%.0s─' {1..40}; echo
printf '%-10s %-16s %s\n' main   tracked.md   "$(verdict tracked.md)"
printf '%-10s %-16s %s\n' main   untracked.md "$(verdict untracked.md)"
git checkout -q -b feat/x
printf '%-10s %-16s %s\n' feat/x tracked.md   "$(verdict tracked.md)"

echo
echo 'what it says when it blocks:'
git checkout -q main
python3 -c 'import json,sys; print(json.dumps({"tool_name":"Edit","tool_input":{"file_path":"tracked.md"}}))' \
  | sh ./guard.sh 2>&1 | head -4 || true

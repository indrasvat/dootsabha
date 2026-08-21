#!/usr/bin/env sh
# PreToolUse(Edit|Write) — refuse to modify a TRACKED file on the default branch.
#
# The only things that land on main are merged PRs and annotated tags. This is the
# rule agents break most easily, because editing feels harmless until the commit.
#
# FAILS OPEN by design. A guard that misfires does not stop an agent, it teaches
# the agent to work around it — so anything it cannot determine (no git, no
# python3, detached HEAD, shallow clone with no origin/HEAD) is allowed through.
# Missing a violation is cheaper than blocking legitimate work.
#
# Input: Claude Code hook JSON on stdin. Exit 2 = block; exit 0 = no decision.
set -u

INPUT=$(cat 2>/dev/null || true)
[ -n "$INPUT" ] || exit 0

command -v git >/dev/null 2>&1 || exit 0
command -v python3 >/dev/null 2>&1 || exit 0

FILE=$(printf '%s' "$INPUT" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
ti = d.get("tool_input") or {}
print(ti.get("file_path") or ti.get("notebook_path") or "")
' 2>/dev/null) || exit 0
[ -n "$FILE" ] || exit 0

BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null) || exit 0
[ -n "$BRANCH" ] && [ "$BRANCH" != "HEAD" ] || exit 0

# Default branch: ask the remote, then the local config, then assume the usual
# names. A cloud checkout often has no origin/HEAD, hence the fallbacks.
DEFAULT=$(git symbolic-ref --quiet --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||')
[ -n "${DEFAULT:-}" ] || DEFAULT=$(git config --get init.defaultBranch 2>/dev/null)
if [ -z "${DEFAULT:-}" ]; then
    case "$BRANCH" in
        main | master) DEFAULT="$BRANCH" ;;
        *) exit 0 ;;
    esac
fi

[ "$BRANCH" = "$DEFAULT" ] || exit 0

# Untracked files are fine: creating scratch output on the default branch breaks
# nothing. Only a file already under version control is protected.
git ls-files --error-unmatch -- "$FILE" >/dev/null 2>&1 || exit 0

cat >&2 <<EOF
Refusing to edit a tracked file on '$BRANCH', the default branch.

  file: $FILE

Branch first, then edit — the working-tree changes follow:

  git checkout -b fix/<slug>       # bug fixes
  git checkout -b feat/<slug>      # features
  git checkout -b chore/<slug>     # tooling / deps / docs
  git checkout -b refactor/<slug>  # non-behavioral changes
EOF
exit 2

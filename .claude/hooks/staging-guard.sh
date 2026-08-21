#!/usr/bin/env sh
# PreToolUse(Bash) — refuse bulk staging.
#
# `git add -A` / `.` / `-u` and `git commit -a` sweep in build artifacts, secrets
# and scratch files, and a committed binary is invisible in review. Naming every
# path also keeps commits small: a long stage list means the commit is too big.
#
# dcg guards `git push --force` but ALLOWS `git add -A`, so nothing covered this.
#
# FAILS OPEN: anything it cannot parse is allowed through. Exit 2 = block.
set -u

INPUT=$(cat 2>/dev/null || true)
[ -n "$INPUT" ] || exit 0
command -v python3 >/dev/null 2>&1 || exit 0

VERDICT=$(printf '%s' "$INPUT" | python3 -c '
import json, re, shlex, sys

try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
if d.get("tool_name") != "Bash":
    sys.exit(0)
cmd = (d.get("tool_input") or {}).get("command") or ""

# Split on shell separators so `cd x && git add -A` is caught too.
for part in re.split(r"&&|\|\||;|\n|\|", cmd):
    try:
        tok = shlex.split(part)
    except ValueError:
        continue
    if len(tok) < 2:
        continue
    # `git` must BE the command, not merely appear in it: `echo git add -A` is
    # talking about the command, not running it, and blocking that is the kind of
    # false positive that teaches an agent to route around the guard.
    j = 0
    while j < len(tok) and re.fullmatch(r"[A-Za-z_][A-Za-z0-9_]*=.*", tok[j]):
        j += 1  # leading VAR=value assignments
    if j >= len(tok) or tok[j] != "git":
        continue
    rest = tok[j + 1:]
    # Skip global options like -C <dir> to reach the subcommand.
    i = 0
    while i < len(rest) and rest[i].startswith("-"):
        i += 2 if rest[i] in ("-C", "-c", "--git-dir", "--work-tree") else 1
    if i >= len(rest):
        continue
    sub, args = rest[i], rest[i + 1:]

    if sub == "add":
        after_sep = False
        for a in args:
            if a == "--":
                # Everything after `--` is a pathspec, so `-A` there names a FILE
                # and is harmless. `.` still means "stage everything", and
                # `git add -- .` is a common spelling that slipped past a loop
                # that simply stopped here.
                after_sep = True
                continue
            if after_sep:
                if a == ".":
                    print("git add -- . stages everything, including files you have not looked at.")
                    sys.exit(0)
                continue
            if a in ("-A", "--all", "-u", "--update", "."):
                print("git add %s stages everything, including files you have not looked at." % a)
                sys.exit(0)
            # Bundled short flags such as -Av
            if re.fullmatch(r"-[A-Za-z]+", a) and ("A" in a[1:] or "u" in a[1:]):
                print("git add %s stages everything, including files you have not looked at." % a)
                sys.exit(0)
    elif sub == "commit":
        for a in args:
            if a == "--":
                break
            if a in ("-a", "--all"):
                print("git commit %s stages every tracked change, bypassing review of what is going in." % a)
                sys.exit(0)
            if re.fullmatch(r"-[A-Za-z]+", a) and "a" in a[1:]:
                print("git commit %s stages every tracked change, bypassing review of what is going in." % a)
                sys.exit(0)
' 2>/dev/null) || exit 0

[ -n "${VERDICT:-}" ] || exit 0

cat >&2 <<EOF
Refusing bulk staging.

  $VERDICT

Name every path instead, then check what you are about to commit:

  git add path/one.go path/two.md
  git diff --cached --stat
EOF
exit 2

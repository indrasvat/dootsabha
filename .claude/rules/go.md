---
paths:
  - "**/*.go"
---

# Go conventions

Only rules the linter does **not** already enforce. `make ci` runs
golangci-lint v2.9.0 and gofumpt; do not hand-check what they check.

- Wrap every returned error with context: `fmt.Errorf("doing x: %w", err)`.
- Subprocesses use `exec.Command`, **never** `exec.CommandContext` —
  CommandContext sends SIGKILL immediately and skips the
  SIGTERM→grace→SIGKILL sequence `internal/core/subprocess.go` implements.
- TTY detection is `isatty.IsTerminal(os.Stdout.Fd())`.
- No `huh` dependency, and nothing needs one. If a form is ever required, add it
  deliberately — and note it has no standalone spinner (v0.8.0 dropped
  `NewSpinner`); write a raw goroutine on stderr instead.
- Truncate user-visible text with `core.TruncateString`, which cuts on a rune
  boundary. A byte-offset cut strands a lead byte and emits invalid UTF-8 into
  another agent's prompt.

## Provider argv is a contract

A provider must pin the flags it sets itself and strip every spelling of them
from user config, or a config entry silently changes what ran while दूतसभा
reports something else. What "every spelling" means depends on the CLI's parser:

- `agy` uses Go's stdlib `flag` — `-model` **is** `--model`, repeats are
  last-wins, and parsing **stops at the first non-flag token** (so the prompt is
  emitted before user flags). See `agyPinned` in `internal/providers/agy.go`.
- `grok` is clap-based — it also accepts the attached short form `-mgrok-9`.
  See `matchPinned` in `internal/providers/grok.go`.

Verify against the real binary before assuming either.

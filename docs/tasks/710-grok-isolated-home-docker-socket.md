# Task 710: Keep grok's read-only sandbox working under the isolated HOME

## Status: DONE

## Depends On
704 (grok provider, isolated HOME). Closes #31.

## Problem

On macOS with Docker Desktop's link in place (`/var/run/docker.sock ->
~/.docker/run/docker.sock`), every grok call through दूतसभा fails before it starts:

```
warning: sandbox could not be applied: socket deny resolution failed: could not
resolve runtime-socket deny path /var/run/docker.sock: endpoint is a symlink to
a path outside the deny list
error: could not apply the 'read-only' sandbox profile; … Refusing to start
```

`grokIsolatedEnv` points `HOME` at an empty temp dir so grok cannot auto-load the
caller's `~/.claude/**`. grok's read-only profile builds part of its
runtime-socket deny list from `$HOME`, and it refuses a socket link whose target
is outside that list. With the swapped HOME, the real `~/.docker/...` target is
outside it.

## PRD Ref
§5.4 (grok provider)

## Real-CLI findings (grok 1.0.70, macOS)

Reproduced exactly as reported: real HOME → `ok`; empty HOME → the error above, exit 1.

- **The deny list is hard-coded** (binary strings): `/var/run/docker.sock`,
  podman/containerd sockets, and two **home-relative** entries,
  `~/.docker/run/docker.sock` and `~/.docker/desktop/docker.sock`. No setting
  moves the anchor; it is `$HOME`.
- **Real HOME + grok's compat switches is not a fix.** `GROK_CLAUDE_*_ENABLED=false`
  / `GROK_CURSOR_*_ENABLED=false` are honoured, but nothing gates
  `~/.claude/plugins/**` or `~/.claude/agents/`. Measured in a headless run's
  `init` event: MCP servers 6 → **2** (both from Claude plugins), skills 187 →
  149, input tokens 32.8k → 24.6k. The empty HOME gives MCP 0.
- **Linking only `~/.docker` into the isolated HOME fixes it**: `ok`, MCP **0**,
  skills 50 (grok's own), input tokens 15.2k.
- **Nothing is weakened.** Under the read-only profile grok can read the whole
  disk anyway; the isolated HOME stops *discovery*, not access. Two behavioural
  checks, real HOME vs linked HOME, identical results:
  - write to the working directory → `Operation not permitted` in both;
  - `connect()` to a live socket via `/var/run/docker.sock` → reached in **both**.
    grok's macOS read-only profile does not block Unix-socket connects even with
    the real HOME. That is upstream behaviour, not something this change alters.

## Design

When the private HOME is created (once per process), link each of
`<real HOME>/.docker/run` and `<real HOME>/.docker/desktop` that exists into it
— the two home-relative entries of grok's deny list, and nothing else. The rest
of `~/.docker` (`config.json` credential helpers, the `mcp` toolkit) stays out
of grok's HOME, and grok writes under `$HOME/.docker` still land in the
throwaway dir.

What the caller's side cannot offer — no such dir, an unsearchable or relative
HOME — is skipped: before #31 none of those failed a grok call, and grok itself
names a missing anchor. Failing to write inside the dir this process just
created is a fault and errors.

Rejected:
- **Link the whole `~/.docker`** (first cut) — exposes config.json and writes
  through to the real dir (adversarial finding).
- **Real HOME + grok's compat switches** — leaks Claude plugins (above).
- **Mirror the real home minus a harness denylist** — a new vendor dir would
  leak silently. If a future grok adds another home-relative socket, grok
  refuses to start with this same named error: fail closed.

## Files

| File | Change |
|---|---|
| `internal/providers/grok.go` | link `.docker/run`, `.docker/desktop` into the isolated HOME |
| `internal/providers/grok_internal_test.go` | pin the env against this case |
| `.shux/scripts/grok-710-evidence.sh` | L4 before/after with the real CLI |

## Done Criteria

- [x] `consult --agent grok` succeeds with `/var/run/docker.sock` linked into the real home
- [x] read-only sandbox still applied (pinned flag unchanged; cwd write refused)
- [x] grok still sees no `~/.claude/**` (MCP 0)
- [x] test pins the env handed to grok for this case
- [x] L1+L2+L3 green (`make check`: 0 lint issues, L3 40/40), L4 evidence read back

## Review

Adversarial pass (4 narrow agents) + a codex/grok council through this
branch's own binary. Fixed: an unsearchable or relative HOME failing every call
(now skipped); whole-`~/.docker` link exposing `config.json` and writing through
(narrowed to the two socket dirs); a file or dangling link at
`.docker/desktop` costing the valid `run` (now carried only if a directory).

Declined: grok refusing a carried dir whose `docker.sock` links outside its list.
grok refuses the same with the real HOME; the private HOME mirrors the real
socket dirs rather than re-implementing grok's resolution rules.

## Visual Test Results

Captured with `.shux/scripts/grok-710-evidence.sh` — real `grok` 1.0.70, real
calls, no mocks. BEFORE is the installed v0.10.1, AFTER is `bin/dootsabha` at
`b37310f`. Frames in `.shux/out/710/` (gitignored; attached to the PR), each
read back:

| Frame | Shows |
|---|---|
| `01-premise` | `/var/run/docker.sock -> ~/.docker/run/docker.sock`, grok 1.0.70 |
| `02-before` | `dootsabha --version` v0.10.1, then grok refusing the read-only profile, exit `3` |
| `03-after` | `bin/dootsabha --version` at `b37310f`, same call answers `PONG`, exit `0` |
| `04-isolated-home` | after a real call, every entry of grok's HOME: only `.docker/run -> ~/.docker/run`, plus `Library/Caches` bytecode its tools wrote |
| `05-read-only` | the prompt, bash's raw `Operation not permitted`, and no file on disk |
| `06-council` | `council --agents claude,codex,grok`: `meta.providers` all `ok`, exit `0` |

# Task 710: Keep grok's read-only sandbox working under the isolated HOME

## Status: IN PROGRESS

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
| `internal/providers/grok.go` | link `.docker` into the isolated HOME |
| `internal/providers/grok_internal_test.go` | pin the env against this case |
| `.shux/scripts/grok-710-evidence.sh` | L4 before/after with the real CLI |

## Done Criteria

- [ ] `consult --agent grok` succeeds with `/var/run/docker.sock` linked into the real home
- [ ] read-only sandbox still applied (pinned flag unchanged; cwd write refused)
- [ ] grok still sees no `~/.claude/**` (MCP 0)
- [ ] test pins the env handed to grok for this case
- [ ] L1+L2+L3 green, L4 evidence read back

## Visual Test Results

Pending.

---
paths:
  - "internal/plugin/**"
  - "plugins/**"
  - "proto/**"
---

# Plugins, extensions, proto

gRPC over `hashicorp/go-plugin`. Handshake cookies are a published contract with
third-party plugin authors — changing one breaks every external plugin:

| Kind | Cookie |
|---|---|
| Provider | `dootsabha-provider-v1` |
| Strategy | `dootsabha-strategy-v1` |
| Hook | `dootsabha-hook-v1` |

- Extensions are `dootsabha-{name}` binaries on `$PATH` or `~/.local/bin`;
  **user-local wins**. Their Tier-2 context is JSON at `$DOOTSABHA_CONTEXT_FILE`.
- `internal/plugin/context_file.go` is a **live path**, not sample data —
  extensions read what it emits. A model default written there must read the
  provider constant, never a literal, or it drifts silently.
- A plugin's advertised `Capabilities` is what a host reads before dispatching
  and nothing else asserts it. When provider behaviour changes (JSON support,
  supported models), update the capabilities and their test together.
- Relaunch a crashed plugin; do not retry the call. Kill plugins explicitly on
  shutdown — `make test-plugins` asserts no orphans survive.

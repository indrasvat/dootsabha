# Task 600: `dootsabha recap` Extension Showcase

## Status: DONE

## Depends On
Phase 3 (extension discovery), Phase 4 (Tier 2 context file)

## Parallelizable With
None (showcase task)

## Problem
Demonstrate how easy it is to create extensions for dootsabha. A self-contained
Python script that adds a new `recap` subcommand using ALL Tier 2 context metadata.
Also updates extension discovery to scan `~/.local/bin` and `/usr/local/bin`.

## PRD Ref
§9.3 (Extension Discovery), §9.4 (Tier 2 Context)

## Files
| File | Action |
|------|--------|
| `internal/plugin/extension.go` | Add `ExtensionDirs()` func |
| `internal/plugin/extension_test.go` | Add L2 tests for `ExtensionDirs()` |
| `internal/cli/root.go` | Wire `ExtensionDirs()` into `FindExtension` call |
| `internal/cli/plugin_cmd.go` | Wire `ExtensionDirs()` into `DiscoverExtensions` call |
| `examples/extensions/dootsabha-recap` | Create Python extension script |
| `.claude/automations/test_dootsabha_recap.py` | L4 visual test |

## Steps
1. Add `ExtensionDirs()` to `extension.go` returning `[~/.local/bin, /usr/local/bin]`
2. Wire `ExtensionDirs()` into `root.go` and `plugin_cmd.go` call sites
3. Add L2 tests for `ExtensionDirs()`
4. Create `examples/extensions/dootsabha-recap` Python script with `rich`
5. Symlink to `~/.local/bin/dootsabha-recap`
6. Verify: `dootsabha recap`, piped, plugin list, standalone
7. Write L4 visual test
8. Run L4 tests

## Verification
- `make check` passes
- `dootsabha recap` shows full briefing with provider matrix
- `dootsabha recap | cat` shows plain text (no ANSI)
- `dootsabha plugin list` shows `recap` as extension
- Standalone `./dootsabha-recap` works in degraded mode
- L4 visual tests: 10/10 pass

## Criteria
- All Tier 2 context fields used (version, session_id, workspace, providers, capabilities, tty, terminal_width)
- Rich output matches Go CLI aesthetic (colors, box chars, dividers)
- Graceful degradation: TTY → piped → standalone

## Commit
`feat(extension): add recap workspace briefing extension with enhanced discovery`

## Session Protocol
1. Read CLAUDE.md, PROGRESS.md, this file
2. Mark IN PROGRESS
3. Implement + test
4. `make check` must pass
5. L4 visual tests pass
6. Mark DONE

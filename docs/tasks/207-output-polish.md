# Task 207: Output Polish — Professional CLI Rendering

## Status: DONE

## Depends On
- 2.4 (Review Command)
- 2.6 (Refine Command)

## Parallelizable With
- None (touches all command output)

## Problem

Current CLI output looks unpolished: no visual separation between pipeline stages and content, headers use heavy `═══` lines, footers have inconsistent formatting, and progress steps lack provider-colored dots. Need unified professional rendering across all commands.

## PRD Ref
- §8.2 Provider Colors
- §8.3 Degradation Rules

## Files
- `internal/output/styles.go` — New helpers: CommandHeader, SectionDivider, ContentSeparator, FooterMetrics
- `internal/output/styles_test.go` — Tests for new helpers (TTY, piped, NO_COLOR, alignment)
- `internal/cli/refine.go` — CommandHeader, provider dots, ContentSeparator, FooterMetrics
- `internal/cli/council.go` — CommandHeader, SectionDivider, provider dots, ContentSeparator, FooterMetrics
- `internal/cli/review.go` — CommandHeader, SectionDivider, FooterMetrics
- `internal/cli/consult.go` — ContentSeparator, FooterMetrics

## Visual Changes

| Element | Before | After |
|---------|--------|-------|
| Header | `═══ Cmd ═══ info` | `┌ Cmd ──────┐ │ info │ └──────┘` |
| Stage divider | `═══ Stage N: Label ═══` | `── Label ── info ─────` |
| Progress dots | None | Provider-colored `●` per step |
| Content separator | None | `────────────` between progress and content |
| Footer | `total: Ns │ cost: $X │ tokens: ...` | `  Ns │ $X │ ... in · ... out` |

## Pipe Degradation

| Element | TTY + Color | TTY + NO_COLOR | Piped |
|---------|-------------|----------------|-------|
| Header box | Rounded + AccentColor | Rounded plain | Name + info plain |
| Section divider | `── Label ──` colored | `── Label ──` plain | `--- Label ---` |
| Provider dot | `●` colored | `*` | `*` |
| Content separator | `──────` MutedColor | `──────` plain | omitted |
| Footer metrics | MutedColor text | plain text | plain text |

## Steps
1. Add 4 helpers to `internal/output/styles.go`
2. Add tests in `internal/output/styles_test.go`
3. Update refine.go rendering
4. Update council.go rendering
5. Update review.go rendering
6. Update consult.go rendering

## Verification
```bash
make ci                # L1+L2 pass
make test-binary       # L3 pass
# L4: visual tests via iTerm2 driver
uv run .claude/automations/test_dootsabha_polish.py
```

## Criteria
- [x] All 4 commands render CommandHeader with rounded border (TTY)
- [x] Provider dots appear in progress steps (refine, council)
- [x] ContentSeparator between progress and content
- [x] FooterMetrics with pipe-delimited format
- [x] Piped output: no ANSI, no box chars, no separator
- [x] `make ci` passes
- [x] L4 screenshots verify alignment

## Commit
`feat(output): professional CLI rendering with rounded headers, section dividers, and provider dots`

## Session Protocol
1. Read CLAUDE.md
2. Read docs/PROGRESS.md
3. Read this file
4. Mark IN PROGRESS
5. Implement + test
6. `make ci` + L4
7. Mark DONE with evidence

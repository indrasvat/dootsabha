# Task 0.6: Cobra Alias Behavior Spike

## Status: PENDING

## Depends On
- None

## Parallelizable With
- All other spikes (0.1–0.5, 0.7–0.8)

## Problem

दूतसभा uses bilingual command names (English + Sanskrit). Cobra's `Aliases` field handles command aliases, but we must verify: help text rendering with aliases, flag aliases, unknown command handler for extension discovery, and any gotchas with non-ASCII alias names.

## PRD Reference
- §1.2 (Design principle: bilingual UX)
- §6.1 (Root command: unknown command → extension discovery)
- §6.2–§6.7 (All commands have Sanskrit aliases)

## Files to Create
- `_spikes/cobra-alias/main.go` — Spike program
- `_spikes/cobra-alias/README.md` — Findings doc

## Execution Steps

### Step 1: Read context
1. Read PRD §6.1 (root command, unknown command handler)
2. Read PRD §1.2 (bilingual UX principle)

### Step 2: Write spike program
- Create Cobra root command with 3 subcommands, each with an alias
- Test: `council` and `sabha` both work
- Test: `--help` shows both names (e.g., `council (sabha)`)
- Test: unknown command triggers custom RunE handler
- Test: flag aliases (e.g., `--agent` and `--doota`)

### Step 3: Test edge cases
- Non-ASCII alias rendering in help text
- Tab completion with aliases
- Unknown command with closest match suggestion
- `cobra.EnablePrefixMatching` behavior with aliases

### Step 4: Document findings
- Alias rendering in help text (does Cobra show aliases?)
- Flag alias mechanism (cobra vs pflag)
- Unknown command hook mechanism
- Recommendations for production

## Verification

### L1: Spike runs
```bash
cd _spikes/cobra-alias && go run main.go --help
go run main.go council
go run main.go sabha  # Should work identical to council
go run main.go unknown-cmd  # Should trigger extension discovery
```

## Completion Criteria

1. Command aliases work (council/sabha invoke same handler)
2. Help text shows bilingual names
3. Unknown command handler works for extension discovery
4. Flag aliases verified
5. README.md with patterns and gotchas

## Commit

```
spike(cobra-alias): validate bilingual aliases and extension discovery

- Command aliases via Cobra Aliases field
- Help text rendering with bilingual names
- Unknown command handler for extension discovery
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §1.2, §6.1
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit

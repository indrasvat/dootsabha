# Task 5.5: Final Acceptance (Clean Install, L5, SKILL Test)

## Status: DONE

## Depends On
- Task 5.3 (SKILL), Task 5.4 (build & release)

## Parallelizable With
- None (final task — comprehensive verification)

## Problem

Final acceptance: clean install from release binary, full L5 acceptance pass, SKILL enables agent discovery, README quick start works end-to-end. This is the ship gate.

## PRD Reference
- §9 (Phase 5 gate: clean install, README, SKILL, L5)
- §10 (Testing strategy — all layers)

## Also Read
- `testing-strategy.md §5` (Anti-hallucination rules — all 10)
- `testing-strategy.md §6` (Task verification checklist)

## Execution Steps

### Step 1: Clean install test
- Download release binary (or `make build` from clean checkout)
- Install: `make install` (symlink into gh extensions)
- Verify: `dootsabha --version` shows correct version

### Step 2: README quick start verification
- Copy-paste every command from README into fresh terminal
- Each must produce expected output
- Screenshots match README screenshots

### Step 3: Full L5 acceptance
```bash
make test-all
```
- L1 (ci-fast), L2 (test), L3 (test-binary), L4 (test-visual), L5 (test-agent)
- All must pass

### Step 4: SKILL test
- Start Claude Code session
- Invoke दूतसभा via SKILL
- Verify agent can: consult, council, check status, parse JSON

### Step 5: Final checklist
- [ ] `dootsabha --version` works
- [ ] `dootsabha status` shows all providers
- [ ] `dootsabha consult "PONG"` works for all 3 agents
- [ ] `dootsabha council "PONG"` produces 3-stage output
- [ ] `dootsabha review "PONG"` works
- [ ] `dootsabha plugin list` shows plugins
- [ ] `--json` produces valid JSON for all commands
- [ ] No ANSI in piped output for any command
- [ ] Exit codes correct for all paths
- [ ] SKILL enables agent discovery
- [ ] README quick start works end-to-end

## Verification

### Full pyramid
```bash
make test-all
```

### Clean install
```bash
make clean build install
dootsabha --version
dootsabha status
```

## Completion Criteria

1. Clean install works
2. README quick start copy-pasteable
3. Full `make test-all` passes
4. SKILL enables agent discovery
5. All checklist items verified
6. `make ci` passes

## Commit

```
chore(acceptance): final acceptance pass — v0.1.0 ready

- Clean install verified
- Full L5 acceptance suite passing
- SKILL tested with Claude Code
- README quick start verified end-to-end
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §9 (Phase 5 gate)
5. Read testing-strategy.md §5, §6
6. Execute steps 1-5
7. Run verification (full pyramid + clean install)
8. Fill Visual Test Results section
9. **Change status to `DONE`**
10. Update `docs/PROGRESS.md`
11. Commit

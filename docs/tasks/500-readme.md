# Task 5.1: README (Hero, Quick Start, Screenshots)

## Status: DONE

## Depends On
- Phase 4 complete

## Parallelizable With
- Task 5.2 (default config + embedded docs)

## Problem

दूतसभा needs a README that serves as the hero page: what it does, quick start (copy-pasteable), screenshots of actual terminal output, and extension development guide.

## PRD Reference
- §1 (Vision & philosophy)
- §2 (Problem statement)
- §6 (All commands — for usage examples)

## Files to Create
- `README.md` — Full README with hero, quick start, screenshots, extension guide

## Execution Steps

### Step 1: Write README structure
- Hero section: one-line vision, badge, screenshot
- Quick start: install, first consult, first council
- Commands reference (brief — point to `--help` for details)
- Extension development guide
- Configuration guide

### Step 2: Capture screenshots
- `dootsabha consult "What is a goroutine?"` — styled output
- `dootsabha council "..." --agents claude,codex,gemini` — 3-stage output
- `dootsabha status` — health table
- Use iTerm2-driver for consistent screenshots

### Step 3: Verify quick start
- Copy-paste every command from README into fresh terminal
- Verify each produces expected output

### Step 4: Write extension guide
- How to create a `dootsabha-{name}` extension
- Context tiers (env vars, context file, core callback)
- Example extension

## Verification

### L3: Quick start works
```bash
# Every command in README quick start section must work
make build
./bin/dootsabha --version
./bin/dootsabha status
./bin/dootsabha consult "Say PONG"
```

## Completion Criteria

1. README has hero, quick start, screenshots, commands, extensions
2. Quick start is copy-pasteable and works
3. Screenshots are actual terminal output
4. Extension guide is complete
5. `make ci` passes

## Commit

```
docs(readme): add README with hero, quick start, screenshots

- Hero section with vision and terminal screenshot
- Copy-pasteable quick start guide
- Command reference with examples
- Extension development guide
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §1, §2
5. Execute steps 1-4
6. Run verification
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit

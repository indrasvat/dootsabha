# Task 1.7: Cobra CLI Wiring (root, consult, status, config)

## Status: DONE

## Depends On
- Task 1.5 (claude provider), Task 1.6 (codex + gemini providers)

## Parallelizable With
- None (final P1 integration task)

## Problem

Wire the providers and output foundation into Cobra commands. After this task, `dootsabha consult`, `dootsabha status`, and `dootsabha config show` all work end-to-end with real CLI output.

## PRD Reference
- §6.1 (Root command — global flags, exit codes, SIGPIPE, extension discovery)
- §6.3 (Consult command — flags, output, acceptance criteria FR-CON-*)
- §6.5 (Status command — health table, acceptance criteria FR-STA-*)
- §6.6 (Config command — show, --commented, --json, redaction)
- §8.1 (stdout=data, stderr=logs)

## Files to Create
- `internal/cli/consult.go` — Consult command implementation
- `internal/cli/status.go` — Status command implementation
- `internal/cli/config_cmd.go` — Config command implementation
- `internal/cli/consult_test.go` — Command tests
- `internal/cli/status_test.go` — Command tests

## Files to Modify
- `internal/cli/root.go` — Add global flags, SIGPIPE handler, subcommand registration

## Execution Steps

### Step 1: Read spike findings
1. Read `_spikes/cobra-alias/README.md` (Spike 0.6 — alias patterns)

### Step 2: Wire root command
- Global flags: `--json`, `--verbose`, `--quiet`, `--timeout`, `--session-timeout`, `--config`
- Bilingual aliases on all flags (e.g., `--timeout` / `--kaalseema`)
- SIGPIPE handler (exit 0, no "broken pipe" error)
- Unknown command → extension discovery stub

### Step 3: Implement consult command
- `consult` (alias: `paraamarsh`)
- Flags: `--agent`/`--doota`, `--model`/`--pratyaya`, `--max-turns`
- RunE: load config → get provider → invoke → render output (TTY or JSON)
- Exit codes: 0 (success), 1 (error), 3 (provider), 4 (timeout)

### Step 4: Implement status command
- `status` (alias: `sthiti`)
- RunE: for each provider → health check → render table
- TTY: lipgloss table with provider dots, colors
- Piped: tab-separated, no colors
- JSON: provider health as structured data

### Step 5: Implement config command
- `config show` (alias: `vinyaas`)
- `--json`, `--commented`, `--reveal`
- Redact sensitive keys by default

### Step 6: L4 visual test
- Create `.claude/automations/test_dootsabha_consult.py` following testing-strategy.md §2
- Verify: TTY output has provider dot + styled text, piped output has no ANSI

## Verification

### L1: Unit tests
```bash
make ci
```

### L3: Real binary
```bash
make build
./bin/dootsabha consult --agent claude "Say PONG"
./bin/dootsabha consult --agent codex "Say PONG"
./bin/dootsabha consult --agent gemini "Say PONG"
./bin/dootsabha status
./bin/dootsabha status --json | python3 -m json.tool
./bin/dootsabha config show
./bin/dootsabha consult "test" | grep -cP '\x1b\['  # Should be 0
```

### L4: Visual
```bash
make test-visual
```

## Completion Criteria

1. `dootsabha consult` works with all 3 agents
2. `dootsabha status` shows health table
3. `dootsabha config show` shows merged config with redaction
4. All commands have bilingual aliases
5. Exit codes correct per §6.1
6. No ANSI in piped output
7. `make ci` passes

## Commit

```
feat(cli): wire consult, status, config commands with bilingual aliases

- consult (paraamarsh): invoke agent with styled output
- status (sthiti): health table with provider dots
- config (vinyaas): show with redaction
- Global flags with bilingual aliases
- SIGPIPE handler, exit codes per §6.1
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §6.1, §6.3, §6.5, §6.6
5. Read spike 0.6 findings
6. Execute steps 1-6
7. Run verification (L1 → L3 → L4)
8. Fill Visual Test Results section
9. **Change status to `DONE`**
10. Update `docs/PROGRESS.md`
11. Commit

# दूतसभा Progress

## Phase 0: Spikes (All Complete)

| Task | Spike | Status | Key Finding |
|------|-------|--------|-------------|
| 0.1 | Codex JSONL | DONE | 3 undocumented event types (error events, cached_input_tokens, error items). bufio.Scanner + json.Unmarshal per line works. |
| 0.2 | Claude JSON | DONE | Must strip CLAUDECODE* env vars entirely (not empty-string). is_error field, not exit code, discriminates errors. |
| 0.3 | Gemini JSON | DONE | Dual-model architecture (flash-lite router + flash main). Wall-clock ~10s but API latency ~1s. No JSON error format — stderr only. |
| 0.4 | go-plugin gRPC | DONE | Handshake 7.6ms median (26x under 200ms gate). Re-launch on crash, don't retry. Kill plugins explicitly. |
| 0.5 | Subprocess Mgmt | DONE | exec.Command (NOT CommandContext) for SIGTERM→grace→SIGKILL. Setpgid works under macOS SIP. errgroup is the reaper. |
| 0.6 | Cobra Alias | DONE | cobra.ArbitraryArgs required for extension discovery. Devanagari aliases work natively. Tab completion needs ValidArgsFunction workaround. |
| 0.7 | Terminal UX | DONE | huh v0.8.0 has NO standalone spinner (use raw goroutine on stderr). All 4 lipgloss pitfalls reproduced and documented. Color gate required for piped output. |
| 0.8 | PTY vs Pipe | DONE | creack/pty NOT needed. All 3 CLIs work via plain os/exec pipe with YOLO+JSON flags. |

### Critical PRD Updates Needed

All 4 items addressed in PRD v1.6.

## Phase 1: Foundation (All Complete)

| Task | Description | Status | Agent (Jaane Bhi Do Yaaro) |
|------|-------------|--------|---------------------------|
| 1.1 | Project Scaffold + Makefile + Gating Hooks | DONE | vinod (Wave 1) |
| 1.2 | Render Context & Output Foundation | DONE | sudhir (Wave 2) |
| 1.3 | Config Manager (Viper) | DONE | shobha (Wave 2) |
| 1.4 | Subprocess Runner | DONE | dmello (Wave 2) |
| 1.5 | Claude Provider (Hardcoded) | DONE | tarneja (Wave 3) |
| 1.6 | Codex + Gemini Providers | DONE | ahuja (Wave 4) |
| 1.7 | CLI Wiring (consult/status/config) | DONE | asrani (Wave 5) |
| 1.8 | Status Bugfix (version, dot column, models) | DONE | — |
| 1.9 | Align code with architecture doc (model, flags) | DONE | — |

### What Works End-to-End
- `dootsabha consult --agent claude/codex/gemini "prompt"` — invokes real CLIs, parses JSON/JSONL
- `dootsabha status` — health table with provider dots (TTY/pipe/JSON modes)
- `dootsabha config show` — merged config with key redaction
- Bilingual aliases: paraamarsh, sthiti, vinyaas + Devanagari
- `make ci` — 0 lint issues, all tests pass
- `make test-binary` — 8/8 L3 smoke tests

## Phase 2: Council Pipeline (All Complete)

| Task | Description | Status | Agent |
|------|-------------|--------|-------|
| 2.1 | Parallel Dispatch (errgroup + progress) | DONE | council-builder |
| 2.2 | Peer Review Stage (32KB truncation, cross-review) | DONE | council-builder |
| 2.3 | Synthesis Stage (chair + fallback + multi-round) | DONE | council-builder |
| 2.4 | Review Command (author + reviewer pipeline) | DONE | review-builder |
| 2.6 | Refine Command (sequential review + incorporation) | PENDING | — |

### What Works End-to-End
- `dootsabha council "prompt"` — 3-stage pipeline: dispatch → peer review → synthesis
- `dootsabha council "prompt" --json` — JSON with dispatch/reviews/synthesis/meta
- `dootsabha council "prompt" --parallel=false` — sequential dispatch mode
- `dootsabha council "prompt" --agents claude,codex --chair codex` — agent/chair override
- `dootsabha council "prompt" --rounds 2` — multi-round with context chaining
- `dootsabha review "prompt" --author codex --reviewer claude` — 2-step pipeline
- `dootsabha review "prompt" --json` — JSON with author/review/meta
- Bilingual aliases: sabha/सभा (council), sameeksha/समीक्षा (review)
- Bilingual flags: --dootas, --adhyaksha, --chakra, --samantar, --kartaa, --pareekshak
- Max 5 agents enforced, 32KB truncation for peer review + synthesis
- Chair failure → fallback to first healthy non-chair agent
- Exit code 5 for partial results (some agents failed)
- Progress rendering on stderr (TTY only)
- `make ci` — 0 lint issues, all tests pass
- `make test-binary` — 8/8 L3 smoke tests

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

1. **huh spinner** — PRD §8 references `huh.NewSpinner()` which doesn't exist in v0.8.0. Replace with raw stderr goroutine pattern.
2. **CLAUDECODE env stripping** — PRD §4.1 should document that CLAUDECODE* vars must be removed from subprocess env, not just unset.
3. **Codex error events** — PRD §4.1 should note that `type:"error"` JSONL lines are non-fatal transport fallback notices.
4. **Gemini dual-model** — PRD §4.1 should document the utility_router + main model architecture in stats.

## Phase 1: Foundation

Not started.

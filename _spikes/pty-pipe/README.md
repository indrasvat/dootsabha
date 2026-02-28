# Spike 007: PTY vs Pipe Subprocess — Findings

**Date:** 2026-02-28
**Go:** 1.26.0 darwin/arm64
**Status:** DONE — all completion criteria met

---

## Summary

**All three CLIs work perfectly via plain `os/exec` pipe mode.** No PTY (`creack/pty`) is needed.
Each CLI produces complete, valid JSON/JSONL output without any interactive blocking when
invoked with their YOLO+JSON flags.

---

## Test Results

| CLI | Version | Pipe Mode | JSON Valid | Blocking | Duration |
|-----|---------|-----------|-----------|----------|----------|
| claude | 2.1.63 | ✅ PASS | ✅ Single JSON object | None | ~6.8s |
| codex | 0.106.0 | ✅ PASS | ✅ JSONL stream | None | ~12.9s |
| gemini | 0.30.0 | ✅ PASS | ✅ Single JSON object | None | ~10.1s |

---

## Per-CLI Spawn Pattern (Production-Ready)

### claude

**Command:**
```bash
claude -p "<prompt>" --output-format json --dangerously-skip-permissions [--model <model>]
```

**Critical requirement — env var stripping:**
```go
// MUST strip CLAUDECODE* and CLAUDE_CODE* from subprocess env.
// Setting CLAUDECODE="" is NOT sufficient — the key must be removed entirely.
// Observed vars in a Claude Code agent session:
//   CLAUDECODE=1, CLAUDE_CODE_ENTRYPOINT=cli, CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1
env := os.Environ()
filtered := make([]string, 0, len(env))
for _, e := range env {
    if !strings.HasPrefix(e, "CLAUDECODE") && !strings.HasPrefix(e, "CLAUDE_CODE") {
        filtered = append(filtered, e)
    }
}
cmd.Env = filtered
```

**Output:** Single JSON object on stdout (buffered — arrives all at once when process exits):
```json
{
  "type": "result",
  "subtype": "success",
  "is_error": false,
  "result": "PONG",
  "duration_ms": 1628,
  "total_cost_usd": 0.075,
  "usage": { "input_tokens": 10, "output_tokens": 46, ... },
  "modelUsage": { "claude-haiku-4-5-20251001": { ... } },
  "session_id": "...",
  "uuid": "..."
}
```

**Error handling:** Discriminate via `is_error` field, NOT exit code.
Exit code 1 can still produce valid JSON with `is_error: true`.

**Stderr:** Typically empty on success; error text on CLAUDECODE env var violation.

---

### codex

**Command:**
```bash
codex exec --json --sandbox danger-full-access "<prompt>"
```

**Output:** JSONL stream (one JSON event per line), streamed as processing happens:
```
{"type":"thread.started","thread_id":"..."}
{"type":"turn.started"}
{"type":"error","message":"Reconnecting... 2/5 (...)"}  # WebSocket → HTTPS fallback (normal)
{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"..."}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"PONG"}}
{"type":"turn.completed","usage":{"input_tokens":18348,"cached_input_tokens":3456,"output_tokens":27}}
```

**Parsing pattern:**
```go
scanner := bufio.NewScanner(stdout)
for scanner.Scan() {
    var event map[string]any
    json.Unmarshal(scanner.Bytes(), &event)
    switch event["type"] {
    case "item.completed":
        item := event["item"].(map[string]any)
        if item["type"] == "agent_message" {
            content = item["text"].(string)
        }
    case "turn.completed":
        usage = event["usage"]
    case "error":
        // WebSocket→HTTPS reconnect errors are NORMAL — codex retries automatically
        // Only treat as fatal if no agent_message arrives
    }
}
```

**WebSocket fallback:** Codex emits `type:"error"` reconnect events when WebSocket
disconnects during startup. This is normal in pipe mode — codex falls back to HTTPS
transport and completes successfully. Do NOT treat these as fatal errors.

**No env var stripping needed** — codex has no nested-session restriction.

---

### gemini

**Command:**
```bash
gemini --yolo --output-format json -p "<prompt>"
# OR positional arg:
gemini --yolo --output-format json "<prompt>"
```

**Output:** Single JSON object on stdout:
```json
{
  "session_id": "...",
  "response": "PONG",
  "stats": {
    "models": {
      "gemini-2.5-flash-lite": { "api": {...}, "tokens": {...} },
      "gemini-3-flash-preview": { "api": {...}, "tokens": {...} }
    },
    "tools": { "totalCalls": 0, ... },
    "files": { "totalLinesAdded": 0, "totalLinesRemoved": 0 }
  }
}
```

**Response field:** `response` (top-level string) — not nested like claude/codex.

**Stderr:** Emits informational messages to stderr even on success:
```
YOLO mode is enabled. All tool calls will be automatically approved.
Loaded cached credentials.
```
This is expected — always redirect/ignore stderr when parsing JSON stdout.

**Multi-model routing:** Gemini uses `gemini-2.5-flash-lite` as a router and
`gemini-3-flash-preview` as main model. Both appear in `stats.models`.

**No env var stripping needed** — gemini has no nested-session restriction.

---

## Key Findings

### 1. Plain pipes are sufficient — `creack/pty` NOT needed

**Decision: Use plain `os/exec` pipe mode for all three CLIs.**

None of the CLIs require a PTY for non-interactive (YOLO+JSON flag) mode. All output
was complete and valid JSON/JSONL without any interactive blocking. The YOLO flags
(`--dangerously-skip-permissions`, `--sandbox danger-full-access`, `--yolo`) effectively
disable all interactive confirmation prompts.

### 2. Claude requires env var stripping (nested-session gotcha)

When spawned from inside a Claude Code session, `CLAUDECODE=1` causes an immediate exit
with a non-JSON error and empty stdout. Must strip all `CLAUDECODE*` and `CLAUDE_CODE*`
env vars from subprocess env.

### 3. Codex WebSocket fallback errors are normal

Codex emits `type:"error"` JSONL lines during WebSocket→HTTPS transport fallback. This
happens in pipe mode and is completely normal — codex retries automatically and delivers
the `agent_message`. Parse these as warnings, not fatal errors.

### 4. Stdout is buffered (delivers all at once on exit)

Claude and gemini buffer stdout until process exit (single JSON object). Codex streams
line-by-line (JSONL). For claude/gemini, use `cmd.Output()` or buffer to `bytes.Buffer`;
for codex, use `bufio.Scanner` on a pipe for streaming.

### 5. Gemini has informational stderr — expected

Gemini writes status messages to stderr even on success. Always redirect stderr separately
and don't fail on non-empty stderr.

### 6. All exit code 0 on success

All three CLIs returned exit code 0 on successful response. Claude can return exit code 1
with valid JSON (error response) — discriminate via `is_error` field.

---

## Production subprocess.go Integration

```go
// Claude
cmd := exec.Command("claude", "-p", prompt,
    "--output-format", "json",
    "--dangerously-skip-permissions",
    "--model", model)
cmd.Env = stripClaudeEnvVars(os.Environ())
out, _ := cmd.Output() // buffers stdout; err != nil on exit 1 but JSON still valid
var result ClaudeResult
json.Unmarshal(out, &result)
// Discriminate errors: result.IsError, not exit code

// Codex
cmd := exec.Command("codex", "exec",
    "--json",
    "--sandbox", "danger-full-access",
    prompt)
stdout, _ := cmd.StdoutPipe()
cmd.Start()
scanner := bufio.NewScanner(stdout)
for scanner.Scan() {
    var event CodexEvent
    json.Unmarshal(scanner.Bytes(), &event)
    if event.Type == "item.completed" && event.Item.Type == "agent_message" {
        content = event.Item.Text
    }
}
cmd.Wait()

// Gemini
cmd := exec.Command("gemini",
    "--yolo",
    "--output-format", "json",
    "-p", prompt)
out, _ := cmd.Output()
var result GeminiResult
json.Unmarshal(out, &result)
content = result.Response
```

---

## Completion Criteria

| # | Criterion | Result |
|---|---|---|
| 1 | All 3 CLIs tested in pipe mode with YOLO+JSON flags | ✅ All tested |
| 2 | JSON output validity confirmed for each | ✅ All parse cleanly |
| 3 | No interactive blocking in pipe mode | ✅ Zero blocking observed |
| 4 | Decision: plain pipe sufficient OR creack/pty needed | ✅ **Plain pipe is sufficient** |

---

## Risks Resolved

| Risk (PRD §11) | Status |
|---|---|
| CLIs need PTY, not pipe | **Resolved** — plain `os/exec` pipe works for all three |
| Interactive prompts block subprocess | **Resolved** — YOLO flags prevent all prompts |
| Claude nested session error | **Resolved** — strip `CLAUDECODE*` + `CLAUDE_CODE*` env vars |
| Codex JSONL parse errors on reconnect events | **Resolved** — treat as warnings, not fatal |

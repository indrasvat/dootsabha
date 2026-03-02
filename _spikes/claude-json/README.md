# Spike: Claude JSON Output Parsing

**Spike ID:** 001
**Status:** DONE
**Date:** 2026-02-28
**CLI version:** claude 2.1.63

## Summary

`claude -p "..." --output-format json --dangerously-skip-permissions` outputs a single-line JSON object to stdout. All four test cases passed against the real CLI.

---

## Exact JSON Schema (verified)

```json
{
  "type":           "result",          // always "result"
  "subtype":        "success",         // always "success" — even on errors
  "is_error":       false,             // true when model/auth fails
  "duration_ms":    2953,              // wall clock ms
  "duration_api_ms": 2574,            // API call ms; 0 on local errors
  "num_turns":      1,
  "result":         "PONG",           // response text; error message on failure
  "stop_reason":    null,             // nullable *string
  "session_id":     "6ee9fd29-...",
  "total_cost_usd": 0.22555875,       // 0.0 on error
  "usage": {
    "input_tokens":               3,
    "cache_creation_input_tokens": 35987,
    "cache_read_input_tokens":    0,
    "output_tokens":              25,
    "server_tool_use": {
      "web_search_requests": 0,
      "web_fetch_requests":  0
    },
    "service_tier":   "standard",
    "cache_creation": {
      "ephemeral_1h_input_tokens": 35987,
      "ephemeral_5m_input_tokens": 0
    },
    "inference_geo": "",
    "iterations":    [],
    "speed":         "standard"
  },
  "modelUsage": {                      // keyed by model ID; EMPTY MAP on error
    "claude-opus-4-6": {
      "inputTokens":               3,
      "outputTokens":              25,
      "cacheReadInputTokens":      0,
      "cacheCreationInputTokens":  35987,
      "webSearchRequests":         0,
      "costUSD":                   0.22555875,
      "contextWindow":             200000,
      "maxOutputTokens":           32000
    }
  },
  "permission_denials": [],
  "fast_mode_state": "off",           // "off" | "on"
  "uuid":            "c0cef853-..."
}
```

---

## Go Type Mapping

| JSON field | Go type | Notes |
|---|---|---|
| `type` | `string` | constant `"result"` |
| `subtype` | `string` | constant `"success"` even for errors |
| `is_error` | `bool` | discriminates success vs. error |
| `result` | `string` | response text or error message |
| `stop_reason` | `*string` | nullable — use pointer |
| `total_cost_usd` | `float64` | 0.0 on error |
| `usage.input_tokens` | `int` | |
| `usage.cache_creation_input_tokens` | `int` | large; ephemeral cache |
| `usage.cache_read_input_tokens` | `int` | |
| `usage.output_tokens` | `int` | |
| `modelUsage` | `map[string]ModelUsage` | empty `{}` on error; key = model ID |
| `session_id` | `string` | UUID format |
| `uuid` | `string` | per-request UUID (differs from session_id) |
| `fast_mode_state` | `string` | `"off"` \| `"on"` |
| `permission_denials` | `[]any` | always empty in `-p` non-interactive mode |
| `usage.iterations` | `[]any` | always empty observed |

---

## Key Findings

### 1. CLAUDECODE nested session gotcha — CONFIRMED

Running `claude -p` when `CLAUDECODE` is set in the environment causes an **immediate, non-JSON error** to stderr and exits with code 1:

```
Error: Claude Code cannot be launched inside another Claude Code session.
Nested sessions share runtime resources and will crash all active sessions.
To bypass this check, unset the CLAUDECODE environment variable.
```

**stdout is empty** in this case — no JSON is written.

**Production fix:** Strip `CLAUDECODE` from subprocess `Env` using `os.Environ()` filtering. Setting `CLAUDECODE=""` is NOT sufficient — the key must be removed entirely.

### 2. Error response format

On model errors (`--model invalid-model-xyz`), claude:
- Exits with code **1**
- Outputs valid JSON to **stdout** (same schema)
- Sets `"is_error": true`
- Sets `"total_cost_usd": 0`
- Sets `"modelUsage": {}` (empty map)
- Puts the error message in `"result"`
- **Emits the JSON twice** (stdout duplication — appears to be a CLI bug)

Discriminate errors via `is_error` field, not exit code alone.

### 3. Model override works

`--model claude-haiku-4-5-20251001` is correctly reflected in `modelUsage` map key. Default model is `claude-opus-4-6`.

### 4. Nullable fields

- `stop_reason` is `null` on success — must use `*string` in Go
- `permission_denials` and `usage.iterations` always appear as `[]` — can use `[]any`

### 5. Cost fields

- `total_cost_usd` (float64) — reliable summary
- `modelUsage[model].costUSD` — same value broken out per model
- Cache creation tokens (ephemeral 1h) are substantial (~36k per cold call)

---

## Production Integration Pattern

```go
// 1. Strip CLAUDECODE from subprocess env
env := make([]string, 0, len(os.Environ()))
for _, kv := range os.Environ() {
    if !strings.HasPrefix(kv, "CLAUDECODE=") {
        env = append(env, kv)
    }
}
cmd.Env = env

// 2. Run claude
out, err := cmd.Output()
// err != nil on exit code 1, but stdout still has JSON for model errors

// 3. Parse JSON (stdout valid even on model errors)
var result ClaudeResult
json.Unmarshal(out, &result)

// 4. Discriminate via is_error (not exit code)
if result.IsError {
    return fmt.Errorf("claude error: %s", result.Result)
}
content := result.Result
```

---

## Test Results

| Test | Result |
|---|---|
| Normal success (CLAUDECODE unset) | PASS — `"PONG"`, cost $0.2256 |
| Model override `--model claude-haiku-4-5-20251001` | PASS — model key in modelUsage |
| Invalid model name | PASS — JSON `is_error: true`, exit 1 |
| CLAUDECODE set (nested session) | PASS — stderr error, empty stdout, exit 1 |

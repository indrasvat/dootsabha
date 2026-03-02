# Spike 002: Gemini CLI JSON Output Parsing

**Status:** DONE
**CLI version tested:** gemini 0.30.0
**Date:** 2026-02-28

---

## Findings

### 1. JSON Schema (exact, verified)

```json
{
  "session_id": "<uuid>",
  "response": "<assistant text>",
  "stats": {
    "models": {
      "<model-name>": {
        "api": {
          "totalRequests": 1,
          "totalErrors": 0,
          "totalLatencyMs": 1337
        },
        "tokens": {
          "input": 803,
          "prompt": 803,
          "candidates": 35,
          "total": 979,
          "cached": 0,
          "thoughts": 141,
          "tool": 0
        },
        "roles": {
          "<role-name>": {
            "totalRequests": 1,
            "totalErrors": 0,
            "totalLatencyMs": 1337,
            "tokens": { ... }
          }
        }
      }
    },
    "tools": {
      "totalCalls": 0,
      "totalSuccess": 0,
      "totalFail": 0,
      "totalDurationMs": 0,
      "totalDecisions": { "accept": 0, "reject": 0, "modify": 0, "auto_accept": 0 },
      "byName": {}
    },
    "files": {
      "totalLinesAdded": 0,
      "totalLinesRemoved": 0
    }
  }
}
```

**Key extraction path:** `resp.Response` — the full assistant text is at the top-level `response` field. No unwrapping needed.

---

### 2. Positional Prompt vs `-p` Flag

Both produce **identical JSON schema** and model set.

| Invocation | Schema | Models |
|---|---|---|
| `gemini --yolo --output-format json "Say PONG"` | ✓ | same |
| `gemini --yolo -p "Say PONG" --output-format json"` | ✓ | same |

**Recommendation:** Use positional prompt for simplicity (`exec.Command("gemini", "--yolo", "--output-format", "json", prompt)`). The `-p` flag offers no advantage.

---

### 3. `--yolo` vs `--approval-mode yolo`

Both flags are **confirmed equivalent** — identical JSON schema, same model set, same token structure.

| Flag | Schema | Models |
|---|---|---|
| `--yolo` | ✓ | `gemini-2.5-flash-lite` + `gemini-3-flash-preview` |
| `--approval-mode yolo` | ✓ | `gemini-2.5-flash-lite` + `gemini-3-flash-preview` |

**Recommendation:** Use `--yolo` (shorter boolean flag).

---

### 4. Dual-Model Architecture (Important)

Gemini v0.30.0 uses **two models per invocation**:

| Model | Role | Purpose |
|---|---|---|
| `gemini-2.5-flash-lite` | `utility_router` | Intent classification / routing |
| `gemini-3-flash-preview` | `main` | Primary response generation |

The `stats.models` field is a **map** (not a list). Model names may vary across gemini versions. The `main` role model is the one producing `response`.

Token totals must be **summed across all models** if you want global usage. Per-model tokens live at `stats.models[name].tokens`.

---

### 5. Token Extraction

```go
// Extract primary model tokens (the "main" role model)
for modelName, stat := range resp.Stats.Models {
    for roleName, role := range stat.Roles {
        if roleName == "main" {
            inputTokens  = role.Tokens.Input
            outputTokens = role.Tokens.Candidates
            // modelName is the primary model
        }
    }
}
```

Token fields available per model:
- `input` — total input tokens sent
- `prompt` — same as input (duplicate field in v0.30.0)
- `candidates` — output/generation tokens
- `total` — input + candidates
- `cached` — cached prompt tokens (0 when no cache)
- `thoughts` — reasoning/thinking tokens (non-zero for thinking models)
- `tool` — tokens consumed by tool calls

---

### 6. Latency: CLI Startup vs API Latency

**Critical finding:** CLI startup overhead dominates wall-clock time.

| Measurement | Value |
|---|---|
| Wall-clock (process start to exit) | ~10s |
| API latency (`totalLatencyMs`, utility_router) | ~900–1337ms |
| API latency (`totalLatencyMs`, main model) | ~1208–1431ms |

The ~8–9 seconds of startup overhead comes from:
- Loading MCP extensions (firebase, gemini-cli-jules, gemini-cli-security)
- Extension server connection/handshake
- Skill loading and conflict detection

For दूतसभा: use `totalLatencyMs` from `stats.models` for API timing, not wall-clock.

---

### 7. Error Handling

- **Success:** stdout = valid JSON, stderr = startup logs (safe to discard)
- **Auth failure / no network:** gemini exits non-zero; stdout is empty or partial; stderr contains error message
- **JSON parse failures:** check `cmd.ProcessState.ExitCode() != 0` before unmarshaling
- **No documented error JSON format** — errors appear only on stderr as plain text

**Pattern for production code:**
```go
if cmd.ProcessState.ExitCode() != 0 {
    return nil, fmt.Errorf("gemini: %s", strings.TrimSpace(stderr.String()))
}
var resp GeminiResponse
if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
    return nil, fmt.Errorf("gemini json: %w", err)
}
```

---

### 8. Go Types (production-ready)

See `main.go` for the full type definitions:
- `GeminiResponse` — top-level envelope
- `GeminiStats` — nested stats
- `GeminiModelStat` — per-model (map key = model name)
- `GeminiRoleStat` — per-role within model
- `GeminiTokenUsage` — token breakdown
- `GeminiToolStats` — tool call aggregates
- `GeminiFileStats` — file line change tracking

---

## Completion Criteria

| Criterion | Status |
|---|---|
| JSON schema captured with Go types | DONE |
| Positional vs `-p` behavior documented | DONE — identical, prefer positional |
| `--yolo` vs `--approval-mode yolo` confirmed equivalent | DONE |
| Error format documented | DONE — stderr only, no JSON |

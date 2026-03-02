# Spike 000: Codex JSONL Event Stream Parsing

**Status:** DONE
**Date:** 2026-02-28
**Codex version:** 0.106.0

## Summary

Parsing Codex JSONL with a line-by-line `json.Decoder` (or `bufio.Scanner` + `json.Unmarshal`) works reliably. The §4.1 spec is mostly accurate but **three undocumented behaviors were discovered** from real CLI invocations.

---

## Real CLI Output (verified 2026-02-28)

### Prompt: "Say PONG"

```jsonl
{"type":"thread.started","thread_id":"019ca5e7-60a5-7c90-b8a6-59ff078ba683"}
{"type":"turn.started"}
{"type":"error","message":"Reconnecting... 2/5 (stream disconnected before completion: websocket closed by server before response.completed)"}
{"type":"error","message":"Reconnecting... 3/5 (stream disconnected before completion: failed to send websocket request: Connection closed normally)"}
{"type":"error","message":"Reconnecting... 4/5 (stream disconnected before completion: failed to send websocket request: Connection closed normally)"}
{"type":"error","message":"Reconnecting... 5/5 (stream disconnected before completion: failed to send websocket request: Connection closed normally)"}
{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Falling back from WebSockets to HTTPS transport. stream disconnected before completion: failed to send websocket request: Connection closed normally"}}
{"type":"item.completed","item":{"id":"item_1","type":"reasoning","text":"**Responding with simple output**"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"PONG"}}
{"type":"turn.completed","usage":{"input_tokens":18343,"cached_input_tokens":3456,"output_tokens":79}}
```

### Prompt: "List the numbers 1 to 5"

```jsonl
{"type":"thread.started","thread_id":"019ca5e7-b877-7c22-b196-892649686924"}
{"type":"turn.started"}
... (same reconnect error events)
{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Falling back from WebSockets to HTTPS transport..."}}
{"type":"item.completed","item":{"id":"item_1","type":"reasoning","text":"**Providing simple number list**"}}
{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"1, 2, 3, 4, 5"}}
{"type":"turn.completed","usage":{"input_tokens":18348,"cached_input_tokens":3456,"output_tokens":266}}
```

---

## Event Types Observed

| Event Type | In §4.1 Spec? | Description |
|------------|---------------|-------------|
| `thread.started` | ✓ | Session init; has `thread_id` (UUIDv7 format) |
| `turn.started` | ✓ | Turn begins; no payload |
| `item.completed` | ✓ | Item produced; `item.type` varies (see below) |
| `turn.completed` | ✓ | Turn ends; has `usage` |
| **`error`** | **✗ NEW** | Top-level reconnect status; has `message`; non-fatal |

### `item.completed` Item Types

| `item.type` | In §4.1 Spec? | Fields | Notes |
|-------------|---------------|--------|-------|
| `reasoning` | ✓ | `text` | Internal chain-of-thought |
| `agent_message` | ✓ | `text` | **Final answer** — extract this |
| **`error`** | **✗ NEW** | `message` | Transport fallback notice; does NOT halt stream |

---

## Undocumented Findings

### 1. Top-level `error` events (non-fatal)
The `error` event type is emitted during WebSocket reconnection attempts. These are **informational**, not fatal — the stream continues and produces a valid `turn.completed`. Parser must skip these or log as warnings.

```json
{"type":"error","message":"Reconnecting... 2/5 (...)"}
```

### 2. `cached_input_tokens` in `turn.completed`
The `usage` object has an undocumented `cached_input_tokens` field:

```json
{"type":"turn.completed","usage":{"input_tokens":18343,"cached_input_tokens":3456,"output_tokens":79}}
```

This is OpenAI-style prompt caching. Update the `Usage` struct to capture it.

### 3. `item.type == "error"` inside `item.completed`
The transport fallback produces an error item *inside* `item.completed`:

```json
{"type":"item.completed","item":{"id":"item_0","type":"error","message":"Falling back from WebSockets to HTTPS transport..."}}
```

This uses a `message` field instead of `text`. Parser must handle this without panicking.

### 4. Token counts are larger than expected
Simple one-word responses use ~18,000 input tokens — suggesting a large system prompt or conversation history is prepended by Codex CLI.

### 5. WebSocket → HTTPS fallback is automatic
Codex CLI retries up to 5 times with exponential backoff, then falls back to HTTPS. This is transparent and produces valid output. No action needed in dootsabha.

---

## Recommended Go Types

```go
// Event is a single JSONL line from `codex --json`.
type Event struct {
    Type    string `json:"type"`
    // thread.started
    ThreadID string `json:"thread_id,omitempty"`
    // item.completed
    Item    *Item  `json:"item,omitempty"`
    // turn.completed
    Usage   *Usage `json:"usage,omitempty"`
    // error (non-fatal reconnect status)
    Message string `json:"message,omitempty"`
}

// Item is embedded in item.completed events.
type Item struct {
    ID      string `json:"id"`
    Type    string `json:"type"`    // "reasoning" | "agent_message" | "error"
    Text    string `json:"text,omitempty"`    // for reasoning + agent_message
    Message string `json:"message,omitempty"` // for type=="error"
}

// Usage is embedded in turn.completed events.
type Usage struct {
    InputTokens       int `json:"input_tokens"`
    CachedInputTokens int `json:"cached_input_tokens"` // undocumented; cache hits
    OutputTokens      int `json:"output_tokens"`
}
```

---

## Parsing Algorithm

```go
// parseJSONL extracts agent message and usage from Codex JSONL output.
// Robust: skips malformed lines, handles missing fields, continues past errors.
func parseJSONL(data []byte) (agentMsg string, usage *Usage, err error) {
    scanner := bufio.NewScanner(bytes.NewReader(data))
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" { continue }
        var ev Event
        if jsonErr := json.Unmarshal([]byte(line), &ev); jsonErr != nil {
            // log warning; continue
            continue
        }
        switch ev.Type {
        case "item.completed":
            if ev.Item != nil && ev.Item.Type == "agent_message" {
                agentMsg = ev.Item.Text
            }
        case "turn.completed":
            usage = ev.Usage
        case "error":
            // non-fatal; log as warning
        }
    }
    return agentMsg, usage, scanner.Err()
}
```

---

## Edge Cases Validated

| Case | Behavior |
|------|----------|
| Empty stream | Returns `""`, `nil`, no error |
| No `agent_message` item | Returns `""`, usage preserved |
| Malformed JSON line | Skipped; parsing continues |
| `error` event type | Skipped; does not halt parsing |
| `item.type == "error"` | Skipped in agent_message extraction |
| Multiple `agent_message` items | Last one wins (last-write-wins) |

---

## Risk Update (§11)

The §11 risk entry "Codex JSONL format changes" remains valid. Specific risks:

1. **`error` event type** — already occurs in v0.106.0; must be handled
2. **`cached_input_tokens`** — present but optional; use `omitempty` logic
3. **Item types may expand** — treat unknown `item.type` as no-op (defensive)
4. **Token counts** — much larger than expected due to Codex system prompt; budget accordingly

---

## Files

- `main.go` — Spike program with mock tests + real CLI invocation
- `go.mod` — `module dootsabha-spike/codex-jsonl` (standalone, no top-level go.mod)
- `README.md` — This file

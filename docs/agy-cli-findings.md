# Antigravity CLI (`agy`) — findings

Everything here was verified by driving the real binary. Nothing is inferred.

## 2026-08-21 — `agy` 1.1.17 (task 707)

### Models

`agy models` prints `id<TAB>display name`. Both spellings are accepted by
`--model`, and matching is case-insensitive (`gemini 3.7 flash (high)` works).

दूतसभा uses the **display name**, because that is what the CLI itself prints back
in its `Available models` error, what `status` and `config show` render, and what
the PRD documents.

The `(High|Medium|Low)` suffix **is** the reasoning-effort selector, not
decoration:

```
$ agy --model "Gemini 3.7 Flash (Low)" --effort high -p ...
Error: invalid model selection: --effort is not supported for model "Gemini 3.7 Flash (Low)"
$ agy --model "Gemini 3.7 Flash (High)" --effort xhigh -p ...
Error: invalid model selection: invalid --effort "xhigh" (valid: low, medium, high)
```

So दूतसभा must never emit `--effort` as a second axis. An unknown model exits 1
and lists the valid set.

Gemini 3.7 Flash shipped 2026-08-13 and is Antigravity's own default. 3.6, 3.5
and 3.1 Pro are all **still listed** — a default bump is not a retirement.

### `--output-format json`

v1.0.8 had no JSON; **1.1.17 does**, and दूतसभा parses it. stdout is exactly one
JSON document; stderr is empty.

```json
{"conversation_id":"…","status":"SUCCESS","response":"PONG\n","error":"",
 "duration_seconds":3.51,"num_turns":1,
 "usage":{"input_tokens":18053,"output_tokens":26,"thinking_tokens":24,
          "cache_read_tokens":0,"total_tokens":18079}}
```

Three properties matter for the parser:

1. **`total_tokens == input + output`, and `thinking_tokens` is a SUBSET of
   `output_tokens`.** Adding thinking to output double-counts every reasoning turn.
2. **On failure the reason is in `error`, and stderr is EMPTY.** A provider that
   reads stderr on a non-zero exit — which is what दूतसभा did before 707 — reports
   a bare `exit code 1` and loses the message entirely.
3. **The envelope never echoes the model.** `Model` is the value दूतसभा sent, so
   model forwarding can only be proven at argv (unit tests) or via a mock that
   echoes it (L3/L5).

There is **no cost field in any output format**. `CostUSD` stays `0`; दूतसभा does
not estimate one from a price list that would go stale.

### `status` is not the failure discriminator

Observed live, on a turn where a `find` tool call timed out but the model still
answered correctly:

```json
{"status":"ERROR","response":"6\n",
 "error":"Find command timed out…: context deadline exceeded"}
```

…with **exit code 0**. `status` is a *tool-level* diagnostic. Treating
`status != "SUCCESS"` as failure would discard a usable answer, so the exit code
decides and a degraded turn is logged at Warn instead.

### Known interaction: `--print-timeout`

`agy --print-timeout` defaults to **5m**, exactly दूतसभा's default per-call
`--timeout`. दूतसभा starts its budget marginally earlier, so on defaults its own
timeout wins the race and the user gets the named exit `4`.

Raising `--timeout` past 5m does **not** raise agy's: the CLI self-terminates at
its own 5m and दूतसभा reports a provider failure rather than a timeout. दूतसभा does
not currently forward its budget to `--print-timeout`. Tracked as task 708.

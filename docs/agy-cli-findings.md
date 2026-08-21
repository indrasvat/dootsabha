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

Print mode gained `--output-format` in **1.1.8** (`agy changelog`); 1.0.8, which
703 shipped against, predates it. दूतसभा therefore requires **1.1.8+** and
`HealthCheck` reports an older build unhealthy — otherwise `status` would show a
green row while every call failed before the prompt ran.

Verified on 1.1.17: stdout is exactly one JSON document; stderr is empty.

```json
{"conversation_id":"…","status":"SUCCESS","response":"PONG\n",
 "duration_seconds":3.51,"num_turns":1,
 "usage":{"input_tokens":18053,"output_tokens":26,"thinking_tokens":24,
          "cache_read_tokens":0,"total_tokens":18079}}
```

`error` is **absent** on success and present on failure — it is not emitted as `""`.

Five properties matter for the parser:

1. **`total_tokens == input + output`, and `thinking_tokens` is a SUBSET of
   `output_tokens`.** Adding thinking to output double-counts every reasoning turn.
2. **On failure the reason is in `error`, and stderr is EMPTY.** A provider that
   reads stderr on a non-zero exit — which is what दूतसभा did before 707 — reports
   a bare `exit code 1` and loses the message entirely.
3. **The envelope never echoes the model.** `Model` is the value दूतसभा sent, so
   model forwarding can only be proven at argv (unit tests) or via a mock that
   echoes it (L3/L5).
4. **Only the fields दूतसभा consumes are decoded.** A strict typed decode of a
   field nothing reads — `duration_seconds` is already a float — turns one
   upstream type change into a discarded answer, and on a failed call into a lost
   error message. Token counts are `json.Number`, so one bad number degrades to
   `0` instead of failing the parse, and the failure text is decoded
   independently of the rest of the envelope.
5. **Exactly one document.** `json.Decode` stops at the first value, so a second
   envelope would be silently dropped and the wrong turn reported as the answer.
   दूतसभा rejects trailing data explicitly.

There is **no cost field in any output format**. `CostUSD` stays `0`; दूतसभा does
not estimate one from a price list that would go stale.

### argv is parsed by Go's stdlib `flag` package

Three consequences, all verified against 1.1.17 — and all of them bit दूतसभा:

1. **`-model` IS `--model`.** Single and double dash are the same flag, so a
   pinned-flag stripper matching only `--model` lets `-model` straight through.
2. **A repeated flag is LAST-WINS.** Config flags are appended after दूतसभा's own,
   so a surviving `-model X` would silently win — and because the envelope never
   echoes the model, nothing downstream could detect that the reported model and
   the model that ran had diverged.
3. **Parsing STOPS at the first non-flag token.** One stray token in
   `providers.agy.flags` — a typo'd value, a bare `true` — swallowed everything
   after it. With `-p <prompt>` at the end, the prompt was never sent at all:

   ```
   $ agy --model "…" --output-format json JUNK -p "hi"
   CLI error: bubbletea: error opening TTY: … /dev/tty: device not configured
   $ echo $?
   0
   ```

   agy abandons print mode and tries to open the interactive TUI, exiting **0**.

दूतसभा therefore normalises leading dashes before matching a pinned flag, emits
`-p <prompt>` **before** the user's flags, and rejects a `providers.*.flags` that
is not a list of strings with exit 6.

There is no attached short form to worry about (contrast grok's `-mgrok-9`): agy's
only single-letter flags are `-c`, `-i` and `-p`, so an `-m` in config reaches the
CLI and is rejected loudly with exit 2.

### `status` is not the failure discriminator

Observed live, on a turn where a `find` tool call timed out but the model still
answered correctly:

```json
{"status":"ERROR","response":"6\n",
 "error":"Find command timed out…: context deadline exceeded"}
```

…with **exit code 0**. `status` is a *tool-level* diagnostic. Treating
`status != "SUCCESS"` as failure would discard a usable answer, so the exit code
decides and a degraded turn is logged at Warn instead — visible on stderr in
text mode, suppressed under `--json` (unless `-v`), so the JSON document stays
clean.

### `--print-timeout` (task 708)

`agy --print-timeout` defaults to **5m**, exactly दूतसभा's default per-call
`--timeout` — and when it fires, agy reports an ordinary ERROR envelope with
exit 1:

```json
{"status":"ERROR","response":"","error":"timeout waiting for response", …}
```

Nothing distinguishes that from any other provider failure, so दूतसभा charged the
caller exit **3** ("nothing usable, try another agent") for what was really exit
**4** ("raise the timeout"). Raising `--timeout` past 5m did not raise agy's.

दूतसभा now forwards the step's remaining window plus a 30s margin, so its own
timer always fires first and keeps ownership of the exit code. The margin clears
the `SIGTERM`→grace→`SIGKILL` sequence (5s by default). `--print-timeout` is
pinned whenever दूतसभा emits one.

**`--print-timeout 0` does not mean "disabled"** — verified on 1.1.17, it fails
instantly with `timeout waiting for response`. A non-positive window is therefore
never emitted.

Residual, by design: with **both** `--timeout 0` and `--session-timeout 0` the
step has no deadline, दूतसभा emits nothing, and agy's own 5m default applies. That
is the one case where a user's own `--print-timeout` in `providers.agy.flags`
survives, which is the escape hatch.

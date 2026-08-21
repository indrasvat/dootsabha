# Task 708: Forward the per-call budget to `agy --print-timeout`

## Status: PENDING

## Depends On
707 (agy JSON + model bump), 705 (timeout scoping)

## Problem

`agy --print-timeout` defaults to **5m** — exactly दूतसभा's default per-call
`--timeout`. दूतसभा starts its budget marginally before the spawn, so on defaults
its own timeout wins and the user gets the named exit `4`.

Raise `--timeout` past 5m and that stops being true: agy self-terminates at its
own 5m, and दूतसभा reports a **provider failure (exit 3)** for what is really a
timeout (exit 4). One code, one caller action — this hands the caller the wrong
one. Found while probing the real binary for 707 (`docs/agy-cli-findings.md`).

## PRD Ref
§6.1 (exit-code precedence), §5.3 (agy provider)

## Sketch

Derive the remaining window from the context deadline and emit
`--print-timeout` slightly ABOVE it, so दूतसभा's own timer always fires first and
keeps ownership of the exit code. Do not compute it from `cfg.Timeout` — CLAUDE.md
forbids a command building its own window; the deadline already on `ctx` is the
single source.

No-deadline contexts must emit no flag at all.

## Done Criteria

- [ ] `--timeout 15m` on a hanging agy exits `4`, not `3`
- [ ] The emitted `--print-timeout` exceeds the context deadline, never trails it
- [ ] A context with no deadline emits no `--print-timeout`
- [ ] `outcome_test.go` guards still pass — no new timeout arithmetic in the CLI layer
- [ ] L5 covers the >5m case with a hanging stub

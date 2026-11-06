"""Hold one --progress-file to its contract (GitHub issue #34) for the L5 suite.

usage: progress-check.py <file> <exit-code> <python expression on ev>

Checks what every file must satisfy, then the case's own expression, where ev
is the list of events. Prints the reason and exits 1 on the first failure.
The contract is skill/references/progress-file.md.
"""

import json
import sys

SECRET = "PROGRESS-SECRET"
STATUSES = {"ok", "error", "timeout", "interrupted"}


def main():
    path, want_code, check = sys.argv[1], int(sys.argv[2]), sys.argv[3]
    with open(path, encoding="utf-8") as f:
        raw = f.read()
    if SECRET in raw:
        sys.exit("prompt text leaked into the progress file")
    if not raw.endswith("\n"):
        sys.exit("last line is not terminated")

    ev = []
    for n, line in enumerate(raw.splitlines(), 1):
        try:
            obj = json.loads(line)
        except ValueError as e:
            sys.exit("line %d is not JSON (%s): %r" % (n, e, line))
        if not isinstance(obj, dict) or "ts" not in obj or "event" not in obj:
            sys.exit("line %d lacks ts/event: %r" % (n, line))
        ev.append(obj)

    if not ev or ev[-1]["event"] != "run_finished":
        sys.exit("run_finished is not the last event: %r" % ev[-1:])
    if [e["event"] for e in ev].count("run_finished") != 1:
        sys.exit("more than one run_finished")
    if any("pid" in e and e["pid"] != ev[-1]["pid"] for e in ev):
        sys.exit("events from more than one process")
    if ev[-1]["exit_code"] != want_code:
        sys.exit("run_finished.exit_code %r, process exited %d" % (ev[-1]["exit_code"], want_code))

    open_calls = {}
    for e in ev:
        if e["event"] == "call_started":
            if e["call"] in open_calls:
                sys.exit("call %r started twice" % e["call"])
            open_calls[e["call"]] = (e["stage"], e["agent"])
        elif e["event"] == "call_finished":
            if open_calls.pop(e["call"], None) != (e["stage"], e["agent"]):
                sys.exit("call_finished without a matching call_started: %r" % e)
            if e["status"] not in STATUSES:
                sys.exit("unknown status %r" % e["status"])
            if (e["status"] == "timeout") != ("scope" in e):
                sys.exit("scope must be present exactly on a timeout: %r" % e)
    if open_calls:
        sys.exit("calls never finished: %r" % open_calls)

    if not eval(check, {}, {"ev": ev}):  # noqa: S307 — the suite's own expressions
        sys.exit("check failed: %s\nevents: %s" % (check, json.dumps(ev)))


if __name__ == "__main__":
    main()

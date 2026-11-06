"""Drive one "signal dootsabha mid-run" case for the L5 suite (GitHub issue #33).

Prints one JSON line the caller asserts on:
  returncode  Popen.returncode: -N means dootsabha died by signal N
  started     agents that started (each records its process group)
  survivors   agent groups still alive after dootsabha exited
  stdout_ok   stdout is empty or exactly one JSON document
  elapsed     seconds from the first signal to exit (null if it hung)

Bash's $? cannot tell "died by SIGTERM" from "exit(143)", which is why this is
Python: a parent that waits on dootsabha sees the difference, and so does a
shell deciding whether Ctrl-C should end its loop.
"""

import argparse
import json
import os
import signal
import subprocess
import sys
import tempfile
import time

AGENT = """#!/usr/bin/env bash
[[ "$1" == "--version" ]] && { echo "0.0.0"; exit 0; }
echo $$ > "%(dir)s/$$.pid"
%(trap)s
sleep 300 &
wait
"""


def group_running(pg):
    """A member of group pg that is not a zombie. Where PID 1 does not reap
    orphans (some CI containers), a killed agent lingers as a zombie and
    killpg(pg, 0) still succeeds on it, although it has stopped."""
    try:
        os.killpg(pg, 0)
    except ProcessLookupError:
        return False
    rows = subprocess.run(["ps", "-A", "-o", "pgid=,stat="], capture_output=True, text=True, check=True).stdout
    return any(f[0] == str(pg) and not f[1].startswith("Z")
               for f in (line.split() for line in rows.splitlines()) if len(f) == 2)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--binary", required=True)
    ap.add_argument("--want", type=int, default=1, help="agents to wait for before signalling")
    ap.add_argument("--signals", default="TERM", help="e.g. TERM or INT,TERM@0.05 (seconds after the agents start); empty sends none")
    ap.add_argument("--ignore-term", action="store_true", help="agents ignore SIGTERM, children too")
    ap.add_argument("--stderr", choices=["pipe", "stalled", "break-after-start"], default="pipe")
    ap.add_argument("--env", action="append", default=[], help="KEY=VAL, applied after the agent defaults")
    ap.add_argument("cmd", nargs=argparse.REMAINDER)
    a = ap.parse_args()

    d = tempfile.mkdtemp(prefix="dootsabha-sig-")
    agent = os.path.join(d, "agent")
    with open(agent, "w") as f:
        f.write(AGENT % {"dir": d, "trap": 'trap "" TERM' if a.ignore_term else ""})
    os.chmod(agent, 0o755)
    env = dict(os.environ)
    for name in ("CLAUDE", "CODEX", "AGY", "GROK"):
        env["DOOTSABHA_PROVIDERS_%s_BINARY" % name] = agent
    env.update(kv.split("=", 1) for kv in a.env)
    cmd = a.cmd[1:] if a.cmd[:1] == ["--"] else a.cmd

    err_r, err_w = os.pipe()
    if a.stderr == "stalled":  # fill it, and never read: the next write blocks
        os.set_blocking(err_w, False)
        try:
            while True:
                os.write(err_w, b"x" * 4096)
        except BlockingIOError:
            pass
        os.set_blocking(err_w, True)
    out = tempfile.TemporaryFile()
    p = subprocess.Popen([a.binary] + cmd, env=env, stdout=out, stderr=err_w)
    os.close(err_w)

    def pids():
        return [int(open(os.path.join(d, f)).read()) for f in os.listdir(d) if f.endswith(".pid")]

    deadline = time.time() + 10
    while len(pids()) < a.want and time.time() < deadline and p.poll() is None:
        time.sleep(0.02)
    started = pids()
    if a.stderr == "break-after-start":  # dootsabha's next stderr write raises SIGPIPE
        os.close(err_r)

    t0 = time.time()
    for spec in filter(None, a.signals.split(",")):
        name, _, at = spec.partition("@")
        if at:
            time.sleep(max(0.0, t0 + float(at) - time.time()))
        if p.poll() is None:
            p.send_signal(getattr(signal, "SIG" + name))
    try:
        rc = p.wait(timeout=20)
        elapsed = round(time.time() - t0, 2)
    except subprocess.TimeoutExpired:
        p.kill()
        rc, elapsed = p.wait(), None

    survivors = []
    for pg in started:
        for _ in range(25):
            if not group_running(pg):
                break
            time.sleep(0.1)
        else:
            survivors.append(pg)
            os.killpg(pg, signal.SIGKILL)

    out.seek(0)
    body = out.read().strip()
    try:
        stdout_ok = not body or json.loads(body) is not None
    except ValueError:
        stdout_ok = False
    started_late = len(pids()) - len(started)
    print(json.dumps({"returncode": rc, "started": len(started), "started_late": started_late,
                      "survivors": len(survivors), "stdout_ok": stdout_ok, "elapsed": elapsed}))


if __name__ == "__main__":
    sys.exit(main())

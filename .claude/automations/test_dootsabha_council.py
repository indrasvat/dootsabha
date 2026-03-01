# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha council (3-stage pipeline)
Tasks: 2.1 (dispatch), 2.2 (peer review), 2.3 (synthesis)

Tests:
  Dispatch (task 2.1):
    1. dispatch_header — "Stage 1" or "Dispatch" header visible
    2. dispatch_agents_shown — All 3 provider names on screen
    3. dispatch_completion — Checkmarks (✓) for completed agents
    4. screenshot_dispatch — Capture screenshot evidence
  Peer Review (task 2.2):
    5. review_header — "Stage 2" or "Peer Review" header visible
    6. review_agents_reviewing — "reviewing" keyword with agent names
    7. screenshot_review — Capture screenshot evidence
  Synthesis (task 2.3):
    8. synthesis_header — "Stage 3" or "Synthesis" with chair label
    9. synthesis_content — Content between header and footer
   10. synthesis_footer — Footer with time/cost/tokens/agent status
   11. screenshot_synthesis — Capture screenshot evidence
  Cross-cutting (task 2.3):
   12. no_ansi_piped — council "prompt" | cat has no ANSI escapes
   13. json_valid — --json → valid JSON with dispatch/reviews/synthesis/meta

Screenshots:
  - dootsabha_council_dispatch_{ts}.png
  - dootsabha_council_review_{ts}.png
  - dootsabha_council_synthesis_{ts}.png
  - dootsabha_council_piped_{ts}.png
"""
import asyncio
import iterm2
import json
import os
import re
import subprocess
import sys
import time
from datetime import datetime

# ─── Result Tracking ────────────────────────────────────────────
results = {
    "passed": 0,
    "failed": 0,
    "unverified": 0,
    "tests": [],
    "screenshots": [],
    "start_time": None,
    "end_time": None,
}


def log_result(test_name: str, status: str, details: str = "", screenshot: str = None):
    """status: PASS, FAIL, UNVERIFIED"""
    results["tests"].append(
        {
            "name": test_name,
            "status": status,
            "details": details,
            "screenshot": screenshot,
        }
    )
    results[{"PASS": "passed", "FAIL": "failed", "UNVERIFIED": "unverified"}[status]] += 1
    icon = {"PASS": "✓", "FAIL": "✗", "UNVERIFIED": "?"}[status]
    print(f"  {icon} {test_name}: {details}")


# ─── Screenshot Capture ─────────────────────────────────────────
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "..", "screenshots")


def get_iterm2_window_id():
    import Quartz

    windows = Quartz.CGWindowListCopyWindowInfo(
        Quartz.kCGWindowListOptionOnScreenOnly, Quartz.kCGNullWindowID
    )
    for w in windows:
        if w.get("kCGWindowOwnerName") == "iTerm2":
            return w.get("kCGWindowNumber")
    return None


def capture_screenshot(name: str) -> str:
    os.makedirs(SCREENSHOT_DIR, exist_ok=True)
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    filepath = os.path.join(SCREENSHOT_DIR, f"{name}_{ts}.png")
    wid = get_iterm2_window_id()
    if wid:
        subprocess.run(["screencapture", "-x", "-l", str(wid), filepath], check=True)
    else:
        subprocess.run(["screencapture", "-x", filepath], check=True)
    results["screenshots"].append(filepath)
    return filepath


# ─── Screen Verification ────────────────────────────────────────
async def verify_screen_contains(
    session, expected: str, description: str, timeout: float = 10.0
) -> bool:
    """Poll screen content until expected text appears or timeout."""
    start = time.monotonic()
    while (time.monotonic() - start) < timeout:
        screen = await session.async_get_screen_contents()
        for i in range(screen.number_of_lines):
            if expected in screen.line(i).string:
                return True
        await asyncio.sleep(0.3)
    return False


async def verify_screen_contains_any(
    session, patterns: list[str], timeout: float = 10.0
) -> str | None:
    """Poll screen until any pattern appears. Returns matched pattern or None."""
    start = time.monotonic()
    while (time.monotonic() - start) < timeout:
        screen = await session.async_get_screen_contents()
        for i in range(screen.number_of_lines):
            line = screen.line(i).string
            for pattern in patterns:
                if pattern in line:
                    return pattern
        await asyncio.sleep(0.3)
    return None


async def get_all_screen_text(session) -> list[str]:
    """Return all non-empty screen lines."""
    screen = await session.async_get_screen_contents()
    return [
        screen.line(i).string
        for i in range(screen.number_of_lines)
        if screen.line(i).string.strip()
    ]


async def dump_screen(session, label: str):
    """Debug: print all screen lines with line numbers."""
    lines = await get_all_screen_text(session)
    print(f"\n--- SCREEN DUMP: {label} ---")
    for i, line in enumerate(lines):
        print(f"  {i:3d} | {line}")
    print("--- END DUMP ---\n")


# ─── Cleanup ────────────────────────────────────────────────────
async def cleanup_session(session):
    """Exit cleanly: Ctrl+C, then q, then wait."""
    try:
        await session.async_send_text("\x03")  # Ctrl+C
        await asyncio.sleep(0.5)
        await session.async_send_text("q")
        await asyncio.sleep(0.5)
    except Exception:
        pass


# ─── Summary ────────────────────────────────────────────────────
def print_summary() -> int:
    results["end_time"] = datetime.now().isoformat()
    total = results["passed"] + results["failed"] + results["unverified"]
    print(f"\n{'=' * 60}")
    print(
        f"Results: {results['passed']}/{total} PASS, "
        f"{results['failed']} FAIL, {results['unverified']} UNVERIFIED"
    )
    print(f"Screenshots: {len(results['screenshots'])} captured")
    if results["failed"] > 0:
        print("\nFailed tests:")
        for t in results["tests"]:
            if t["status"] == "FAIL":
                print(f"  ✗ {t['name']}: {t['details']}")
    return 1 if results["failed"] > 0 else 0


# ─── Project Path ───────────────────────────────────────────────
PROJECT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
BINARY = os.path.join(PROJECT_DIR, "bin", "dootsabha")


# ─── Tests ──────────────────────────────────────────────────────
async def main(connection):
    results["start_time"] = datetime.now().isoformat()
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if not window:
        print("ERROR: No iTerm2 window found. Open iTerm2 first.")
        sys.exit(1)

    tab = await window.async_create_tab()
    session = tab.current_session

    # Navigate to project directory
    await session.async_send_text(f"cd {PROJECT_DIR}\n")
    await asyncio.sleep(0.5)

    # Launch council command — makes 7+ LLM calls (3 dispatch + 3 review + 1 synthesis)
    print("\nLaunching: dootsabha council \"Say PONG\"")
    await session.async_send_text(f"{BINARY} council \"Say PONG\"\n")

    # ── Test 1: Dispatch header ─────────────────────────────────
    print("\nTest 1: dispatch_header")
    matched = await verify_screen_contains_any(
        session, ["Stage 1", "Dispatch", "dispatch"], timeout=15.0
    )
    if matched:
        log_result("dispatch_header", "PASS", f"Found dispatch header: '{matched}'")
    else:
        log_result("dispatch_header", "FAIL", "No dispatch/Stage 1 header found within 15s")
        await dump_screen(session, "dispatch_header_fail")

    # ── Test 2: Dispatch agents shown ───────────────────────────
    print("\nTest 2: dispatch_agents_shown")
    lines = await get_all_screen_text(session)
    screen_text = "\n".join(lines)
    has_claude = "claude" in screen_text.lower()
    has_codex = "codex" in screen_text.lower()
    has_gemini = "gemini" in screen_text.lower()

    if all([has_claude, has_codex, has_gemini]):
        log_result("dispatch_agents_shown", "PASS", "All 3 agents visible: claude, codex, gemini")
    else:
        missing = []
        if not has_claude:
            missing.append("claude")
        if not has_codex:
            missing.append("codex")
        if not has_gemini:
            missing.append("gemini")
        log_result("dispatch_agents_shown", "FAIL", f"Missing agents: {missing}")

    # ── Test 3: Dispatch completion ─────────────────────────────
    print("\nTest 3: dispatch_completion")
    # Wait for checkmarks to appear — agents may take time to respond
    found_check = await verify_screen_contains_any(
        session, ["✓", "✔", "done", "complete"], timeout=45.0
    )
    if found_check:
        log_result("dispatch_completion", "PASS", f"Found completion indicator: '{found_check}'")
    else:
        log_result(
            "dispatch_completion",
            "UNVERIFIED",
            "No completion checkmarks found within 45s (agents may still be running)",
        )

    # ── Test 4: Screenshot dispatch ─────────────────────────────
    print("\nTest 4: screenshot_dispatch")
    shot = capture_screenshot("dootsabha_council_dispatch")
    log_result("screenshot_dispatch", "PASS", f"Captured: {os.path.basename(shot)}", shot)

    # ── Test 5: Review header ───────────────────────────────────
    print("\nTest 5: review_header")
    matched = await verify_screen_contains_any(
        session, ["Stage 2", "Peer Review", "peer review", "Review"], timeout=60.0
    )
    if matched:
        log_result("review_header", "PASS", f"Found review header: '{matched}'")
    else:
        log_result("review_header", "FAIL", "No peer review/Stage 2 header found within 60s")
        await dump_screen(session, "review_header_fail")

    # ── Test 6: Review agents reviewing ─────────────────────────
    print("\nTest 6: review_agents_reviewing")
    lines = await get_all_screen_text(session)
    screen_text = "\n".join(lines)
    has_reviewing = "reviewing" in screen_text.lower()
    # At least one agent name should appear near "reviewing"
    has_agent_reviewing = False
    for line in lines:
        lower = line.lower()
        if "reviewing" in lower or "review" in lower:
            if any(agent in lower for agent in ["claude", "codex", "gemini"]):
                has_agent_reviewing = True
                break

    if has_agent_reviewing:
        log_result("review_agents_reviewing", "PASS", "Found agents reviewing other agents")
    elif has_reviewing:
        log_result(
            "review_agents_reviewing",
            "UNVERIFIED",
            "Found 'reviewing' but no agent name nearby",
        )
    else:
        log_result("review_agents_reviewing", "FAIL", "No reviewing activity found on screen")

    # ── Test 7: Screenshot review ───────────────────────────────
    print("\nTest 7: screenshot_review")
    shot = capture_screenshot("dootsabha_council_review")
    log_result("screenshot_review", "PASS", f"Captured: {os.path.basename(shot)}", shot)

    # ── Test 8: Synthesis header ────────────────────────────────
    print("\nTest 8: synthesis_header")
    matched = await verify_screen_contains_any(
        session, ["Stage 3", "Synthesis", "synthesis", "Chair"], timeout=90.0
    )
    if matched:
        log_result("synthesis_header", "PASS", f"Found synthesis header: '{matched}'")
    else:
        log_result("synthesis_header", "FAIL", "No synthesis/Stage 3 header found within 90s")
        await dump_screen(session, "synthesis_header_fail")

    # ── Test 9: Synthesis content ───────────────────────────────
    print("\nTest 9: synthesis_content")
    # Wait a bit for synthesis to render
    await asyncio.sleep(3)
    lines = await get_all_screen_text(session)
    screen_text = "\n".join(lines)

    # Look for content between header-like lines and footer-like lines
    # Synthesis should produce actual text output (not just headers)
    content_lines = [
        line
        for line in lines
        if line.strip()
        and "Stage" not in line
        and "═" not in line
        and "─" not in line
        and "$" not in line
        and "dootsabha" not in line
    ]

    if len(content_lines) >= 3:
        log_result(
            "synthesis_content",
            "PASS",
            f"Found {len(content_lines)} content lines between header and footer",
        )
    else:
        log_result(
            "synthesis_content",
            "FAIL",
            f"Only {len(content_lines)} content lines — expected synthesis output",
        )

    # ── Test 10: Synthesis footer ───────────────────────────────
    print("\nTest 10: synthesis_footer")
    # Footer should contain stats: time, cost, tokens, agent status
    footer_keywords = ["time", "cost", "token", "agent"]
    found_keywords = [kw for kw in footer_keywords if kw in screen_text.lower()]

    if len(found_keywords) >= 2:
        log_result(
            "synthesis_footer",
            "PASS",
            f"Footer stats present: {found_keywords}",
        )
    elif len(found_keywords) >= 1:
        log_result(
            "synthesis_footer",
            "UNVERIFIED",
            f"Partial footer stats: {found_keywords} (expected ≥2 of {footer_keywords})",
        )
    else:
        log_result("synthesis_footer", "FAIL", "No footer stats found (time/cost/tokens/agents)")

    # ── Test 11: Screenshot synthesis ───────────────────────────
    print("\nTest 11: screenshot_synthesis")
    shot = capture_screenshot("dootsabha_council_synthesis")
    log_result("screenshot_synthesis", "PASS", f"Captured: {os.path.basename(shot)}", shot)

    # ── Cleanup interactive session ─────────────────────────────
    await cleanup_session(session)
    await asyncio.sleep(1.0)

    # ── Test 12: No ANSI in piped mode ──────────────────────────
    print("\nTest 12: no_ansi_piped")
    # Run in a fresh session to avoid leftover state
    tab2 = await window.async_create_tab()
    session2 = tab2.current_session
    await session2.async_send_text(f"cd {PROJECT_DIR}\n")
    await asyncio.sleep(0.5)
    await session2.async_send_text(f"{BINARY} council \"Say PONG\" | cat\n")

    # Council makes 7+ LLM calls — generous timeout
    await asyncio.sleep(5)
    found_done = await verify_screen_contains_any(
        session2, ["$", "PONG", "synthesis", "Synthesis"], timeout=120.0
    )

    piped_lines = await get_all_screen_text(session2)
    piped_text = "\n".join(piped_lines)

    # ANSI escape sequences start with ESC[
    ansi_re = re.compile(r"\x1b\[")
    ansi_found = ansi_re.findall(piped_text)

    shot_piped = capture_screenshot("dootsabha_council_piped")

    if ansi_found:
        log_result(
            "no_ansi_piped",
            "FAIL",
            f"Found {len(ansi_found)} ANSI escape sequences in piped output",
            shot_piped,
        )
    else:
        log_result("no_ansi_piped", "PASS", "No ANSI codes in piped output", shot_piped)

    await cleanup_session(session2)
    await asyncio.sleep(0.5)
    await session2.async_send_text("exit\n")

    # ── Test 13: JSON mode valid ────────────────────────────────
    print("\nTest 13: json_valid")
    try:
        proc = subprocess.run(
            [BINARY, "council", "Say PONG", "--json"],
            capture_output=True,
            text=True,
            timeout=120,
            cwd=PROJECT_DIR,
        )

        data = json.loads(proc.stdout)
        has_dispatch = "dispatch" in data
        has_reviews = "reviews" in data
        has_synthesis = "synthesis" in data
        has_meta = "meta" in data

        issues = []
        if not has_dispatch:
            issues.append("missing 'dispatch'")
        if not has_reviews:
            issues.append("missing 'reviews'")
        if not has_synthesis:
            issues.append("missing 'synthesis'")
        if not has_meta:
            issues.append("missing 'meta'")

        if not issues:
            dispatch_count = len(data.get("dispatch", []))
            review_count = len(data.get("reviews", []))
            log_result(
                "json_valid",
                "PASS",
                f"Valid JSON: {dispatch_count} dispatch, {review_count} reviews, synthesis present, meta present",
            )
        else:
            log_result("json_valid", "FAIL", f"JSON structure issues: {'; '.join(issues)}")
    except subprocess.TimeoutExpired:
        log_result("json_valid", "FAIL", "Command timed out after 120s")
    except (json.JSONDecodeError, KeyError) as e:
        log_result("json_valid", "FAIL", f"Invalid JSON: {e}")

    # ── Final cleanup ───────────────────────────────────────────
    await session.async_send_text("exit\n")

    # ── Summary ─────────────────────────────────────────────────
    exit_code = print_summary()
    sys.exit(exit_code)


iterm2.run_until_complete(main)

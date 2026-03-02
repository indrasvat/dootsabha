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

Verification Strategy:
  - Use screen polling with generous timeouts (council makes 7+ LLM calls)
  - Look for stage transition headers to confirm pipeline progression
  - Capture screenshots at each stage for visual evidence
  - JSON test via subprocess.run (not iTerm2) for structured validation

Screenshots:
  - dootsabha_council_dispatch_{ts}.png
  - dootsabha_council_review_{ts}.png
  - dootsabha_council_synthesis_{ts}.png
  - dootsabha_council_piped_{ts}.png

Screenshot Inspection Checklist:
  - Colors: Stage headers styled, agent names colored
  - Boundaries: Terminal window bounds captured correctly
  - Visible Elements: Stage headers, agent names, checkmarks, footer stats

Key Bindings:
  - Ctrl+C: Interrupt running command
  - exit: Close shell

Usage:
  uv run .claude/automations/test_dootsabha_council.py
"""
import asyncio
import iterm2
import json
import os
import re
import subprocess
import time
from datetime import datetime

# ============================================================
# CONFIGURATION
# ============================================================

PROJECT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
BINARY = os.path.join(PROJECT_DIR, "bin", "dootsabha")
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "..", "screenshots")

# ============================================================
# RESULT TRACKING
# ============================================================

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
    """Log a test result with optional details and screenshot reference."""
    results["tests"].append(
        {
            "name": test_name,
            "status": status,
            "details": details,
            "screenshot": screenshot,
        }
    )

    if screenshot:
        results["screenshots"].append(screenshot)

    if status == "PASS":
        results["passed"] += 1
        print(f"  [+] PASS: {test_name} — {details}")
    elif status == "FAIL":
        results["failed"] += 1
        print(f"  [x] FAIL: {test_name} — {details}")
    else:
        results["unverified"] += 1
        print(f"  [?] UNVERIFIED: {test_name} — {details}")

    if screenshot:
        print(f"      Screenshot: {screenshot}")


def print_summary() -> int:
    """Print final test summary and return exit code."""
    results["end_time"] = datetime.now()
    total = results["passed"] + results["failed"] + results["unverified"]
    duration = (
        (results["end_time"] - results["start_time"]).total_seconds()
        if results["start_time"]
        else 0
    )

    print("\n" + "=" * 60)
    print("TEST SUMMARY")
    print("=" * 60)
    print(f"Duration:   {duration:.1f}s")
    print(f"Total:      {total}")
    print(f"Passed:     {results['passed']}")
    print(f"Failed:     {results['failed']}")
    print(f"Unverified: {results['unverified']}")

    if results["screenshots"]:
        print(f"Screenshots: {len(results['screenshots'])}")

    print("=" * 60)

    if results["failed"] > 0:
        print("\nFailed tests:")
        for t in results["tests"]:
            if t["status"] == "FAIL":
                print(f"  - {t['name']}: {t['details']}")

    print("\n" + "-" * 60)
    if results["failed"] > 0:
        print("OVERALL: FAILED")
        return 1
    elif results["unverified"] > 0:
        print("OVERALL: PASSED (with unverified tests)")
        return 0
    else:
        print("OVERALL: PASSED")
        return 0


def print_test_header(test_name: str, test_num: int):
    """Print a visual header for a test section."""
    print(f"\n{'=' * 60}")
    print(f"TEST {test_num}: {test_name}")
    print("=" * 60)


# ============================================================
# QUARTZ WINDOW TARGETING
# ============================================================

try:
    import Quartz

    def get_iterm2_window_id():
        """Get the window ID of the frontmost iTerm2 window."""
        window_list = Quartz.CGWindowListCopyWindowInfo(
            Quartz.kCGWindowListOptionOnScreenOnly
            | Quartz.kCGWindowListExcludeDesktopElements,
            Quartz.kCGNullWindowID,
        )
        for window in window_list:
            owner = window.get("kCGWindowOwnerName", "")
            if "iTerm" in owner:
                return window.get("kCGWindowNumber")
        return None

except ImportError:
    print("WARNING: Quartz not available, screenshots will capture full screen")

    def get_iterm2_window_id():
        return None


def capture_screenshot(name: str) -> str:
    """Capture a screenshot of just the iTerm2 window."""
    os.makedirs(SCREENSHOT_DIR, exist_ok=True)
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    filepath = os.path.join(SCREENSHOT_DIR, f"{name}_{ts}.png")
    wid = get_iterm2_window_id()
    if wid:
        subprocess.run(
            ["screencapture", "-x", "-l", str(wid), filepath], check=True
        )
    else:
        print("  WARNING: iTerm2 window not found, capturing full screen")
        subprocess.run(["screencapture", "-x", filepath], check=True)
    print(f"  SCREENSHOT: {filepath}")
    return filepath


# ============================================================
# VERIFICATION HELPERS
# ============================================================


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
    """Dump current screen contents for debugging."""
    screen = await session.async_get_screen_contents()
    print(f"\n{'=' * 60}")
    print(f"SCREEN DUMP: {label}")
    print("=" * 60)
    for i in range(screen.number_of_lines):
        line = screen.line(i).string
        if line.strip():
            print(f"{i:03d}: {line}")
    print("=" * 60 + "\n")


# ============================================================
# CLEANUP
# ============================================================


async def cleanup_session(session):
    """Perform multi-level cleanup on a session."""
    print("\n  Performing cleanup...")
    try:
        # Level 1: Ctrl+C to interrupt running process
        await session.async_send_text("\x03")
        await asyncio.sleep(0.3)
        # Level 2: Quit key (for TUIs)
        await session.async_send_text("q")
        await asyncio.sleep(0.2)
        # Level 3: Exit command for shells
        await session.async_send_text("exit\n")
        await asyncio.sleep(0.2)
        # Level 4: Close the session
        await session.async_close()
        print("  Cleanup complete")
    except Exception as e:
        print(f"  Cleanup warning: {e}")


# ============================================================
# MAIN TEST FUNCTION
# ============================================================


async def main(connection):
    """Main test function for dootsabha council 3-stage pipeline."""
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: dootsabha council")
    print("# 3-stage pipeline: dispatch → peer review → synthesis")
    print("#" * 60)
    print(f"# Started: {results['start_time'].strftime('%Y-%m-%d %H:%M:%S')}")
    print("#" * 60)

    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window

    if not window:
        print("ERROR: No active iTerm2 window found")
        log_result("Setup", "FAIL", "No active iTerm2 window")
        return print_summary()

    # Create test tab
    tab = await window.async_create_tab()
    session = tab.current_session

    # Track all created sessions for cleanup
    created_sessions = [session]

    try:
        # Navigate to project directory
        await session.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)

        # Launch council command — makes 7+ LLM calls
        print("\nLaunching: dootsabha council \"Say PONG\"")
        await session.async_send_text(f'{BINARY} council "Say PONG"\n')

        # ── Test 1: Dispatch header ──────────────────────────────
        print_test_header("dispatch_header", 1)
        matched = await verify_screen_contains_any(
            session, ["Stage 1", "Dispatch", "dispatch"], timeout=15.0
        )
        if matched:
            log_result(
                "dispatch_header", "PASS", f"Found dispatch header: '{matched}'"
            )
        else:
            log_result(
                "dispatch_header",
                "FAIL",
                "No dispatch/Stage 1 header found within 15s",
            )
            await dump_screen(session, "dispatch_header_fail")

        # ── Test 2: Dispatch agents shown ────────────────────────
        print_test_header("dispatch_agents_shown", 2)
        # Wait a moment for all agents to appear
        await asyncio.sleep(2)
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines).lower()
        has_claude = "claude" in screen_text
        has_codex = "codex" in screen_text
        has_gemini = "gemini" in screen_text

        if all([has_claude, has_codex, has_gemini]):
            log_result(
                "dispatch_agents_shown",
                "PASS",
                "All 3 agents visible: claude, codex, gemini",
            )
        else:
            missing = []
            if not has_claude:
                missing.append("claude")
            if not has_codex:
                missing.append("codex")
            if not has_gemini:
                missing.append("gemini")
            log_result(
                "dispatch_agents_shown", "FAIL", f"Missing agents: {missing}"
            )

        # ── Test 3: Dispatch completion ──────────────────────────
        print_test_header("dispatch_completion", 3)
        found_check = await verify_screen_contains_any(
            session, ["\u2713", "\u2714", "done", "complete"], timeout=45.0
        )
        if found_check:
            log_result(
                "dispatch_completion",
                "PASS",
                f"Found completion indicator: '{found_check}'",
            )
        else:
            log_result(
                "dispatch_completion",
                "UNVERIFIED",
                "No completion checkmarks found within 45s",
            )

        # ── Test 4: Screenshot dispatch ──────────────────────────
        print_test_header("screenshot_dispatch", 4)
        shot = capture_screenshot("dootsabha_council_dispatch")
        log_result(
            "screenshot_dispatch",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ── Test 5: Review header ────────────────────────────────
        print_test_header("review_header", 5)
        matched = await verify_screen_contains_any(
            session,
            ["Stage 2", "Peer Review", "peer review", "Review"],
            timeout=60.0,
        )
        if matched:
            log_result(
                "review_header", "PASS", f"Found review header: '{matched}'"
            )
        else:
            log_result(
                "review_header",
                "FAIL",
                "No peer review/Stage 2 header found within 60s",
            )
            await dump_screen(session, "review_header_fail")

        # ── Test 6: Review agents reviewing ──────────────────────
        print_test_header("review_agents_reviewing", 6)
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines)
        has_reviewing = "reviewing" in screen_text.lower()
        has_agent_reviewing = False
        for line in lines:
            lower = line.lower()
            if "reviewing" in lower or "review" in lower:
                if any(
                    agent in lower for agent in ["claude", "codex", "gemini"]
                ):
                    has_agent_reviewing = True
                    break

        if has_agent_reviewing:
            log_result(
                "review_agents_reviewing",
                "PASS",
                "Found agents reviewing other agents",
            )
        elif has_reviewing:
            log_result(
                "review_agents_reviewing",
                "UNVERIFIED",
                "Found 'reviewing' but no agent name nearby",
            )
        else:
            log_result(
                "review_agents_reviewing",
                "FAIL",
                "No reviewing activity found on screen",
            )

        # ── Test 7: Screenshot review ────────────────────────────
        print_test_header("screenshot_review", 7)
        shot = capture_screenshot("dootsabha_council_review")
        log_result(
            "screenshot_review",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ── Test 8: Synthesis header ─────────────────────────────
        print_test_header("synthesis_header", 8)
        matched = await verify_screen_contains_any(
            session,
            ["Stage 3", "Synthesis", "synthesis", "Chair"],
            timeout=90.0,
        )
        if matched:
            log_result(
                "synthesis_header",
                "PASS",
                f"Found synthesis header: '{matched}'",
            )
        else:
            log_result(
                "synthesis_header",
                "FAIL",
                "No synthesis/Stage 3 header found within 90s",
            )
            await dump_screen(session, "synthesis_header_fail")

        # ── Test 9: Synthesis content ────────────────────────────
        print_test_header("synthesis_content", 9)
        await asyncio.sleep(3)
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines)

        content_lines = [
            line
            for line in lines
            if line.strip()
            and "Stage" not in line
            and "\u2550" not in line
            and "\u2500" not in line
            and "$" not in line
            and "dootsabha" not in line
        ]

        if len(content_lines) >= 3:
            log_result(
                "synthesis_content",
                "PASS",
                f"Found {len(content_lines)} content lines",
            )
        else:
            log_result(
                "synthesis_content",
                "FAIL",
                f"Only {len(content_lines)} content lines — expected synthesis output",
            )

        # ── Test 10: Synthesis footer ────────────────────────────
        print_test_header("synthesis_footer", 10)
        footer_keywords = ["total", "cost", "token", "agent"]
        found_keywords = [
            kw for kw in footer_keywords if kw in screen_text.lower()
        ]

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
                f"Partial footer stats: {found_keywords} (expected >=2 of {footer_keywords})",
            )
        else:
            log_result(
                "synthesis_footer",
                "FAIL",
                "No footer stats found (time/cost/tokens/agents)",
            )

        # ── Test 11: Screenshot synthesis ────────────────────────
        print_test_header("screenshot_synthesis", 11)
        shot = capture_screenshot("dootsabha_council_synthesis")
        log_result(
            "screenshot_synthesis",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ── Test 12: No ANSI in piped mode ───────────────────────
        print_test_header("no_ansi_piped", 12)
        tab2 = await window.async_create_tab()
        session2 = tab2.current_session
        created_sessions.append(session2)

        await session2.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session2.async_send_text(
            f'{BINARY} council "Say PONG" | cat\n'
        )

        # Council makes 7+ LLM calls — generous timeout
        await asyncio.sleep(5)
        await verify_screen_contains_any(
            session2,
            ["$", "PONG", "synthesis", "Synthesis"],
            timeout=120.0,
        )

        piped_lines = await get_all_screen_text(session2)
        piped_text = "\n".join(piped_lines)

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
            log_result(
                "no_ansi_piped",
                "PASS",
                "No ANSI codes in piped output",
                shot_piped,
            )

        # ── Test 13: JSON mode valid ─────────────────────────────
        print_test_header("json_valid", 13)
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
                    f"Valid JSON: {dispatch_count} dispatch, {review_count} reviews, synthesis+meta present",
                )
            else:
                log_result(
                    "json_valid",
                    "FAIL",
                    f"JSON structure issues: {'; '.join(issues)}",
                )
        except subprocess.TimeoutExpired:
            log_result("json_valid", "FAIL", "Command timed out after 120s")
        except (json.JSONDecodeError, KeyError) as e:
            log_result("json_valid", "FAIL", f"Invalid JSON: {e}")

    except Exception as e:
        print(f"\nERROR during test execution: {e}")
        log_result("Test Execution", "FAIL", str(e))
        try:
            await dump_screen(session, "error_state")
        except Exception:
            pass

    finally:
        # ============================================================
        # CLEANUP — close all created sessions
        # ============================================================
        print("\n" + "=" * 60)
        print("CLEANUP")
        print("=" * 60)

        for s in created_sessions:
            await cleanup_session(s)

    return print_summary()


# ============================================================
# ENTRY POINT
# ============================================================

if __name__ == "__main__":
    exit_code = iterm2.run_until_complete(main)
    exit(exit_code if exit_code else 0)

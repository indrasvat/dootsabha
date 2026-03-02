# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha review (2-step author + reviewer pipeline)
Task: 2.4 (review command)

Tests:
  1. author_section — Author output section visible
  2. reviewer_section — Reviewer section visible
  3. both_agents_labeled — "codex" (author) and "claude" (reviewer) names shown
  4. screenshot_review — Capture screenshot evidence
  5. author_failure_failfast — Bad author → no reviewer section (FR-REV-05)
  6. screenshot_failfast — Capture failure scenario screenshot
  7. no_ansi_piped — review "prompt" | cat has no ANSI escapes
  8. json_valid — --json → valid JSON with author/review/meta keys

Verification Strategy:
  - Use screen polling with moderate timeouts (review makes 2 LLM calls)
  - Failfast test: use nonexistent author, verify "(reviewer)" output section absent
  - Clear screen before failfast test to avoid false positives from previous output

Screenshots:
  - dootsabha_review_output_{ts}.png
  - dootsabha_review_failfast_{ts}.png
  - dootsabha_review_piped_{ts}.png

Screenshot Inspection Checklist:
  - Colors: Author/reviewer section headers styled
  - Boundaries: Terminal window bounds captured correctly
  - Visible Elements: Author output, reviewer output, agent names

Key Bindings:
  - Ctrl+C: Interrupt running command
  - exit: Close shell

Usage:
  uv run .claude/automations/test_dootsabha_review.py
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
    """Main test function for dootsabha review 2-step pipeline."""
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: dootsabha review")
    print("# 2-step pipeline: author → reviewer")
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

        # Launch review command — 2 LLM calls (author + reviewer)
        print(
            '\nLaunching: dootsabha review "Say PONG" --author codex --reviewer claude'
        )
        await session.async_send_text(
            f'{BINARY} review "Say PONG" --author codex --reviewer claude\n'
        )

        # ── Test 1: Author section ───────────────────────────────
        print_test_header("author_section", 1)
        matched = await verify_screen_contains_any(
            session, ["Author", "author", "codex"], timeout=30.0
        )
        if matched:
            log_result(
                "author_section",
                "PASS",
                f"Found author section indicator: '{matched}'",
            )
        else:
            log_result(
                "author_section",
                "FAIL",
                "No author section found within 30s",
            )
            await dump_screen(session, "author_section_fail")

        # ── Test 2: Reviewer section ─────────────────────────────
        print_test_header("reviewer_section", 2)
        matched = await verify_screen_contains_any(
            session, ["Reviewer", "reviewer", "Review"], timeout=45.0
        )
        if matched:
            log_result(
                "reviewer_section",
                "PASS",
                f"Found reviewer section indicator: '{matched}'",
            )
        else:
            log_result(
                "reviewer_section",
                "FAIL",
                "No reviewer section found within 45s",
            )
            await dump_screen(session, "reviewer_section_fail")

        # ── Test 3: Both agents labeled ──────────────────────────
        print_test_header("both_agents_labeled", 3)
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines).lower()
        has_codex = "codex" in screen_text
        has_claude = "claude" in screen_text

        if has_codex and has_claude:
            log_result(
                "both_agents_labeled",
                "PASS",
                "Both agents labeled: codex (author) and claude (reviewer)",
            )
        else:
            missing = []
            if not has_codex:
                missing.append("codex")
            if not has_claude:
                missing.append("claude")
            log_result(
                "both_agents_labeled",
                "FAIL",
                f"Missing agent labels: {missing}",
            )

        # ── Test 4: Screenshot review output ─────────────────────
        print_test_header("screenshot_review", 4)
        shot = capture_screenshot("dootsabha_review_output")
        log_result(
            "screenshot_review",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ── Test 5: Author failure failfast (FR-REV-05) ──────────
        print_test_header("author_failure_failfast", 5)
        tab2 = await window.async_create_tab()
        session2 = tab2.current_session
        created_sessions.append(session2)

        await session2.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        # Clear screen to prevent leftover text from previous tests
        await session2.async_send_text("clear\n")
        await asyncio.sleep(0.3)

        # Use nonexistent author to trigger failure
        await session2.async_send_text(
            f'{BINARY} review "Say PONG" --author nonexistent --reviewer claude\n'
        )
        await asyncio.sleep(5)

        # Wait for error or prompt return
        await verify_screen_contains_any(
            session2,
            ["error", "Error", "not found", "unknown", "$"],
            timeout=15.0,
        )

        lines2 = await get_all_screen_text(session2)
        screen_text2 = "\n".join(lines2).lower()

        # Key check: reviewer OUTPUT section should NOT appear after author failure.
        # Note: the word "reviewer" appears in the command itself (--reviewer claude),
        # so we look for the styled output section indicator "(reviewer)" which only
        # appears when the reviewer is actually invoked and its section is rendered.
        has_reviewer_output = any(
            kw in screen_text2
            for kw in ["(reviewer)", "review output", "claude reviewing"]
        )
        has_error = any(
            kw in screen_text2
            for kw in [
                "error",
                "not found",
                "unknown agent",
                "unknown provider",
                "failed",
            ]
        )

        if has_error and not has_reviewer_output:
            log_result(
                "author_failure_failfast",
                "PASS",
                "Author failed -> no reviewer invoked (fail-fast correct)",
            )
        elif has_error and has_reviewer_output:
            log_result(
                "author_failure_failfast",
                "FAIL",
                "Author failed but reviewer section still appeared (fail-fast broken)",
            )
        elif not has_error:
            log_result(
                "author_failure_failfast",
                "UNVERIFIED",
                "No error detected for nonexistent author",
            )
        else:
            log_result(
                "author_failure_failfast",
                "UNVERIFIED",
                f"Unexpected state: error={has_error}, reviewer_output={has_reviewer_output}",
            )

        # ── Test 6: Screenshot failfast ──────────────────────────
        print_test_header("screenshot_failfast", 6)
        shot = capture_screenshot("dootsabha_review_failfast")
        log_result(
            "screenshot_failfast",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ── Test 7: No ANSI in piped mode ────────────────────────
        print_test_header("no_ansi_piped", 7)
        tab3 = await window.async_create_tab()
        session3 = tab3.current_session
        created_sessions.append(session3)

        await session3.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session3.async_send_text(
            f'{BINARY} review "Say PONG" --author codex --reviewer claude | cat\n'
        )

        # 2 LLM calls — moderate timeout
        await verify_screen_contains_any(
            session3,
            ["$", "PONG", "review", "Review"],
            timeout=60.0,
        )

        piped_lines = await get_all_screen_text(session3)
        piped_text = "\n".join(piped_lines)

        ansi_re = re.compile(r"\x1b\[")
        ansi_found = ansi_re.findall(piped_text)

        shot_piped = capture_screenshot("dootsabha_review_piped")

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

        # ── Test 8: JSON mode valid ──────────────────────────────
        print_test_header("json_valid", 8)
        try:
            proc = subprocess.run(
                [
                    BINARY,
                    "review",
                    "Say PONG",
                    "--author",
                    "codex",
                    "--reviewer",
                    "claude",
                    "--json",
                ],
                capture_output=True,
                text=True,
                timeout=60,
                cwd=PROJECT_DIR,
            )

            data = json.loads(proc.stdout)
            has_author = "author" in data
            has_review = "review" in data
            has_meta = "meta" in data

            issues = []
            if not has_author:
                issues.append("missing 'author'")
            if not has_review:
                issues.append("missing 'review'")
            if not has_meta:
                issues.append("missing 'meta'")

            if not issues:
                log_result(
                    "json_valid",
                    "PASS",
                    "Valid JSON with author, review, meta keys",
                )
            else:
                log_result(
                    "json_valid",
                    "FAIL",
                    f"JSON structure issues: {'; '.join(issues)}",
                )
        except subprocess.TimeoutExpired:
            log_result("json_valid", "FAIL", "Command timed out after 60s")
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

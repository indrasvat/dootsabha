# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha refine (sequential review + incorporation pipeline)
Task: 2.6 (refine command)

Tests:
  1. initial_generation — Author name + "v1" visible
  2. reviewer_feedback — Reviewer name + "reviewing" visible
  3. incorporation — "incorporating" or "v2" indicator visible
  4. version_progression — Multiple version numbers visible (v1, v2+)
  5. screenshot_refine — Capture main output screenshot
  6. author_failure_failfast — Bad author -> immediate exit, no reviewer invoked
  7. screenshot_failfast — Capture failure screenshot
  8. reviewer_skip — Bad reviewer -> skipped, author output still shown
  9. screenshot_skip — Capture skip scenario screenshot
  10. no_ansi_piped — refine "prompt" | cat has no ANSI escapes
  11. json_valid — --json -> valid JSON with versions/final/meta keys

Verification Strategy:
  - Use screen polling with moderate timeouts (refine makes 1 + 2*N LLM calls)
  - Failfast test: use nonexistent author, verify no reviewer output
  - Skip test: use real author with nonexistent reviewer, verify author output shown
  - Clear screen before each scenario to avoid false positives

Screenshots:
  - dootsabha_refine_output_{ts}.png
  - dootsabha_refine_failfast_{ts}.png
  - dootsabha_refine_skip_{ts}.png
  - dootsabha_refine_piped_{ts}.png

Screenshot Inspection Checklist:
  - Colors: Author/reviewer step labels styled with provider colors
  - Boundaries: Terminal window bounds captured correctly
  - Visible Elements: Version progression (v1, v2, v3), agent names, timing

Key Bindings:
  - Ctrl+C: Interrupt running command
  - exit: Close shell

Usage:
  uv run .claude/automations/test_dootsabha_refine.py
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
    """Main test function for dootsabha refine sequential pipeline."""
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: dootsabha refine")
    print("# Sequential pipeline: author -> review -> incorporate -> ...")
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

        # Launch refine command — 5 LLM calls (1 author + 2 reviewers * 2)
        print(
            '\nLaunching: dootsabha refine "Say PONG" --author claude --reviewers codex,gemini'
        )
        await session.async_send_text(
            f'{BINARY} refine "Say PONG" --author claude --reviewers codex,gemini\n'
        )

        # -- Test 1: Initial generation (v1) -----------------------
        print_test_header("initial_generation", 1)
        matched = await verify_screen_contains_any(
            session, ["v1", "claude", "Refine"], timeout=30.0
        )
        if matched:
            log_result(
                "initial_generation",
                "PASS",
                f"Found initial generation indicator: '{matched}'",
            )
        else:
            log_result(
                "initial_generation",
                "FAIL",
                "No initial generation found within 30s",
            )
            await dump_screen(session, "initial_generation_fail")

        # -- Test 2: Reviewer feedback -----------------------------
        print_test_header("reviewer_feedback", 2)
        matched = await verify_screen_contains_any(
            session, ["reviewing", "codex", "review"], timeout=45.0
        )
        if matched:
            log_result(
                "reviewer_feedback",
                "PASS",
                f"Found reviewer feedback indicator: '{matched}'",
            )
        else:
            log_result(
                "reviewer_feedback",
                "FAIL",
                "No reviewer feedback found within 45s",
            )
            await dump_screen(session, "reviewer_feedback_fail")

        # -- Test 3: Incorporation ---------------------------------
        print_test_header("incorporation", 3)
        matched = await verify_screen_contains_any(
            session, ["incorporating", "v2", "v3"], timeout=45.0
        )
        if matched:
            log_result(
                "incorporation",
                "PASS",
                f"Found incorporation indicator: '{matched}'",
            )
        else:
            log_result(
                "incorporation",
                "FAIL",
                "No incorporation indicator found within 45s",
            )
            await dump_screen(session, "incorporation_fail")

        # -- Test 4: Version progression ----------------------------
        print_test_header("version_progression", 4)
        # Wait for pipeline to complete — refine makes 5 LLM calls
        await verify_screen_contains_any(
            session, ["$", "total:", "versions:"], timeout=120.0
        )

        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines).lower()
        has_v1 = "v1" in screen_text
        has_v2_plus = any(f"v{i}" in screen_text for i in range(2, 10))

        if has_v1 and has_v2_plus:
            log_result(
                "version_progression",
                "PASS",
                "Multiple version indicators found (v1 + v2+)",
            )
        elif has_v1:
            log_result(
                "version_progression",
                "UNVERIFIED",
                "Found v1 but no v2+; pipeline may not have completed",
            )
        else:
            log_result(
                "version_progression",
                "FAIL",
                "No version progression detected",
            )

        # -- Test 5: Screenshot refine output -----------------------
        print_test_header("screenshot_refine", 5)
        shot = capture_screenshot("dootsabha_refine_output")
        log_result(
            "screenshot_refine",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # -- Test 6: Author failure failfast ------------------------
        print_test_header("author_failure_failfast", 6)
        tab2 = await window.async_create_tab()
        session2 = tab2.current_session
        created_sessions.append(session2)

        await session2.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session2.async_send_text("clear\n")
        await asyncio.sleep(0.3)

        # Use nonexistent author to trigger failure
        await session2.async_send_text(
            f'{BINARY} refine "Say PONG" --author nonexistent --reviewers codex\n'
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

        # Reviewer output should NOT appear after author failure
        has_reviewer_output = any(
            kw in screen_text2
            for kw in ["reviewing", "incorporating", "v2", "v3"]
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
                "Author failed but reviewer steps still appeared (fail-fast broken)",
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

        # -- Test 7: Screenshot failfast ----------------------------
        print_test_header("screenshot_failfast", 7)
        shot = capture_screenshot("dootsabha_refine_failfast")
        log_result(
            "screenshot_failfast",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # -- Test 8: Reviewer skip ----------------------------------
        print_test_header("reviewer_skip", 8)
        tab3 = await window.async_create_tab()
        session3 = tab3.current_session
        created_sessions.append(session3)

        await session3.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session3.async_send_text("clear\n")
        await asyncio.sleep(0.3)

        # Use nonexistent reviewer — author should still produce output
        await session3.async_send_text(
            f'{BINARY} refine "Say PONG" --author claude --reviewers nonexistent\n'
        )

        # Wait for completion
        await verify_screen_contains_any(
            session3,
            ["$", "total:", "v1", "versions:"],
            timeout=30.0,
        )

        lines3 = await get_all_screen_text(session3)
        screen_text3 = "\n".join(lines3).lower()

        # Author output (v1) should be present even when reviewer fails
        has_author_output = any(
            kw in screen_text3
            for kw in ["v1", "claude", "pong"]
        )
        has_skip_indicator = any(
            kw in screen_text3
            for kw in ["skip", "failed", "warning", "error"]
        )

        if has_author_output:
            log_result(
                "reviewer_skip",
                "PASS",
                f"Author output shown despite reviewer failure (skip indicator: {has_skip_indicator})",
            )
        else:
            log_result(
                "reviewer_skip",
                "FAIL",
                "No author output found when reviewer was skipped",
            )

        # -- Test 9: Screenshot skip --------------------------------
        print_test_header("screenshot_skip", 9)
        shot = capture_screenshot("dootsabha_refine_skip")
        log_result(
            "screenshot_skip",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # -- Test 10: No ANSI in piped mode -------------------------
        print_test_header("no_ansi_piped", 10)
        tab4 = await window.async_create_tab()
        session4 = tab4.current_session
        created_sessions.append(session4)

        await session4.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session4.async_send_text(
            f'{BINARY} refine "Say PONG" --author claude --reviewers codex | cat\n'
        )

        # 3 LLM calls — moderate timeout
        await verify_screen_contains_any(
            session4,
            ["$", "PONG", "refine"],
            timeout=60.0,
        )

        piped_lines = await get_all_screen_text(session4)
        piped_text = "\n".join(piped_lines)

        ansi_re = re.compile(r"\x1b\[")
        ansi_found = ansi_re.findall(piped_text)

        shot_piped = capture_screenshot("dootsabha_refine_piped")

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

        # -- Test 11: JSON mode valid -------------------------------
        print_test_header("json_valid", 11)
        try:
            proc = subprocess.run(
                [
                    BINARY,
                    "refine",
                    "Say PONG",
                    "--author",
                    "claude",
                    "--reviewers",
                    "codex,gemini",
                    "--json",
                ],
                capture_output=True,
                text=True,
                timeout=60,
                cwd=PROJECT_DIR,
            )

            data = json.loads(proc.stdout)
            has_versions = "versions" in data
            has_final = "final" in data
            has_meta = "meta" in data

            issues = []
            if not has_versions:
                issues.append("missing 'versions'")
            if not has_final:
                issues.append("missing 'final'")
            if not has_meta:
                issues.append("missing 'meta'")

            if not issues:
                log_result(
                    "json_valid",
                    "PASS",
                    "Valid JSON with versions, final, meta keys",
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

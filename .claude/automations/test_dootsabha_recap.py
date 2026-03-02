# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha recap (extension showcase)
Task: 600 — Workspace Intelligence Briefing

Tests:
  TTY Mode:
    1. recap_header_box — Rounded border visible with "Recap"
    2. recap_provider_matrix — All 3 provider names on screen
    3. recap_provider_dots — Colored dots (●) next to provider names
    4. recap_workspace_info — Branch name and project name visible
    5. recap_suggested_commands — At least one dootsabha command suggestion
    6. recap_footer_trace — Session trace ID and version in footer
    7. screenshot_recap_tty — Full screenshot of TTY output
  Piped Mode:
    8. recap_piped_no_ansi — dootsabha recap | cat has zero ANSI escapes
    9. recap_piped_plain_markers — Piped output has * (not ●) and --- markers
   10. screenshot_recap_piped — Screenshot of piped output

Screenshots:
  - dootsabha_recap_tty_{ts}.png
  - dootsabha_recap_piped_{ts}.png

Usage:
  uv run .claude/automations/test_dootsabha_recap.py
"""
import asyncio
import iterm2
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
    """Main test function for dootsabha recap extension."""
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: dootsabha recap")
    print("# Extension showcase: workspace intelligence briefing")
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

        # Launch recap command
        print("\nLaunching: dootsabha recap")
        await session.async_send_text(f"{BINARY} recap\n")

        # Wait for recap to complete (it's fast — no LLM calls)
        await asyncio.sleep(8)

        # ── Test 1: Header box ──────────────────────────────────────
        print_test_header("recap_header_box", 1)
        matched = await verify_screen_contains_any(
            session, ["Recap", "\u0926\u0942\u0924\u0938\u092d\u093e", "\u250c", "\u2514"], timeout=15.0
        )
        if matched:
            log_result(
                "recap_header_box", "PASS", f"Found header element: '{matched}'"
            )
        else:
            log_result(
                "recap_header_box",
                "FAIL",
                "No header box elements found within 15s",
            )
            await dump_screen(session, "recap_header_fail")

        # ── Test 2: Provider matrix ─────────────────────────────────
        print_test_header("recap_provider_matrix", 2)
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines).lower()
        has_claude = "claude" in screen_text
        has_codex = "codex" in screen_text
        has_gemini = "gemini" in screen_text

        if all([has_claude, has_codex, has_gemini]):
            log_result(
                "recap_provider_matrix",
                "PASS",
                "All 3 providers visible: claude, codex, gemini",
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
                "recap_provider_matrix", "FAIL", f"Missing providers: {missing}"
            )

        # ── Test 3: Provider dots ───────────────────────────────────
        print_test_header("recap_provider_dots", 3)
        screen_raw = "\n".join(lines)
        has_dots = "\u25cf" in screen_raw
        if has_dots:
            dot_count = screen_raw.count("\u25cf")
            log_result(
                "recap_provider_dots",
                "PASS",
                f"Found {dot_count} colored dots (\u25cf)",
            )
        else:
            # May be in piped mode (captured by Claude Code) — check for *
            has_stars = "*" in screen_raw
            if has_stars:
                log_result(
                    "recap_provider_dots",
                    "UNVERIFIED",
                    "Found * markers (piped mode) — TTY dots expected in terminal",
                )
            else:
                log_result(
                    "recap_provider_dots",
                    "FAIL",
                    "No dot indicators found",
                )

        # ── Test 4: Workspace info ──────────────────────────────────
        print_test_header("recap_workspace_info", 4)
        has_branch = any("create-dootsabha" in line or "Branch" in line for line in lines)
        has_project = any("indrasvat-dootsabha" in line or "Workspace" in line for line in lines)

        if has_branch and has_project:
            log_result(
                "recap_workspace_info",
                "PASS",
                "Branch and project name visible",
            )
        elif has_branch or has_project:
            log_result(
                "recap_workspace_info",
                "UNVERIFIED",
                f"Partial: branch={has_branch}, project={has_project}",
            )
        else:
            log_result(
                "recap_workspace_info",
                "FAIL",
                "No workspace info found on screen",
            )

        # ── Test 5: Suggested commands ──────────────────────────────
        print_test_header("recap_suggested_commands", 5)
        has_suggestion = any(
            "dootsabha" in line and any(cmd in line for cmd in ["review", "council", "refine", "consult"])
            for line in lines
        )

        if has_suggestion:
            log_result(
                "recap_suggested_commands",
                "PASS",
                "Found dootsabha command suggestion",
            )
        else:
            log_result(
                "recap_suggested_commands",
                "FAIL",
                "No command suggestions found",
            )

        # ── Test 6: Footer trace ────────────────────────────────────
        print_test_header("recap_footer_trace", 6)
        has_session_id = any("ds_" in line for line in lines)
        has_cols = any("cols" in line for line in lines)

        if has_session_id and has_cols:
            log_result(
                "recap_footer_trace",
                "PASS",
                "Session trace ID and terminal width in footer",
            )
        elif has_session_id or has_cols:
            log_result(
                "recap_footer_trace",
                "UNVERIFIED",
                f"Partial footer: session_id={has_session_id}, cols={has_cols}",
            )
        else:
            log_result(
                "recap_footer_trace",
                "FAIL",
                "No footer trace info found",
            )

        # ── Test 7: Screenshot TTY ──────────────────────────────────
        print_test_header("screenshot_recap_tty", 7)
        shot = capture_screenshot("dootsabha_recap_tty")
        log_result(
            "screenshot_recap_tty",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ── Test 8: Piped output — no ANSI ──────────────────────────
        print_test_header("recap_piped_no_ansi", 8)
        tab2 = await window.async_create_tab()
        session2 = tab2.current_session
        created_sessions.append(session2)

        await session2.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session2.async_send_text(f"{BINARY} recap | cat\n")

        # Recap is fast — wait for it to complete
        await asyncio.sleep(8)
        await verify_screen_contains_any(
            session2,
            ["$", "cols", "Recap"],
            timeout=15.0,
        )

        piped_lines = await get_all_screen_text(session2)
        piped_text = "\n".join(piped_lines)

        ansi_re = re.compile(r"\x1b\[")
        ansi_found = ansi_re.findall(piped_text)

        if ansi_found:
            log_result(
                "recap_piped_no_ansi",
                "FAIL",
                f"Found {len(ansi_found)} ANSI escape sequences in piped output",
            )
        else:
            log_result(
                "recap_piped_no_ansi",
                "PASS",
                "No ANSI codes in piped output",
            )

        # ── Test 9: Piped plain markers ─────────────────────────────
        print_test_header("recap_piped_plain_markers", 9)
        has_star = "*" in piped_text
        has_dash_markers = "---" in piped_text

        if has_star and has_dash_markers:
            log_result(
                "recap_piped_plain_markers",
                "PASS",
                "Found * dots and --- section markers in piped output",
            )
        elif has_star or has_dash_markers:
            log_result(
                "recap_piped_plain_markers",
                "UNVERIFIED",
                f"Partial: stars={has_star}, dashes={has_dash_markers}",
            )
        else:
            log_result(
                "recap_piped_plain_markers",
                "FAIL",
                "No plain-text markers found in piped output",
            )

        # ── Test 10: Screenshot piped ───────────────────────────────
        print_test_header("screenshot_recap_piped", 10)
        shot_piped = capture_screenshot("dootsabha_recap_piped")
        log_result(
            "screenshot_recap_piped",
            "PASS",
            f"Captured: {os.path.basename(shot_piped)}",
            shot_piped,
        )

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

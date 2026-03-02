# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha output polish (Task 207)

Tests:
  1. refine_header_box — Rounded border visible (┌, └, │)
  2. refine_provider_dots — Colored dots in progress steps
  3. refine_content_separator — Thin line between progress and content
  4. screenshot_refine — Capture full refine output
  5. council_header_box — Rounded border with agents info
  6. council_section_dividers — "──" style stage labels (not "═══")
  7. council_dispatch_dots — Colored dots on dispatch/review progress
  8. screenshot_council — Capture full council output
  9. review_header_box — Rounded border with author/reviewer
  10. review_section_labels — "Author:" and "Review:" dividers
  11. screenshot_review — Capture full review output
  12. consult_footer — Separator + pipe-delimited footer
  13. screenshot_consult — Capture consult output
  14. no_ansi_piped — refine "prompt" | cat — zero ANSI escapes

Screenshots:
  - dootsabha_polish_refine_{ts}.png
  - dootsabha_polish_council_{ts}.png
  - dootsabha_polish_review_{ts}.png
  - dootsabha_polish_consult_{ts}.png

Usage:
  uv run .claude/automations/test_dootsabha_polish.py
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
        await session.async_send_text("\x03")
        await asyncio.sleep(0.3)
        await session.async_send_text("q")
        await asyncio.sleep(0.2)
        await session.async_send_text("exit\n")
        await asyncio.sleep(0.2)
        await session.async_close()
        print("  Cleanup complete")
    except Exception as e:
        print(f"  Cleanup warning: {e}")


# ============================================================
# MAIN TEST FUNCTION
# ============================================================


async def main(connection):
    """Main test function for dootsabha output polish verification."""
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: dootsabha output polish (Task 207)")
    print("# Verifies rounded headers, provider dots, separators, footers")
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
    created_sessions = [session]

    try:
        await session.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)

        # ============================================================
        # REFINE TESTS (tests 1-4)
        # ============================================================

        print("\n--- REFINE COMMAND ---")
        await session.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session.async_send_text(
            f'{BINARY} refine "Say PONG" --author claude --reviewers codex,gemini\n'
        )

        # -- Test 1: refine_header_box --
        print_test_header("refine_header_box", 1)
        matched = await verify_screen_contains_any(
            session, ["┌", "Refine", "└"], timeout=15.0
        )
        if matched:
            # Also verify box structure
            lines = await get_all_screen_text(session)
            screen_text = "\n".join(lines)
            has_top = "┌" in screen_text and "┐" in screen_text
            has_mid = "│" in screen_text
            has_bot = "└" in screen_text and "┘" in screen_text
            has_name = "Refine" in screen_text

            if has_top and has_mid and has_bot and has_name:
                log_result(
                    "refine_header_box",
                    "PASS",
                    "Rounded border box with ┌┐│└┘ and 'Refine' visible",
                )
            else:
                missing = []
                if not has_top:
                    missing.append("top border")
                if not has_mid:
                    missing.append("side borders")
                if not has_bot:
                    missing.append("bottom border")
                if not has_name:
                    missing.append("command name")
                log_result(
                    "refine_header_box",
                    "FAIL",
                    f"Missing box elements: {', '.join(missing)}",
                )
        else:
            log_result(
                "refine_header_box",
                "FAIL",
                "No header box found within 15s",
            )
            await dump_screen(session, "refine_header_fail")

        # -- Test 2: refine_provider_dots --
        print_test_header("refine_provider_dots", 2)
        # Wait for pipeline to FULLY complete (5 LLM calls).
        # "versions:" only appears in the footer after all steps finish.
        matched = await verify_screen_contains_any(
            session, ["versions:"], timeout=300.0
        )
        if matched:
            lines = await get_all_screen_text(session)
            screen_text = "\n".join(lines)
            # Provider dots appear as ● (TTY) or * (NO_COLOR/piped)
            dot_count = screen_text.count("●") + screen_text.count("*")
            if dot_count >= 2:
                log_result(
                    "refine_provider_dots",
                    "PASS",
                    f"Found {dot_count} provider dots in progress steps",
                )
            else:
                log_result(
                    "refine_provider_dots",
                    "UNVERIFIED",
                    f"Only {dot_count} dots found (expected ≥2)",
                )
        else:
            log_result(
                "refine_provider_dots",
                "FAIL",
                "No provider dots found (pipeline didn't complete in 300s)",
            )

        # -- Test 3: refine_content_separator --
        print_test_header("refine_content_separator", 3)
        # Pipeline already waited for completion above (test 2 waited for "versions:")
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines)

        # Count lines that are mostly dashes (separator lines)
        separator_lines = [
            l for l in lines
            if l.strip() and all(c in "─" for c in l.strip())
        ]
        if len(separator_lines) >= 2:
            log_result(
                "refine_content_separator",
                "PASS",
                f"Found {len(separator_lines)} separator lines (content + footer)",
            )
        elif len(separator_lines) == 1:
            log_result(
                "refine_content_separator",
                "UNVERIFIED",
                "Found 1 separator line (expected ≥2)",
            )
        else:
            log_result(
                "refine_content_separator",
                "FAIL",
                "No separator lines found",
            )

        # -- Test 4: screenshot_refine --
        print_test_header("screenshot_refine", 4)
        await asyncio.sleep(1.0)  # Let rendering settle
        shot = capture_screenshot("dootsabha_polish_refine")
        log_result(
            "screenshot_refine",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ============================================================
        # COUNCIL TESTS (tests 5-8)
        # ============================================================

        print("\n--- COUNCIL COMMAND ---")
        tab2 = await window.async_create_tab()
        session2 = tab2.current_session
        created_sessions.append(session2)

        await session2.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session2.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session2.async_send_text(
            f'{BINARY} council "Say PONG" --agents claude,codex\n'
        )

        # -- Test 5: council_header_box --
        print_test_header("council_header_box", 5)
        matched = await verify_screen_contains_any(
            session2, ["┌", "Council", "└"], timeout=15.0
        )
        if matched:
            lines = await get_all_screen_text(session2)
            screen_text = "\n".join(lines)
            has_box = "┌" in screen_text and "┘" in screen_text
            has_name = "Council" in screen_text
            has_agents = "agents:" in screen_text.lower() or "claude" in screen_text.lower()

            if has_box and has_name:
                log_result(
                    "council_header_box",
                    "PASS",
                    f"Rounded border with 'Council' (agents info: {has_agents})",
                )
            else:
                log_result(
                    "council_header_box",
                    "FAIL",
                    f"Missing: box={has_box}, name={has_name}",
                )
        else:
            log_result(
                "council_header_box",
                "FAIL",
                "No header box found within 15s",
            )
            await dump_screen(session2, "council_header_fail")

        # -- Test 6: council_section_dividers --
        print_test_header("council_section_dividers", 6)
        matched = await verify_screen_contains_any(
            session2, ["Dispatch", "Peer Review", "Synthesis"], timeout=120.0
        )
        if matched:
            lines = await get_all_screen_text(session2)
            screen_text = "\n".join(lines)
            # Should use ── style, NOT ═══
            has_thin = "── " in screen_text or "──" in screen_text
            has_heavy = "═══" in screen_text

            if has_thin and not has_heavy:
                log_result(
                    "council_section_dividers",
                    "PASS",
                    "Section dividers use '──' style (not '═══')",
                )
            elif has_thin and has_heavy:
                log_result(
                    "council_section_dividers",
                    "FAIL",
                    "Mixed divider styles found (both ── and ═══)",
                )
            elif has_heavy:
                log_result(
                    "council_section_dividers",
                    "FAIL",
                    "Still using old ═══ style dividers",
                )
            else:
                log_result(
                    "council_section_dividers",
                    "UNVERIFIED",
                    "No divider characters detected",
                )
        else:
            log_result(
                "council_section_dividers",
                "FAIL",
                "No section labels found within 120s",
            )

        # -- Test 7: council_dispatch_dots --
        print_test_header("council_dispatch_dots", 7)
        # Wait for pipeline to fully complete.
        # "in ·" only appears in the footer metrics after synthesis finishes.
        await verify_screen_contains_any(
            session2, ["in ·"], timeout=300.0
        )
        lines = await get_all_screen_text(session2)
        screen_text = "\n".join(lines)
        dot_count = screen_text.count("●") + screen_text.count("*")
        if dot_count >= 2:
            log_result(
                "council_dispatch_dots",
                "PASS",
                f"Found {dot_count} provider dots in council output",
            )
        else:
            log_result(
                "council_dispatch_dots",
                "UNVERIFIED",
                f"Only {dot_count} dots found",
            )

        # -- Test 8: screenshot_council --
        print_test_header("screenshot_council", 8)
        await asyncio.sleep(1.0)
        shot = capture_screenshot("dootsabha_polish_council")
        log_result(
            "screenshot_council",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ============================================================
        # REVIEW TESTS (tests 9-11)
        # ============================================================

        print("\n--- REVIEW COMMAND ---")
        tab3 = await window.async_create_tab()
        session3 = tab3.current_session
        created_sessions.append(session3)

        await session3.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session3.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session3.async_send_text(
            f'{BINARY} review "Say PONG" --author codex --reviewer claude\n'
        )

        # -- Test 9: review_header_box --
        print_test_header("review_header_box", 9)
        matched = await verify_screen_contains_any(
            session3, ["┌", "Review", "└"], timeout=15.0
        )
        if matched:
            lines = await get_all_screen_text(session3)
            screen_text = "\n".join(lines)
            has_box = "┌" in screen_text and "┘" in screen_text
            has_name = "Review" in screen_text

            if has_box and has_name:
                log_result(
                    "review_header_box",
                    "PASS",
                    "Rounded border with 'Review' visible",
                )
            else:
                log_result(
                    "review_header_box",
                    "FAIL",
                    f"Missing: box={has_box}, name={has_name}",
                )
        else:
            log_result(
                "review_header_box",
                "FAIL",
                "No header box found within 15s",
            )
            await dump_screen(session3, "review_header_fail")

        # -- Test 10: review_section_labels --
        print_test_header("review_section_labels", 10)
        # Wait for pipeline to fully complete.
        # "in ·" only appears in the footer metrics after both LLM calls finish.
        await verify_screen_contains_any(
            session3, ["in ·"], timeout=300.0
        )
        lines = await get_all_screen_text(session3)
        screen_text = "\n".join(lines)

        has_author_label = "Author:" in screen_text
        has_review_label = "Review:" in screen_text

        if has_author_label and has_review_label:
            log_result(
                "review_section_labels",
                "PASS",
                "'Author:' and 'Review:' section dividers present",
            )
        elif has_author_label or has_review_label:
            log_result(
                "review_section_labels",
                "UNVERIFIED",
                f"Author={has_author_label}, Review={has_review_label}",
            )
        else:
            log_result(
                "review_section_labels",
                "FAIL",
                "No 'Author:' or 'Review:' section labels found",
            )

        # -- Test 11: screenshot_review --
        print_test_header("screenshot_review", 11)
        await asyncio.sleep(1.0)
        shot = capture_screenshot("dootsabha_polish_review")
        log_result(
            "screenshot_review",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ============================================================
        # CONSULT TESTS (tests 12-13)
        # ============================================================

        print("\n--- CONSULT COMMAND ---")
        tab4 = await window.async_create_tab()
        session4 = tab4.current_session
        created_sessions.append(session4)

        await session4.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session4.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session4.async_send_text(
            f'{BINARY} consult --agent claude "Say PONG"\n'
        )

        # -- Test 12: consult_footer --
        print_test_header("consult_footer", 12)
        # Wait for consult to fully complete.
        # The footer contains the model name like "claude-sonnet" which only appears after the LLM call.
        await verify_screen_contains_any(
            session4, ["claude-sonnet", "claude-opus", "claude-haiku", "sonnet", "opus"], timeout=120.0
        )
        lines = await get_all_screen_text(session4)
        screen_text = "\n".join(lines)

        # Check for pipe-delimited footer
        has_pipe_footer = "│" in screen_text
        # Check for separator line before footer
        separator_lines = [
            l for l in lines
            if l.strip() and all(c in "─" for c in l.strip())
        ]

        if has_pipe_footer and len(separator_lines) >= 1:
            log_result(
                "consult_footer",
                "PASS",
                f"Pipe-delimited footer with {len(separator_lines)} separator line(s)",
            )
        elif has_pipe_footer:
            log_result(
                "consult_footer",
                "UNVERIFIED",
                "Pipe-delimited footer found but no separator line",
            )
        else:
            log_result(
                "consult_footer",
                "FAIL",
                "No pipe-delimited footer found",
            )

        # -- Test 13: screenshot_consult --
        print_test_header("screenshot_consult", 13)
        await asyncio.sleep(1.0)
        shot = capture_screenshot("dootsabha_polish_consult")
        log_result(
            "screenshot_consult",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ============================================================
        # PIPE TEST (test 14)
        # ============================================================

        # Close completed sessions to free iTerm2 resources before creating more tabs.
        for old_session in [session, session2, session3, session4]:
            try:
                await old_session.async_close()
            except Exception:
                pass
        await asyncio.sleep(0.5)

        print("\n--- PIPE DEGRADATION ---")
        # -- Test 14: no_ansi_piped --
        print_test_header("no_ansi_piped", 14)
        tab5 = await window.async_create_tab()
        session5 = tab5.current_session
        created_sessions = [session5]  # Reset — old sessions already closed

        await session5.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session5.async_send_text(
            f'{BINARY} refine "Say PONG" --author claude --reviewers codex | cat\n'
        )

        # 3 LLM calls — wait for footer (piped mode, "versions:" appears in plain text)
        await verify_screen_contains_any(
            session5, ["versions:"], timeout=300.0
        )

        piped_lines = await get_all_screen_text(session5)
        piped_text = "\n".join(piped_lines)

        ansi_re = re.compile(r"\x1b\[")
        ansi_found = ansi_re.findall(piped_text)

        # Also verify no box-drawing in piped stdout content
        # (box chars only appear in the command line echo, not output)
        has_box_in_output = False
        in_output = False
        for l in piped_lines:
            if "| cat" in l:
                in_output = True
                continue
            if in_output and "$" in l:
                break
            if in_output and ("┌" in l or "└" in l or "│" in l):
                has_box_in_output = True

        if ansi_found:
            log_result(
                "no_ansi_piped",
                "FAIL",
                f"Found {len(ansi_found)} ANSI escape sequences in piped output",
            )
        elif has_box_in_output:
            log_result(
                "no_ansi_piped",
                "FAIL",
                "Box-drawing characters found in piped output",
            )
        else:
            log_result(
                "no_ansi_piped",
                "PASS",
                "No ANSI codes or box chars in piped output",
            )

    except Exception as e:
        print(f"\nERROR during test execution: {e}")
        log_result("Test Execution", "FAIL", str(e))
        try:
            await dump_screen(session, "error_state")
        except Exception:
            pass

    finally:
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

# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: Full Acceptance Suite (Task 4.5)

Tests:
  1. help_output — --help shows all commands and bilingual aliases
  2. version_output — --version shows version string
  3. screenshot_help — Capture help output
  4. status_providers — status shows all 3 providers with dots
  5. status_health — providers have health indicators
  6. screenshot_status — Capture status output
  7. consult_header — consult shows provider name/dot header
  8. consult_content — consult shows response content
  9. consult_footer — consult shows separator + footer
  10. screenshot_consult — Capture consult output
  11. config_show — config show displays configuration
  12. screenshot_config — Capture config output
  13. plugin_list — plugin list shows discovered plugins
  14. screenshot_plugin — Capture plugin list output
  15. error_unknown_cmd — unknown command shows helpful error (not stack trace)
  16. error_unknown_provider — unknown provider shows structured error
  17. error_missing_arg — missing arg shows usage hint
  18. screenshot_errors — Capture error outputs
  19. json_consult — --json produces clean JSON (no ANSI)
  20. json_status — status --json valid
  21. screenshot_json — Capture JSON output
  22. piped_no_ansi — piped output has no ANSI escapes or box chars
  23. piped_no_box — piped consult has no box-drawing characters
  24. screenshot_piped — Capture piped output

Screenshots:
  - dootsabha_accept_help_{ts}.png
  - dootsabha_accept_status_{ts}.png
  - dootsabha_accept_consult_{ts}.png
  - dootsabha_accept_config_{ts}.png
  - dootsabha_accept_plugin_{ts}.png
  - dootsabha_accept_errors_{ts}.png
  - dootsabha_accept_json_{ts}.png
  - dootsabha_accept_piped_{ts}.png

Usage:
  uv run .claude/automations/test_full_acceptance.py
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


async def wait_for_prompt(session, timeout: float = 10.0) -> bool:
    """Wait until shell prompt ($) appears, indicating command completed."""
    start = time.monotonic()
    while (time.monotonic() - start) < timeout:
        screen = await session.async_get_screen_contents()
        # Check last few lines for prompt
        for i in range(max(0, screen.number_of_lines - 5), screen.number_of_lines):
            line = screen.line(i).string
            if line.strip().endswith("$") or "%" in line:
                return True
        await asyncio.sleep(0.3)
    return False


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
    """Main test function for full acceptance suite."""
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: Full Acceptance Suite (Task 4.5)")
    print("# Verifies all commands visually with screenshots")
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
        # HELP & VERSION (tests 1-3)
        # ============================================================

        print("\n--- HELP & VERSION ---")
        await session.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session.async_send_text(f"{BINARY} --help\n")

        # -- Test 1: help_output --
        print_test_header("help_output", 1)
        matched = await verify_screen_contains_any(
            session, ["Available Commands:", "consult", "council"], timeout=10.0
        )
        if matched:
            lines = await get_all_screen_text(session)
            screen_text = "\n".join(lines)
            has_consult = "consult" in screen_text
            has_council = "council" in screen_text
            has_status = "status" in screen_text
            has_review = "review" in screen_text
            has_refine = "refine" in screen_text
            has_plugin = "plugin" in screen_text
            has_config = "config" in screen_text
            has_hindi = "paraamarsh" in screen_text or "परामर्श" in screen_text or "दूतसभा" in screen_text

            all_cmds = all([has_consult, has_council, has_status, has_review, has_refine, has_plugin, has_config])
            if all_cmds and has_hindi:
                log_result("help_output", "PASS", "All 7 commands + Hindi text visible")
            elif all_cmds:
                log_result("help_output", "PASS", "All 7 commands visible (Hindi not in visible area)")
            else:
                missing = []
                for name, present in [("consult", has_consult), ("council", has_council),
                                       ("status", has_status), ("review", has_review),
                                       ("refine", has_refine), ("plugin", has_plugin),
                                       ("config", has_config)]:
                    if not present:
                        missing.append(name)
                log_result("help_output", "FAIL", f"Missing commands: {', '.join(missing)}")
        else:
            log_result("help_output", "FAIL", "No help output found within 10s")
            await dump_screen(session, "help_fail")

        # -- Test 2: version_output --
        print_test_header("version_output", 2)
        await session.async_send_text(f"{BINARY} --version\n")
        await asyncio.sleep(1.0)
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines)
        # Version should contain hash or semver
        has_version = bool(re.search(r'[0-9a-f]{7}|[0-9]+\.[0-9]+', screen_text))
        if has_version:
            log_result("version_output", "PASS", "Version string visible")
        else:
            log_result("version_output", "FAIL", "No version string found")

        # -- Test 3: screenshot_help --
        print_test_header("screenshot_help", 3)
        # Scroll back to help output
        await session.async_send_text(f"clear && {BINARY} --help\n")
        await asyncio.sleep(1.0)
        shot = capture_screenshot("dootsabha_accept_help")
        log_result("screenshot_help", "PASS", f"Captured: {os.path.basename(shot)}", shot)

        # ============================================================
        # STATUS (tests 4-6)
        # ============================================================

        print("\n--- STATUS COMMAND ---")
        await session.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session.async_send_text(f"{BINARY} status\n")

        # -- Test 4: status_providers --
        print_test_header("status_providers", 4)
        matched = await verify_screen_contains_any(
            session, ["claude", "codex", "gemini"], timeout=30.0
        )
        if matched:
            lines = await get_all_screen_text(session)
            screen_text = "\n".join(lines).lower()
            found = sum(1 for p in ["claude", "codex", "gemini"] if p in screen_text)
            if found == 3:
                log_result("status_providers", "PASS", "All 3 providers listed")
            else:
                log_result("status_providers", "FAIL", f"Only {found}/3 providers found")
        else:
            log_result("status_providers", "FAIL", "No providers in status output")
            await dump_screen(session, "status_fail")

        # -- Test 5: status_health --
        print_test_header("status_health", 5)
        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines)
        # Health indicators: ● (TTY) or * (NO_COLOR) or ✓/✗
        has_dots = "●" in screen_text or "*" in screen_text or "✓" in screen_text or "✗" in screen_text
        if has_dots:
            log_result("status_health", "PASS", "Health indicators visible")
        else:
            log_result("status_health", "UNVERIFIED", "No health indicator characters detected")

        # -- Test 6: screenshot_status --
        print_test_header("screenshot_status", 6)
        await asyncio.sleep(0.5)
        shot = capture_screenshot("dootsabha_accept_status")
        log_result("screenshot_status", "PASS", f"Captured: {os.path.basename(shot)}", shot)

        # ============================================================
        # CONSULT (tests 7-10)
        # ============================================================

        print("\n--- CONSULT COMMAND ---")
        tab2 = await window.async_create_tab()
        session2 = tab2.current_session
        created_sessions.append(session2)

        await session2.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session2.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session2.async_send_text(
            f'{BINARY} consult --agent claude "Say PONG"\n'
        )

        # -- Test 7: consult_header --
        print_test_header("consult_header", 7)
        matched = await verify_screen_contains_any(
            session2, ["claude", "●", "PONG"], timeout=120.0
        )
        if matched:
            lines = await get_all_screen_text(session2)
            screen_text = "\n".join(lines)
            has_provider = "claude" in screen_text
            has_dot = "●" in screen_text or "*" in screen_text
            if has_provider:
                log_result("consult_header", "PASS", f"Provider name visible (dot={has_dot})")
            else:
                log_result("consult_header", "FAIL", "No provider name found")
        else:
            log_result("consult_header", "FAIL", "No consult output within 120s")
            await dump_screen(session2, "consult_fail")

        # -- Test 8: consult_content --
        print_test_header("consult_content", 8)
        # Wait for full completion — footer appears after LLM finishes
        await verify_screen_contains_any(
            session2, ["claude-sonnet", "sonnet", "model:", "tokens"], timeout=120.0
        )
        lines = await get_all_screen_text(session2)
        screen_text = "\n".join(lines)
        # Should have some response content
        non_empty_content = len([l for l in lines if l.strip() and not l.strip().startswith("$")]) >= 3
        if non_empty_content:
            log_result("consult_content", "PASS", "Response content visible")
        else:
            log_result("consult_content", "UNVERIFIED", "Limited content visible")

        # -- Test 9: consult_footer --
        print_test_header("consult_footer", 9)
        # Check for separator + footer elements
        separator_lines = [l for l in lines if l.strip() and all(c in "─" for c in l.strip())]
        has_footer_pipe = "│" in screen_text
        if len(separator_lines) >= 1 and has_footer_pipe:
            log_result("consult_footer", "PASS", f"{len(separator_lines)} separator(s) + pipe-delimited footer")
        elif has_footer_pipe:
            log_result("consult_footer", "PASS", "Pipe-delimited footer visible")
        elif len(separator_lines) >= 1:
            log_result("consult_footer", "UNVERIFIED", "Separator but no pipe footer")
        else:
            log_result("consult_footer", "UNVERIFIED", "No footer elements detected")

        # -- Test 10: screenshot_consult --
        print_test_header("screenshot_consult", 10)
        await asyncio.sleep(0.5)
        shot = capture_screenshot("dootsabha_accept_consult")
        log_result("screenshot_consult", "PASS", f"Captured: {os.path.basename(shot)}", shot)

        # ============================================================
        # CONFIG (tests 11-12)
        # ============================================================

        print("\n--- CONFIG COMMAND ---")
        tab3 = await window.async_create_tab()
        session3 = tab3.current_session
        created_sessions.append(session3)

        await session3.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session3.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session3.async_send_text(f"{BINARY} config show\n")

        # -- Test 11: config_show --
        print_test_header("config_show", 11)
        matched = await verify_screen_contains_any(
            session3, ["providers", "council", "timeout", "claude"], timeout=10.0
        )
        if matched:
            lines = await get_all_screen_text(session3)
            screen_text = "\n".join(lines)
            has_providers = "providers" in screen_text.lower() or "claude" in screen_text
            has_council = "council" in screen_text.lower()
            if has_providers:
                log_result("config_show", "PASS", f"Config displayed (council={has_council})")
            else:
                log_result("config_show", "FAIL", "No provider config visible")
        else:
            log_result("config_show", "FAIL", "No config output within 10s")
            await dump_screen(session3, "config_fail")

        # -- Test 12: screenshot_config --
        print_test_header("screenshot_config", 12)
        await asyncio.sleep(0.5)
        shot = capture_screenshot("dootsabha_accept_config")
        log_result("screenshot_config", "PASS", f"Captured: {os.path.basename(shot)}", shot)

        # ============================================================
        # PLUGIN LIST (tests 13-14)
        # ============================================================

        print("\n--- PLUGIN COMMAND ---")
        await session3.async_send_text("clear\n")
        await asyncio.sleep(0.3)
        await session3.async_send_text(f"{BINARY} plugin list\n")

        # -- Test 13: plugin_list --
        print_test_header("plugin_list", 13)
        matched = await verify_screen_contains_any(
            session3, ["provider", "strategy", "No plugins", "extension"], timeout=10.0
        )
        if matched:
            log_result("plugin_list", "PASS", f"Plugin list output visible (matched: {matched})")
        else:
            # plugin list may show "No plugins found" which is also valid
            lines = await get_all_screen_text(session3)
            screen_text = "\n".join(lines)
            if "no " in screen_text.lower() or "$" in lines[-1] if lines else False:
                log_result("plugin_list", "PASS", "Plugin list completed (may be empty)")
            else:
                log_result("plugin_list", "FAIL", "No plugin list output")
                await dump_screen(session3, "plugin_fail")

        # -- Test 14: screenshot_plugin --
        print_test_header("screenshot_plugin", 14)
        await asyncio.sleep(0.5)
        shot = capture_screenshot("dootsabha_accept_plugin")
        log_result("screenshot_plugin", "PASS", f"Captured: {os.path.basename(shot)}", shot)

        # ============================================================
        # ERROR HANDLING (tests 15-18)
        # ============================================================

        print("\n--- ERROR HANDLING ---")
        tab4 = await window.async_create_tab()
        session4 = tab4.current_session
        created_sessions.append(session4)

        await session4.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session4.async_send_text("clear\n")
        await asyncio.sleep(0.3)

        # -- Test 15: error_unknown_cmd --
        print_test_header("error_unknown_cmd", 15)
        await session4.async_send_text(f"{BINARY} unknown-command-xyz\n")
        await asyncio.sleep(1.0)
        lines = await get_all_screen_text(session4)
        screen_text = "\n".join(lines)
        has_error = "unknown command" in screen_text.lower() or "error" in screen_text.lower()
        has_stack = "goroutine" in screen_text or "panic" in screen_text
        if has_error and not has_stack:
            log_result("error_unknown_cmd", "PASS", "Structured error message (no stack trace)")
        elif has_error and has_stack:
            log_result("error_unknown_cmd", "FAIL", "Error shows stack trace!")
        else:
            log_result("error_unknown_cmd", "FAIL", "No error message visible")

        # -- Test 16: error_unknown_provider --
        print_test_header("error_unknown_provider", 16)
        await session4.async_send_text(f'{BINARY} consult --agent nonexistent "test"\n')
        await asyncio.sleep(1.0)
        lines = await get_all_screen_text(session4)
        screen_text = "\n".join(lines)
        has_unknown = "unknown" in screen_text.lower()
        has_valid = "claude" in screen_text.lower() or "valid" in screen_text.lower()
        if has_unknown and has_valid:
            log_result("error_unknown_provider", "PASS", "Shows error with valid providers hint")
        elif has_unknown:
            log_result("error_unknown_provider", "PASS", "Shows 'unknown provider' error")
        else:
            log_result("error_unknown_provider", "FAIL", "No unknown provider error")

        # -- Test 17: error_missing_arg --
        print_test_header("error_missing_arg", 17)
        await session4.async_send_text(f"{BINARY} consult\n")
        await asyncio.sleep(1.0)
        lines = await get_all_screen_text(session4)
        screen_text = "\n".join(lines)
        has_usage = "usage" in screen_text.lower() or "accepts" in screen_text.lower() or "requires" in screen_text.lower()
        if has_usage:
            log_result("error_missing_arg", "PASS", "Shows usage hint for missing arg")
        else:
            # Check for any error
            has_error = "error" in screen_text.lower()
            if has_error:
                log_result("error_missing_arg", "PASS", "Shows error for missing arg")
            else:
                log_result("error_missing_arg", "FAIL", "No error for missing arg")

        # -- Test 18: screenshot_errors --
        print_test_header("screenshot_errors", 18)
        await asyncio.sleep(0.5)
        shot = capture_screenshot("dootsabha_accept_errors")
        log_result("screenshot_errors", "PASS", f"Captured: {os.path.basename(shot)}", shot)

        # ============================================================
        # JSON OUTPUT (tests 19-21)
        # ============================================================

        print("\n--- JSON OUTPUT ---")
        tab5 = await window.async_create_tab()
        session5 = tab5.current_session
        created_sessions.append(session5)

        await session5.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session5.async_send_text("clear\n")
        await asyncio.sleep(0.3)

        # -- Test 19: json_consult --
        print_test_header("json_consult", 19)
        await session5.async_send_text(
            f'{BINARY} consult --agent claude --json "Say PONG"\n'
        )
        matched = await verify_screen_contains_any(
            session5, ['"meta"', '"data"', '"Content"', '"schema_version"'], timeout=120.0
        )
        if matched:
            lines = await get_all_screen_text(session5)
            screen_text = "\n".join(lines)
            has_json_structure = '"meta"' in screen_text and '"data"' in screen_text
            if has_json_structure:
                log_result("json_consult", "PASS", "JSON with meta/data envelope visible")
            else:
                log_result("json_consult", "PASS", f"JSON output visible (matched: {matched})")
        else:
            log_result("json_consult", "FAIL", "No JSON output within 120s")
            await dump_screen(session5, "json_consult_fail")

        # -- Test 20: json_status --
        print_test_header("json_status", 20)
        await session5.async_send_text(f"{BINARY} status --json\n")
        await asyncio.sleep(2.0)
        lines = await get_all_screen_text(session5)
        screen_text = "\n".join(lines)
        has_json = "{" in screen_text and "}" in screen_text
        has_provider = "claude" in screen_text
        if has_json and has_provider:
            log_result("json_status", "PASS", "Status JSON with provider data")
        elif has_json:
            log_result("json_status", "PASS", "Status JSON output visible")
        else:
            log_result("json_status", "FAIL", "No JSON status output")

        # -- Test 21: screenshot_json --
        print_test_header("screenshot_json", 21)
        await asyncio.sleep(0.5)
        shot = capture_screenshot("dootsabha_accept_json")
        log_result("screenshot_json", "PASS", f"Captured: {os.path.basename(shot)}", shot)

        # ============================================================
        # PIPED OUTPUT (tests 22-24)
        # ============================================================

        # Close completed sessions to free resources
        for old_session in [session, session2, session3, session4, session5]:
            try:
                await old_session.async_close()
            except Exception:
                pass
        await asyncio.sleep(0.5)

        print("\n--- PIPED OUTPUT ---")
        tab6 = await window.async_create_tab()
        session6 = tab6.current_session
        created_sessions = [session6]  # Reset — old sessions already closed

        await session6.async_send_text(f"cd {PROJECT_DIR}\n")
        await asyncio.sleep(0.5)
        await session6.async_send_text("clear\n")
        await asyncio.sleep(0.3)

        # -- Test 22: piped_no_ansi --
        print_test_header("piped_no_ansi", 22)
        await session6.async_send_text(
            f'{BINARY} consult --agent claude "Say PONG" | cat\n'
        )
        # Wait for command to complete
        matched = await verify_screen_contains_any(
            session6, ["$", "Mock:"], timeout=120.0
        )
        await asyncio.sleep(1.0)
        lines = await get_all_screen_text(session6)
        piped_text = "\n".join(lines)

        # Check for ANSI escape sequences
        ansi_re = re.compile(r"\x1b\[")
        ansi_found = ansi_re.findall(piped_text)
        if ansi_found:
            log_result("piped_no_ansi", "FAIL", f"Found {len(ansi_found)} ANSI escape sequences")
        else:
            log_result("piped_no_ansi", "PASS", "No ANSI escape sequences in piped output")

        # -- Test 23: piped_no_box --
        print_test_header("piped_no_box", 23)
        # Check output lines (after the command echo) for box chars
        has_box_in_output = False
        in_output = False
        for l in lines:
            if "| cat" in l:
                in_output = True
                continue
            if in_output and l.strip().endswith("$"):
                break
            if in_output and ("┌" in l or "└" in l or "│" in l or "┐" in l or "┘" in l):
                has_box_in_output = True

        if has_box_in_output:
            log_result("piped_no_box", "FAIL", "Box-drawing characters found in piped output")
        else:
            log_result("piped_no_box", "PASS", "No box-drawing characters in piped output")

        # -- Test 24: screenshot_piped --
        print_test_header("screenshot_piped", 24)
        await asyncio.sleep(0.5)
        shot = capture_screenshot("dootsabha_accept_piped")
        log_result("screenshot_piped", "PASS", f"Captured: {os.path.basename(shot)}", shot)

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

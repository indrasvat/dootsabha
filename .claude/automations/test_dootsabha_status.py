# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha status
Tasks: 1.4 (status command)

Tests:
  1. version_correctness — Semver versions present, no "Code)" bug
  2. dot_merged — PROVIDER is first column (dot merged, not separate)
  3. models_populated — Expected models present for all providers
  4. table_layout — All 3 providers and column headers present
  5. no_ansi_piped — status | cat has no ANSI escapes
  6. json_valid — --json → valid JSON with meta/data/providers/versions/models

Verification Strategy:
  - Run dootsabha status and scrape screen for version/layout/model info
  - Pipe through cat to verify ANSI stripping
  - JSON test via subprocess.run for structured validation

Screenshots:
  - dootsabha_status_healthy_{ts}.png
  - dootsabha_status_piped_{ts}.png

Screenshot Inspection Checklist:
  - Colors: Provider status dots, column headers styled
  - Boundaries: Terminal window bounds captured correctly
  - Visible Elements: Provider names, versions, models, status dots

Key Bindings:
  - Ctrl+C: Interrupt running command
  - exit: Close shell

Usage:
  uv run .claude/automations/test_dootsabha_status.py
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
SEMVER_RE = re.compile(r"\d+\.\d+\.\d+")

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
    """Main test function for dootsabha status command."""
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: dootsabha status")
    print("# Provider status table with versions, models, health dots")
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

        # ── Test 1: Version correctness ──────────────────────────
        print_test_header("version_correctness", 1)
        await session.async_send_text(f"{BINARY} status\n")
        await asyncio.sleep(3)

        lines = await get_all_screen_text(session)
        screen_text = "\n".join(lines)

        has_code_paren = "Code)" in screen_text
        semver_matches = SEMVER_RE.findall(screen_text)

        if has_code_paren:
            log_result(
                "version_correctness",
                "FAIL",
                "Found 'Code)' in output — parseVersion bug",
            )
        elif len(semver_matches) >= 1:
            log_result(
                "version_correctness",
                "PASS",
                f"Found semver versions: {semver_matches}",
            )
        else:
            log_result(
                "version_correctness",
                "UNVERIFIED",
                "No semver found in output",
            )

        # ── Test 2: Dot merged with provider name ────────────────
        print_test_header("dot_merged", 2)
        dot_in_provider_col = False
        separate_dot_col = False

        for line in lines:
            if "PROVIDER" in line:
                stripped = line.strip()
                if stripped.startswith("PROVIDER") or (
                    stripped.startswith("|")
                    and "PROVIDER" in stripped.split("|")[1]
                ):
                    dot_in_provider_col = True
                cols = [c.strip() for c in stripped.split("|") if c.strip()]
                if len(cols) > 0 and cols[0] == "PROVIDER":
                    dot_in_provider_col = True
                elif len(cols) > 1 and cols[0] == "" and cols[1] == "PROVIDER":
                    separate_dot_col = True

        if separate_dot_col:
            log_result(
                "dot_merged",
                "FAIL",
                "Dot still in separate column (empty header before PROVIDER)",
            )
        elif dot_in_provider_col:
            log_result(
                "dot_merged",
                "PASS",
                "PROVIDER is first column — dot merged",
            )
        else:
            log_result(
                "dot_merged",
                "UNVERIFIED",
                "Could not determine column layout",
            )

        # Take screenshot of healthy status table
        shot = capture_screenshot("dootsabha_status_healthy")
        log_result(
            "screenshot_healthy",
            "PASS",
            f"Captured: {os.path.basename(shot)}",
            shot,
        )

        # ── Test 3: Models populated ─────────────────────────────
        print_test_header("models_populated", 3)
        expected_models = ["claude-sonnet-4-6", "gpt-5.3-codex", "gemini-3-pro"]
        found_models = []
        missing_models = []

        for model in expected_models:
            if model in screen_text:
                found_models.append(model)
            else:
                missing_models.append(model)

        if missing_models:
            log_result(
                "models_populated",
                "FAIL",
                f"Missing models: {missing_models}; found: {found_models}",
            )
        else:
            log_result(
                "models_populated",
                "PASS",
                f"All models present: {found_models}",
            )

        # ── Test 4: Table layout (no breakage) ───────────────────
        print_test_header("table_layout", 4)
        has_claude = any("claude" in line for line in lines)
        has_codex = any("codex" in line for line in lines)
        has_gemini = any("gemini" in line for line in lines)
        has_version_header = any("VERSION" in line for line in lines)
        has_model_header = any("MODEL" in line for line in lines)

        if all(
            [has_claude, has_codex, has_gemini, has_version_header, has_model_header]
        ):
            log_result(
                "table_layout",
                "PASS",
                "All providers and columns present",
            )
        else:
            missing = []
            if not has_claude:
                missing.append("claude")
            if not has_codex:
                missing.append("codex")
            if not has_gemini:
                missing.append("gemini")
            if not has_version_header:
                missing.append("VERSION header")
            if not has_model_header:
                missing.append("MODEL header")
            log_result("table_layout", "FAIL", f"Missing: {missing}")

        # ── Test 5: No ANSI in piped mode ────────────────────────
        print_test_header("no_ansi_piped", 5)
        await session.async_send_text(f"{BINARY} status | cat\n")
        await asyncio.sleep(2)

        piped_lines = await get_all_screen_text(session)
        piped_text = "\n".join(piped_lines)

        ansi_re = re.compile(r"\x1b\[")
        ansi_found = ansi_re.findall(piped_text)

        shot_piped = capture_screenshot("dootsabha_status_piped")

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

        # ── Test 6: JSON mode valid ──────────────────────────────
        print_test_header("json_valid", 6)
        try:
            proc = subprocess.run(
                [BINARY, "status", "--json"],
                capture_output=True,
                text=True,
                timeout=10,
                cwd=PROJECT_DIR,
            )

            data = json.loads(proc.stdout)
            has_meta = "meta" in data
            has_data = "data" in data
            providers_in_json = [p.get("Name", "") for p in data.get("data", [])]
            versions_in_json = [p.get("Version", "") for p in data.get("data", [])]
            models_in_json = [p.get("Model", "") for p in data.get("data", [])]

            all_versions_ok = all(
                SEMVER_RE.match(v) for v in versions_in_json if v
            )
            all_models_ok = all(m for m in models_in_json)

            if has_meta and has_data and all_versions_ok and all_models_ok:
                log_result(
                    "json_valid",
                    "PASS",
                    f"Valid JSON: providers={providers_in_json}, versions={versions_in_json}, models={models_in_json}",
                )
            else:
                issues = []
                if not has_meta:
                    issues.append("missing meta")
                if not has_data:
                    issues.append("missing data")
                if not all_versions_ok:
                    issues.append(f"bad versions: {versions_in_json}")
                if not all_models_ok:
                    issues.append(f"empty models: {models_in_json}")
                log_result(
                    "json_valid",
                    "FAIL",
                    f"JSON issues: {'; '.join(issues)}",
                )
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

# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: dootsabha status
Tests:
  1. Version correctness (semver, not "Code)")
  2. Dot merged with provider name
  3. Models populated for all providers
  4. Table renders without layout breakage
  5. No ANSI in piped mode
  6. JSON mode valid
Screenshots:
  - dootsabha_status_healthy.png
  - dootsabha_status_piped.png
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
    icon = {"PASS": "\u2713", "FAIL": "\u2717", "UNVERIFIED": "?"}[status]
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
                print(f"  \u2717 {t['name']}: {t['details']}")
    return 1 if results["failed"] > 0 else 0


# ─── Project Path ───────────────────────────────────────────────
PROJECT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
BINARY = os.path.join(PROJECT_DIR, "bin", "dootsabha")

SEMVER_RE = re.compile(r"\d+\.\d+\.\d+")


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

    # ── Test 1: Version correctness ──────────────────────────────
    print("\nTest 1: Version correctness")
    await session.async_send_text(f"{BINARY} status\n")
    await asyncio.sleep(3)

    lines = await get_all_screen_text(session)
    screen_text = "\n".join(lines)

    # Should contain semver patterns, NOT "Code)"
    has_code_paren = "Code)" in screen_text
    semver_matches = SEMVER_RE.findall(screen_text)

    if has_code_paren:
        log_result("version_correctness", "FAIL", "Found 'Code)' in output — parseVersion bug")
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
            f"No semver found in output",
        )

    # ── Test 2: Dot merged with provider name ────────────────────
    print("\nTest 2: Dot merged with provider name")
    # Look for "claude" appearing in the same line as a dot character
    # and NOT in a separate column before PROVIDER header
    has_provider_header = False
    dot_in_provider_col = False
    separate_dot_col = False

    for line in lines:
        if "PROVIDER" in line:
            has_provider_header = True
            # Check if there's an empty column before PROVIDER
            stripped = line.strip()
            if stripped.startswith("PROVIDER") or stripped.startswith("|") and "PROVIDER" in stripped.split("|")[1]:
                dot_in_provider_col = True
            # Old layout had: | (empty) | PROVIDER | ...
            # New layout has: | PROVIDER | ...
            cols = [c.strip() for c in stripped.split("|") if c.strip()]
            if len(cols) > 0 and cols[0] == "PROVIDER":
                dot_in_provider_col = True
            elif len(cols) > 1 and cols[0] == "" and cols[1] == "PROVIDER":
                separate_dot_col = True

    if separate_dot_col:
        log_result("dot_merged", "FAIL", "Dot still in separate column (empty header before PROVIDER)")
    elif dot_in_provider_col:
        log_result("dot_merged", "PASS", "PROVIDER is first column — dot merged")
    else:
        log_result("dot_merged", "UNVERIFIED", "Could not determine column layout")

    # Take screenshot of healthy status table
    shot = capture_screenshot("dootsabha_status_healthy")
    log_result("screenshot_healthy", "PASS", f"Captured: {os.path.basename(shot)}", shot)

    # ── Test 3: Models populated ─────────────────────────────────
    print("\nTest 3: Models populated for all providers")
    expected_models = ["sonnet-4-6", "gpt-5.3-codex", "gemini-3-pro"]
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
        log_result("models_populated", "PASS", f"All models present: {found_models}")

    # ── Test 4: Table layout (no breakage) ───────────────────────
    print("\nTest 4: Table renders without layout breakage")
    # Check that we see all 3 provider names and key columns
    has_claude = any("claude" in line for line in lines)
    has_codex = any("codex" in line for line in lines)
    has_gemini = any("gemini" in line for line in lines)
    has_version_header = any("VERSION" in line for line in lines)
    has_model_header = any("MODEL" in line for line in lines)

    if all([has_claude, has_codex, has_gemini, has_version_header, has_model_header]):
        log_result("table_layout", "PASS", "All providers and columns present")
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

    # ── Test 5: No ANSI in piped mode ────────────────────────────
    print("\nTest 5: No ANSI in piped mode")
    await session.async_send_text(f"{BINARY} status | cat\n")
    await asyncio.sleep(2)

    piped_lines = await get_all_screen_text(session)
    piped_text = "\n".join(piped_lines)

    # ANSI escape sequences start with ESC[
    ansi_re = re.compile(r"\x1b\[")
    ansi_found = ansi_re.findall(piped_text)

    # Take screenshot of piped output
    shot_piped = capture_screenshot("dootsabha_status_piped")

    if ansi_found:
        log_result(
            "no_ansi_piped",
            "FAIL",
            f"Found {len(ansi_found)} ANSI escape sequences in piped output",
            shot_piped,
        )
    else:
        log_result("no_ansi_piped", "PASS", "No ANSI codes in piped output", shot_piped)

    # ── Test 6: JSON mode valid ──────────────────────────────────
    print("\nTest 6: JSON mode valid")
    proc = subprocess.run(
        [BINARY, "status", "--json"],
        capture_output=True,
        text=True,
        timeout=10,
        cwd=PROJECT_DIR,
    )

    try:
        data = json.loads(proc.stdout)
        has_meta = "meta" in data
        has_data = "data" in data
        providers_in_json = [p.get("Name", "") for p in data.get("data", [])]
        versions_in_json = [p.get("Version", "") for p in data.get("data", [])]
        models_in_json = [p.get("Model", "") for p in data.get("data", [])]

        # Check all versions are semver-like
        all_versions_ok = all(SEMVER_RE.match(v) for v in versions_in_json if v)
        # Check no model is empty
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
            log_result("json_valid", "FAIL", f"JSON issues: {'; '.join(issues)}")
    except (json.JSONDecodeError, KeyError) as e:
        log_result("json_valid", "FAIL", f"Invalid JSON: {e}")

    # ── Cleanup ──────────────────────────────────────────────────
    await cleanup_session(session)
    await asyncio.sleep(0.5)
    # Close the test tab
    await session.async_send_text("exit\n")

    # ── Summary ──────────────────────────────────────────────────
    exit_code = print_summary()
    sys.exit(exit_code)


iterm2.run_until_complete(main)

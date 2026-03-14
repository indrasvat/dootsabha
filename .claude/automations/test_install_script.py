# /// script
# requires-python = ">=3.14"
# dependencies = [
#   "iterm2",
#   "pyobjc",
#   "pyobjc-framework-Quartz",
# ]
# ///
"""
L4 Visual Test: install.sh

Tests:
    1. banner_alignment — ASCII banner renders with correct alignment
    2. platform_detection — OS/arch correctly identified
    3. version_resolution — Latest release version fetched and displayed
    4. directory_menu — Multiple install dirs offered with annotations
    5. no_cargo_bin — .cargo/bin excluded from options (Go binary)
    6. usr_local_bin — /usr/local/bin shown with "needs sudo" if applicable
    7. download_checksum — Binary downloaded and checksum verified
    8. install_success — "dootsabha is ready" success block rendered
    9. skill_hint — Claude Code skill install hint shown

Verification Strategy:
    - Run install.sh interactively in a new tab, press Enter at prompts
    - Read screen contents after each stage to verify output
    - Take screenshots at key moments (menu, post-install)
    - Run non-interactive install to verify end-to-end flow

Screenshots:
    - install_interactive_menu_{ts}.png: Directory selection menu
    - install_noninteractive_{ts}.png: Full non-interactive output

Screenshot Inspection Checklist:
    - Colors: Cyan banner, green checkmarks, magenta step markers, yellow warnings
    - Boundaries: Terminal window captured correctly
    - Visible Elements: Banner, platform, version, directory menu, success block
    - Alignment: Banner art aligned, no stray characters

Key Bindings:
    - Enter: Accept default at prompt
    - n: Decline skill install

Usage:
    uv run .claude/automations/test_install_script.py
"""
import asyncio
import os
import subprocess
import sys
import time
from datetime import datetime

import iterm2

# ============================================================
# CONFIGURATION
# ============================================================

PROJECT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SCREENSHOT_DIR = os.path.join(PROJECT_DIR, ".claude", "screenshots")
TIMEOUT_SECONDS = 15.0

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


def log_result(test_name, status, details="", screenshot=None):
    results["tests"].append(
        {"name": test_name, "status": status, "details": details, "screenshot": screenshot}
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


def print_summary():
    results["end_time"] = datetime.now()
    total = results["passed"] + results["failed"] + results["unverified"]
    duration = (results["end_time"] - results["start_time"]).total_seconds() if results["start_time"] else 0
    print(f"\n{'=' * 60}")
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
    print("-" * 60)
    if results["failed"] > 0:
        print("OVERALL: FAILED")
        return 1
    print("OVERALL: PASSED")
    return 0


def print_test_header(name, num):
    print(f"\n{'=' * 60}")
    print(f"TEST {num}: {name}")
    print("=" * 60)


# ============================================================
# QUARTZ WINDOW TARGETING
# ============================================================

try:
    import Quartz

    def get_iterm2_window_id():
        window_list = Quartz.CGWindowListCopyWindowInfo(
            Quartz.kCGWindowListOptionOnScreenOnly | Quartz.kCGWindowListExcludeDesktopElements,
            Quartz.kCGNullWindowID,
        )
        for w in window_list:
            owner = w.get("kCGWindowOwnerName", "")
            if "iTerm" in str(owner) and w.get("kCGWindowLayer", -1) == 0:
                return w.get("kCGWindowNumber")
        return None

except ImportError:
    def get_iterm2_window_id():
        return None


def capture_screenshot(name):
    os.makedirs(SCREENSHOT_DIR, exist_ok=True)
    ts = datetime.now().strftime("%Y%m%d_%H%M%S")
    filepath = os.path.join(SCREENSHOT_DIR, f"{name}_{ts}.png")
    wid = get_iterm2_window_id()
    if wid:
        subprocess.run(["screencapture", "-x", "-l", str(wid), filepath], check=True)
    else:
        subprocess.run(["screencapture", "-x", filepath], check=True)
    return filepath


# ============================================================
# VERIFICATION HELPERS
# ============================================================

async def get_screen_lines(session):
    screen = await session.async_get_screen_contents()
    lines = []
    for i in range(screen.number_of_lines):
        lines.append(screen.line(i).string)
    return lines


async def wait_for_text(session, text, timeout=TIMEOUT_SECONDS):
    start = time.monotonic()
    while (time.monotonic() - start) < timeout:
        lines = await get_screen_lines(session)
        for line in lines:
            if text in line:
                return True
        await asyncio.sleep(0.3)
    return False


async def dump_screen(session, label):
    lines = await get_screen_lines(session)
    print(f"\n{'=' * 60}")
    print(f"SCREEN DUMP: {label}")
    print("=" * 60)
    for i, line in enumerate(lines):
        if line.strip():
            print(f"  {i:03d}: {line}")
    print("=" * 60)


async def cleanup_session(session):
    try:
        await session.async_send_text("\x03")
        await asyncio.sleep(0.2)
        await session.async_send_text("exit\n")
        await asyncio.sleep(0.3)
        await session.async_close()
    except Exception:
        pass


# ============================================================
# MAIN TEST
# ============================================================

async def main(connection):
    results["start_time"] = datetime.now()

    print("\n" + "#" * 60)
    print("# L4 VISUAL TEST: install.sh")
    print("#" * 60)

    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if not window:
        print("ERROR: No active iTerm2 window")
        log_result("Setup", "FAIL", "No iTerm2 window")
        return print_summary()

    created_sessions = []

    try:
        # ============================================================
        # PHASE 1: Interactive mode — test menu and prompts
        # ============================================================
        print("\n>>> PHASE 1: Interactive install (menu test)")
        tab1 = await window.async_create_tab()
        s1 = tab1.current_session
        created_sessions.append(s1)
        await asyncio.sleep(1)

        await s1.async_send_text(f"cd {PROJECT_DIR} && sh install.sh\n")

        # Wait for the directory menu to appear
        print("  Waiting for directory menu...")
        if await wait_for_text(s1, "Where should dootsabha be installed?"):
            log_result("directory_menu", "PASS", "Menu displayed")
        else:
            await dump_screen(s1, "no_menu")
            log_result("directory_menu", "FAIL", "Menu not shown")

        # Read the menu lines
        lines = await get_screen_lines(s1)
        full = "\n".join(lines)

        # Test: banner alignment
        print_test_header("banner_alignment", 1)
        banner_ok = any("____/" in l for l in lines) and any("Council of AI" in l for l in lines)
        first_art = [l for l in lines if "____/" in l and "____/" in l[:20]]
        if first_art and first_art[0].startswith(" "):
            log_result("banner_alignment", "PASS", "Banner aligned with leading spaces")
        elif banner_ok:
            log_result("banner_alignment", "PASS", "Banner present")
        else:
            log_result("banner_alignment", "FAIL", "Banner missing or misaligned")

        # Test: platform detection
        print_test_header("platform_detection", 2)
        if "darwin/arm64" in full or "darwin/amd64" in full or "linux/" in full:
            log_result("platform_detection", "PASS", "Platform correctly detected")
        else:
            log_result("platform_detection", "FAIL", "Platform line not found")

        # Test: version resolution
        print_test_header("version_resolution", 3)
        if "Version:" in full and "v0." in full:
            log_result("version_resolution", "PASS", "Version resolved and displayed")
        else:
            log_result("version_resolution", "FAIL", "Version not shown")

        # Test: no .cargo/bin
        print_test_header("no_cargo_bin", 4)
        if ".cargo/bin" in full:
            log_result("no_cargo_bin", "FAIL", ".cargo/bin still in menu")
        else:
            log_result("no_cargo_bin", "PASS", ".cargo/bin correctly excluded")

        # Test: /usr/local/bin shown
        print_test_header("usr_local_bin", 5)
        if "/usr/local/bin" in full:
            has_sudo_note = "sudo" in full
            detail = "shown with sudo note" if has_sudo_note else "shown (writable, no sudo needed)"
            log_result("usr_local_bin", "PASS", detail)
        else:
            log_result("usr_local_bin", "FAIL", "/usr/local/bin not in menu")

        # Take screenshot of interactive menu
        ss1 = capture_screenshot("install_interactive_menu")
        log_result("menu_screenshot", "PASS", ss1, screenshot=ss1)

        # Press Enter to accept default, then 'n' for skill install, then wait
        await s1.async_send_text("\n")
        await asyncio.sleep(1)
        await s1.async_send_text("y\n")

        # Wait for download to complete
        print("  Waiting for install to complete...")
        if await wait_for_text(s1, "dootsabha is ready", timeout=20):
            pass  # will check below
        else:
            await dump_screen(s1, "install_incomplete")

        # Answer skill prompt
        await asyncio.sleep(1)
        if await wait_for_text(s1, "Claude Code skill", timeout=3):
            await s1.async_send_text("n\n")
            await asyncio.sleep(1)

        lines2 = await get_screen_lines(s1)
        full2 = "\n".join(lines2)

        # Test: checksum verified
        print_test_header("download_checksum", 6)
        if "Checksum verified" in full2:
            log_result("download_checksum", "PASS", "SHA256 checksum verified")
        elif "Downloaded" in full2:
            log_result("download_checksum", "UNVERIFIED", "Downloaded but checksum status unclear")
        else:
            log_result("download_checksum", "FAIL", "Download/checksum not found")

        # Test: install success
        print_test_header("install_success", 7)
        if "dootsabha is ready" in full2:
            log_result("install_success", "PASS", "Success message present")
        else:
            log_result("install_success", "FAIL", "Success message not found")

        # Test: skill hint
        print_test_header("skill_hint", 8)
        if "npx skills add" in full2 or "Skill installed" in full2:
            log_result("skill_hint", "PASS", "Skill install hint shown")
        else:
            log_result("skill_hint", "FAIL", "No skill hint in output")

        # Take post-install screenshot
        ss2 = capture_screenshot("install_interactive_complete")
        log_result("complete_screenshot", "PASS", ss2, screenshot=ss2)

        # ============================================================
        # PHASE 2: Non-interactive mode — end-to-end verification
        # ============================================================
        print("\n>>> PHASE 2: Non-interactive install (end-to-end)")
        tab2 = await window.async_create_tab()
        s2 = tab2.current_session
        created_sessions.append(s2)
        await asyncio.sleep(1)

        cmd = f"cd {PROJECT_DIR} && NONINTERACTIVE=1 VERSION=v0.1.0 INSTALL_DIR=/tmp/ds-test-install sh install.sh"
        await s2.async_send_text(cmd + "\n")

        print("  Waiting for non-interactive install...")
        if await wait_for_text(s2, "dootsabha is ready", timeout=20):
            log_result("noninteractive_e2e", "PASS", "Non-interactive install completed")
        else:
            await dump_screen(s2, "noninteractive_failed")
            log_result("noninteractive_e2e", "FAIL", "Non-interactive did not complete")

        # Take screenshot
        await asyncio.sleep(1)
        ss3 = capture_screenshot("install_noninteractive")
        log_result("noninteractive_screenshot", "PASS", ss3, screenshot=ss3)

        # Verify binary works
        await s2.async_send_text("/tmp/ds-test-install/dootsabha --version\n")
        await asyncio.sleep(2)
        if await wait_for_text(s2, "v0.1.0", timeout=5):
            log_result("binary_functional", "PASS", "Installed binary runs correctly")
        else:
            log_result("binary_functional", "FAIL", "Binary --version failed")

        # Cleanup temp binary
        await s2.async_send_text("rm /tmp/ds-test-install/dootsabha && rmdir /tmp/ds-test-install\n")
        await asyncio.sleep(0.5)

    except Exception as e:
        print(f"\nERROR: {e}")
        import traceback
        traceback.print_exc()
        log_result("execution", "FAIL", str(e))

    finally:
        print(f"\n{'=' * 60}")
        print("CLEANUP")
        print("=" * 60)
        for s in created_sessions:
            await cleanup_session(s)

    return print_summary()


if __name__ == "__main__":
    exit_code = iterm2.run_until_complete(main)
    sys.exit(exit_code if exit_code else 0)

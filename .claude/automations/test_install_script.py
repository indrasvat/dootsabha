# /// script
# requires-python = ">=3.14"
# dependencies = ["iterm2", "pyobjc", "pyobjc-framework-Quartz"]
# ///
"""
L4 Visual Test: install.sh output

Tests:
  1. banner_renders — ASCII banner renders with proper alignment
  2. platform_detected — OS/arch detected correctly
  3. version_resolved — Version displayed
  4. install_completes — "dootsabha is ready" appears
  5. no_alignment_issues — No visible misalignment in output

Verification Strategy:
  - Run install.sh in non-interactive mode in a new iTerm2 tab
  - Read screen contents and verify key output lines
  - Take screenshot for visual inspection

Screenshots:
  - install_output_{ts}.png

Usage:
  uv run .claude/automations/test_install_script.py
"""
import asyncio
import os
import re
import sys
from datetime import datetime

import iterm2

PROJECT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
SCREENSHOT_DIR = os.path.join(os.path.dirname(__file__), "..", "screenshots")

results = {"passed": 0, "failed": 0, "tests": [], "screenshots": []}


def log_result(name, status, details="", screenshot=None):
    results["tests"].append({"name": name, "status": status, "details": details})
    if screenshot:
        results["screenshots"].append(screenshot)
    if status == "PASS":
        results["passed"] += 1
        print(f"  [+] PASS: {name} — {details}")
    else:
        results["failed"] += 1
        print(f"  [x] FAIL: {name} — {details}")
    if screenshot:
        print(f"      Screenshot: {screenshot}")


def print_summary():
    total = results["passed"] + results["failed"]
    print(f"\n{'=' * 60}")
    print(f"RESULTS: {results['passed']}/{total} passed")
    if results["screenshots"]:
        print(f"Screenshots: {len(results['screenshots'])}")
    print("=" * 60)
    return 1 if results["failed"] > 0 else 0


def capture_screenshot(name):
    try:
        import Quartz

        os.makedirs(SCREENSHOT_DIR, exist_ok=True)
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        filepath = os.path.join(SCREENSHOT_DIR, f"{name}_{ts}.png")

        window_list = Quartz.CGWindowListCopyWindowInfo(
            Quartz.kCGWindowListOptionOnScreenOnly
            | Quartz.kCGWindowListExcludeDesktopElements,
            Quartz.kCGNullWindowID,
        )
        for w in window_list:
            owner = w.get("kCGWindowOwnerName", "")
            if "iTerm" in str(owner) and w.get("kCGWindowLayer", -1) == 0:
                wid = w["kCGWindowNumber"]
                bounds = w["kCGWindowBounds"]
                rect = Quartz.CGRectMake(
                    bounds["X"], bounds["Y"], bounds["Width"], bounds["Height"]
                )
                image = Quartz.CGWindowListCreateImage(
                    rect,
                    Quartz.kCGWindowListOptionIncludingWindow,
                    wid,
                    Quartz.kCGWindowImageDefault,
                )
                if image:
                    url = Quartz.CFURLCreateWithFileSystemPath(
                        None, filepath, Quartz.kCFURLPOSIXPathStyle, False
                    )
                    dest = Quartz.CGImageDestinationCreateWithURL(
                        url, "public.png", 1, None
                    )
                    if dest:
                        Quartz.CGImageDestinationAddImage(dest, image, None)
                        Quartz.CGImageDestinationFinalize(dest)
                        return filepath
                break
    except Exception as e:
        print(f"  Screenshot error: {e}")
    return None


async def main(connection):
    app = await iterm2.async_get_app(connection)
    window = app.current_terminal_window
    if not window:
        print("ERROR: No iTerm2 window")
        return 1

    created_sessions = []
    try:
        tab = await window.async_create_tab()
        session = tab.current_session
        created_sessions.append(session)

        # Run the installer
        cmd = f"clear && NONINTERACTIVE=1 VERSION=v0.1.0 sh {PROJECT_DIR}/install.sh"
        await session.async_send_text(cmd + "\n")
        await asyncio.sleep(12)

        # Read screen
        screen = await session.async_get_screen_contents()
        lines = []
        for i in range(screen.number_of_lines):
            line = screen.line(i).string
            lines.append(line)
        full_output = "\n".join(lines)

        print("\n--- Screen contents ---")
        for line in lines:
            if line.strip():
                print(f"  | {line}")
        print("--- End screen ---\n")

        # Test 1: Banner renders
        banner_found = any("dootsabha" in line or "____/" in line for line in lines)
        log_result(
            "banner_renders",
            "PASS" if banner_found else "FAIL",
            "ASCII banner present" if banner_found else "Banner not found",
        )

        # Test 2: Platform detected
        platform_found = any(
            "Platform:" in line and ("darwin" in line or "linux" in line)
            for line in lines
        )
        log_result(
            "platform_detected",
            "PASS" if platform_found else "FAIL",
            "OS/arch detected" if platform_found else "Platform line not found",
        )

        # Test 3: Version resolved
        version_found = any("Version:" in line and "v0.1.0" in line for line in lines)
        log_result(
            "version_resolved",
            "PASS" if version_found else "FAIL",
            "v0.1.0 displayed" if version_found else "Version not found",
        )

        # Test 4: Install completes
        ready_found = any("dootsabha is ready" in line for line in lines)
        log_result(
            "install_completes",
            "PASS" if ready_found else "FAIL",
            "Success message present" if ready_found else "Ready message not found",
        )

        # Test 5: Banner alignment — first art line should have leading spaces
        art_lines = [l for l in lines if "____/" in l or "/ __" in l or "\\__,_" in l]
        alignment_ok = True
        for al in art_lines:
            # The banner lines should not be flush-left (except the bottom two)
            if al.strip().startswith("____/") and not al.startswith(" "):
                alignment_ok = False
        log_result(
            "banner_alignment",
            "PASS" if alignment_ok else "FAIL",
            "No alignment issues" if alignment_ok else "Banner misaligned",
        )

        # Screenshot
        ss = capture_screenshot("install_output")
        if ss:
            log_result("screenshot_captured", "PASS", ss, screenshot=ss)
        else:
            log_result("screenshot_captured", "FAIL", "Could not capture")

    except Exception as e:
        print(f"ERROR: {e}")
        import traceback

        traceback.print_exc()
    finally:
        for s in created_sessions:
            try:
                await s.async_send_text("\x03")
                await asyncio.sleep(0.2)
                await s.async_send_text("exit\n")
                await asyncio.sleep(0.5)
                await s.async_close()
            except Exception:
                pass

    return print_summary()


if __name__ == "__main__":
    exit_code = iterm2.run_until_complete(main)
    sys.exit(exit_code)

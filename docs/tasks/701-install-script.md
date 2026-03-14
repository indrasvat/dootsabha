# Task 701: Install Script (`curl | sh`)

## Status: DONE

## Depends On
Release workflow (v0.1.0 published with platform binaries)

## Problem
Installation UX is poor. Users clone the repo and `make build`, or download a
binary to a random path that's not on `$PATH`. The friend's issue: installed to
`~/dootsabha/bin/dootsabha` — not on PATH, Claude Code couldn't find it.

Need a polished `install.sh` that:
- Detects OS/arch, downloads correct binary from GitHub Releases
- Installs to a directory already on `$PATH` (no manual PATH editing)
- Pretty themed output with ASCII banner (Roman transliteration only)
- Interactive (default) and non-interactive modes
- No `sudo` required
- `curl -fsSL https://raw.githubusercontent.com/.../install.sh | sh`

## Files
| File | Action |
|------|--------|
| `install.sh` | Create installer script |
| `README.md` | Update installation section |
| `docs/tasks/701-install-script.md` | This task |

## Steps
1. Create `install.sh` with OS/arch detection, PATH-aware install dir selection
2. Add ASCII banner, interactive/non-interactive modes
3. Test on macOS (darwin/arm64)
4. Update README installation section
5. Verify `curl | sh` pattern works

## Done Criteria
- [ ] `curl -fsSL .../install.sh | sh` installs to a PATH directory
- [ ] Interactive mode offers directory choice
- [ ] Non-interactive mode uses best default
- [ ] Pretty ASCII banner output
- [ ] OS/arch detection correct for darwin/linux × amd64/arm64
- [ ] Verifies binary works after install (`dootsabha --version`)
- [ ] PATH warning if install dir not on PATH
- [ ] `make ci` passes

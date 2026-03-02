# Task 5.4: Build & Release (CI, v0.1.0)

## Status: DONE

## Depends On
- Task 5.1 (README), Task 5.2 (default config)

## Parallelizable With
- Task 5.3 (SKILL)

## Problem

Set up the release pipeline: CI passes all test layers, binary builds for macOS and Linux, version tag triggers release with cross-compiled binaries and checksums.

## PRD Reference
- §7.3 (Compatibility: macOS arm64/amd64, Linux amd64/arm64)

## Files to Modify
- `.github/workflows/ci.yml` — Ensure full test pyramid runs
- `.github/workflows/release.yml` — Cross-compilation, checksums, release

## Execution Steps

### Step 1: Verify CI workflow
- ci.yml runs: lint, test, vet, build
- On PR: full `make ci`
- On push to main: full `make ci` + `make test-binary`

### Step 2: Configure release workflow
- Triggered by tag push (v*)
- Cross-compile for: darwin/arm64, darwin/amd64, linux/amd64, linux/arm64
- Generate checksums
- Create GitHub release with binaries

### Step 3: Test release locally
- `make build` produces correct binary
- Version injection via ldflags works
- Cross-compile test: `GOOS=linux GOARCH=amd64 make build`

### Step 4: Tag and release
- `git tag v0.1.0`
- Push tag to trigger release workflow
- Verify release on GitHub

## Verification

### L1: CI passes
```bash
make ci
```

### L3: Build artifacts
```bash
make build
./bin/dootsabha --version  # Should show v0.1.0 or dev
GOOS=linux GOARCH=amd64 go build -o /tmp/dootsabha-linux ./cmd/dootsabha
file /tmp/dootsabha-linux  # Should show ELF 64-bit
```

## Completion Criteria

1. CI runs full test pyramid on PR
2. Release workflow cross-compiles for 4 targets
3. Checksums generated
4. Version injection works
5. `make ci` passes

## Commit

```
feat(release): configure CI and release pipeline for v0.1.0

- CI workflow: lint + test + vet + build on PR
- Release workflow: cross-compile for macOS/Linux (arm64/amd64)
- Version injection via ldflags
- Checksum generation
```

## Session Protocol

1. Read CLAUDE.md
2. Read this task file
3. **Change status to `IN PROGRESS`**
4. Read PRD §7.3
5. Execute steps 1-4
6. Run verification (L1 → L3)
7. **Change status to `DONE`**
8. Update `docs/PROGRESS.md`
9. Commit

#!/bin/bash
# Cross-compile dootsabha with version ldflags.
# Called by release workflow. Receives tag as $1.
set -euo pipefail

TAG="${1:-dev}"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "none")"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
MODULE="github.com/indrasvat/dootsabha"

LDFLAGS="-s -w"
LDFLAGS="${LDFLAGS} -X ${MODULE}/internal/version.Version=${TAG}"
LDFLAGS="${LDFLAGS} -X ${MODULE}/internal/version.Commit=${COMMIT}"
LDFLAGS="${LDFLAGS} -X ${MODULE}/internal/version.Date=${DATE}"

platforms=(
    "darwin/amd64"
    "darwin/arm64"
    "linux/amd64"
    "linux/arm64"
)

mkdir -p dist
for platform in "${platforms[@]}"; do
    goos="${platform%/*}"
    goarch="${platform#*/}"
    echo "Building ${goos}/${goarch}..."
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -trimpath -ldflags="${LDFLAGS}" \
        -o "dist/dootsabha-${goos}-${goarch}" ./cmd/dootsabha
done
echo "Build complete."
ls -lh dist/

#!/bin/sh
# install.sh — dootsabha installer
# curl -fsSL https://raw.githubusercontent.com/indrasvat/dootsabha/main/install.sh | sh
#
# Options (env vars):
#   INSTALL_DIR=/path    Override install directory
#   VERSION=v0.1.0       Install specific version (default: latest)
#   NONINTERACTIVE=1     Skip prompts, use defaults
#   INSTALL_SKILL=0      Skip agent skill install (default: install if npx exists)
#   GITHUB_TOKEN=token   Optional token for GitHub API fallback
set -eu

REPO="indrasvat/dootsabha"
SKILL_PACKAGE="indrasvat/dootsabha"

# ── Colors ───────────────────────────────────────────────────────────
setup_colors() {
    if [ -t 1 ] && command -v tput >/dev/null 2>&1; then
        BOLD=$(tput bold 2>/dev/null || true)
        DIM=$(tput dim 2>/dev/null || true)
        RESET=$(tput sgr0 2>/dev/null || true)
        RED=$(tput setaf 1 2>/dev/null || true)
        GREEN=$(tput setaf 2 2>/dev/null || true)
        YELLOW=$(tput setaf 3 2>/dev/null || true)
        MAGENTA=$(tput setaf 5 2>/dev/null || true)
        CYAN=$(tput setaf 6 2>/dev/null || true)
    else
        BOLD="" DIM="" RESET="" RED="" GREEN="" YELLOW="" MAGENTA="" CYAN=""
    fi
}

info()  { printf "%s\n" "  ${CYAN}>${RESET} $*"; }
step()  { printf "%s\n" "  ${MAGENTA}${BOLD}>${RESET} $*"; }
ok()    { printf "%s\n" "  ${GREEN}${BOLD}*${RESET} $*"; }
warn()  { printf "%s\n" "  ${YELLOW}${BOLD}!${RESET} $*"; }
err()   { printf "%s\n" "  ${RED}${BOLD}x${RESET} $*" >&2; }
dim()   { printf "%s\n" "    ${DIM}$*${RESET}"; }
rule()  { printf "  %s\n" "${DIM}──────────────────────────────────────────${RESET}"; }

# ── Banner ───────────────────────────────────────────────────────────
banner() {
    printf "\n%s" "${CYAN}${BOLD}"
    cat <<'ART'
       __            __             __    __
  ____/ /___  ____  / /__________ _/ /_  / /_  ____ _
 / __  / __ \/ __ \/ __/ ___/ __ `/ __ \/ __ \/ __ `/
/ /_/ / /_/ / /_/ / /_(__  ) /_/ / /_/ / / / / /_/ /
\__,_/\____/\____/\__/____/\__,_/_.___/_/ /_/\__,_/
ART
    printf "%s" "${RESET}"
    printf "  %s\n" "${DIM}Council of AI Messengers${RESET}"
    rule
    printf "\n"
}

# ── Platform detection ───────────────────────────────────────────────
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        darwin) OS="darwin" ;;
        linux)  OS="linux" ;;
        *)      err "Unsupported OS: $OS"; exit 1 ;;
    esac

    case "$ARCH" in
        x86_64|amd64)  ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *)             err "Unsupported architecture: $ARCH"; exit 1 ;;
    esac

    PLATFORM="${OS}-${ARCH}"
}

# ── Version resolution ───────────────────────────────────────────────
latest_version_from_web() {
    url=$(curl -fsSI -o /dev/null -w '%{redirect_url}' \
        "https://github.com/${REPO}/releases/latest" 2>/dev/null) || return 1
    case "$url" in
        */tag/*) ;;
        *) return 1 ;;
    esac
    tag=${url##*/tag/}
    tag=${tag%%[!A-Za-z0-9._+-]*}
    [ -n "$tag" ] || return 1
    printf "%s\n" "$tag"
}

github_api_fetch() {
    outfile="$1"
    url="$2"
    if [ -n "${GITHUB_TOKEN:-}" ]; then
        code=$(curl -sSL -H "Authorization: Bearer ${GITHUB_TOKEN}" \
            -H "X-GitHub-Api-Version: 2022-11-28" \
            -o "$outfile" -w '%{http_code}' "$url" 2>/dev/null) || code=""
    else
        code=$(curl -sSL -o "$outfile" -w '%{http_code}' "$url" 2>/dev/null) || code=""
    fi
    printf "%s\n" "${code:-000}"
}

parse_first_tag() {
    sed -n '/"tag_name":/ { s/.*"tag_name": *"\([^"]*\)".*/\1/p; q; }'
}

resolve_version() {
    if [ -n "${VERSION:-}" ]; then
        return
    fi

    step "Resolving latest release..."

    web_tag=$(latest_version_from_web 2>/dev/null) || web_tag=""
    if [ -n "$web_tag" ]; then
        VERSION="$web_tag"
        return
    fi

    tmp_api=$(mktemp "${TMPDIR:-/tmp}/dootsabha-api.XXXXXX" 2>/dev/null || mktemp)
    status=$(github_api_fetch "$tmp_api" "https://api.github.com/repos/${REPO}/releases/latest")

    case "$status" in
        200)
            VERSION=$(parse_first_tag < "$tmp_api" || true)
            ;;
        403|429)
            rm -f "$tmp_api"
            err "GitHub API rate-limited while resolving latest release."
            dim "Retry with VERSION=vX.Y.Z to skip latest lookup, or set GITHUB_TOKEN."
            exit 1
            ;;
        000)
            rm -f "$tmp_api"
            err "Network error while resolving latest release"
            exit 1
            ;;
        *)
            rm -f "$tmp_api"
            err "GitHub API returned HTTP ${status} while resolving latest release"
            dim "Retry with VERSION=vX.Y.Z to skip latest lookup."
            exit 1
            ;;
    esac

    rm -f "$tmp_api"
    if [ -z "${VERSION:-}" ]; then
        err "Could not determine latest version"
        dim "Retry with VERSION=vX.Y.Z to skip latest lookup."
        exit 1
    fi
}

# ── Install directory selection ──────────────────────────────────────
#
# Strategy: scan $PATH for writable directories that don't need sudo.
# Preferred dirs (shown first if on PATH): ~/.local/bin, ~/bin, ~/go/bin, etc.
# Then any other writable $PATH dirs. Fallback: ~/.local/bin.
find_install_dir() {
    if [ -n "${INSTALL_DIR:-}" ]; then
        return
    fi

    # Preferred dirs — shown first if they exist on PATH (in priority order)
    preferred="$HOME/.local/bin $HOME/bin $HOME/go/bin"

    candidates=""
    seen=""

    # Pass 1: preferred dirs that are on PATH and writable (or creatable)
    for dir in $preferred; do
        if echo ":$PATH:" | grep -q ":${dir}:" 2>/dev/null; then
            if [ -d "$dir" ] && [ -w "$dir" ]; then
                candidates="${candidates}${dir}\n"
                seen="${seen}:${dir}:"
            elif [ ! -e "$dir" ]; then
                candidates="${candidates}${dir}\n"
                seen="${seen}:${dir}:"
            fi
        fi
    done

    # Pass 2: scan PATH for other writable general-purpose bin dirs
    # Skip system dirs, tool-specific dirs, and already-seen
    IFS=':'
    for dir in $PATH; do
        case "$dir" in
            "") continue ;;
            /usr/*|/bin|/sbin|/opt/homebrew/*|/nix/*) continue ;;
            */.sdkman/*|*/.volta/*|*/.bun/*|*/.krew/*|*/.antigravity/*|*/.cargo/*) continue ;;
            */Library/*|*/Applications/*|*/.npm/*|*/.pnpm*|*/.yarn/*) continue ;;
        esac
        if echo "$seen" | grep -q ":${dir}:" 2>/dev/null; then continue; fi
        if [ -d "$dir" ] && [ -w "$dir" ]; then
            candidates="${candidates}${dir}\n"
            seen="${seen}:${dir}:"
        fi
    done
    unset IFS

    # Pass 3: /usr/local/bin — always offer if it exists (it's on PATH everywhere)
    if [ -d "/usr/local/bin" ]; then
        if ! echo "$seen" | grep -q ":/usr/local/bin:" 2>/dev/null; then
            candidates="${candidates}/usr/local/bin\n"
        fi
    fi

    # Fallback: ~/.local/bin even if not on PATH (we'll warn)
    if [ -z "$candidates" ]; then
        candidates="$HOME/.local/bin"
    fi

    # Pick the first candidate as default
    DEFAULT_DIR=$(printf "%b" "$candidates" | head -1)

    if [ "${NONINTERACTIVE:-}" = "1" ]; then
        # In non-interactive mode, never default to a dir that needs sudo —
        # it would hang or fail without a TTY. Fall back to ~/.local/bin.
        if [ -d "$DEFAULT_DIR" ] && ! [ -w "$DEFAULT_DIR" ]; then
            INSTALL_DIR="$HOME/.local/bin"
        else
            INSTALL_DIR="$DEFAULT_DIR"
        fi
        return
    fi

    # Interactive: show options
    printf "\n"
    step "Where should dootsabha be installed?"
    printf "\n"

    i=1
    printf "%b" "$candidates" | while IFS= read -r dir; do
        if [ -z "$dir" ]; then continue; fi
        marker=""
        if [ "$dir" = "$DEFAULT_DIR" ]; then
            marker=" ${GREEN}(recommended)${RESET}"
        fi
        notes=""
        if echo ":$PATH:" | grep -q ":${dir}:" 2>/dev/null; then
            notes=" ${DIM}on PATH${RESET}"
        else
            notes=" ${YELLOW}not on PATH${RESET}"
        fi
        if [ -d "$dir" ] && ! [ -w "$dir" ]; then
            notes="${notes}  ${YELLOW}needs sudo${RESET}"
        fi
        printf "    %s%s%d%s) %s%s  %s\n" "$BOLD" "$CYAN" "$i" "$RESET" "$dir" "$marker" "$notes"
        i=$((i + 1))
    done

    printf "\n"
    printf "  %sChoice [%s1%s]: %s" "$BOLD" "$CYAN" "$RESET$BOLD" "$RESET"

    # Attempting the open IS the test. /dev/tty passes both -e and -r even when
    # it cannot be opened — a piped `curl … | sh` with no controlling terminal
    # gets ENXIO ("Device not configured") — so the old existence check let a
    # doomed read through, and `set -e` turned that into an aborted install.
    choice=""
    if [ -t 0 ]; then
        read -r choice || choice=""
    else
        read -r choice 2>/dev/null </dev/tty || choice=""
    fi

    if [ -z "$choice" ] || [ "$choice" = "1" ]; then
        INSTALL_DIR="$DEFAULT_DIR"
    elif echo "$choice" | grep -qE '^[0-9]+$'; then
        INSTALL_DIR=$(printf "%b" "$candidates" | sed -n "${choice}p")
        if [ -z "$INSTALL_DIR" ]; then
            INSTALL_DIR="$DEFAULT_DIR"
        fi
    else
        # User entered a custom path
        INSTALL_DIR="$choice"
    fi
}

# ── Download and install ─────────────────────────────────────────────
download_and_install() {
    BINARY_NAME="dootsabha-${PLATFORM}"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

    step "Downloading ${BOLD}${VERSION}${RESET} for ${BOLD}${PLATFORM}${RESET}..."
    dim "$URL"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    if ! curl -fsSL -o "${TMPDIR}/${BINARY_NAME}" "$URL"; then
        err "Download failed. Check that ${VERSION} has a ${PLATFORM} binary."
        dim "Releases: https://github.com/${REPO}/releases"
        exit 1
    fi
    ok "Downloaded"

    # Verify checksum
    step "Verifying checksum..."
    if curl -fsSL -o "${TMPDIR}/checksums.txt" "$CHECKSUM_URL" 2>/dev/null; then
        expected=$(grep "$BINARY_NAME" "${TMPDIR}/checksums.txt" | awk '{print $1}')
        if [ -n "$expected" ]; then
            if command -v sha256sum >/dev/null 2>&1; then
                actual=$(sha256sum "${TMPDIR}/${BINARY_NAME}" | awk '{print $1}')
            elif command -v shasum >/dev/null 2>&1; then
                actual=$(shasum -a 256 "${TMPDIR}/${BINARY_NAME}" | awk '{print $1}')
            else
                actual=""
                warn "No sha256sum or shasum found — skipping"
            fi
            if [ -n "$actual" ]; then
                if [ "$actual" = "$expected" ]; then
                    ok "Checksum verified ${DIM}(sha256)${RESET}"
                else
                    err "Checksum mismatch!"
                    dim "Expected: $expected"
                    dim "Got:      $actual"
                    exit 1
                fi
            fi
        else
            warn "Binary not in checksums.txt — skipping"
        fi
    else
        warn "Could not fetch checksums — skipping"
    fi

    # Determine if we need elevated permissions
    SUDO=""
    if [ -d "$INSTALL_DIR" ] && ! [ -w "$INSTALL_DIR" ]; then
        SUDO="sudo"
        info "Elevated permissions required for ${INSTALL_DIR}"
    fi

    # Create install dir if needed
    if [ ! -d "$INSTALL_DIR" ]; then
        step "Creating ${INSTALL_DIR}..."
        $SUDO mkdir -p "$INSTALL_DIR"
        ok "Created"
    fi

    # Install
    step "Installing..."
    chmod +x "${TMPDIR}/${BINARY_NAME}"
    $SUDO mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/dootsabha"
    ok "Installed to ${BOLD}${INSTALL_DIR}/dootsabha${RESET}"
}

# ── Post-install checks ─────────────────────────────────────────────
post_install() {
    # Check if install dir is on PATH
    if ! echo ":$PATH:" | grep -q ":${INSTALL_DIR}:" 2>/dev/null; then
        printf "\n"
        rule
        warn "${BOLD}${INSTALL_DIR}${RESET}${YELLOW} is not on your PATH${RESET}"
        printf "\n"
        info "Add it to your shell profile:"
        printf "\n"

        SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
        case "$SHELL_NAME" in
            zsh)
                dim "echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
                ;;
            bash)
                dim "echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
                ;;
            fish)
                dim "fish_add_path ${INSTALL_DIR}"
                ;;
            *)
                dim "export PATH=\"${INSTALL_DIR}:\$PATH\""
                ;;
        esac
        rule
    fi

    # Verify it works
    printf "\n"
    step "Verifying installation..."
    if command -v dootsabha >/dev/null 2>&1; then
        INSTALLED_VERSION=$(dootsabha --version 2>/dev/null | head -1)
        ok "${INSTALLED_VERSION}"
    elif [ -x "${INSTALL_DIR}/dootsabha" ]; then
        INSTALLED_VERSION=$("${INSTALL_DIR}/dootsabha" --version 2>/dev/null | head -1)
        ok "${INSTALLED_VERSION}"
    fi

    # Offer Claude Code skill install
    install_skill

    # Success
    printf "\n"
    rule
    printf "\n"
    printf "  %s%s  dootsabha is ready.%s\n" "$GREEN" "$BOLD" "$RESET"
    printf "\n"
    dim "Get started:"
    dim "  dootsabha status              # check agent health"
    dim "  dootsabha consult -a codex    # ask a single agent"
    dim "  dootsabha council \"question\"  # multi-agent council"
    printf "\n"
    dim "Docs: https://github.com/${REPO}"
    printf "\n"
}

# ── Agent skill install ───────────────────────────────────────────────
install_skill() {
    if [ "${INSTALL_SKILL:-1}" = "0" ]; then
        dim "Skipped skill install because INSTALL_SKILL=0"
        return
    fi

    if ! command -v npx >/dev/null 2>&1; then
        warn "npx not found — skipping agent skill"
        dim "Install later: npx skills add ${SKILL_PACKAGE} --global --yes"
        return
    fi

    printf "\n"
    rule
    printf "\n"
    step "Installing agent skill"
    dim "Running: npx skills add ${SKILL_PACKAGE} --global --yes"
    if npx --yes skills add "${SKILL_PACKAGE}" --global --yes >/dev/null 2>&1; then
        ok "Skill installed"
        dim "AI agents can auto-discover dootsabha commands"
    else
        warn "Skill install failed — you can add it later:"
        dim "npx skills add ${SKILL_PACKAGE} --global --yes"
    fi
}

# ── Main ─────────────────────────────────────────────────────────────
main() {
    setup_colors
    banner
    detect_platform
    ok "Platform: ${BOLD}${OS}/${ARCH}${RESET}"
    resolve_version
    ok "Version:  ${BOLD}${VERSION}${RESET}"
    find_install_dir
    ok "Target:   ${BOLD}${INSTALL_DIR}${RESET}"

    # Confirm in interactive mode
    if [ "${NONINTERACTIVE:-}" != "1" ]; then
        printf "\n"
        printf "  %sProceed? [Y/n] %s" "$BOLD" "$RESET"
        confirm="y"
        if [ -t 0 ]; then
            read -r confirm || confirm="y"
        else
            read -r confirm 2>/dev/null </dev/tty || confirm="y"
        fi
        case "$confirm" in
            [nN]*) info "Cancelled."; exit 0 ;;
        esac
    fi

    printf "\n"
    download_and_install
    post_install
}

main "$@"

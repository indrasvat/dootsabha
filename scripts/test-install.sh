#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BIN="${ROOT}/bin/dootsabha"

if [[ ! -x "${BIN}" ]]; then
  echo "bin/dootsabha is required; run make build first" >&2
  exit 1
fi

case "$(uname -s | tr '[:upper:]' '[:lower:]')" in
  darwin) TEST_OS=darwin ;;
  linux) TEST_OS=linux ;;
  *) echo "unsupported test OS" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) TEST_ARCH=amd64 ;;
  arm64|aarch64) TEST_ARCH=arm64 ;;
  *) echo "unsupported test arch" >&2; exit 1 ;;
esac

TEST_PLATFORM="${TEST_OS}-${TEST_ARCH}"
TEST_SHA=$(shasum -a 256 "${BIN}" | awk '{print $1}')

tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

make_fake_tools() {
  local mode=$1
  local dir=$2

  mkdir -p "${dir}"
  cat >"${dir}/curl" <<'SH'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >>"${FAKE_CURL_LOG}"

out=""
write_status=0
head_request=0
url=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    -o)
      out="$2"
      shift 2
      ;;
    -w)
      write_status=1
      shift 2
      ;;
    -H)
      shift 2
      ;;
    -*)
      [[ "$1" == *I* ]] && head_request=1
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

if [[ ${head_request} -eq 1 && "${url}" == "https://github.com/indrasvat/dootsabha/releases/latest" ]]; then
  if [[ "${FAKE_CURL_MODE}" == "api403" ]]; then
    exit 22
  fi
  printf 'https://github.com/indrasvat/dootsabha/releases/tag/v9.9.9'
  exit 0
fi

if [[ "${url}" == https://api.github.com/repos/indrasvat/dootsabha/releases/latest ]]; then
  : >"${out}"
  if [[ "${FAKE_CURL_MODE}" == "api403" ]]; then
    [[ ${write_status} -eq 1 ]] && printf '403'
    exit 0
  fi
  printf '{"tag_name":"v8.8.8"}\n' >"${out}"
  [[ ${write_status} -eq 1 ]] && printf '200'
  exit 0
fi

case "${url}" in
  https://github.com/indrasvat/dootsabha/releases/download/*/checksums.txt)
    printf '%s  dootsabha-%s\n' "${TEST_SHA}" "${TEST_PLATFORM}" >"${out}"
    ;;
  https://github.com/indrasvat/dootsabha/releases/download/*/dootsabha-*)
    cp "${TEST_BINARY}" "${out}"
    ;;
  *)
    echo "unexpected curl URL: ${url}" >&2
    exit 2
    ;;
esac
SH
  chmod +x "${dir}/curl"

  cat >"${dir}/npx" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >>"${FAKE_NPX_LOG}"
exit 0
SH
  chmod +x "${dir}/npx"
}

run_install() {
  local name=$1
  local mode=$2
  shift 2

  local test_dir="${tmp}/${name}"
  local fakebin="${test_dir}/fakebin"
  mkdir -p "${test_dir}"
  make_fake_tools "${mode}" "${fakebin}"

  FAKE_CURL_LOG="${test_dir}/curl.log" \
  FAKE_NPX_LOG="${test_dir}/npx.log" \
  FAKE_CURL_MODE="${mode}" \
  TEST_BINARY="${BIN}" \
  TEST_SHA="${TEST_SHA}" \
  TEST_PLATFORM="${TEST_PLATFORM}" \
  PATH="${fakebin}:$PATH" \
  NONINTERACTIVE="${RUN_NONINTERACTIVE-1}" \
  INSTALL_DIR="${test_dir}/install" \
  "$@" sh "${ROOT}/install.sh" >"${test_dir}/stdout" 2>"${test_dir}/stderr"

  printf '%s\n' "${test_dir}"
}

assert_contains() {
  local file=$1
  local pattern=$2
  if ! grep -qE -- "${pattern}" "${file}"; then
    echo "expected ${file} to contain ${pattern}" >&2
    echo "--- ${file} ---" >&2
    cat "${file}" >&2
    exit 1
  fi
}

assert_not_contains() {
  local file=$1
  local pattern=$2
  if grep -qE -- "${pattern}" "${file}"; then
    echo "expected ${file} not to contain ${pattern}" >&2
    echo "--- ${file} ---" >&2
    cat "${file}" >&2
    exit 1
  fi
}

latest_dir=$(run_install latest ok env)
[[ -x "${latest_dir}/install/dootsabha" ]]
assert_contains "${latest_dir}/stdout" 'Version:.*v9\.9\.9'
assert_contains "${latest_dir}/stdout" 'Skill installed'
assert_contains "${latest_dir}/npx.log" '--yes skills add indrasvat/dootsabha --global --yes'
assert_contains "${latest_dir}/curl.log" 'github.com/indrasvat/dootsabha/releases/latest'
assert_not_contains "${latest_dir}/curl.log" 'api.github.com'

explicit_dir=$(run_install explicit ok env VERSION=v7.7.7 INSTALL_SKILL=0)
[[ -x "${explicit_dir}/install/dootsabha" ]]
assert_contains "${explicit_dir}/stdout" 'Version:.*v7\.7\.7'
assert_contains "${explicit_dir}/stdout" 'Skipped skill install because INSTALL_SKILL=0'
assert_contains "${explicit_dir}/curl.log" 'releases/download/v7\.7\.7/dootsabha-'
assert_not_contains "${explicit_dir}/curl.log" 'releases/latest|api.github.com'
[[ ! -e "${explicit_dir}/npx.log" ]]

rate_dir="${tmp}/rate-limit"
mkdir -p "${rate_dir}"
make_fake_tools api403 "${rate_dir}/fakebin"
set +e
FAKE_CURL_LOG="${rate_dir}/curl.log" \
FAKE_NPX_LOG="${rate_dir}/npx.log" \
FAKE_CURL_MODE=api403 \
TEST_BINARY="${BIN}" \
TEST_SHA="${TEST_SHA}" \
TEST_PLATFORM="${TEST_PLATFORM}" \
PATH="${rate_dir}/fakebin:$PATH" \
NONINTERACTIVE=1 \
INSTALL_DIR="${rate_dir}/install" \
sh "${ROOT}/install.sh" >"${rate_dir}/stdout" 2>"${rate_dir}/stderr"
status=$?
set -e
if [[ ${status} -eq 0 ]]; then
  echo "expected rate-limit install to fail" >&2
  exit 1
fi
assert_contains "${rate_dir}/stderr" 'GitHub API rate-limited'
assert_contains "${rate_dir}/stdout" 'VERSION=vX.Y.Z'

# --- Regression for #23: no controlling terminal must not abort the install ---
#
# `curl … | sh` from CI, a container or an agent shell has no controlling
# terminal. /dev/tty still passes -e and -r there — access(2) succeeds — but
# open(2) returns ENXIO, so the old existence check let a doomed `read` through
# and `set -eu` turned that into an aborted install at the directory prompt.
#
# Every other test here passes NONINTERACTIVE=1, which skips the prompt
# entirely. That is exactly why this shipped, so this one must NOT set it.
#
# setsid() guarantees no controlling terminal whether or not the person running
# the suite has a terminal — without it this test would hang on a real tty,
# waiting for someone to answer the prompt.
if command -v python3 >/dev/null 2>&1; then
  tty_home="${tmp}/tty-home"
  tty_dir="${tmp}/tty-fallback"
  mkdir -p "${tty_home}/.local/bin" "${tty_dir}/fakebin"
  make_fake_tools ok "${tty_dir}/fakebin"

  set +e
  FAKE_CURL_LOG="${tty_dir}/curl.log" \
  FAKE_NPX_LOG="${tty_dir}/npx.log" \
  FAKE_CURL_MODE=ok \
  TEST_BINARY="${BIN}" \
  TEST_SHA="${TEST_SHA}" \
  TEST_PLATFORM="${TEST_PLATFORM}" \
  HOME="${tty_home}" \
  PATH="${tty_dir}/fakebin:${tty_home}/.local/bin:/usr/bin:/bin" \
  INSTALL_SKILL=0 \
  python3 -c 'import os,sys; os.setsid(); os.execvp(sys.argv[1], sys.argv[1:])' \
    sh "${ROOT}/install.sh" >"${tty_dir}/stdout" 2>"${tty_dir}/stderr" </dev/null
  status=$?
  set -e

  if [[ ${status} -ne 0 ]]; then
    echo "interactive install with no controlling terminal exited ${status}" >&2
    cat "${tty_dir}/stderr" >&2
    exit 1
  fi
  if grep -q 'Device not configured\|/dev/tty' "${tty_dir}/stderr"; then
    echo "installer tripped over /dev/tty instead of falling back" >&2
    cat "${tty_dir}/stderr" >&2
    exit 1
  fi
  assert_contains "${tty_dir}/stdout" 'dootsabha is ready'
  # It must have taken the default, inside the sandboxed HOME — never the real one.
  if [[ ! -x "${tty_home}/.local/bin/dootsabha" ]]; then
    echo "expected the default dir to be chosen inside the test HOME" >&2
    exit 1
  fi
fi

echo "install.sh tests passed"

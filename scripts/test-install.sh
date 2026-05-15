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
  NONINTERACTIVE=1 \
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

echo "install.sh tests passed"

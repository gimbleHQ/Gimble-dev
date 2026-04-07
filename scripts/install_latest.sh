#!/usr/bin/env bash
set -euo pipefail

REPO="${GIMBLE_REPO:-gimbleHQ/Gimble-dev}"
API_BASE="https://api.github.com/repos/${REPO}"

LOG_FILE=""
RUNTIME_PY="python3"

log() { printf "%s\n" "$*"; }
err() { printf "ERROR: %s\n" "$*" >&2; exit 1; }
err_with_log() {
  local msg="$1"
  printf "ERROR: %s\n" "${msg}" >&2
  if [[ -n "${LOG_FILE:-}" ]]; then
    printf "See log: %s\n" "${LOG_FILE}" >&2
  fi
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1
}

init_log() {
  if [[ -n "${LOG_FILE:-}" ]]; then
    return 0
  fi
  LOG_FILE="$(mktemp -t gimble-install.XXXXXX 2>/dev/null || mktemp "/tmp/gimble-install.XXXXXX")"
}

run_quiet() {
  init_log
  "$@" >>"${LOG_FILE}" 2>&1
}

cleanup_log() {
  if [[ -n "${LOG_FILE:-}" && -f "${LOG_FILE}" ]]; then
    rm -f "${LOG_FILE}"
  fi
}

is_darwin() {
  [[ "$(uname -s)" == "Darwin" ]]
}

is_linux() {
  [[ "$(uname -s)" == "Linux" ]]
}

python_venv_ok() {
  local py="${1:-python3}"
  local tmp
  tmp="$(mktemp -d)"
  if run_quiet "${py}" -m venv "${tmp}/venv"; then
    if run_quiet "${tmp}/venv/bin/python" -m pip --version; then
      rm -rf "${tmp}"
      return 0
    fi
  fi
  rm -rf "${tmp}"
  return 1
}

python_virtualenv_ok() {
  local py="${1:-python3}"
  local tmp
  tmp="$(mktemp -d)"
  if run_quiet "${py}" -m virtualenv "${tmp}/venv"; then
    if run_quiet "${tmp}/venv/bin/python" -m pip --version; then
      rm -rf "${tmp}"
      return 0
    fi
  fi
  rm -rf "${tmp}"
  return 1
}

python_virtualenv_ok_download() {
  local py="${1:-python3}"
  local tmp
  tmp="$(mktemp -d)"
  if run_quiet "${py}" -m virtualenv --download "${tmp}/venv"; then
    if run_quiet "${tmp}/venv/bin/python" -m pip --version; then
      rm -rf "${tmp}"
      return 0
    fi
  fi
  rm -rf "${tmp}"
  return 1
}

python_version_minor() {
  local py="${1:-python3}"
  "${py}" - <<'PY'
import sys
print(f"{sys.version_info[0]}.{sys.version_info[1]}")
PY
}

python_version_ge() {
  local py="${1:-python3}"
  local min_major="${2:-3}"
  local min_minor="${3:-8}"
  local ver major minor
  ver="$("${py}" - <<'PY'
import sys
print(f"{sys.version_info[0]}.{sys.version_info[1]}")
PY
  )" || return 1
  major="${ver%%.*}"
  minor="${ver#*.}"
  if [[ -z "${major}" || -z "${minor}" ]]; then
    return 1
  fi
  if [[ "${major}" -gt "${min_major}" ]]; then
    return 0
  fi
  if [[ "${major}" -eq "${min_major}" && "${minor}" -ge "${min_minor}" ]]; then
    return 0
  fi
  return 1
}

ensure_virtualenv_module() {
  local py="${1:-python3}"
  if run_quiet "${py}" -m virtualenv --version; then
    return 0
  fi
  if run_quiet "${py}" -m pip --version; then
    log "Installing virtualenv into ${py} environment..."
    if run_quiet "${py}" -m pip install --upgrade virtualenv; then
      return 0
    fi
  fi
  return 1
}

ensure_system_venv_pkg() {
  if ! is_linux; then
    return 1
  fi
  local pm
  pm="$(detect_pkg_manager)" || return 1
  case "${pm}" in
    apt) install_pkgs_linux "${pm}" python3-venv ;;
    dnf|yum|zypper|xbps) install_pkgs_linux "${pm}" python3-virtualenv ;;
    pacman) install_pkgs_linux "${pm}" python-virtualenv ;;
    apk) install_pkgs_linux "${pm}" py3-virtualenv ;;
    emerge) install_pkgs_linux "${pm}" dev-python/virtualenv ;;
    *) return 1 ;;
  esac
  return 0
}

select_runtime_python() {
  local candidates=()
  local py
  candidates+=(python3)
  for v in 3.12 3.11 3.10 3.9 3.8; do
    candidates+=("python${v}")
  done

  for py in "${candidates[@]}"; do
    if ! need_cmd "${py}"; then
      continue
    fi
    if ! python_version_ge "${py}" 3 8; then
      continue
    fi
    if python_venv_ok "${py}"; then
      RUNTIME_PY="${py}"
      return 0
    fi
  done

  for py in "${candidates[@]}"; do
    if ! need_cmd "${py}"; then
      continue
    fi
    if ! python_version_ge "${py}" 3 8; then
      continue
    fi
    if python_virtualenv_ok "${py}" || python_virtualenv_ok_download "${py}" || ensure_virtualenv_module "${py}"; then
      RUNTIME_PY="${py}"
      return 0
    fi
  done

  return 1
}

has_supported_python() {
  local candidates=()
  local py
  candidates+=(python3)
  for v in 3.12 3.11 3.10 3.9 3.8; do
    candidates+=("python${v}")
  done
  for py in "${candidates[@]}"; do
    if ! need_cmd "${py}"; then
      continue
    fi
    if python_version_ge "${py}" 3 8; then
      return 0
    fi
  done
  return 1
}

go_version_minor() {
  local ver major minor
  ver="$(go version 2>/dev/null | awk '{print $3}' | sed 's/^go//')" || return 1
  major="${ver%%.*}"
  minor="${ver#*.}"
  minor="${minor%%.*}"
  if [[ -z "${major}" || -z "${minor}" ]]; then
    return 1
  fi
  printf "%s.%s" "${major}" "${minor}"
}

go_version_ok() {
  if ! need_cmd go; then
    return 1
  fi
  local ver major minor
  ver="$(go_version_minor)" || return 1
  major="${ver%%.*}"
  minor="${ver#*.}"
  if [[ "${major}" -gt 1 ]]; then
    return 0
  fi
  if [[ "${major}" -eq 1 && "${minor}" -ge 22 ]]; then
    return 0
  fi
  return 1
}

go_os_arch() {
  local os arch
  os="$(uname -s 2>/dev/null | tr -d '\r\n')" || return 1
  case "${os}" in
    Darwin) os="darwin" ;;
    Linux) os="linux" ;;
    *) return 1 ;;
  esac
  arch="$(uname -m 2>/dev/null | tr -d '\r\n')" || return 1
  case "${arch}" in
    *x86_64*|*amd64*) arch="amd64" ;;
    *aarch64*|*arm64*) arch="arm64" ;;
    *i386*|*i686*) arch="386" ;;
    *armv6l*|*armv7l*) arch="armv6l" ;;
    *) return 1 ;;
  esac
  printf "%s %s" "${os}" "${arch}"
}

sha256_file() {
  local file="$1"
  if need_cmd sha256sum; then
    sha256sum "${file}" | awk '{print $1}'
    return 0
  fi
  if need_cmd shasum; then
    shasum -a 256 "${file}" | awk '{print $1}'
    return 0
  fi
  return 1
}

install_go_tarball() {
  ensure_sudo

  local os arch tmp json_file info_file filename sha url used_version_endpoint
  if ! read -r os arch < <(go_os_arch); then
    local raw_os raw_arch
    raw_os="$(uname -s 2>/dev/null | tr -d '\r\n')"
    raw_arch="$(uname -m 2>/dev/null | tr -d '\r\n')"
    case "${raw_os}" in
      Darwin) os="darwin" ;;
      Linux) os="linux" ;;
      *) os="" ;;
    esac
    case "${raw_arch}" in
      *x86_64*|*amd64*) arch="amd64" ;;
      *aarch64*|*arm64*) arch="arm64" ;;
      *i386*|*i686*) arch="386" ;;
      *armv6l*|*armv7l*) arch="armv6l" ;;
      *) arch="" ;;
    esac
    if [[ -z "${os}" || -z "${arch}" ]]; then
      err "Unsupported OS/arch for Go install (uname -s='${raw_os}' uname -m='${raw_arch}')."
    fi
  fi

  tmp="$(mktemp -d)"
  json_file="${tmp}/go.json"
  info_file="${tmp}/go.info"

  init_log
  local version_line version_minor
  used_version_endpoint="false"
  version_line="$(curl -fsSL "https://go.dev/VERSION?m=text" 2>>"${LOG_FILE}" || curl -fsSL "https://golang.org/VERSION?m=text" 2>>"${LOG_FILE}" || true)"
  version_line="$(printf "%s" "${version_line}" | head -n1 | tr -d '\r\n')"
  if [[ "${version_line}" =~ ^go1\. ]]; then
    version_minor="${version_line#go1.}"
    version_minor="${version_minor%%.*}"
    if [[ -n "${version_minor}" && "${version_minor}" -ge 22 ]]; then
      filename="${version_line}.${os}-${arch}.tar.gz"
      used_version_endpoint="true"
    fi
  fi

  if [[ -z "${filename}" ]]; then
    if ! run_quiet curl -fsSL "https://go.dev/dl/?mode=json" -o "${json_file}"; then
      err_with_log "Failed to download Go release manifest."
    fi

    local pybin=""
    if need_cmd python3; then
      pybin="python3"
    elif need_cmd python; then
      pybin="python"
    else
      err "Python is required to resolve Go download metadata. Install python3 or install Go manually."
    fi

    if ! "${pybin}" - "${json_file}" "${os}" "${arch}" >"${info_file}" 2>>"${LOG_FILE}" <<'PY'
import json
import sys

path, os_name, arch = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path, "r", encoding="utf-8") as f:
    data = json.load(f)

def pick_release(releases, want_minor=None):
    for rel in releases:
        version = rel.get("version", "")
        if not version.startswith("go1."):
            continue
        if want_minor is not None and not version.startswith(f"go1.{want_minor}."):
            continue
        for fobj in rel.get("files", []):
            if fobj.get("kind") != "archive":
                continue
            if fobj.get("os") == os_name and fobj.get("arch") == arch:
                return fobj.get("filename", ""), fobj.get("sha256", "")
    return "", ""

# Prefer latest 1.22.x; if not available, fall back to latest >=1.22.
filename, sha = pick_release(data, want_minor=22)
if not filename:
    for rel in data:
        version = rel.get("version", "")
        if not version.startswith("go1."):
            continue
        parts = version[3:].split(".", 1)
        try:
            minor = int(parts[0])
        except Exception:
            continue
        if minor < 22:
            continue
        for fobj in rel.get("files", []):
            if fobj.get("kind") != "archive":
                continue
            if fobj.get("os") == os_name and fobj.get("arch") == arch:
                filename, sha = fobj.get("filename", ""), fobj.get("sha256", "")
                break
        if filename:
            break

if filename and sha:
    print(filename, sha)
    sys.exit(0)
sys.exit(1)
PY
    then
      true
    fi

    read -r filename sha <"${info_file}" 2>/dev/null || true
    used_version_endpoint="false"
  fi

  if [[ -z "${filename}" ]]; then
    err_with_log "Failed to resolve Go 1.22+ download for ${os}/${arch}."
  fi

  url="https://go.dev/dl/${filename}"
  if ! run_quiet curl -fsSL "${url}" -o "${tmp}/${filename}"; then
    err_with_log "Failed to download Go ${filename}."
  fi

  # Checksum verification disabled per current install policy.

  if ! run_quiet "${SUDO_CMD[@]}" rm -rf /usr/local/go; then
    err_with_log "Failed to remove existing /usr/local/go."
  fi
  if ! run_quiet "${SUDO_CMD[@]}" tar -C /usr/local -xzf "${tmp}/${filename}"; then
    err_with_log "Failed to extract Go tarball."
  fi
  if ! run_quiet "${SUDO_CMD[@]}" mkdir -p /usr/local/bin; then
    err_with_log "Failed to create /usr/local/bin."
  fi
  if ! run_quiet "${SUDO_CMD[@]}" ln -sf /usr/local/go/bin/go /usr/local/bin/go; then
    err_with_log "Failed to link go binary."
  fi
  export PATH="/usr/local/go/bin:${PATH}"
}

SUDO_CMD=()
ensure_sudo() {
  if [[ "$(id -u)" -eq 0 ]]; then
    SUDO_CMD=()
    return 0
  fi
  if need_cmd sudo; then
    SUDO_CMD=(sudo)
    return 0
  fi
  err "sudo is required to install dependencies. Install Go/Python manually or rerun as root."
}

detect_pkg_manager() {
  if need_cmd apt-get; then
    echo "apt"
    return 0
  fi
  if need_cmd dnf; then
    echo "dnf"
    return 0
  fi
  if need_cmd yum; then
    echo "yum"
    return 0
  fi
  if need_cmd pacman; then
    echo "pacman"
    return 0
  fi
  if need_cmd apk; then
    echo "apk"
    return 0
  fi
  if need_cmd zypper; then
    echo "zypper"
    return 0
  fi
  if need_cmd xbps-install; then
    echo "xbps"
    return 0
  fi
  if need_cmd emerge; then
    echo "emerge"
    return 0
  fi
  return 1
}

install_pkgs_linux() {
  local pm="$1"
  shift
  local pkgs=("$@")
  ensure_sudo

  case "${pm}" in
    apt)
      if ! run_quiet "${SUDO_CMD[@]}" env DEBIAN_FRONTEND=noninteractive apt-get update -y; then
        err_with_log "Failed to update apt package index."
      fi
      if ! run_quiet "${SUDO_CMD[@]}" env DEBIAN_FRONTEND=noninteractive apt-get install -y "${pkgs[@]}"; then
        err_with_log "Failed to install packages with apt."
      fi
      ;;
    dnf)
      if ! run_quiet "${SUDO_CMD[@]}" dnf install -y "${pkgs[@]}"; then
        err_with_log "Failed to install packages with dnf."
      fi
      ;;
    yum)
      if ! run_quiet "${SUDO_CMD[@]}" yum install -y "${pkgs[@]}"; then
        err_with_log "Failed to install packages with yum."
      fi
      ;;
    pacman)
      if ! run_quiet "${SUDO_CMD[@]}" pacman -Sy --noconfirm "${pkgs[@]}"; then
        err_with_log "Failed to install packages with pacman."
      fi
      ;;
    apk)
      if ! run_quiet "${SUDO_CMD[@]}" apk add --no-cache "${pkgs[@]}"; then
        err_with_log "Failed to install packages with apk."
      fi
      ;;
    zypper)
      if ! run_quiet "${SUDO_CMD[@]}" zypper --non-interactive install -y "${pkgs[@]}"; then
        err_with_log "Failed to install packages with zypper."
      fi
      ;;
    xbps)
      if ! run_quiet "${SUDO_CMD[@]}" xbps-install -Sy "${pkgs[@]}"; then
        err_with_log "Failed to install packages with xbps-install."
      fi
      ;;
    emerge)
      if ! run_quiet "${SUDO_CMD[@]}" emerge -n "${pkgs[@]}"; then
        err_with_log "Failed to install packages with emerge."
      fi
      ;;
    *)
      err "Unsupported package manager on Linux. Install dependencies manually."
      ;;
  esac
}

install_pkgs_linux_best_effort() {
  local pm="$1"
  shift
  local pkgs=("$@")
  ensure_sudo

  case "${pm}" in
    apt)
      run_quiet "${SUDO_CMD[@]}" env DEBIAN_FRONTEND=noninteractive apt-get install -y "${pkgs[@]}" || true
      ;;
    dnf)
      run_quiet "${SUDO_CMD[@]}" dnf install -y "${pkgs[@]}" || true
      ;;
    yum)
      run_quiet "${SUDO_CMD[@]}" yum install -y "${pkgs[@]}" || true
      ;;
    pacman)
      run_quiet "${SUDO_CMD[@]}" pacman -Sy --noconfirm "${pkgs[@]}" || true
      ;;
    apk)
      run_quiet "${SUDO_CMD[@]}" apk add --no-cache "${pkgs[@]}" || true
      ;;
    zypper)
      run_quiet "${SUDO_CMD[@]}" zypper --non-interactive install -y "${pkgs[@]}" || true
      ;;
    xbps)
      run_quiet "${SUDO_CMD[@]}" xbps-install -Sy "${pkgs[@]}" || true
      ;;
    emerge)
      run_quiet "${SUDO_CMD[@]}" emerge -n "${pkgs[@]}" || true
      ;;
    *)
      true
      ;;
  esac
}

normalize_tag() {
  local raw="${1:-}"
  [[ -n "${raw}" ]] || return 1
  if [[ "${raw}" == v* ]]; then
    printf "%s" "${raw}"
  else
    printf "v%s" "${raw}"
  fi
}

resolve_latest_tag() {
  local release_tag latest_tag
  if need_cmd python3; then
    release_tag="$(curl -fsSL "${API_BASE}/releases/latest" | python3 -c 'import sys, json; d=json.load(sys.stdin); print(d.get("tag_name", ""))' 2>/dev/null || true)"
  else
    release_tag="$(curl -fsSL "${API_BASE}/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1 || true)"
  fi

  latest_tag="$(curl -fsSL "${API_BASE}/tags" | sed -n 's/.*"name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1 || true)"

  if [[ -n "${latest_tag}" && "${latest_tag}" != "${release_tag}" ]]; then
    printf "%s" "${latest_tag}"
    return
  fi

  if [[ -n "${release_tag}" ]]; then
    printf "%s" "${release_tag}"
    return
  fi

  [[ -n "${latest_tag}" ]] || err "Could not resolve latest release tag for ${REPO}"
  printf "%s" "${latest_tag}"
}


install_go_if_missing() {
  if go_version_ok; then
    return 0
  fi

  if is_darwin; then
    if need_cmd brew; then
      if need_cmd go; then
        if ! run_quiet brew upgrade go; then
          err_with_log "Failed to upgrade Go with Homebrew."
        fi
      else
        if ! run_quiet brew install go; then
          err_with_log "Failed to install Go with Homebrew."
        fi
      fi
      if go_version_ok; then
        return 0
      fi
    fi
    install_go_tarball
    if go_version_ok; then
      return 0
    fi
    err_with_log "Go 1.22+ is required but could not be installed."
  fi

  if is_linux; then
    install_go_tarball
    if go_version_ok; then
      return 0
    fi
    err_with_log "Go 1.22+ is required but could not be installed."
  fi

  err "Go is required to build Gimble. Install Go and rerun this script."
}

ensure_python_runtime() {
  if is_darwin; then
    if ! need_cmd python3; then
      need_cmd brew || err "Python3 is required. Install Homebrew first or install Python manually."
      if ! run_quiet brew install python; then
        err_with_log "Failed to install Python3 with Homebrew."
      fi
    fi

    if ! python3 -m pip --version >/dev/null 2>&1; then
      if ! run_quiet python3 -m ensurepip --upgrade; then
        if need_cmd brew; then
          if ! run_quiet brew install python; then
            err_with_log "Failed to provision pip for Python3."
          fi
        else
          err_with_log "Python3 pip is missing. Install Python3 (with pip) and rerun."
        fi
      fi
    fi

    if ! has_supported_python; then
      err "Python 3.8+ is required. Install a newer Python version and rerun."
    fi

    if select_runtime_python; then
      return 0
    fi

    if need_cmd brew; then
      if ! run_quiet brew install python; then
        err_with_log "Python3 venv is missing."
      fi
    else
      err_with_log "Python3 venv is missing. Install Python3 (with venv) and rerun."
    fi

    if select_runtime_python; then
      return 0
    fi

    err_with_log "Python 3.8+ venv is still unavailable after installing dependencies."
  fi

  if is_linux; then
    local pm
    pm="$(detect_pkg_manager)" || err "Unsupported Linux distro. Install Python3 manually."

    if ! need_cmd python3; then
      case "${pm}" in
        apt) install_pkgs_linux "${pm}" python3 ;;
        dnf|yum|apk|zypper|xbps) install_pkgs_linux "${pm}" python3 ;;
        pacman) install_pkgs_linux "${pm}" python ;;
        emerge) install_pkgs_linux "${pm}" dev-lang/python ;;
        *) err "Unsupported Linux distro. Install Python3 manually." ;;
      esac
    fi

    if ! python3 -m pip --version >/dev/null 2>&1; then
      if run_quiet python3 -m ensurepip --upgrade; then
        true
      else
        case "${pm}" in
          apt) install_pkgs_linux "${pm}" python3-pip ;;
          dnf|yum|zypper|xbps) install_pkgs_linux "${pm}" python3-pip ;;
          pacman) install_pkgs_linux "${pm}" python-pip ;;
          apk) install_pkgs_linux "${pm}" py3-pip ;;
          emerge) install_pkgs_linux "${pm}" dev-python/pip ;;
          *) err "Unsupported Linux distro. Install pip manually." ;;
        esac
      fi
    fi

    if ! has_supported_python; then
      err "Python 3.8+ is required. Install a newer Python version and rerun."
    fi

    if select_runtime_python; then
      return 0
    fi

    case "${pm}" in
      apt) install_pkgs_linux "${pm}" python3-venv ;;
      dnf|yum) install_pkgs_linux "${pm}" python3-virtualenv ;;
      pacman) install_pkgs_linux "${pm}" python-virtualenv ;;
      apk) install_pkgs_linux "${pm}" py3-virtualenv ;;
      zypper|xbps) install_pkgs_linux "${pm}" python3-virtualenv ;;
      emerge) install_pkgs_linux "${pm}" dev-python/virtualenv ;;
      *) err "Unsupported Linux distro. Install python venv manually." ;;
    esac

    if select_runtime_python; then
      return 0
    fi

    case "${pm}" in
      apt) install_pkgs_linux "${pm}" python3-virtualenv ;;
      dnf|yum|zypper|xbps) install_pkgs_linux "${pm}" python3-virtualenv ;;
      pacman) install_pkgs_linux "${pm}" python-virtualenv ;;
      apk) install_pkgs_linux "${pm}" py3-virtualenv ;;
      emerge) install_pkgs_linux "${pm}" dev-python/virtualenv ;;
      *) true ;;
    esac

    if select_runtime_python; then
      return 0
    fi

    err_with_log "Python 3.8+ venv/virtualenv is still unavailable after installing dependencies."
    return 0
  fi

  err "Unsupported OS. Python3 is required to install the Gimble runtime."
}

setup_python_runtime() {
  local srcdir="$1"
  local setup_script="${srcdir}/python/setup_runtime.sh"
  if [[ ! -f "${setup_script}" ]]; then
    err "missing runtime setup script: ${setup_script}"
  fi

  if run_quiet sh "${setup_script}"; then
    return 0
  fi

  local base_dir
  if is_darwin; then
    base_dir="${HOME}/Library/Application Support/gimble"
  else
    base_dir="${HOME}/.config/gimble"
  fi
  local venv_dir="${base_dir}/pyenv"

  if ! run_quiet mkdir -p "${base_dir}"; then
    err_with_log "Failed to create Gimble runtime directory."
  fi
  local py="${RUNTIME_PY:-python3}"
  # Clean up a broken or partial venv before creating a new one.
  if [[ -d "${venv_dir}" && ! -x "${venv_dir}/bin/python3" && ! -x "${venv_dir}/bin/python" ]]; then
    rm -rf "${venv_dir}" || true
  fi
  if ! run_quiet "${py}" -m venv "${venv_dir}"; then
    ensure_system_venv_pkg || true
    ensure_virtualenv_module "${py}" || true
  fi
  if ! run_quiet "${py}" -m venv "${venv_dir}"; then
    if run_quiet "${py}" -m virtualenv --download "${venv_dir}" || run_quiet "${py}" -m virtualenv "${venv_dir}"; then
      true
    else
      err_with_log "Failed to create Gimble Python venv."
    fi
  fi
  if ! run_quiet "${venv_dir}/bin/python3" -m pip install --upgrade pip; then
    err_with_log "Failed to upgrade pip in Gimble venv."
  fi
  if ! run_quiet "${venv_dir}/bin/python3" -m pip install -r "${srcdir}/python/requirements-core.txt"; then
    err_with_log "Failed to install Gimble Python runtime requirements."
  fi
}

install_python_assets() {
  local srcdir="$1"
  local target_dir
  local fallback_dir

  if [[ "$(uname -s)" == "Darwin" && -d "/opt/homebrew/share" ]]; then
    target_dir="/opt/homebrew/share/gimble"
  elif [[ -d "/usr/local/share" ]]; then
    target_dir="/usr/local/share/gimble"
  else
    target_dir="${HOME}/.local/share/gimble"
  fi

  if mkdir -p "${target_dir}" 2>/dev/null; then
    if cp -R "${srcdir}/python" "${target_dir}/" 2>/dev/null; then
      log "Installed python assets to ${target_dir}/python"
      return 0
    fi
  fi

  if need_cmd sudo; then
    if sudo mkdir -p "${target_dir}" && sudo cp -R "${srcdir}/python" "${target_dir}/"; then
      log "Installed python assets to ${target_dir}/python"
      return 0
    fi
  fi

  fallback_dir="${HOME}/.local/share/gimble"
  mkdir -p "${fallback_dir}"
  if cp -R "${srcdir}/python" "${fallback_dir}/"; then
    log "Installed python assets to ${fallback_dir}/python"
    return 0
  fi
  err "Failed to install python assets to ${target_dir} or ${fallback_dir}."
}

install_binary() {
  local src_bin="$1"
  local target

  if [[ "$(uname -s)" == "Darwin" && -d "/opt/homebrew/bin" ]]; then
    target="/opt/homebrew/bin/gimble"
  else
    target="/usr/local/bin/gimble"
  fi

  if install -m 0755 "${src_bin}" "${target}" 2>/dev/null; then
    log "Installed gimble to ${target}"
    return 0
  fi

  if need_cmd sudo && sudo install -m 0755 "${src_bin}" "${target}"; then
    log "Installed gimble to ${target}"
    return 0
  fi

  mkdir -p "${HOME}/.local/bin"
  install -m 0755 "${src_bin}" "${HOME}/.local/bin/gimble"
  log "Installed gimble to ${HOME}/.local/bin/gimble"
  log "Add this to PATH if needed: export PATH=\"${HOME}/.local/bin:$PATH\""
}

main() {
  need_cmd curl || err "curl is required"
  need_cmd tar || err "tar is required"

  local tag version tarball url srcdir
  tmpdir=""

  if [[ -n "${GIMBLE_TAG:-}" ]]; then
    tag="$(normalize_tag "${GIMBLE_TAG}")" || err "Invalid GIMBLE_TAG: '${GIMBLE_TAG}'"
    log "Using requested tag ${tag}"
  elif [[ $# -gt 0 && -n "${1:-}" ]]; then
    tag="$(normalize_tag "${1}")" || err "Invalid tag argument: '${1}'"
    log "Using requested tag ${tag}"
  else
    tag="$(resolve_latest_tag)"
  fi

  version="${tag#v}"
  url="https://github.com/${REPO}/archive/refs/tags/${tag}.tar.gz"

  log "Installing Gimble ${tag} from ${REPO}"
  install_go_if_missing

  tmpdir="$(mktemp -d)"
  trap 'tmpdir_safe="${tmpdir:-}"; [ -n "${tmpdir_safe}" ] && rm -rf "${tmpdir_safe}"' EXIT

  tarball="${tmpdir}/gimble.tar.gz"
  curl -fsSL "${url}" -o "${tarball}"
  tar -xzf "${tarball}" -C "${tmpdir}"

  srcdir="$(find "${tmpdir}" -maxdepth 1 -type d -name 'Gimble-dev-*' | head -n1)"
  [[ -n "${srcdir}" ]] || err "Could not locate extracted source directory"

  (cd "${srcdir}" && go build -ldflags "-X main.version=${version}" -o "${tmpdir}/gimble" ./cmd/gimble)

  install_binary "${tmpdir}/gimble"
  install_python_assets "${srcdir}"
  ensure_python_runtime
  setup_python_runtime "${srcdir}"

  log "Installed version: $(gimble --version || true)"
  log "Next: run 'gimble'"
  cleanup_log
}

main "$@"

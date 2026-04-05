#!/usr/bin/env bash
set -euo pipefail

REPO="${GIMBLE_REPO:-Saketspradhan/Gimble-dev}"
API_BASE="https://api.github.com/repos/${REPO}"

log() { printf "%s\n" "$*"; }
err() { printf "ERROR: %s\n" "$*" >&2; exit 1; }

need_cmd() {
  command -v "$1" >/dev/null 2>&1
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
  if need_cmd go; then
    return 0
  fi

  local os
  os="$(uname -s)"

  if [[ "${os}" == "Darwin" ]]; then
    need_cmd brew || err "Go is required. Install Homebrew first or install Go manually."
    log "Go not found. Installing Go with Homebrew..."
    brew install go
    return 0
  fi

  if [[ "${os}" == "Linux" ]]; then
    if need_cmd apt-get; then
      log "Go not found. Installing Go with apt..."
      sudo apt-get update -y
      sudo apt-get install -y golang-go
      return 0
    fi

    if need_cmd dnf; then
      log "Go not found. Installing Go with dnf..."
      sudo dnf install -y golang
      return 0
    fi

    if need_cmd yum; then
      log "Go not found. Installing Go with yum..."
      sudo yum install -y golang
      return 0
    fi

    if need_cmd pacman; then
      log "Go not found. Installing Go with pacman..."
      sudo pacman -Sy --noconfirm go
      return 0
    fi
  fi

  err "Go is required to build Gimble. Install Go and rerun this script."
}

install_python_assets() {
  local srcdir="$1"
  local target_dir

  if [[ "$(uname -s)" == "Darwin" && -d "/opt/homebrew/share" ]]; then
    target_dir="/opt/homebrew/share/gimble"
  elif [[ -d "/usr/local/share" ]]; then
    target_dir="/usr/local/share/gimble"
  else
    target_dir="${HOME}/.local/share/gimble"
  fi

  mkdir -p "${target_dir}"
  cp -R "${srcdir}/python" "${target_dir}/"
  log "Installed python assets to ${target_dir}/python"
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

  tag="$(resolve_latest_tag)"
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

  log "Installed version: $(gimble --version || true)"
  log "Next: run 'gimble'"
}

main "$@"

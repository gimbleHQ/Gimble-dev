#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <version-or-tag>" >&2
  exit 2
fi

RAW="$1"
VERSION="${RAW#v}"
TAG="v${VERSION}"
FORMULA="Formula/gimble.rb"
REPO_URL="https://github.com/gimbleHQ/Gimble-dev"
TARBALL_URL="${REPO_URL}/archive/refs/tags/${TAG}.tar.gz"

if [[ ! -f "${FORMULA}" ]]; then
  echo "formula not found: ${FORMULA}" >&2
  exit 1
fi

TMP="$(mktemp -d)"
trap 'rm -rf "${TMP}"' EXIT

curl -fsSL "${TARBALL_URL}" -o "${TMP}/${TAG}.tar.gz"
SHA256="$(shasum -a 256 "${TMP}/${TAG}.tar.gz" | awk '{print $1}')"

perl -0pi -e "s/version \"[^\"]+\"/version \"${VERSION}\"/g" "${FORMULA}"
perl -0pi -e "s@url \"https://github.com/gimbleHQ/Gimble-dev/archive/refs/tags/v[^\"]+\.tar\.gz\"@url \"${TARBALL_URL}\"@g" "${FORMULA}"
perl -0pi -e "s/sha256 \"[a-f0-9]{64}\"/sha256 \"${SHA256}\"/g" "${FORMULA}"
perl -0pi -e "s/main\.version=[0-9]+\.[0-9]+\.[0-9]+/main.version=${VERSION}/g" "${FORMULA}"

echo "Updated ${FORMULA} -> version ${VERSION}"
echo "sha256: ${SHA256}"

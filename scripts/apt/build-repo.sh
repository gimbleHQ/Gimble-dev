#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <version> <repo-root> [deb-files...]" >&2
  exit 1
fi

VERSION="$1"
REPO_ROOT="$2"
shift 2

if [[ $# -eq 0 ]]; then
  mapfile -t DEB_FILES < <(find dist -maxdepth 1 -type f -name 'gimble_*_*.deb' | sort)
else
  DEB_FILES=("$@")
fi

if [[ ${#DEB_FILES[@]} -eq 0 ]]; then
  echo "no .deb files found" >&2
  exit 1
fi

rm -rf "$REPO_ROOT"
mkdir -p "$REPO_ROOT/pool/main/g/gimble"
mkdir -p "$REPO_ROOT/dists/stable/main/binary-amd64"
mkdir -p "$REPO_ROOT/dists/stable/main/binary-arm64"

for deb in "${DEB_FILES[@]}"; do
  cp "$deb" "$REPO_ROOT/pool/main/g/gimble/"
done

pushd "$REPO_ROOT" >/dev/null

dpkg-scanpackages --arch amd64 pool > dists/stable/main/binary-amd64/Packages
gzip -fk dists/stable/main/binary-amd64/Packages

dpkg-scanpackages --arch arm64 pool > dists/stable/main/binary-arm64/Packages
gzip -fk dists/stable/main/binary-arm64/Packages

apt-ftparchive \
  -o APT::FTPArchive::Release::Origin="gimble" \
  -o APT::FTPArchive::Release::Label="gimble" \
  -o APT::FTPArchive::Release::Suite="stable" \
  -o APT::FTPArchive::Release::Codename="stable" \
  -o APT::FTPArchive::Release::Architectures="amd64 arm64" \
  -o APT::FTPArchive::Release::Components="main" \
  -o APT::FTPArchive::Release::Description="gimble apt repository" \
  release dists/stable > dists/stable/Release

cat > VERSION <<VERSION_EOF
${VERSION}
VERSION_EOF

popd >/dev/null

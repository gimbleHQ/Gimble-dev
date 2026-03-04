#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: $0 <repo-root>" >&2
  exit 1
fi

REPO_ROOT="$1"

: "${GPG_PRIVATE_KEY_B64:?GPG_PRIVATE_KEY_B64 is required}"
: "${GPG_KEY_ID:?GPG_KEY_ID is required}"
: "${GPG_PASSPHRASE:?GPG_PASSPHRASE is required}"

export GNUPGHOME
GNUPGHOME="$(mktemp -d)"
trap 'rm -rf "$GNUPGHOME"' EXIT

echo "$GPG_PRIVATE_KEY_B64" | base64 --decode | gpg --batch --import

RELEASE_FILE="$REPO_ROOT/dists/stable/Release"
INRELEASE_FILE="$REPO_ROOT/dists/stable/InRelease"
RELEASE_SIG_FILE="$REPO_ROOT/dists/stable/Release.gpg"

gpg --batch --yes --pinentry-mode loopback --passphrase "$GPG_PASSPHRASE" \
  --local-user "$GPG_KEY_ID" --armor --detach-sign \
  --output "$RELEASE_SIG_FILE" "$RELEASE_FILE"

gpg --batch --yes --pinentry-mode loopback --passphrase "$GPG_PASSPHRASE" \
  --local-user "$GPG_KEY_ID" --clearsign \
  --output "$INRELEASE_FILE" "$RELEASE_FILE"

gpg --batch --yes --output "$REPO_ROOT/gimble-archive-keyring.gpg" --export "$GPG_KEY_ID"

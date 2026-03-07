#!/usr/bin/env sh
set -eu

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

if [ "$(uname -s)" = "Darwin" ]; then
  BASE_DIR="$HOME/Library/Application Support/gimble"
else
  BASE_DIR="$HOME/.config/gimble"
fi

VENV_DIR="$BASE_DIR/pyenv"
mkdir -p "$BASE_DIR"

python3 -m venv "$VENV_DIR" >/dev/null 2>&1
"$VENV_DIR/bin/python3" -m pip install --upgrade --quiet pip >/dev/null 2>&1

# Core runtime is mandatory for gim chat to boot on first install.
"$VENV_DIR/bin/python3" -m pip install --quiet -r "$ROOT_DIR/python/requirements-core.txt" >/dev/null 2>&1

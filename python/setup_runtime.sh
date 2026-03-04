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

python3 -m venv "$VENV_DIR"
"$VENV_DIR/bin/python3" -m pip install --upgrade pip
"$VENV_DIR/bin/python3" -m pip install -r "$ROOT_DIR/python/requirements.txt"

echo "Gimble Python runtime ready: $VENV_DIR"
echo "You can now run: gimble -> gim chat"

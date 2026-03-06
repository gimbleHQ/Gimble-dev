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

# Core runtime is mandatory for gim chat to boot on first install.
"$VENV_DIR/bin/python3" -m pip install -r "$ROOT_DIR/python/requirements-core.txt"

# Optional local-LLM extras are best-effort; chat should still work without them.
if ! "$VENV_DIR/bin/python3" -m pip install -r "$ROOT_DIR/python/requirements-optional-local.txt"; then
  echo "warning: optional local model deps could not be installed; Groq/OpenAI chat will still work." >&2
fi

echo "Gimble Python runtime ready: $VENV_DIR"
echo "You can now run: gimble -> gim chat"

#!/usr/bin/env sh
set -eu

VERSION="${1:?version required}"
ARCH="${2:?architecture required (amd64 or arm64)}"
APP="gimble"

case "$ARCH" in
  amd64|arm64) ;;
  *)
    echo "unsupported arch: $ARCH" >&2
    exit 1
    ;;
esac

BIN_PATH="dist/${APP}-linux-${ARCH}"
if [ ! -f "$BIN_PATH" ]; then
  echo "missing binary: $BIN_PATH" >&2
  echo "run: make build-linux" >&2
  exit 1
fi

PKG_ROOT=".pkg/${APP}_${VERSION}_${ARCH}"
rm -rf "$PKG_ROOT"
mkdir -p "$PKG_ROOT/DEBIAN" "$PKG_ROOT/usr/bin"

install -m 0755 "$BIN_PATH" "$PKG_ROOT/usr/bin/${APP}"

cat > "$PKG_ROOT/DEBIAN/control" <<CONTROL
Package: ${APP}
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: ${ARCH}
Maintainer: Gimble Team <dev@gimble.dev>
Description: Gimble CLI
 Gimble command-line tool.
CONTROL

dpkg-deb --build "$PKG_ROOT" "dist/${APP}_${VERSION}_${ARCH}.deb"
echo "Created dist/${APP}_${VERSION}_${ARCH}.deb"

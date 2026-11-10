#!/bin/sh
# Installs the jot release binary for this OS and arch to $JOT_INSTALL_DIR (default ~/.local/bin).
# Usage: curl -fsSL https://raw.githubusercontent.com/moneytosms/jot/main/install-release.sh | sh
# Env: JOT_VERSION (default latest, e.g. v0.1.0), JOT_INSTALL_DIR (default $HOME/.local/bin)
set -eu

REPO=https://github.com/moneytosms/jot
VERSION=${JOT_VERSION:-latest}
DIR=${JOT_INSTALL_DIR:-$HOME/.local/bin}

case $(uname -s) in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) echo "unsupported OS: $(uname -s). Use a release asset manually or build from source (see README)." >&2; exit 1 ;;
esac
case $(uname -m) in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "unsupported arch: $(uname -m). Use a release asset manually or build from source (see README)." >&2; exit 1 ;;
esac

if [ "$VERSION" = latest ]; then
  BASE=$REPO/releases/latest/download
else
  BASE=$REPO/releases/download/$VERSION
fi
ASSET=jot-$os-$arch

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM

curl -fsSL -o "$TMP/SHA256SUMS" "$BASE/SHA256SUMS" || { echo "download failed: $BASE/SHA256SUMS" >&2; exit 1; }
curl -fsSL -o "$TMP/$ASSET" "$BASE/$ASSET" || { echo "download failed: $BASE/$ASSET" >&2; exit 1; }

if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$TMP/$ASSET" | awk '{print $1}')
else
  got=$(shasum -a 256 "$TMP/$ASSET" | awk '{print $1}')
fi
want=$(awk -v f="$ASSET" '$2 == f || $2 == "*" f {print $1}' "$TMP/SHA256SUMS")
if [ -z "$want" ] || [ "$got" != "$want" ]; then
  echo "checksum verification failed for $ASSET" >&2
  exit 1
fi

mkdir -p "$DIR"
cp "$TMP/$ASSET" "$DIR/jot"
chmod 755 "$DIR/jot"
echo "installed jot to $DIR/jot"
case ":$PATH:" in
  *":$DIR:"*) ;;
  *) echo "$DIR is not on PATH. add it with: export PATH=\"$DIR:\$PATH\"" ;;
esac
echo "run: jot help"

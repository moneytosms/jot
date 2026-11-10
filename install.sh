#!/usr/bin/env bash
#
# Install jot from a source checkout.
#
#   ./install.sh                     install into ~/.local/bin
#   ./install.sh --prefix /usr/local install into /usr/local/bin
#   BINDIR=~/bin ./install.sh        install into ~/bin
#
# Requires Go 1.22 or newer. Builds with the version from git when available.

set -euo pipefail

BIN_NAME="jot"
PREFIX="${PREFIX:-$HOME/.local}"
BINDIR="${BINDIR:-$PREFIX/bin}"
VERSION="${VERSION:-}"

usage() {
	cat <<'EOF'
Usage: ./install.sh [options]

Options:
  --prefix <dir>   install prefix, binary goes into <dir>/bin (default ~/.local)
  --bin-dir <dir>  install the binary directly into <dir>
  --debug          build without stripping symbols
  -h, --help       show this help

Environment:
  PREFIX           same as --prefix
  BINDIR           same as --bin-dir
  VERSION          version string to embed (default: git describe)
  GO               go binary to use (default: go)
EOF
}

DEBUG=0
while [ $# -gt 0 ]; do
	case "$1" in
	--prefix)
		[ $# -ge 2 ] || { echo "install.sh: --prefix needs a value" >&2; exit 2; }
		PREFIX="$2"
		BINDIR="$2/bin"
		shift 2
		;;
	--bin-dir)
		[ $# -ge 2 ] || { echo "install.sh: --bin-dir needs a value" >&2; exit 2; }
		BINDIR="$2"
		shift 2
		;;
	--debug)
		DEBUG=1
		shift
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		echo "install.sh: unknown option $1" >&2
		usage >&2
		exit 2
		;;
	esac
done

GO="${GO:-go}"
if ! command -v "$GO" >/dev/null 2>&1; then
	echo "install.sh: Go is required to build jot; install it from https://go.dev/dl/" >&2
	exit 1
fi

# Build from the repository this script lives in.
cd "$(dirname "$0")"

if [ -z "$VERSION" ]; then
	if command -v git >/dev/null 2>&1 && git rev-parse --git-dir >/dev/null 2>&1; then
		VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)"
	else
		VERSION="0.0.0-dev"
	fi
fi

LDFLAGS="-X main.version=$VERSION"
if [ "$DEBUG" -eq 0 ]; then
	LDFLAGS="-s -w $LDFLAGS"
fi

TMP_BIN="$(mktemp "${TMPDIR:-/tmp}/jot-build.XXXXXX")"
trap 'rm -f "$TMP_BIN"' EXIT

echo "Building jot $VERSION ..."
"$GO" build -trimpath -ldflags "$LDFLAGS" -o "$TMP_BIN" ./cmd/jot

mkdir -p "$BINDIR"
install -m 0755 "$TMP_BIN" "$BINDIR/$BIN_NAME"

echo "Installed $BINDIR/$BIN_NAME"
"$BINDIR/$BIN_NAME" version

case ":${PATH}:" in
*":$BINDIR:"*)
	echo
	echo "Next: run \`jot setup\` to pick your journal directory and editor."
	;;
*)
	echo
	echo "$BINDIR is not on your PATH yet. Add it to your shell config, for example:"
	echo "  echo 'export PATH=\"$BINDIR:\$PATH\"' >> ~/.bashrc && source ~/.bashrc"
	echo "  # zsh: ~/.zshrc   fish: ~/.config/fish/config.fish"
	echo
	echo "Then run \`jot setup\`."
	;;
esac

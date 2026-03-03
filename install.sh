#!/bin/sh
set -eu

# hlgo installer — downloads the latest release binary from GitHub.
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/timbrinded/hlgo/main/install.sh | sh
#
# Environment variables:
#   HLGO_INSTALL_DIR  — override install directory (default: ~/.local/bin)

REPO="timbrinded/hlgo"
INSTALL_DIR="${HLGO_INSTALL_DIR:-$HOME/.local/bin}"

fail() { printf 'Error: %s\n' "$1" >&2; exit 1; }

# Detect OS
case "$(uname -s)" in
    Linux*)  OS="linux"  ;;
    Darwin*) OS="darwin" ;;
    *)       fail "unsupported OS: $(uname -s)" ;;
esac

# Detect architecture
case "$(uname -m)" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)             fail "unsupported architecture: $(uname -m)" ;;
esac

# Fetch latest release tag
printf 'Fetching latest release...\n'
TAG=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" \
    | grep '"tag_name"' | head -1 | cut -d '"' -f 4)

[ -z "$TAG" ] && fail "could not determine latest release tag"

VERSION="${TAG#v}"
ARCHIVE="hlgo_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${TAG}/${ARCHIVE}"

printf 'Installing hlgo %s (%s/%s)...\n' "$TAG" "$OS" "$ARCH"

# Download and extract
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -sSfL -o "${TMPDIR}/${ARCHIVE}" "$URL" || fail "download failed: $URL"
tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR"

# Install binary
mkdir -p "$INSTALL_DIR"
mv "${TMPDIR}/hlgo" "${INSTALL_DIR}/hlgo"
chmod +x "${INSTALL_DIR}/hlgo"

printf 'Installed hlgo %s to %s/hlgo\n' "$TAG" "$INSTALL_DIR"

# Check PATH
case ":$PATH:" in
    *":${INSTALL_DIR}:"*) ;;
    *) printf '\nNote: %s is not in your PATH. Add it with:\n  export PATH="%s:$PATH"\n' "$INSTALL_DIR" "$INSTALL_DIR" ;;
esac

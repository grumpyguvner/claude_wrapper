#!/usr/bin/env bash
set -euo pipefail

REPO="grumpyguvner/claude_wrapper"
BINARY="claude-wrapper"
INSTALL_DIR="/usr/local/bin"

# Check platform
OS="$(uname -s)"
ARCH="$(uname -m)"

if [ "$OS" != "Linux" ]; then
    echo "error: only Linux is supported (detected: ${OS})" >&2
    exit 1
fi

if [ "$ARCH" != "x86_64" ]; then
    echo "error: only amd64/x86_64 is supported (detected: ${ARCH})" >&2
    exit 1
fi

# Get latest version
echo "Fetching latest release..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$LATEST" ]; then
    echo "error: could not determine latest release" >&2
    exit 1
fi

echo "Installing ${BINARY} ${LATEST}..."

# Download binary to temp file
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

curl -fsSL "https://github.com/${REPO}/releases/download/${LATEST}/${BINARY}" -o "$TMP"
chmod +x "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMP" "${INSTALL_DIR}/${BINARY}"
else
    echo "Installing to ${INSTALL_DIR} (requires sudo)..."
    sudo mv "$TMP" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed ${BINARY} ${LATEST} to ${INSTALL_DIR}/${BINARY}"

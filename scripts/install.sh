#!/bin/bash
# install.sh - Install claude-wrapper from the latest GitHub release
#
# Usage:
#   curl -fsSL https://github.com/grumpyguvner/claude_wrapper/releases/latest/download/install.sh | bash

set -e

REPO="grumpyguvner/claude_wrapper"
BINARY_NAME="claude-wrapper"
ASSET_NAME="claude-wrapper-linux-amd64"

# --- Platform check ---

OS=$(uname -s)
ARCH=$(uname -m)

if [ "$OS" != "Linux" ] || [ "$ARCH" != "x86_64" ]; then
    echo "Error: only Linux amd64 is supported (detected: $OS $ARCH)"
    exit 1
fi

# --- Determine install directory ---

INSTALL_DIR="$HOME/.local/bin"
mkdir -p "$INSTALL_DIR"

# --- Download latest release ---

echo "Fetching latest release..."
DOWNLOAD_URL=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep -o "\"browser_download_url\": *\"[^\"]*${ASSET_NAME}\"" \
    | head -1 \
    | cut -d'"' -f4)

if [ -z "$DOWNLOAD_URL" ]; then
    echo "Error: could not find $ASSET_NAME in latest release"
    exit 1
fi

TMPFILE=$(mktemp)
trap 'rm -f "$TMPFILE"' EXIT

echo "Downloading $ASSET_NAME..."
curl -fsSL -o "$TMPFILE" "$DOWNLOAD_URL"
chmod +x "$TMPFILE"

# --- Install binary ---

INSTALL_PATH="$INSTALL_DIR/$BINARY_NAME"
install -m 755 "$TMPFILE" "$INSTALL_PATH"

echo "Installed $BINARY_NAME to $INSTALL_PATH"

# --- Add alias to shell rc files ---

ALIAS_LINE="alias claude='claude-wrapper'"

add_alias() {
    local rc_file="$1"
    if [ -f "$rc_file" ]; then
        if ! grep -qF "$ALIAS_LINE" "$rc_file"; then
            echo "" >> "$rc_file"
            echo "$ALIAS_LINE" >> "$rc_file"
            echo "Added alias to $rc_file"
        else
            echo "Alias already present in $rc_file"
        fi
    fi
}

add_alias "$HOME/.bashrc"
add_alias "$HOME/.zshrc"

# If neither rc file exists, create .bashrc with the alias
if [ ! -f "$HOME/.bashrc" ] && [ ! -f "$HOME/.zshrc" ]; then
    echo "$ALIAS_LINE" > "$HOME/.bashrc"
    echo "Created $HOME/.bashrc with alias"
fi

# --- Print version ---

VERSION=$("$INSTALL_PATH" --version 2>/dev/null || echo "unknown")
echo ""
echo "claude-wrapper $VERSION installed successfully!"
echo ""
echo "Restart your shell or run:"
echo "  source ~/.bashrc"
echo ""

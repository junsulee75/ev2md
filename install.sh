#!/bin/bash

REPO='junsulee75/ev2md'
INSTALL_DIR="${1:-$HOME/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
[ "$ARCH" = "x86_64" ] && ARCH=amd64
[ "$ARCH" = "arm64" ]  && ARCH=arm64

if [ "$OS" = "windows" ]; then
  ASSET="ev2md_windows_amd64.exe"
  DEST="$INSTALL_DIR/ev2md.exe"
else
  ASSET="ev2md_${OS}_${ARCH}"
  DEST="$INSTALL_DIR/ev2md"
fi

echo "Fetching latest release..."
VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\(.*\)".*/\1/')

if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest release version."
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$VERSION/$ASSET"

echo "Downloading ev2md $VERSION ($ASSET)..."
mkdir -p "$INSTALL_DIR"
curl -L "$URL" -o "$DEST"
chmod +x "$DEST"

# Remove quarantine attribute (macOS only)
if command -v xattr &>/dev/null; then
  echo "Removing macOS quarantine attribute..."
  xattr -d com.apple.quarantine "$DEST" 2>/dev/null || true
fi

echo "Installed: $DEST"

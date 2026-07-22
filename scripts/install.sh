#!/bin/sh
set -e

REPO="dotdevlabs/loopctl"
INSTALL_DIR="${LOOPCTL_INSTALL_DIR:-/usr/local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

LATEST=$(curl -sSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' \
  | sed 's/.*"tag_name": "\(.*\)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "Could not determine latest release tag."
  exit 1
fi

URL="https://github.com/${REPO}/releases/download/${LATEST}/loopctl_${OS}_${ARCH}.tar.gz"

echo "Installing loopctl ${LATEST} (${OS}/${ARCH})..."
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -sSL "$URL" -o "$TMP/loopctl.tar.gz"
tar -xzf "$TMP/loopctl.tar.gz" -C "$TMP"
install -m 0755 "$TMP/loopctl" "$INSTALL_DIR/loopctl"

echo "loopctl installed to $INSTALL_DIR/loopctl"
loopctl version

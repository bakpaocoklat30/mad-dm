#!/usr/bin/env bash
set -e

# Base directory
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENGINE_DIR="$ROOT_DIR/engine"
BIN_DIR="$ROOT_DIR/bin"

mkdir -p "$BIN_DIR"

echo "=================================================="
echo "   🔨 MAD-DM Multi-OS Cross-Compilation"
echo "=================================================="

cd "$ENGINE_DIR"

# Targets: OS/ARCH
TARGETS=(
  "darwin/arm64:mad-dm-cli-mac-arm64:mad-dm-server-mac-arm64"
  "darwin/amd64:mad-dm-cli-mac-intel:mad-dm-server-mac-intel"
  "windows/amd64:mad-dm-cli-windows-amd64.exe:mad-dm-server-windows-amd64.exe"
  "linux/amd64:mad-dm-cli-linux-amd64:mad-dm-server-linux-amd64"
)

for TARGET in "${TARGETS[@]}"; do
  IFS=":" read -r PLATFORM CLI_NAME SRV_NAME <<< "$TARGET"
  IFS="/" read -r GOOS GOARCH <<< "$PLATFORM"

  echo "📦 Compiling for $GOOS / $GOARCH ..."

  # Build CLI
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$BIN_DIR/$CLI_NAME" ./cmd/cli
  # Build Server Daemon
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build -ldflags="-s -w" -o "$BIN_DIR/$SRV_NAME" ./cmd/server

  echo "   ✅ Generated: $BIN_DIR/$CLI_NAME"
  echo "   ✅ Generated: $BIN_DIR/$SRV_NAME"
done

echo ""
echo "🎉 All binaries compiled successfully!"
ls -lh "$BIN_DIR"

#!/bin/bash
set -e

echo "================================================"
echo "  Installing HFT POC Dependencies (macOS)"
echo "================================================"

# 检查操作系统
if [[ "$OSTYPE" != "darwin"* ]]; then
    echo "ERROR: This script is for macOS only"
    echo "For Linux, please install dependencies manually"
    exit 1
fi

# 检查 Homebrew
if ! command -v brew &> /dev/null; then
    echo "ERROR: Homebrew is not installed"
    echo "Install from: https://brew.sh"
    exit 1
fi

echo ""
echo "[1/6] Installing build tools..."
brew install cmake

echo ""
echo "[2/6] Installing Protobuf..."
brew install protobuf

echo ""
echo "[3/6] Installing gRPC..."
brew install grpc

echo ""
echo "[4/6] Installing Go..."
if ! command -v go &> /dev/null; then
    brew install go
else
    echo "Go already installed: $(go version)"
fi

echo ""
echo "[5/6] Installing NATS Server..."
brew install nats-server

echo ""
echo "[6/6] Installing NATS C Client (from source)..."
if ! pkg-config --exists libnats 2>/dev/null; then
    echo "NATS C Client not found, building from source..."
    $(dirname "$0")/install_nats_c.sh
else
    echo "NATS C Client already installed: $(pkg-config --modversion libnats)"
fi

echo ""
echo "================================================"
echo "  ✅ All dependencies installed successfully!"
echo "================================================"
echo ""
echo "Installed versions:"
echo "  CMake:    $(cmake --version | head -1)"
echo "  Protobuf: $(protoc --version)"
echo "  gRPC:     $(pkg-config --modversion grpc++)"
echo "  Go:       $(go version)"
echo "  NATS C:   $(pkg-config --modversion libnats 2>/dev/null || echo 'N/A')"
echo ""
echo "Next steps:"
echo "  1. Build C++ Gateway:  ./scripts/build_gateway.sh"
echo "  2. Build Go Client:    ./scripts/build_golang.sh"
echo "================================================"

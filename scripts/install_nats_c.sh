#!/bin/bash
set -e

echo "================================================"
echo "  Installing NATS C Client from source"
echo "================================================"

NATS_VERSION="3.8.0"
BUILD_DIR="/tmp/nats.c-build"
INSTALL_PREFIX="/usr/local"

echo "[1/5] Creating build directory..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

echo "[2/5] Downloading NATS.c ${NATS_VERSION}..."
curl -L "https://github.com/nats-io/nats.c/archive/refs/tags/v${NATS_VERSION}.tar.gz" -o nats.c.tar.gz
tar xzf nats.c.tar.gz
cd "nats.c-${NATS_VERSION}"

echo "[3/5] Building NATS.c..."
mkdir build
cd build
cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_INSTALL_PREFIX="$INSTALL_PREFIX" \
    -DNATS_BUILD_STREAMING=OFF

make -j$(sysctl -n hw.ncpu)

echo "[4/5] Installing NATS.c (requires sudo)..."
sudo make install

echo "[5/5] Cleaning up..."
cd /
rm -rf "$BUILD_DIR"

echo ""
echo "================================================"
echo "  NATS.c installed successfully!"
echo "================================================"
echo "Library: $INSTALL_PREFIX/lib/libnats.dylib"
echo "Headers: $INSTALL_PREFIX/include/nats.h"
echo ""
echo "Verify installation:"
echo "  pkg-config --modversion libnats"
echo "================================================"

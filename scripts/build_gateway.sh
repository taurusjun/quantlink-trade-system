#!/bin/bash
set -e

echo "================================================"
echo "  Building HFT MD Gateway (C++)"
echo "================================================"

# 检查依赖
check_dependency() {
    if ! command -v $1 &> /dev/null; then
        echo "ERROR: $1 is not installed"
        echo "Install with: $2"
        exit 1
    fi
}

echo "[1/5] Checking dependencies..."
check_dependency "cmake" "brew install cmake"
check_dependency "protoc" "brew install protobuf"
check_dependency "pkg-config" "brew install pkg-config"

# 检查gRPC
if ! pkg-config --exists grpc++; then
    echo "ERROR: gRPC is not installed"
    echo "Install with: brew install grpc"
    exit 1
fi

# 检查NATS.c
if ! pkg-config --exists libnats; then
    echo "WARNING: NATS.c is not installed"
    echo "Install with: brew install nats-c"
    echo "Continuing anyway..."
fi

# 创建build目录
cd "$(dirname "$0")/.."
GATEWAY_DIR="gateway"
BUILD_DIR="$GATEWAY_DIR/build"

echo "[2/5] Creating build directory..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"
cd "$BUILD_DIR"

echo "[3/5] Running CMake..."
cmake .. \
    -DCMAKE_BUILD_TYPE=Release \
    -DCMAKE_EXPORT_COMPILE_COMMANDS=ON

echo "[4/5] Building..."
make -j$(sysctl -n hw.ncpu)

echo "[5/5] Done!"
echo ""
echo "Built executables:"
echo "  - md_gateway      (Market Data Gateway)"
echo "  - ors_gateway     (Order Routing Service Gateway)"
echo "  - md_simulator    (Market data simulator)"
echo "  - md_benchmark    (Performance benchmark tool)"
echo ""
echo "Quick start (MD Gateway):"
echo "  Terminal 1: ./gateway/build/md_simulator 1000"
echo "  Terminal 2: ./gateway/build/md_gateway"
echo ""
echo "Quick start (ORS Gateway):"
echo "  Terminal 1: ./gateway/build/ors_gateway"
echo ""
echo "See QUICKSTART.md and USAGE.md for details"
echo "================================================"

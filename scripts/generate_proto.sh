#!/bin/bash
# Protobuf 代码生成脚本
# 用途：从 .proto 文件生成 Go 代码

set -e

PROJECT_ROOT="/Users/user/PWorks/RD/quantlink-trade-system"
PROTO_DIR="$PROJECT_ROOT/gateway/proto"
GO_OUT_DIR="$PROJECT_ROOT/golang"

echo "╔═══════════════════════════════════════════════════════════╗"
echo "║         Protobuf Code Generation Script                  ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

# 检查工具是否安装
check_tool() {
    if ! command -v $1 &> /dev/null; then
        echo "❌ Error: $1 not found"
        echo "   Please install: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
        exit 1
    fi
}

echo "[1/4] Checking required tools..."
check_tool protoc
check_tool protoc-gen-go
check_tool protoc-gen-go-grpc
echo "✅ All tools found"
echo ""

# 创建输出目录
echo "[2/4] Creating output directories..."
mkdir -p "$GO_OUT_DIR/pkg/proto/ors"
mkdir -p "$GO_OUT_DIR/pkg/proto/common"
mkdir -p "$GO_OUT_DIR/pkg/proto/md"
echo "✅ Directories created"
echo ""

# 生成 protobuf 代码
echo "[3/4] Generating Go code from proto files..."

cd "$PROTO_DIR"

# 生成 common.proto
if [ -f "common.proto" ]; then
    echo "  - Generating common.proto..."
    protoc --go_out="$GO_OUT_DIR" \
           --go_opt=module=github.com/yourusername/quantlink-trade-system \
           --go-grpc_out="$GO_OUT_DIR" \
           --go-grpc_opt=module=github.com/yourusername/quantlink-trade-system \
           --proto_path=. \
           common.proto
    echo "    ✅ common.pb.go generated"
fi

# 生成 order.proto
if [ -f "order.proto" ]; then
    echo "  - Generating order.proto..."
    protoc --go_out="$GO_OUT_DIR" \
           --go_opt=module=github.com/yourusername/quantlink-trade-system \
           --go-grpc_out="$GO_OUT_DIR" \
           --go-grpc_opt=module=github.com/yourusername/quantlink-trade-system \
           --proto_path=. \
           order.proto
    echo "    ✅ order.pb.go generated"
fi

# 生成 market_data.proto
if [ -f "market_data.proto" ]; then
    echo "  - Generating market_data.proto..."
    protoc --go_out="$GO_OUT_DIR" \
           --go_opt=module=github.com/yourusername/quantlink-trade-system \
           --go-grpc_out="$GO_OUT_DIR" \
           --go-grpc_opt=module=github.com/yourusername/quantlink-trade-system \
           --proto_path=. \
           market_data.proto
    echo "    ✅ market_data.pb.go generated"
fi

echo ""

# 验证生成的文件
echo "[4/4] Verifying generated files..."
if [ -d "$GO_OUT_DIR/pkg/proto/ors" ]; then
    FILE_COUNT=$(find "$GO_OUT_DIR/pkg/proto" -name "*.pb.go" | wc -l | tr -d ' ')
    echo "✅ Generated $FILE_COUNT protobuf files"
    echo ""
    echo "Generated files:"
    find "$GO_OUT_DIR/pkg/proto" -name "*.pb.go" -exec echo "  - {}" \;
else
    echo "⚠️  Warning: Some files may not have been generated"
fi

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║         Protobuf Generation Complete                     ║"
echo "╚═══════════════════════════════════════════════════════════╝"

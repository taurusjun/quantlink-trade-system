#!/bin/bash
set -e

echo "================================================"
echo "  Building HFT MD Client (Golang)"
echo "================================================"

cd "$(dirname "$0")/.."
GOLANG_DIR="golang"

echo "[1/4] Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed"
    echo "Install with: brew install go"
    exit 1
fi

echo "Go version: $(go version)"

cd "$GOLANG_DIR"

echo "[2/4] Installing protoc-gen-go plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

echo "[3/4] Generating protobuf Go code..."
mkdir -p pkg/proto/md
mkdir -p pkg/proto/common

protoc -I ../gateway/proto \
    --go_out=./pkg/proto/md \
    --go_opt=paths=source_relative \
    --go-grpc_out=./pkg/proto/md \
    --go-grpc_opt=paths=source_relative \
    ../gateway/proto/market_data.proto

protoc -I ../gateway/proto \
    --go_out=./pkg/proto/common \
    --go_opt=paths=source_relative \
    --go-grpc_out=./pkg/proto/common \
    --go-grpc_opt=paths=source_relative \
    ../gateway/proto/common.proto

echo "[4/4] Building client..."
go mod tidy
go build -o bin/md_client ./cmd/md_client

echo ""
echo "Executable: $GOLANG_DIR/bin/md_client"
echo ""
echo "Run with: ./golang/bin/md_client -gateway localhost:50051 -symbols ag2412"
echo "================================================"

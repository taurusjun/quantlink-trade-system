#!/bin/bash
set -e

echo "================================================"
echo "  HFT POC Integration Test"
echo "================================================"

cd "$(dirname "$0")/.."

# 检查NATS是否运行
if ! pgrep -x "nats-server" > /dev/null; then
    echo "[Test] Starting NATS server..."
    nats-server &
    NATS_PID=$!
    sleep 2
else
    echo "[Test] NATS server already running"
    NATS_PID=""
fi

# 启动模拟器（共享内存生产者）
echo "[Test] Starting MD Simulator..."
./gateway/build/md_simulator 1000 &
SIMULATOR_PID=$!
sleep 2

# 启动Gateway（共享内存消费者）
echo "[Test] Starting MD Gateway (SHM mode)..."
./gateway/build/md_gateway_shm &
GATEWAY_PID=$!
sleep 3

# 运行Golang客户端测试
echo "[Test] Running gRPC client test..."
timeout 10s ./golang/bin/md_client \
    -gateway localhost:50051 \
    -symbols ag2412 \
    || echo "[Test] gRPC client test completed"

echo "[Test] Running NATS client test..."
timeout 10s ./golang/bin/md_client \
    -nats \
    -nats-url nats://localhost:4222 \
    -symbols ag2412 \
    || echo "[Test] NATS client test completed"

# 清理
echo "[Test] Cleaning up..."
kill $GATEWAY_PID 2>/dev/null || true
kill $SIMULATOR_PID 2>/dev/null || true
if [ -n "$NATS_PID" ]; then
    kill $NATS_PID 2>/dev/null || true
fi
# 清理共享内存
rm -f /tmp/hft_md_* 2>/dev/null || true

echo "================================================"
echo "  Test completed successfully!"
echo "================================================"

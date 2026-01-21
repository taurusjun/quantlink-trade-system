#!/bin/bash
set -e

echo "================================================"
echo "  MD Gateway + NATS Integration Test"
echo "================================================"

PROJECT_ROOT="/Users/user/PWorks/RD/hft-poc"
BUILD_DIR="$PROJECT_ROOT/gateway/build"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 清理函数
cleanup() {
    echo ""
    echo -e "${YELLOW}[Cleanup] Stopping all processes...${NC}"

    if [ ! -z "$NATS_PID" ]; then
        kill $NATS_PID 2>/dev/null || true
        echo -e "${GREEN}[Cleanup] NATS server stopped${NC}"
    fi

    if [ ! -z "$SIMULATOR_PID" ]; then
        kill $SIMULATOR_PID 2>/dev/null || true
        echo -e "${GREEN}[Cleanup] Simulator stopped${NC}"
    fi

    if [ ! -z "$GATEWAY_PID" ]; then
        kill $GATEWAY_PID 2>/dev/null || true
        echo -e "${GREEN}[Cleanup] Gateway stopped${NC}"
    fi

    if [ ! -z "$SUBSCRIBER_PID" ]; then
        kill $SUBSCRIBER_PID 2>/dev/null || true
        echo -e "${GREEN}[Cleanup] NATS subscriber stopped${NC}"
    fi

    # 清理共享内存
    rm -f /tmp/hft_md_* 2>/dev/null || true
    rm -f /dev/shm/hft_md_* 2>/dev/null || true

    echo -e "${GREEN}[Cleanup] Done${NC}"
}

trap cleanup EXIT INT TERM

# 检查可执行文件
echo "[1/7] Checking prerequisites..."
if [ ! -f "$BUILD_DIR/md_simulator" ]; then
    echo -e "${RED}Error: md_simulator not found. Run ./scripts/build_gateway.sh first${NC}"
    exit 1
fi

if [ ! -f "$BUILD_DIR/md_gateway_shm" ]; then
    echo -e "${RED}Error: md_gateway_shm not found. Run ./scripts/build_gateway.sh first${NC}"
    exit 1
fi

if ! command -v nats-server &> /dev/null; then
    echo -e "${RED}Error: nats-server not found. Install with: brew install nats-server${NC}"
    exit 1
fi

echo -e "${GREEN}[1/7] Prerequisites OK${NC}"

# 启动NATS服务器
echo "[2/7] Starting NATS server..."
nats-server --port 4222 > /tmp/nats.log 2>&1 &
NATS_PID=$!
sleep 2

if ! ps -p $NATS_PID > /dev/null; then
    echo -e "${RED}Error: Failed to start NATS server${NC}"
    cat /tmp/nats.log
    exit 1
fi
echo -e "${GREEN}[2/7] NATS server started (PID: $NATS_PID)${NC}"

# 启动NATS订阅者（后台监听）
echo "[3/7] Starting NATS subscriber..."
nats sub "md.>" --count 100 > /tmp/nats_messages.log 2>&1 &
SUBSCRIBER_PID=$!
sleep 1
echo -e "${GREEN}[3/7] NATS subscriber started (PID: $SUBSCRIBER_PID)${NC}"

# 启动模拟器
echo "[4/7] Starting market data simulator..."
cd "$BUILD_DIR"
./md_simulator 1000 queue > /tmp/simulator.log 2>&1 &
SIMULATOR_PID=$!
sleep 2

if ! ps -p $SIMULATOR_PID > /dev/null; then
    echo -e "${RED}Error: Failed to start simulator${NC}"
    cat /tmp/simulator.log
    exit 1
fi
echo -e "${GREEN}[4/7] Simulator started (PID: $SIMULATOR_PID, 1000 Hz)${NC}"

# 启动Gateway
echo "[5/7] Starting MD Gateway with NATS..."
./md_gateway_shm queue > /tmp/gateway.log 2>&1 &
GATEWAY_PID=$!
sleep 3

if ! ps -p $GATEWAY_PID > /dev/null; then
    echo -e "${RED}Error: Failed to start gateway${NC}"
    cat /tmp/gateway.log
    exit 1
fi
echo -e "${GREEN}[5/7] Gateway started (PID: $GATEWAY_PID)${NC}"

# 等待一些数据积累
echo "[6/7] Running test for 10 seconds..."
for i in {10..1}; do
    echo -ne "\r  Remaining: ${i}s "
    sleep 1
done
echo ""

# 检查结果
echo "[7/7] Checking results..."
echo ""

# 检查模拟器输出
echo -e "${YELLOW}Simulator Status:${NC}"
tail -3 /tmp/simulator.log | grep -E "(Pushed|Dropped)" || echo "  No data found"

# 检查Gateway输出
echo ""
echo -e "${YELLOW}Gateway Status:${NC}"
tail -5 /tmp/gateway.log | grep -E "(Processed|Read)" || echo "  No data found"

# 检查NATS消息
echo ""
echo -e "${YELLOW}NATS Messages Received:${NC}"
MSG_COUNT=$(grep -c "md\." /tmp/nats_messages.log 2>/dev/null || echo "0")
if [ "$MSG_COUNT" -gt 0 ]; then
    echo -e "  ${GREEN}✓ Received $MSG_COUNT messages${NC}"
    echo ""
    echo "  Sample messages:"
    grep "md\." /tmp/nats_messages.log | head -5 | sed 's/^/    /'
else
    echo -e "  ${RED}✗ No messages received${NC}"
    echo ""
    echo "  Possible issues:"
    echo "    1. NATS not enabled in Gateway (check ENABLE_NATS compile flag)"
    echo "    2. NATS connection failed (check gateway logs)"
    echo "    3. Gateway not receiving data from shared memory"
fi

echo ""
echo "================================================"
echo "  Test Complete"
echo "================================================"
echo "Full logs available:"
echo "  - NATS server:   /tmp/nats.log"
echo "  - Simulator:     /tmp/simulator.log"
echo "  - Gateway:       /tmp/gateway.log"
echo "  - NATS messages: /tmp/nats_messages.log"
echo "================================================"

# 询问是否保持运行
echo ""
read -p "Keep services running? (y/N): " -t 10 -n 1 -r keep_running || keep_running="n"
echo ""

if [[ $keep_running =~ ^[Yy]$ ]]; then
    echo "Services are still running. Press Ctrl+C to stop."
    wait
else
    echo "Stopping all services..."
fi

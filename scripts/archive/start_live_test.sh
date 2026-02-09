#!/bin/bash
# 实盘端到端测试启动脚本

set -e

echo "╔═══════════════════════════════════════════════════════╗"
echo "║          实盘端到端测试 - CTP环境                        ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 清理旧进程
echo "[1/9] 清理旧进程..."
pkill -9 ctp_md_gateway 2>/dev/null || true
pkill -9 md_gateway 2>/dev/null || true
pkill -9 ors_gateway 2>/dev/null || true
pkill -9 counter_bridge 2>/dev/null || true
pkill -9 trader 2>/dev/null || true
sleep 2

# 清理共享内存
echo "[2/9] 清理共享内存..."
for shm in /dev/shm/hft_*; do
    [ -e "$shm" ] && rm -f "$shm" 2>/dev/null || true
done

# 创建必要目录
echo "[3/9] 创建目录..."
mkdir -p log test_logs ctp_flow

# 检查NATS
echo "[4/9] 检查NATS..."
if ! pgrep -x "nats-server" > /dev/null; then
    echo "启动NATS..."
    nats-server > test_logs/nats.log 2>&1 &
    sleep 2
fi

# 启动CTP行情网关
echo "[5/9] 启动CTP行情网关..."
if [ -f "config/ctp_md.secret.yaml" ]; then
    ./gateway/build/ctp_md_gateway -c config/ctp_md.yaml > test_logs/ctp_md.log 2>&1 &
    CTP_MD_PID=$!
    echo "CTP行情网关 PID: $CTP_MD_PID"
    sleep 5
else
    echo "⚠️  警告: config/ctp_md.secret.yaml 不存在，跳过CTP行情网关"
fi

# 启动md_gateway
echo "[6/9] 启动md_gateway..."
./gateway/build/md_gateway md_queue > test_logs/md_gateway.log 2>&1 &
MD_PID=$!
echo "md_gateway PID: $MD_PID"
sleep 2

# 启动ors_gateway
echo "[7/9] 启动ors_gateway..."
./gateway/build/ors_gateway > test_logs/ors_gateway.log 2>&1 &
ORS_PID=$!
echo "ors_gateway PID: $ORS_PID"
sleep 2

# 启动counter_bridge
echo "[8/9] 启动counter_bridge..."
if [ -f "config/ctp/ctp_td.secret.yaml" ]; then
    ./gateway/build/counter_bridge ctp:config/ctp/ctp_td.yaml > test_logs/counter_bridge.log 2>&1 &
    COUNTER_PID=$!
    echo "counter_bridge PID: $COUNTER_PID"
    sleep 5
else
    echo "⚠️  警告: config/ctp/ctp_td.secret.yaml 不存在，使用模拟Counter"
    ./gateway/build/counter_gateway > test_logs/counter_gateway.log 2>&1 &
    COUNTER_PID=$!
    echo "counter_gateway (模拟) PID: $COUNTER_PID"
    sleep 2
fi

# 启动trader
echo "[9/9] 启动trader..."
./bin/trader -config config/trader.test.yaml > test_logs/trader.log 2>&1 &
TRADER_PID=$!
echo "trader PID: $TRADER_PID"
sleep 5

# 保存PIDs
echo "$CTP_MD_PID $MD_PID $ORS_PID $COUNTER_PID $TRADER_PID" > test_logs/pids.txt

echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║              所有组件启动完成                            ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""
echo "进程列表:"
echo "  CTP行情: $CTP_MD_PID"
echo "  MD Gateway: $MD_PID"
echo "  ORS Gateway: $ORS_PID"
echo "  Counter: $COUNTER_PID"
echo "  Trader: $TRADER_PID"
echo ""
echo "监控命令:"
echo "  tail -f test_logs/trader.log | grep 'Order sent'"
echo "  tail -f test_logs/counter_bridge.log"
echo "  tail -f test_logs/ctp_md.log"
echo ""
echo "Dashboard: http://localhost:9201"
echo ""

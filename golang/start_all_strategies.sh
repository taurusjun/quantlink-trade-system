#!/bin/bash
# Start All Strategy Instances
# 对应 tbsrc 的批量启动脚本

echo "Starting all strategy instances..."
echo ""

# 创建日志目录
mkdir -p log

# 策略 1: ag2502-ag2504 配对套利
echo "[1/3] Starting ag2502-ag2504 pair trading (Strategy ID: 92201)..."
nohup ./QuantlinkTrader \
    --config ./config/trader.ag2502.ag2504.yaml \
    --strategy-id 92201 \
    --mode simulation \
    >> nohup.out.92201 2>&1 &
echo $! > trader.92201.pid
echo "✓ Started (PID: $(cat trader.92201.pid))"
echo ""

# 策略 2: al2502-al2503 配对套利
echo "[2/3] Starting al2502-al2503 pair trading (Strategy ID: 93201)..."
nohup ./QuantlinkTrader \
    --config ./config/trader.al2502.al2503.yaml \
    --strategy-id 93201 \
    --mode simulation \
    >> nohup.out.93201 2>&1 &
echo $! > trader.93201.pid
echo "✓ Started (PID: $(cat trader.93201.pid))"
echo ""

# 策略 3: rb2505-rb2510 配对套利
echo "[3/3] Starting rb2505-rb2510 pair trading (Strategy ID: 41231)..."
nohup ./QuantlinkTrader \
    --config ./config/trader.rb2505.rb2510.yaml \
    --strategy-id 41231 \
    --mode simulation \
    >> nohup.out.41231 2>&1 &
echo $! > trader.41231.pid
echo "✓ Started (PID: $(cat trader.41231.pid))"
echo ""

echo "════════════════════════════════════════════════════════════"
echo "All strategies started successfully!"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "Running strategy instances:"
ps aux | grep QuantlinkTrader | grep -v grep
echo ""
echo "To stop all strategies, run: ./stop_all_strategies.sh"

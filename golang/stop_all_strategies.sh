#!/bin/bash
# Stop All Strategy Instances

echo "Stopping all strategy instances..."
echo ""

# 停止策略 1
if [ -f trader.92201.pid ]; then
    PID=$(cat trader.92201.pid)
    echo "[1/3] Stopping ag2502-ag2504 (PID: $PID)..."
    kill -INT $PID 2>/dev/null || echo "  Already stopped"
    rm -f trader.92201.pid
    echo "✓ Stopped"
else
    echo "[1/3] ag2502-ag2504 not running (no PID file)"
fi
echo ""

# 停止策略 2
if [ -f trader.93201.pid ]; then
    PID=$(cat trader.93201.pid)
    echo "[2/3] Stopping al2502-al2503 (PID: $PID)..."
    kill -INT $PID 2>/dev/null || echo "  Already stopped"
    rm -f trader.93201.pid
    echo "✓ Stopped"
else
    echo "[2/3] al2502-al2503 not running (no PID file)"
fi
echo ""

# 停止策略 3
if [ -f trader.41231.pid ]; then
    PID=$(cat trader.41231.pid)
    echo "[3/3] Stopping rb2505-rb2510 (PID: $PID)..."
    kill -INT $PID 2>/dev/null || echo "  Already stopped"
    rm -f trader.41231.pid
    echo "✓ Stopped"
else
    echo "[3/3] rb2505-rb2510 not running (no PID file)"
fi
echo ""

# 等待进程退出
sleep 2

echo "════════════════════════════════════════════════════════════"
echo "All strategies stopped"
echo "════════════════════════════════════════════════════════════"
echo ""

# 检查是否还有运行的进程
RUNNING=$(ps aux | grep QuantlinkTrader | grep -v grep | wc -l)
if [ $RUNNING -gt 0 ]; then
    echo "Warning: Some processes may still be running:"
    ps aux | grep QuantlinkTrader | grep -v grep
fi

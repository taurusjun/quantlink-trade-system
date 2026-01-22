#!/bin/bash
# Activate All Trading Strategies
# 对应 tbsrc: pkill -SIGUSR1 TradeBot

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "════════════════════════════════════════════════════════════"
echo "Activating All Trading Strategies"
echo "════════════════════════════════════════════════════════════"
echo ""

# Find all QuantlinkTrader processes
PIDS=$(pgrep -f "QuantlinkTrader")

if [ -z "$PIDS" ]; then
    echo "No QuantlinkTrader processes found"
    exit 1
fi

echo "Found QuantlinkTrader processes:"
ps aux | grep QuantlinkTrader | grep -v grep | awk '{print "  PID " $2 " - " $11 " " $12 " " $13 " " $14}'
echo ""

# Send SIGUSR1 to all processes
echo "Sending SIGUSR1 (activate) signal to all processes..."
pkill -SIGUSR1 -f "QuantlinkTrader"

if [ $? -eq 0 ]; then
    echo "✓ Signals sent successfully"
    echo "✓ All strategies should now be activated"
else
    echo "✗ Failed to send signals"
    exit 1
fi

echo "════════════════════════════════════════════════════════════"

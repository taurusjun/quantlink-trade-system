#!/bin/bash
# Deactivate All Trading Strategies (Squareoff)
# 对应 tbsrc: pkill -SIGTSTP TradeBot

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "════════════════════════════════════════════════════════════"
echo "Deactivating All Trading Strategies (Squareoff)"
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

echo "This will:"
echo "  - Stop all strategies from generating signals"
echo "  - Cancel all pending orders"
echo "  - Close all open positions (flatten)"
echo "  - Keep all processes running"
echo ""

# Send SIGUSR2 to all processes
echo "Sending SIGUSR2 (deactivate/squareoff) signal to all processes..."
pkill -SIGUSR2 -f "QuantlinkTrader"

if [ $? -eq 0 ]; then
    echo "✓ Signals sent successfully"
    echo "✓ All strategies should now be deactivating"
    echo ""
    echo "To re-activate all:"
    echo "  ./startAllTrades.sh"
else
    echo "✗ Failed to send signals"
    exit 1
fi

echo "════════════════════════════════════════════════════════════"

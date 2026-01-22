#!/bin/bash
# Deactivate Trading Strategy (Squareoff)
# 对应 tbsrc stopTrade.pl
# 发送 SIGUSR2 信号停止策略并平仓

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Parse arguments
STRATEGY_ID=$1

if [ -z "$STRATEGY_ID" ]; then
    echo "Usage: $0 <strategy_id>"
    echo ""
    echo "Example:"
    echo "  $0 92201      # Deactivate strategy 92201 (squareoff)"
    echo ""
    exit 1
fi

# PID file
PID_FILE="trader.$STRATEGY_ID.pid"

# Check if PID file exists
if [ ! -f "$PID_FILE" ]; then
    echo "Error: Strategy $STRATEGY_ID not found (no PID file: $PID_FILE)"
    echo ""
    echo "Available strategies:"
    ls trader.*.pid 2>/dev/null | sed 's/trader\.\(.*\)\.pid/  \1/'
    exit 1
fi

# Read PID
PID=$(cat "$PID_FILE")

if [ -z "$PID" ]; then
    echo "Error: Invalid PID file: $PID_FILE"
    exit 1
fi

# Check if process is running
if ! ps -p $PID > /dev/null 2>&1; then
    echo "Error: Strategy $STRATEGY_ID process not running (PID: $PID)"
    rm -f "$PID_FILE"
    exit 1
fi

echo "════════════════════════════════════════════════════════════"
echo "Deactivating Trading Strategy (Squareoff)"
echo "════════════════════════════════════════════════════════════"
echo "Strategy ID: $STRATEGY_ID"
echo "Process PID: $PID"
echo ""
echo "This will:"
echo "  - Stop generating new trading signals"
echo "  - Cancel all pending orders"
echo "  - Close all open positions (flatten)"
echo "  - Keep the process running (can be re-activated)"
echo ""

# Send SIGUSR2 signal
kill -SIGUSR2 $PID

if [ $? -eq 0 ]; then
    echo "✓ SIGUSR2 signal sent successfully"
    echo "✓ Strategy $STRATEGY_ID should now be deactivating"
    echo ""
    echo "Check log file for confirmation:"
    echo "  tail -f log/trader.*.$STRATEGY_ID.log"
    echo ""
    echo "To re-activate:"
    echo "  ./startTrade.sh $STRATEGY_ID"
else
    echo "✗ Failed to send signal to process $PID"
    exit 1
fi

echo "════════════════════════════════════════════════════════════"

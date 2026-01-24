#!/bin/bash
# Auto-generated start script from backtest optimization
# Generated: 2026-01-24 20:20:10
# Optimization goal: sharpe
# Sharpe Ratio: 0.00

# Check if NATS is running
if ! pgrep -x "nats-server" > /dev/null; then
    echo "Starting NATS server..."
    nats-server &
    sleep 2
fi

# Start trader
echo "Starting trader for ag2502/ag2504 (Strategy 92201)..."
nohup ./bin/trader -config config/trader_92201_optimized_20260124.yaml > log/nohup_92201.out 2>&1 &

# Display process info
sleep 2
echo "Trader started. PID: $(pgrep -f trader | tail -1)"
echo "Log file: log/trader_92201.log"
echo "API endpoint: http://localhost:9201"
echo ""
echo "To activate strategy, run:"
echo "  curl -X POST http://localhost:9201/api/v1/strategy/activate"

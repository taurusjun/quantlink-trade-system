#!/bin/bash

# Start NATS
nats-server > test_logs/nats.log 2>&1 &
sleep 2

# Start MD Simulator
./gateway/build/md_simulator > test_logs/md_simulator.log 2>&1 &
sleep 2

# Start MD Gateway
./gateway/build/md_gateway > test_logs/md_gateway.log 2>&1 &
sleep 2

# Start ORS Gateway
./gateway/build/ors_gateway > test_logs/ors_gateway.log 2>&1 &
sleep 2

# Start Counter Gateway
./gateway/build/counter_gateway > test_logs/counter_gateway.log 2>&1 &
sleep 2

# Start Trader
./bin/trader -config config/trader.hot_reload.test.yaml > test_logs/trader.log 2>&1 &
sleep 5

echo "All services started"
echo "Dashboard: http://localhost:9301/dashboard"
echo ""
echo "To activate ag_pairwise strategy:"
echo "curl -X POST http://localhost:9301/api/v1/strategies/ag_pairwise/activate -H 'Content-Type: application/json'"

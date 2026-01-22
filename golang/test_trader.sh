#!/bin/bash
# Test script for QuantlinkTrader

echo "Testing QuantlinkTrader..."
echo ""

# Test 1: Version
echo "1. Testing --version"
./QuantlinkTrader --version
echo ""

# Test 2: Help
echo "2. Testing --help"
./QuantlinkTrader --help
echo ""

# Test 3: Run with config (will run for 5 seconds then kill)
echo "3. Testing run with config (5 second test)"
./QuantlinkTrader --config ./config/trader.yaml &
PID=$!
sleep 5
kill $PID 2>/dev/null || true
echo ""
echo "Test completed!"

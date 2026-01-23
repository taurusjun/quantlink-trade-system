#!/bin/bash
# Test script for injecting mock market data to QuantlinkTrader
# This simulates market data for ag2502 and ag2504 pairs trading

API_URL="http://localhost:9201"

echo "========================================="
echo "Mock Market Data Testing Script"
echo "========================================="
echo ""

# Test 1: Check health
echo "[1/6] Checking trader health..."
curl -s "${API_URL}/api/v1/health" | jq '.'
echo ""

# Test 2: Get initial status
echo "[2/6] Getting initial strategy status..."
curl -s "${API_URL}/api/v1/strategy/status" | jq '.'
echo ""

# Test 3: Send market data for ag2502 (bid=5000, ask=5002)
echo "[3/6] Sending market data for ag2502..."
curl -s -X POST "${API_URL}/api/v1/test-market-data" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "ag2502",
    "exchange": "SHFE",
    "bid_price": [5000, 4999, 4998],
    "ask_price": [5002, 5003, 5004],
    "bid_qty": [100, 80, 60],
    "ask_qty": [100, 80, 60]
  }' | jq '.'
echo ""

# Test 4: Send market data for ag2504 (bid=5010, ask=5012) - 价差扩大
echo "[4/6] Sending market data for ag2504 (spread widening)..."
curl -s -X POST "${API_URL}/api/v1/test-market-data" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "ag2504",
    "exchange": "SHFE",
    "bid_price": [5010, 5009, 5008],
    "ask_price": [5012, 5013, 5014],
    "bid_qty": [100, 80, 60],
    "ask_qty": [100, 80, 60]
  }' | jq '.'
echo ""

# Wait a moment for indicators to update
sleep 2

# Test 5: Check updated status (should show indicators)
echo "[5/6] Checking updated strategy status (should show indicators)..."
curl -s "${API_URL}/api/v1/strategy/status" | jq '.'
echo ""

# Test 6: Send more data with bigger spread to trigger conditions
echo "[6/6] Sending market data with bigger spread (should trigger conditions)..."
curl -s -X POST "${API_URL}/api/v1/test-market-data" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "ag2502",
    "exchange": "SHFE",
    "bid_price": [4980, 4979, 4978],
    "ask_price": [4982, 4983, 4984],
    "bid_qty": [100, 80, 60],
    "ask_qty": [100, 80, 60]
  }' | jq '.'

curl -s -X POST "${API_URL}/api/v1/test-market-data" \
  -H "Content-Type: application/json" \
  -d '{
    "symbol": "ag2504",
    "exchange": "SHFE",
    "bid_price": [5030, 5029, 5028],
    "ask_price": [5032, 5033, 5034],
    "bid_qty": [100, 80, 60],
    "ask_qty": [100, 80, 60]
  }' | jq '.'
echo ""

sleep 2

# Final status check
echo ""
echo "========================================="
echo "Final Status Check"
echo "========================================="
curl -s "${API_URL}/api/v1/strategy/status" | jq '{
  strategy_id: .data.strategy_id,
  running: .data.running,
  active: .data.active,
  conditions_met: .data.conditions_met,
  eligible: .data.eligible,
  eligible_reason: .data.eligible_reason,
  signal_strength: .data.signal_strength,
  indicators: .data.indicators
}'
echo ""

echo "========================================="
echo "Test Complete!"
echo "========================================="
echo ""
echo "Now open the Web UI in your browser to see the results:"
echo "file:///Users/user/PWorks/RD/quantlink-trade-system/golang/trader_ui.html"
echo ""

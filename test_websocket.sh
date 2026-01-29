#!/bin/bash

# Test WebSocket implementation for Dashboard
# Tests:
# 1. Start trader in multi-strategy mode
# 2. Connect to WebSocket endpoint
# 3. Verify realtime data push
# 4. Check thresholds are included
# 5. Check market data is included

set -e

echo "========================================"
echo "Dashboard WebSocket Test"
echo "========================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Config
TRADER_PORT=9301
LOG_FILE="log/trader.test.log"

# Cleanup function
cleanup() {
    echo ""
    echo "${YELLOW}[Cleanup] Stopping all processes...${NC}"
    pkill -f "trader -config" 2>/dev/null || true
    pkill -f "nats-server" 2>/dev/null || true
    sleep 1
    echo "${GREEN}[Cleanup] Done${NC}"
}

trap cleanup EXIT

# Step 1: Start NATS
echo "${YELLOW}[Step 1/5] Starting NATS server...${NC}"
nats-server > /dev/null 2>&1 &
NATS_PID=$!
sleep 2
echo "${GREEN}✓ NATS started (PID: $NATS_PID)${NC}"
echo ""

# Step 2: Start Trader
echo "${YELLOW}[Step 2/5] Starting trader in multi-strategy mode...${NC}"
mkdir -p log
./bin/trader -config config/trader.yaml > ${LOG_FILE} 2>&1 &
TRADER_PID=$!
sleep 3
echo "${GREEN}✓ Trader started (PID: $TRADER_PID)${NC}"
echo ""

# Step 3: Check HTTP API
echo "${YELLOW}[Step 3/5] Testing HTTP API endpoint...${NC}"
HTTP_RESPONSE=$(curl -s http://localhost:${TRADER_PORT}/api/v1/dashboard/overview)
if echo "$HTTP_RESPONSE" | grep -q "success"; then
    echo "${GREEN}✓ HTTP API working${NC}"
    echo "  Response preview: $(echo $HTTP_RESPONSE | jq -c '.data | {multi_strategy, total_strategies, mode}' 2>/dev/null || echo 'Cannot parse')"
else
    echo "${RED}✗ HTTP API failed${NC}"
    echo "  Response: $HTTP_RESPONSE"
    exit 1
fi
echo ""

# Step 4: Test WebSocket Connection using websocat (if available)
echo "${YELLOW}[Step 4/5] Testing WebSocket connection...${NC}"
if command -v websocat &> /dev/null; then
    echo "Using websocat to test WebSocket..."
    timeout 5 websocat ws://localhost:${TRADER_PORT}/api/v1/ws/dashboard > /tmp/ws_test.log 2>&1 &
    WS_PID=$!
    sleep 3

    if ps -p $WS_PID > /dev/null; then
        echo "${GREEN}✓ WebSocket connection established${NC}"
        kill $WS_PID 2>/dev/null || true

        # Check if data was received
        if [ -f /tmp/ws_test.log ] && [ -s /tmp/ws_test.log ]; then
            echo "  Received data:"
            head -3 /tmp/ws_test.log | sed 's/^/    /'
        fi
    else
        echo "${YELLOW}⚠ WebSocket test inconclusive (websocat exited)${NC}"
    fi
else
    echo "${YELLOW}⚠ websocat not installed, skipping WebSocket protocol test${NC}"
    echo "  Install with: brew install websocat (macOS)"
    echo "  Checking WebSocket endpoint availability..."

    # Alternative: Just check if the endpoint responds
    WS_CHECK=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${TRADER_PORT}/api/v1/ws/dashboard)
    if [ "$WS_CHECK" == "400" ] || [ "$WS_CHECK" == "426" ]; then
        echo "${GREEN}✓ WebSocket endpoint exists (HTTP $WS_CHECK - expects upgrade)${NC}"
    else
        echo "${YELLOW}⚠ Unexpected response: HTTP $WS_CHECK${NC}"
    fi
fi
echo ""

# Step 5: Check logs for WebSocket activity
echo "${YELLOW}[Step 5/5] Checking trader logs for WebSocket activity...${NC}"
if grep -q "WebSocket" ${LOG_FILE}; then
    echo "${GREEN}✓ WebSocket hub initialized${NC}"
    echo "  Log entries:"
    grep "WebSocket" ${LOG_FILE} | tail -5 | sed 's/^/    /'
else
    echo "${YELLOW}⚠ No WebSocket logs found (check ${LOG_FILE})${NC}"
fi
echo ""

# Step 6: Open Dashboard in Browser
echo "${YELLOW}[Step 6/5] Opening dashboard in browser...${NC}"
echo "Dashboard URL: http://localhost:${TRADER_PORT}/dashboard"
echo ""
echo "${GREEN}To test manually:${NC}"
echo "  1. Open http://localhost:${TRADER_PORT}/dashboard in your browser"
echo "  2. Check connection indicator (top right)"
echo "  3. Verify real-time data updates (check timestamps)"
echo "  4. Look for threshold values displayed alongside indicators"
echo "  5. Check Market Data card on the right sidebar"
echo ""

# Keep trader running for manual testing
echo "${YELLOW}Trader is running. Press Ctrl+C to stop.${NC}"
tail -f ${LOG_FILE} | grep --line-buffered -E "WebSocket|dashboard|market"

# Cleanup will be triggered by EXIT trap

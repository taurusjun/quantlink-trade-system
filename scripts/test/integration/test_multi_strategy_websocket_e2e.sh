#!/bin/bash

# 多策略WebSocket端到端测试
# 测试内容:
# 1. 启动完整网关链路
# 2. 启动多策略trader
# 3. 验证WebSocket连接和数据推送
# 4. 验证Dashboard显示指标+阈值
# 5. 验证实时行情推送

set -e

echo "========================================"
echo "多策略WebSocket端到端测试"
echo "========================================"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Config
CONFIG_FILE="config/trader.hot_reload.test.yaml"
LOG_DIR="test_logs"
TRADER_LOG="${LOG_DIR}/trader_ws_e2e.log"
MD_SIMULATOR_LOG="${LOG_DIR}/md_simulator_ws.log"
MD_GATEWAY_LOG="${LOG_DIR}/md_gateway_ws.log"
API_PORT=9301

# 创建日志目录
mkdir -p ${LOG_DIR}

# Cleanup function
cleanup() {
    echo ""
    echo "${YELLOW}[Cleanup] Stopping all processes...${NC}"
    pkill -f "trader -config" 2>/dev/null || true
    pkill -f "md_simulator" 2>/dev/null || true
    pkill -f "md_gateway" 2>/dev/null || true
    pkill -f "nats-server" 2>/dev/null || true
    sleep 2
    echo "${GREEN}[Cleanup] Done${NC}"
}

trap cleanup EXIT

# 清理旧进程
echo "${YELLOW}[Step 0/10] Cleaning up old processes...${NC}"
cleanup
sleep 1
echo ""

# Step 1: 启动 NATS
echo "${YELLOW}[Step 1/10] Starting NATS server...${NC}"
nats-server > /dev/null 2>&1 &
NATS_PID=$!
sleep 2

if ps -p $NATS_PID > /dev/null; then
    echo "${GREEN}✓ NATS started (PID: $NATS_PID)${NC}"
else
    echo "${RED}✗ NATS failed to start${NC}"
    exit 1
fi
echo ""

# Step 2: 启动 MD Simulator
echo "${YELLOW}[Step 2/10] Starting MD Simulator...${NC}"
./gateway/build/md_simulator > ${MD_SIMULATOR_LOG} 2>&1 &
MD_SIM_PID=$!
sleep 2

if ps -p $MD_SIM_PID > /dev/null; then
    echo "${GREEN}✓ MD Simulator started (PID: $MD_SIM_PID)${NC}"
    echo "  Simulating: ag2603, ag2605, au2604, au2606"
else
    echo "${RED}✗ MD Simulator failed to start${NC}"
    exit 1
fi
echo ""

# Step 3: 启动 MD Gateway
echo "${YELLOW}[Step 3/10] Starting MD Gateway...${NC}"
./gateway/build/md_gateway > ${MD_GATEWAY_LOG} 2>&1 &
MD_GW_PID=$!
sleep 3

if ps -p $MD_GW_PID > /dev/null; then
    echo "${GREEN}✓ MD Gateway started (PID: $MD_GW_PID)${NC}"
else
    echo "${RED}✗ MD Gateway failed to start${NC}"
    exit 1
fi
echo ""

# Step 4: 启动 Trader (多策略模式)
echo "${YELLOW}[Step 4/10] Starting Trader (multi-strategy mode)...${NC}"
./bin/trader -config ${CONFIG_FILE} > ${TRADER_LOG} 2>&1 &
TRADER_PID=$!
sleep 5

if ps -p $TRADER_PID > /dev/null; then
    echo "${GREEN}✓ Trader started (PID: $TRADER_PID)${NC}"
else
    echo "${RED}✗ Trader failed to start${NC}"
    cat ${TRADER_LOG}
    exit 1
fi
echo ""

# Step 5: 验证HTTP API
echo "${YELLOW}[Step 5/10] Verifying HTTP API endpoints...${NC}"
HTTP_RESPONSE=$(curl -s http://localhost:${API_PORT}/api/v1/dashboard/overview)
if echo "$HTTP_RESPONSE" | grep -q "success"; then
    echo "${GREEN}✓ HTTP API is working${NC}"
    MULTI_STRATEGY=$(echo $HTTP_RESPONSE | jq -r '.data.multi_strategy' 2>/dev/null)
    TOTAL_STRATEGIES=$(echo $HTTP_RESPONSE | jq -r '.data.total_strategies' 2>/dev/null)
    echo "  Multi-Strategy: ${MULTI_STRATEGY}"
    echo "  Total Strategies: ${TOTAL_STRATEGIES}"

    if [ "$MULTI_STRATEGY" != "true" ]; then
        echo "${RED}✗ Not in multi-strategy mode!${NC}"
        exit 1
    fi
else
    echo "${RED}✗ HTTP API failed${NC}"
    echo "  Response: $HTTP_RESPONSE"
    exit 1
fi
echo ""

# Step 6: 检查策略列表
echo "${YELLOW}[Step 6/10] Checking strategy list...${NC}"
STRATEGIES_RESPONSE=$(curl -s http://localhost:${API_PORT}/api/v1/strategies)
if echo "$STRATEGIES_RESPONSE" | grep -q "success"; then
    echo "${GREEN}✓ Strategies endpoint working${NC}"
    echo "$STRATEGIES_RESPONSE" | jq '.data.strategies[] | {id: .id, type: .type, running: .running}' 2>/dev/null | head -20
else
    echo "${RED}✗ Strategies endpoint failed${NC}"
fi
echo ""

# Step 7: 检查WebSocket端点
echo "${YELLOW}[Step 7/10] Checking WebSocket endpoint...${NC}"
WS_CHECK=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:${API_PORT}/api/v1/ws/dashboard)
if [ "$WS_CHECK" == "400" ] || [ "$WS_CHECK" == "426" ]; then
    echo "${GREEN}✓ WebSocket endpoint exists (HTTP $WS_CHECK - expects upgrade)${NC}"
else
    echo "${YELLOW}⚠ Unexpected response: HTTP $WS_CHECK${NC}"
fi
echo ""

# Step 8: 测试WebSocket连接（如果有websocat）
echo "${YELLOW}[Step 8/10] Testing WebSocket connection...${NC}"
if command -v websocat &> /dev/null; then
    echo "Using websocat to capture WebSocket data..."
    timeout 8 websocat ws://localhost:${API_PORT}/api/v1/ws/dashboard 2>/dev/null > /tmp/ws_capture.log &
    WS_PID=$!
    sleep 6

    if [ -f /tmp/ws_capture.log ] && [ -s /tmp/ws_capture.log ]; then
        echo "${GREEN}✓ WebSocket data received${NC}"
        echo ""
        echo "${BLUE}[WebSocket Data Sample]${NC}"

        # 提取第一条dashboard_update消息
        FIRST_MSG=$(grep -m 1 "dashboard_update" /tmp/ws_capture.log 2>/dev/null || echo "")
        if [ -n "$FIRST_MSG" ]; then
            echo "$FIRST_MSG" | jq '.' 2>/dev/null || echo "$FIRST_MSG"
            echo ""

            # 验证关键字段
            echo "${BLUE}[Verifying Data Fields]${NC}"

            # 检查strategies字段
            HAS_STRATEGIES=$(echo "$FIRST_MSG" | jq -r 'has("data") and .data | has("strategies")' 2>/dev/null)
            if [ "$HAS_STRATEGIES" == "true" ]; then
                echo "${GREEN}✓ Strategies data present${NC}"

                # 检查第一个策略的数据
                FIRST_STRAT_ID=$(echo "$FIRST_MSG" | jq -r '.data.strategies | to_entries | .[0].key' 2>/dev/null)
                if [ -n "$FIRST_STRAT_ID" ] && [ "$FIRST_STRAT_ID" != "null" ]; then
                    echo "  Strategy ID: ${FIRST_STRAT_ID}"

                    # 检查indicators
                    HAS_INDICATORS=$(echo "$FIRST_MSG" | jq -r ".data.strategies.\"${FIRST_STRAT_ID}\" | has(\"indicators\")" 2>/dev/null)
                    if [ "$HAS_INDICATORS" == "true" ]; then
                        echo "${GREEN}  ✓ Indicators field present${NC}"
                        INDICATOR_COUNT=$(echo "$FIRST_MSG" | jq -r ".data.strategies.\"${FIRST_STRAT_ID}\".indicators | length" 2>/dev/null)
                        echo "    Count: ${INDICATOR_COUNT}"
                    fi

                    # 检查thresholds
                    HAS_THRESHOLDS=$(echo "$FIRST_MSG" | jq -r ".data.strategies.\"${FIRST_STRAT_ID}\" | has(\"thresholds\")" 2>/dev/null)
                    if [ "$HAS_THRESHOLDS" == "true" ]; then
                        echo "${GREEN}  ✓ Thresholds field present${NC}"
                        THRESHOLD_COUNT=$(echo "$FIRST_MSG" | jq -r ".data.strategies.\"${FIRST_STRAT_ID}\".thresholds | length" 2>/dev/null)
                        echo "    Count: ${THRESHOLD_COUNT}"

                        # 显示阈值内容
                        echo "    Thresholds:"
                        echo "$FIRST_MSG" | jq -r ".data.strategies.\"${FIRST_STRAT_ID}\".thresholds" 2>/dev/null | sed 's/^/      /'
                    else
                        echo "${YELLOW}  ⚠ Thresholds field missing${NC}"
                    fi
                fi
            fi

            # 检查market_data字段
            HAS_MARKET_DATA=$(echo "$FIRST_MSG" | jq -r 'has("data") and .data | has("market_data")' 2>/dev/null)
            if [ "$HAS_MARKET_DATA" == "true" ]; then
                echo "${GREEN}✓ Market data present${NC}"
                MARKET_DATA_COUNT=$(echo "$FIRST_MSG" | jq -r '.data.market_data | length' 2>/dev/null)
                echo "  Symbols: ${MARKET_DATA_COUNT}"

                # 显示第一个行情数据
                FIRST_SYMBOL=$(echo "$FIRST_MSG" | jq -r '.data.market_data | to_entries | .[0].key' 2>/dev/null)
                if [ -n "$FIRST_SYMBOL" ] && [ "$FIRST_SYMBOL" != "null" ]; then
                    echo "  Sample (${FIRST_SYMBOL}):"
                    echo "$FIRST_MSG" | jq -r ".data.market_data.\"${FIRST_SYMBOL}\"" 2>/dev/null | sed 's/^/    /'
                fi
            else
                echo "${YELLOW}⚠ Market data field missing or empty${NC}"
            fi

            # 检查positions字段
            HAS_POSITIONS=$(echo "$FIRST_MSG" | jq -r 'has("data") and .data | has("positions")' 2>/dev/null)
            if [ "$HAS_POSITIONS" == "true" ]; then
                echo "${GREEN}✓ Positions field present${NC}"
            fi
        else
            echo "${YELLOW}⚠ No dashboard_update messages found${NC}"
        fi
    else
        echo "${YELLOW}⚠ No WebSocket data captured (file empty or missing)${NC}"
    fi

    kill $WS_PID 2>/dev/null || true
else
    echo "${YELLOW}⚠ websocat not installed, skipping WebSocket data capture${NC}"
    echo "  Install with: brew install websocat (macOS)"
fi
echo ""

# Step 9: 检查Trader日志
echo "${YELLOW}[Step 9/10] Checking trader logs for WebSocket activity...${NC}"
if grep -q "WebSocket.*Hub started" ${TRADER_LOG}; then
    echo "${GREEN}✓ WebSocket Hub initialized${NC}"
fi

if grep -q "Client connected" ${TRADER_LOG}; then
    CONN_COUNT=$(grep -c "Client connected" ${TRADER_LOG})
    echo "${GREEN}✓ WebSocket clients connected: ${CONN_COUNT}${NC}"
fi

if grep -q "Client disconnected" ${TRADER_LOG}; then
    DISCONN_COUNT=$(grep -c "Client disconnected" ${TRADER_LOG})
    echo "  Disconnections: ${DISCONN_COUNT}"
fi
echo ""

# Step 10: 显示Dashboard访问信息
echo "${YELLOW}[Step 10/10] Dashboard Access Information${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "${GREEN}✓ All services are running!${NC}"
echo ""
echo "Dashboard URL: ${BLUE}http://localhost:${API_PORT}/dashboard${NC}"
echo ""
echo "Manual Testing Steps:"
echo "  1. Open the dashboard URL in your browser"
echo "  2. Check connection indicator (top right) - should be green"
echo "  3. Verify 3 strategy cards are displayed:"
echo "     - ag_pairwise (pairwise_arb)"
echo "     - ag_passive (passive)"
echo "     - au_pairwise (pairwise_arb)"
echo "  4. Check indicators display format: \"value / threshold\""
echo "     Example: \"2.35 / 2.0\" for z_score"
echo "  5. Verify \"Market Data\" card on right sidebar shows:"
echo "     - ag2603, ag2605, au2604, au2606"
echo "     - Last Price, Bid/Ask prices"
echo "  6. Check timestamp updates every second"
echo "  7. Activate a strategy and verify real-time status updates"
echo ""
echo "API Endpoints:"
echo "  - Dashboard Overview: http://localhost:${API_PORT}/api/v1/dashboard/overview"
echo "  - Strategies List:    http://localhost:${API_PORT}/api/v1/strategies"
echo "  - WebSocket:          ws://localhost:${API_PORT}/api/v1/ws/dashboard"
echo ""
echo "Logs:"
echo "  - Trader:       ${TRADER_LOG}"
echo "  - MD Simulator: ${MD_SIMULATOR_LOG}"
echo "  - MD Gateway:   ${MD_GATEWAY_LOG}"
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "${YELLOW}Press Ctrl+C to stop all services and exit${NC}"
echo ""

# 持续监控WebSocket活动
echo "${BLUE}[Monitoring WebSocket Activity]${NC}"
tail -f ${TRADER_LOG} | grep --line-buffered -E "WebSocket|dashboard|market_data|Client"

# Cleanup will be triggered by EXIT trap

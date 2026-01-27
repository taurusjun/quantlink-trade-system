#!/bin/bash
# 检查CTP端到端测试状态

PROJECT_ROOT="/Users/user/PWorks/RD/quantlink-trade-system"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "╔═══════════════════════════════════════════════════════╗"
echo "║          CTP端到端测试状态检查                          ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 检查各个组件
echo -e "${YELLOW}[1] NATS服务器${NC}"
if pgrep -x "nats-server" > /dev/null; then
    echo -e "${GREEN}✓ 运行中${NC}"
else
    echo -e "${RED}✗ 未运行${NC}"
fi

echo ""
echo -e "${YELLOW}[2] CTP行情网关${NC}"
if pgrep -f "ctp_md_gateway" > /dev/null; then
    PID=$(pgrep -f "ctp_md_gateway")
    echo -e "${GREEN}✓ 运行中 (PID: $PID)${NC}"
    tail -3 log/ctp_md_gateway.log 2>/dev/null | sed 's/^/  /'
else
    echo -e "${RED}✗ 未运行${NC}"
fi

echo ""
echo -e "${YELLOW}[3] MD Gateway${NC}"
if pgrep -f "md_gateway" > /dev/null; then
    PID=$(pgrep -f "md_gateway")
    echo -e "${GREEN}✓ 运行中 (PID: $PID)${NC}"
else
    echo -e "${RED}✗ 未运行${NC}"
fi

echo ""
echo -e "${YELLOW}[4] ORS Gateway${NC}"
if pgrep -f "ors_gateway" > /dev/null; then
    PID=$(pgrep -f "ors_gateway")
    echo -e "${GREEN}✓ 运行中 (PID: $PID)${NC}"
else
    echo -e "${RED}✗ 未运行${NC}"
fi

echo ""
echo -e "${YELLOW}[5] Counter Bridge${NC}"
if pgrep -f "counter_bridge" > /dev/null; then
    PID=$(pgrep -f "counter_bridge")
    echo -e "${GREEN}✓ 运行中 (PID: $PID)${NC}"
else
    echo -e "${RED}✗ 未运行${NC}"
fi

echo ""
echo -e "${YELLOW}[6] Trader（策略引擎）${NC}"
if pgrep -f "trader -config" > /dev/null; then
    PID=$(pgrep -f "trader -config")
    echo -e "${GREEN}✓ 运行中 (PID: $PID)${NC}"
    echo ""
    echo "  最新日志:"
    tail -5 log/trader.test.log 2>/dev/null | sed 's/^/  /'
else
    echo -e "${RED}✗ 未运行${NC}"
fi

echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║                  策略状态查询                           ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 查询策略状态
if pgrep -f "trader -config" > /dev/null; then
    curl -s http://localhost:9201/api/v1/strategy/status 2>/dev/null && echo "" || echo "API未响应"
else
    echo "Trader未运行，无法查询策略状态"
fi

echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║                  订单统计                              ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

if [ -f log/trader.test.log ]; then
    ORDER_COUNT=$(grep -c "Order sent" log/trader.test.log 2>/dev/null || echo "0")
    echo "总订单数: $ORDER_COUNT"

    if [ $ORDER_COUNT -gt 0 ]; then
        echo ""
        echo "最近5笔订单:"
        grep "Order sent" log/trader.test.log 2>/dev/null | tail -5 | sed 's/^/  /'
    fi
else
    echo "无日志文件"
fi

echo ""

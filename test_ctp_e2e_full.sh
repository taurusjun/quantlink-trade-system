#!/bin/bash
# CTP实盘端到端测试脚本
# 测试链路：CTP行情 → md_gateway → NATS → trader → ors_gateway → counter_bridge(CTP交易)

set -e

PROJECT_ROOT="/Users/user/PWorks/RD/quantlink-trade-system"
cd "$PROJECT_ROOT"

echo "╔═══════════════════════════════════════════════════════╗"
echo "║     CTP实盘端到端测试 - 完整链路                        ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 清理旧进程
echo -e "${YELLOW}[1/8] 清理旧进程...${NC}"
pkill -9 ctp_md_gateway 2>/dev/null || true
pkill -9 md_gateway 2>/dev/null || true
pkill -9 ors_gateway 2>/dev/null || true
pkill -9 counter_bridge 2>/dev/null || true
pkill -9 trader 2>/dev/null || true
sleep 2

# 清理共享内存
echo -e "${YELLOW}[2/8] 清理共享内存...${NC}"
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true

# 创建日志目录
mkdir -p log

# 检查NATS是否运行
echo -e "${YELLOW}[3/8] 检查NATS服务...${NC}"
if ! pgrep -x "nats-server" > /dev/null; then
    echo -e "${RED}NATS服务未运行！${NC}"
    echo "启动NATS: nats-server &"
    exit 1
fi
echo -e "${GREEN}✓ NATS服务运行中${NC}"

# 启动CTP行情网关
echo -e "${YELLOW}[4/8] 启动CTP行情网关...${NC}"
./gateway/build/ctp_md_gateway -c config/ctp_md.yaml > log/ctp_md_gateway.log 2>&1 &
CTP_MD_PID=$!
echo "PID: $CTP_MD_PID"
sleep 3

# 检查CTP行情网关状态
if ! kill -0 $CTP_MD_PID 2>/dev/null; then
    echo -e "${RED}✗ CTP行情网关启动失败${NC}"
    tail -20 log/ctp_md_gateway.log
    exit 1
fi
echo -e "${GREEN}✓ CTP行情网关运行中${NC}"

# 启动MD Gateway（共享内存 → NATS）
echo -e "${YELLOW}[5/8] 启动MD Gateway...${NC}"
./gateway/build/md_gateway > log/md_gateway.log 2>&1 &
MD_GATEWAY_PID=$!
echo "PID: $MD_GATEWAY_PID"
sleep 2

# 启动ORS Gateway（订单路由）
echo -e "${YELLOW}[6/8] 启动ORS Gateway...${NC}"
./gateway/build/ors_gateway > log/ors_gateway.log 2>&1 &
ORS_GATEWAY_PID=$!
echo "PID: $ORS_GATEWAY_PID"
sleep 2

# 启动Counter Bridge（CTP交易桥接）
echo -e "${YELLOW}[7/8] 启动Counter Bridge...${NC}"
./gateway/build/counter_bridge ctp:config/ctp/ctp_td.yaml > log/counter_bridge.log 2>&1 &
COUNTER_PID=$!
echo "PID: $COUNTER_PID"
sleep 3

# 启动Trader（策略引擎）
echo -e "${YELLOW}[8/8] 启动Trader（策略引擎）...${NC}"
./bin/trader -config config/trader.test.yaml > log/trader.test.log 2>&1 &
TRADER_PID=$!
echo "PID: $TRADER_PID"
sleep 3

# 显示所有进程状态
echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║               所有组件启动完成                           ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""
echo -e "${GREEN}运行中的进程:${NC}"
echo "  CTP行情网关: PID $CTP_MD_PID"
echo "  MD Gateway:  PID $MD_GATEWAY_PID"
echo "  ORS Gateway: PID $ORS_GATEWAY_PID"
echo "  Counter Bridge: PID $COUNTER_PID"
echo "  Trader:      PID $TRADER_PID"
echo ""

# 检查进程状态
echo -e "${YELLOW}检查进程状态...${NC}"
ALL_OK=true

for pid in $CTP_MD_PID $MD_GATEWAY_PID $ORS_GATEWAY_PID $COUNTER_PID $TRADER_PID; do
    if ! kill -0 $pid 2>/dev/null; then
        echo -e "${RED}✗ 进程 $pid 已停止${NC}"
        ALL_OK=false
    fi
done

if [ "$ALL_OK" = true ]; then
    echo -e "${GREEN}✓ 所有进程运行正常${NC}"
else
    echo -e "${RED}✗ 部分进程异常，请检查日志${NC}"
fi

echo ""
echo "╔═══════════════════════════════════════════════════════╗"
echo "║                   监控命令                             ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""
echo "查看CTP行情:"
echo "  tail -f log/ctp_md_gateway.log"
echo ""
echo "查看策略日志:"
echo "  tail -f log/trader.test.log"
echo ""
echo "查看订单生成:"
echo "  tail -f log/trader.test.log | grep 'Order sent'"
echo ""
echo "激活策略（等待5秒后执行）:"
echo "  curl -X POST http://localhost:9201/api/v1/strategy/activate \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"strategy_id\": \"test_92201\"}'"
echo ""
echo "查看策略状态:"
echo "  curl http://localhost:9201/api/v1/strategy/status"
echo ""
echo "停止所有服务:"
echo "  pkill -f 'ctp_md_gateway|md_gateway|ors_gateway|counter_bridge|trader'"
echo ""

# 保存进程ID
echo "$CTP_MD_PID $MD_GATEWAY_PID $ORS_GATEWAY_PID $COUNTER_PID $TRADER_PID" > /tmp/ctp_e2e_pids.txt

echo -e "${GREEN}脚本执行完成！${NC}"
echo "进程ID已保存到: /tmp/ctp_e2e_pids.txt"

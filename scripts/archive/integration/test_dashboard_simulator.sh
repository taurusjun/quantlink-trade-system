#!/bin/bash
# Dashboard 模拟器端到端测试脚本
# 使用模拟行情数据测试多策略Dashboard

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

API_PORT=9301
LOG_DIR="$PROJECT_ROOT/test_logs"
mkdir -p "$LOG_DIR"

PID_FILE="$LOG_DIR/dashboard_sim_pids.txt"
rm -f "$PID_FILE"

cleanup() {
    echo -e "\n${YELLOW}清理测试进程...${NC}"
    if [ -f "$PID_FILE" ]; then
        while read -r line; do
            pid=$(echo "$line" | awk '{print $1}')
            name=$(echo "$line" | cut -d' ' -f2-)
            kill "$pid" 2>/dev/null && echo -e "${YELLOW}停止: $name${NC}" || true
        done < "$PID_FILE"
    fi
    pkill -f "md_simulator" 2>/dev/null || true
    pkill -f "md_gateway.*queue" 2>/dev/null || true
    pkill -f "ors_gateway" 2>/dev/null || true
    pkill -f "counter_gateway" 2>/dev/null || true
    pkill -f "trader.*dashboard.sim" 2>/dev/null || true
    echo -e "${GREEN}✓ 清理完成${NC}"
}

trap cleanup EXIT INT TERM

start_process() {
    local name=$1
    local cmd=$2
    local log=$3
    echo -e "${CYAN}启动: $name${NC}"
    eval "$cmd" > "$log" 2>&1 &
    local pid=$!
    echo "$pid $name" >> "$PID_FILE"
    sleep 2
    if kill -0 "$pid" 2>/dev/null; then
        echo -e "${GREEN}✓ $name (PID: $pid)${NC}"
        return 0
    else
        echo -e "${RED}✗ $name 失败${NC}"
        tail -5 "$log"
        return 1
    fi
}

wait_port() {
    local port=$1
    local max=10
    for i in $(seq 1 $max); do
        lsof -i :$port >/dev/null 2>&1 && return 0
        sleep 1
    done
    return 1
}

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Dashboard 模拟器端到端测试                             ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}\n"

# 1. 检查/启动 NATS
echo -e "${YELLOW}[1/6] NATS${NC}"
if ! lsof -i :4222 >/dev/null 2>&1; then
    nats-server > "$LOG_DIR/nats.log" 2>&1 &
    sleep 2
fi
echo -e "${GREEN}✓ NATS 运行中${NC}\n"

# 2. 行情模拟器
echo -e "${YELLOW}[2/6] 行情模拟器${NC}"
start_process "md_simulator" "./gateway/build/md_simulator 50 queue" "$LOG_DIR/md_simulator.log"

# 3. MD Gateway
echo -e "\n${YELLOW}[3/6] MD Gateway${NC}"
start_process "md_gateway" "./gateway/build/md_gateway queue" "$LOG_DIR/md_gateway.log"
wait_port 50051

# 4. ORS Gateway
echo -e "\n${YELLOW}[4/6] ORS Gateway${NC}"
start_process "ors_gateway" "./gateway/build/ors_gateway" "$LOG_DIR/ors_gateway.log"
wait_port 50052

# 5. Counter Gateway
echo -e "\n${YELLOW}[5/6] Counter Gateway${NC}"
start_process "counter_gateway" "./gateway/build/counter_gateway" "$LOG_DIR/counter_gateway.log"

# 6. Trader (使用模拟器配置)
echo -e "\n${YELLOW}[6/6] Multi-Strategy Trader${NC}"
cd golang
start_process "trader" "./trader -config config/trader.dashboard.sim.yaml" "$LOG_DIR/trader_dashboard.log"
cd ..
wait_port $API_PORT

# 验证
echo -e "\n${YELLOW}验证系统${NC}"
sleep 2

health=$(curl -s "http://localhost:$API_PORT/api/v1/health" 2>/dev/null)
if echo "$health" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ API 健康${NC}"
else
    echo -e "${RED}✗ API 异常${NC}"
fi

overview=$(curl -s "http://localhost:$API_PORT/api/v1/dashboard/overview" 2>/dev/null)
total=$(echo "$overview" | grep -o '"total_strategies":[0-9]*' | grep -o '[0-9]*')
echo -e "${GREEN}✓ 策略数量: $total${NC}"

# 打开 Dashboard
echo -e "\n${YELLOW}打开 Dashboard${NC}"
DASHBOARD_URL="http://localhost:8080/dashboard.html"

# 检查 HTTP 服务器
if ! lsof -i :8080 >/dev/null 2>&1; then
    cd golang/web && python3 -m http.server 8080 > /dev/null 2>&1 &
    cd ../..
    sleep 1
fi

if command -v open &>/dev/null; then
    open "$DASHBOARD_URL"
elif command -v xdg-open &>/dev/null; then
    xdg-open "$DASHBOARD_URL"
fi
echo -e "${GREEN}✓ Dashboard: $DASHBOARD_URL${NC}"

# 状态摘要
echo -e "\n${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}系统运行中${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "  Dashboard:  ${GREEN}$DASHBOARD_URL${NC}"
echo -e "  API:        ${GREEN}http://localhost:$API_PORT${NC}"
echo -e "  模式:       ${GREEN}simulation${NC}"
echo -e ""
echo -e "${CYAN}测试命令:${NC}"
echo -e "  curl http://localhost:$API_PORT/api/v1/dashboard/overview | jq"
echo -e "  curl http://localhost:$API_PORT/api/v1/strategies | jq"
echo -e "  curl -X POST http://localhost:$API_PORT/api/v1/strategies/ag_pairwise/activate"
echo -e ""
echo -e "${YELLOW}按 Ctrl+C 停止测试${NC}\n"

# 监控循环
while true; do
    sleep 10
    indicators=$(curl -s "http://localhost:$API_PORT/api/v1/indicators/realtime" 2>/dev/null)
    if [ -n "$indicators" ]; then
        echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} 策略指标:"
        echo "$indicators" | jq -r '.data.strategies | to_entries[] | "  \(.key): z=\(.value.indicators.z_score // "N/A" | tostring | .[0:6]) corr=\(.value.indicators.correlation // "N/A" | tostring | .[0:5])"' 2>/dev/null || true
    fi
done

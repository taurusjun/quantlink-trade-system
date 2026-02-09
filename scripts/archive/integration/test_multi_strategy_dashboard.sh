#!/bin/bash
# 多策略Dashboard端到端测试脚本
# Multi-Strategy Dashboard End-to-End Test Script

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

# 配置
TRADER_CONFIG="config/trader.multi.test.yaml"
API_PORT=9301
LOG_DIR="$PROJECT_ROOT/test_logs"
mkdir -p "$LOG_DIR"

# PID文件
PID_FILE="$LOG_DIR/multi_strategy_pids.txt"
rm -f "$PID_FILE"

# 清理函数
cleanup() {
    echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}清理测试进程...${NC}"
    echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"

    if [ -f "$PID_FILE" ]; then
        while read -r line; do
            pid=$(echo "$line" | awk '{print $1}')
            name=$(echo "$line" | cut -d' ' -f2-)
            if kill -0 "$pid" 2>/dev/null; then
                echo -e "${YELLOW}停止: $name (PID: $pid)${NC}"
                kill "$pid" 2>/dev/null || true
            fi
        done < "$PID_FILE"
    fi

    sleep 2
    pkill -9 -f "md_simulator" 2>/dev/null || true
    pkill -9 -f "md_gateway" 2>/dev/null || true
    pkill -9 -f "ors_gateway" 2>/dev/null || true
    pkill -9 -f "counter_gateway" 2>/dev/null || true
    pkill -9 -f "trader.*multi.test" 2>/dev/null || true

    echo -e "${GREEN}✓ 清理完成${NC}"
}

trap cleanup EXIT INT TERM

# 启动进程
start_process() {
    local name=$1
    local cmd=$2
    local log=$3

    echo -e "${CYAN}启动: $name${NC}"
    echo -e "${BLUE}命令: $cmd${NC}"

    eval "$cmd" > "$log" 2>&1 &
    local pid=$!
    echo "$pid $name" >> "$PID_FILE"

    sleep 2

    if kill -0 "$pid" 2>/dev/null; then
        echo -e "${GREEN}✓ $name 启动成功 (PID: $pid)${NC}\n"
        return 0
    else
        echo -e "${RED}✗ $name 启动失败${NC}"
        tail -10 "$log"
        return 1
    fi
}

# 等待端口
wait_for_port() {
    local port=$1
    local name=$2
    local max_wait=10

    echo -e "${CYAN}等待 $name 端口 $port...${NC}"
    for i in $(seq 1 $max_wait); do
        if lsof -i :$port >/dev/null 2>&1; then
            echo -e "${GREEN}✓ 端口 $port 就绪${NC}\n"
            return 0
        fi
        sleep 1
    done
    echo -e "${RED}✗ 端口 $port 超时${NC}\n"
    return 1
}

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     多策略Dashboard端到端测试                              ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}\n"

echo -e "${CYAN}配置信息:${NC}"
echo -e "  配置文件: ${GREEN}$TRADER_CONFIG${NC}"
echo -e "  API端口:  ${GREEN}$API_PORT${NC}"
echo -e "  Dashboard: ${GREEN}golang/web/dashboard.html${NC}\n"

# ═══════════════════════════════════════════════════════════
# 检查NATS
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}[1/6] 检查NATS服务${NC}"
if ! lsof -i :4222 >/dev/null 2>&1; then
    echo -e "${RED}✗ NATS未运行，启动中...${NC}"
    nats-server > "$LOG_DIR/nats.log" 2>&1 &
    sleep 2
fi
echo -e "${GREEN}✓ NATS服务运行中${NC}\n"

# ═══════════════════════════════════════════════════════════
# 启动行情模拟器
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}[2/6] 启动行情模拟器${NC}"
start_process "md_simulator" "./gateway/build/md_simulator 100 queue" "$LOG_DIR/md_simulator.log"

# ═══════════════════════════════════════════════════════════
# 启动MD Gateway
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}[3/6] 启动行情网关${NC}"
start_process "md_gateway" "./gateway/build/md_gateway queue" "$LOG_DIR/md_gateway.log"
wait_for_port 50051 "MD Gateway"

# ═══════════════════════════════════════════════════════════
# 启动ORS Gateway
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}[4/6] 启动订单网关${NC}"
start_process "ors_gateway" "./gateway/build/ors_gateway" "$LOG_DIR/ors_gateway.log"
wait_for_port 50052 "ORS Gateway"

# ═══════════════════════════════════════════════════════════
# 启动Counter Gateway
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}[5/6] 启动柜台网关${NC}"
start_process "counter_gateway" "./gateway/build/counter_gateway" "$LOG_DIR/counter_gateway.log"

# ═══════════════════════════════════════════════════════════
# 启动多策略Trader
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}[6/6] 启动多策略Trader${NC}"
cd golang
start_process "multi_trader" "./trader -config $TRADER_CONFIG" "$LOG_DIR/multi_trader.log"
cd ..

wait_for_port $API_PORT "Trader API"

# ═══════════════════════════════════════════════════════════
# 验证API
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}验证API端点${NC}\n"

echo -e "${CYAN}[健康检查]${NC}"
health=$(curl -s "http://localhost:$API_PORT/api/v1/health")
if echo "$health" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ API健康检查通过${NC}"
else
    echo -e "${RED}✗ API健康检查失败${NC}"
fi

echo -e "\n${CYAN}[Dashboard Overview]${NC}"
overview=$(curl -s "http://localhost:$API_PORT/api/v1/dashboard/overview")
if echo "$overview" | grep -q '"multi_strategy":true'; then
    total=$(echo "$overview" | grep -o '"total_strategies":[0-9]*' | grep -o '[0-9]*')
    echo -e "${GREEN}✓ 多策略模式已启用，共 $total 个策略${NC}"
else
    echo -e "${RED}✗ Dashboard Overview失败${NC}"
fi

echo -e "\n${CYAN}[策略列表]${NC}"
strategies=$(curl -s "http://localhost:$API_PORT/api/v1/strategies")
echo "$strategies" | python3 -c "
import sys, json
try:
    d = json.load(sys.stdin)
    for s in d['data']['strategies']:
        status = '🟢' if s['running'] else '🔴'
        active = '激活' if s['active'] else '未激活'
        print(f\"  {status} {s['id']} ({s['type']}) - {active}\")
except:
    print('  解析失败')
" 2>/dev/null

# ═══════════════════════════════════════════════════════════
# 打开Dashboard
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}打开Dashboard页面${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

DASHBOARD_PATH="$PROJECT_ROOT/golang/web/dashboard.html"
echo -e "${CYAN}Dashboard路径: $DASHBOARD_PATH${NC}"
echo -e "${CYAN}API地址: http://localhost:$API_PORT${NC}\n"

# 打开浏览器
if command -v open &> /dev/null; then
    open "$DASHBOARD_PATH"
    echo -e "${GREEN}✓ Dashboard已在浏览器打开${NC}"
elif command -v xdg-open &> /dev/null; then
    xdg-open "$DASHBOARD_PATH"
    echo -e "${GREEN}✓ Dashboard已在浏览器打开${NC}"
else
    echo -e "${YELLOW}请手动打开: $DASHBOARD_PATH${NC}"
fi

echo -e "\n${YELLOW}注意: 在Dashboard页面中设置API端口为 $API_PORT${NC}"

# ═══════════════════════════════════════════════════════════
# 监控运行状态
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}系统运行中 - 按Ctrl+C停止${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}查看日志:${NC}"
echo -e "  tail -f $LOG_DIR/multi_trader.log"
echo -e "  tail -f $LOG_DIR/md_gateway.log"

echo -e "\n${CYAN}API测试:${NC}"
echo -e "  curl http://localhost:$API_PORT/api/v1/dashboard/overview | jq"
echo -e "  curl http://localhost:$API_PORT/api/v1/strategies | jq"
echo -e "  curl http://localhost:$API_PORT/api/v1/indicators/realtime | jq"

echo -e "\n${CYAN}激活策略:${NC}"
echo -e "  curl -X POST http://localhost:$API_PORT/api/v1/strategies/ag_pairwise/activate"
echo -e "  curl -X POST http://localhost:$API_PORT/api/v1/strategies/cu_passive/activate"
echo -e "  curl -X POST http://localhost:$API_PORT/api/v1/strategies/al_aggressive/activate"

echo -e "\n"

# 定期输出状态
while true; do
    sleep 10
    echo -e "${BLUE}[$(date '+%H:%M:%S')] 系统状态检查${NC}"

    # 检查进程
    running=0
    if [ -f "$PID_FILE" ]; then
        while read -r line; do
            pid=$(echo "$line" | awk '{print $1}')
            if kill -0 "$pid" 2>/dev/null; then
                ((running++))
            fi
        done < "$PID_FILE"
    fi
    echo -e "  运行进程: $running/5"

    # 检查策略状态
    overview=$(curl -s "http://localhost:$API_PORT/api/v1/dashboard/overview" 2>/dev/null)
    if [ -n "$overview" ]; then
        active=$(echo "$overview" | grep -o '"active_strategies":[0-9]*' | grep -o '[0-9]*' || echo "0")
        total=$(echo "$overview" | grep -o '"total_strategies":[0-9]*' | grep -o '[0-9]*' || echo "0")
        echo -e "  激活策略: $active/$total"
    fi
    echo ""
done

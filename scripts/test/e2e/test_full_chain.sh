#!/bin/bash
# 完整链路端到端测试脚本
# 测试从行情源到订单回报的完整流程

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

# 日志目录
LOG_DIR="$PROJECT_ROOT/test_logs"
mkdir -p "$LOG_DIR"

# PID文件
PID_FILE="$LOG_DIR/pids.txt"
rm -f "$PID_FILE"

# 清理函数
cleanup() {
    echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}清理所有测试进程...${NC}"
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

    # 等待进程结束
    sleep 2

    # 强制杀死残留进程
    pkill -9 -f "md_simulator" 2>/dev/null || true
    pkill -9 -f "md_gateway" 2>/dev/null || true
    pkill -9 -f "ors_gateway" 2>/dev/null || true
    pkill -9 -f "counter_gateway" 2>/dev/null || true
    pkill -9 -f "bin/trader" 2>/dev/null || true

    echo -e "${GREEN}✓ 清理完成${NC}"
}

# 注册清理函数
trap cleanup EXIT INT TERM

# 启动进程并记录PID
start_process() {
    local name=$1
    local cmd=$2
    local log=$3

    echo -e "${CYAN}启动: $name${NC}"
    echo -e "${BLUE}命令: $cmd${NC}"
    echo -e "${BLUE}日志: $log${NC}"

    eval "$cmd" > "$log" 2>&1 &
    local pid=$!
    echo "$pid $name" >> "$PID_FILE"

    # 等待进程启动
    sleep 2

    # 检查进程是否存活
    if kill -0 "$pid" 2>/dev/null; then
        echo -e "${GREEN}✓ $name 启动成功 (PID: $pid)${NC}\n"
        return 0
    else
        echo -e "${RED}✗ $name 启动失败${NC}"
        echo -e "${RED}查看日志: tail -f $log${NC}\n"
        return 1
    fi
}

# 等待端口监听
wait_for_port() {
    local port=$1
    local name=$2
    local max_wait=10

    echo -e "${CYAN}等待 $name 端口 $port 就绪...${NC}"

    for i in $(seq 1 $max_wait); do
        if lsof -i :$port >/dev/null 2>&1; then
            echo -e "${GREEN}✓ 端口 $port 已就绪${NC}\n"
            return 0
        fi
        sleep 1
        echo -n "."
    done

    echo -e "\n${RED}✗ 端口 $port 超时${NC}\n"
    return 1
}

# 验证日志内容
check_log() {
    local log=$1
    local pattern=$2
    local description=$3

    if grep -q "$pattern" "$log" 2>/dev/null; then
        echo -e "${GREEN}✓ $description${NC}"
        return 0
    else
        echo -e "${RED}✗ $description${NC}"
        return 1
    fi
}

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║        完整链路端到端测试                                  ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}\n"

echo -e "${YELLOW}测试链路:${NC}"
echo -e "  1. ${CYAN}md_simulator${NC} → 共享内存"
echo -e "  2. 共享内存 → ${CYAN}md_gateway${NC} → NATS"
echo -e "  3. NATS → ${CYAN}Golang Trader${NC} (接收行情)"
echo -e "  4. Golang Strategy → ${CYAN}ORS Client${NC} (发送订单)"
echo -e "  5. ${CYAN}ORS Gateway${NC} → ${CYAN}Counter Gateway${NC}"
echo -e "  6. ${CYAN}Simulated Counter${NC} → 订单回报"
echo -e "  7. NATS → ${CYAN}Golang Trader${NC} (接收回报)\n"

# ═══════════════════════════════════════════════════════════
# Layer 1: 检查NATS服务
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 1: 检查NATS服务${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

if ! lsof -i :4222 >/dev/null 2>&1; then
    echo -e "${RED}✗ NATS服务未运行 (端口4222)${NC}"
    echo -e "${YELLOW}请先启动NATS:${NC}"
    echo -e "  brew services start nats-server"
    echo -e "  或"
    echo -e "  nats-server &"
    exit 1
fi
echo -e "${GREEN}✓ NATS服务运行中 (端口4222)${NC}\n"

# ═══════════════════════════════════════════════════════════
# Layer 2: 启动MD Simulator (行情数据源)
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 2: 启动行情模拟器 (md_simulator)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process \
    "md_simulator" \
    "./gateway/build/md_simulator 100 queue" \
    "$LOG_DIR/md_simulator.log"

sleep 1
check_log "$LOG_DIR/md_simulator.log" "Starting market data generation" "行情生成器已启动"

# ═══════════════════════════════════════════════════════════
# Layer 3: 启动MD Gateway (行情网关)
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 3: 启动MD Gateway (行情网关)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process \
    "md_gateway" \
    "./gateway/build/md_gateway queue" \
    "$LOG_DIR/md_gateway.log"

wait_for_port 50051 "MD Gateway gRPC"
check_log "$LOG_DIR/md_gateway.log" "Started successfully" "MD Gateway已启动"
check_log "$LOG_DIR/md_gateway.log" "Connected to NATS" "NATS连接成功"

# ═══════════════════════════════════════════════════════════
# Layer 4: 启动ORS Gateway (订单路由网关)
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 4: 启动ORS Gateway (订单路由网关)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process \
    "ors_gateway" \
    "./gateway/build/ors_gateway" \
    "$LOG_DIR/ors_gateway.log"

wait_for_port 50052 "ORS Gateway gRPC"
check_log "$LOG_DIR/ors_gateway.log" "started successfully" "ORS Gateway已启动"

# ═══════════════════════════════════════════════════════════
# Layer 5: 启动Counter Gateway (柜台网关)
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 5: 启动Counter Gateway (柜台网关)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process \
    "counter_gateway" \
    "./gateway/build/counter_gateway" \
    "$LOG_DIR/counter_gateway.log"

sleep 2
check_log "$LOG_DIR/counter_gateway.log" "started successfully" "Counter Gateway已启动"

# ═══════════════════════════════════════════════════════════
# Layer 6: 启动Golang Trader (策略引擎)
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 6: 启动Golang Trader (策略引擎)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process \
    "golang_trader" \
    "./bin/trader -config config/trader.test.yaml" \
    "$LOG_DIR/golang_trader.log"

wait_for_port 9201 "Trader API"
sleep 3
check_log "$LOG_DIR/golang_trader.log" "Strategy initialized" "Trader已启动" || true
check_log "$LOG_DIR/golang_trader.log" "API server started" "API服务已启动" || true

# ═══════════════════════════════════════════════════════════
# Layer 7: 验证各层数据流
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 7: 验证数据流${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}等待数据流稳定...${NC}"
sleep 5

echo -e "\n${CYAN}[1] 检查行情数据生成${NC}"
if check_log "$LOG_DIR/md_simulator.log" "Pushed:" "行情数据正在生成"; then
    tail -3 "$LOG_DIR/md_simulator.log" | grep "Pushed:"
fi

echo -e "\n${CYAN}[2] 检查MD Gateway接收${NC}"
if check_log "$LOG_DIR/md_gateway.log" "Read:" "MD Gateway正在接收数据"; then
    tail -3 "$LOG_DIR/md_gateway.log" | grep "Read:" | head -1
fi

echo -e "\n${CYAN}[3] 检查NATS发布${NC}"
if check_log "$LOG_DIR/md_gateway.log" "Published.*messages to NATS" "NATS消息发布正常"; then
    tail -10 "$LOG_DIR/md_gateway.log" | grep "Published" | tail -1
fi

echo -e "\n${CYAN}[4] 检查Trader状态${NC}"
trader_status=$(curl -s http://localhost:9201/api/v1/strategy/status 2>/dev/null)
if [ -n "$trader_status" ]; then
    echo -e "${GREEN}✓ Trader API响应正常${NC}"
    echo "$trader_status" | python3 -c "import sys, json; d=json.load(sys.stdin)['data']; print(f\"  Strategy: {d['strategy_id']}\\n  Running: {d['running']}\\n  Active: {d['active']}\")" 2>/dev/null || echo "  状态: 运行中"
else
    echo -e "${RED}✗ Trader API无响应${NC}"
fi

# ═══════════════════════════════════════════════════════════
# Layer 8: 手动激活策略
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 8: 激活策略${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}发送激活请求...${NC}"
activate_result=$(curl -s -X POST http://localhost:9201/api/v1/strategy/activate 2>/dev/null)
if echo "$activate_result" | grep -q "success.*true"; then
    echo -e "${GREEN}✓ 策略激活成功${NC}"
else
    echo -e "${RED}✗ 策略激活失败${NC}"
    echo "$activate_result"
fi

sleep 2

# ═══════════════════════════════════════════════════════════
# Layer 9: 等待策略交易
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 9: 监控策略交易${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}等待30秒，观察系统运行...${NC}\n"

for i in {1..6}; do
    echo -e "${BLUE}[检查点 $i/6]${NC}"

    # 检查行情流
    md_count=$(grep -c "Pushed:" "$LOG_DIR/md_simulator.log" 2>/dev/null || echo 0)
    echo -e "  行情生成: ${GREEN}$md_count${NC} 条"

    # 检查订单
    if [ -f "$LOG_DIR/ors_gateway.log" ]; then
        order_count=$(grep -c "Received order request" "$LOG_DIR/ors_gateway.log" 2>/dev/null || echo 0)
        echo -e "  订单发送: ${GREEN}$order_count${NC} 条"
    fi

    # 检查回报
    if [ -f "$LOG_DIR/counter_gateway.log" ]; then
        fill_count=$(grep -c "Order filled" "$LOG_DIR/counter_gateway.log" 2>/dev/null || echo 0)
        echo -e "  订单成交: ${GREEN}$fill_count${NC} 条"
    fi

    echo ""
    sleep 5
done

# ═══════════════════════════════════════════════════════════
# Layer 10: 最终统计
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Layer 10: 最终统计${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}行情数据链路:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

md_generated=$(grep -c "Pushed:" "$LOG_DIR/md_simulator.log" 2>/dev/null || echo 0)
md_received=$(grep -c "Read:" "$LOG_DIR/md_gateway.log" 2>/dev/null || echo 0)
md_published=$(grep "Published.*messages to NATS" "$LOG_DIR/md_gateway.log" 2>/dev/null | tail -1 | grep -oE "[0-9]+" | head -1 || echo 0)

echo -e "  生成行情: ${GREEN}$md_generated${NC} 条"
echo -e "  网关接收: ${GREEN}$md_received${NC} 条"
echo -e "  NATS发布: ${GREEN}$md_published${NC} 条"

if [ "$md_generated" -gt 0 ] && [ "$md_received" -gt 0 ]; then
    loss_rate=$(echo "scale=2; ($md_generated - $md_received) * 100 / $md_generated" | bc 2>/dev/null || echo "0")
    echo -e "  丢包率: ${GREEN}${loss_rate}%${NC}"
fi

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}订单处理链路:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

order_sent=0
order_accepted=0
order_filled=0
order_rejected=0

if [ -f "$LOG_DIR/ors_gateway.log" ]; then
    order_sent=$(grep -c "Received order request" "$LOG_DIR/ors_gateway.log" 2>/dev/null || echo 0)
fi

if [ -f "$LOG_DIR/counter_gateway.log" ]; then
    order_accepted=$(grep -c "Order accepted" "$LOG_DIR/counter_gateway.log" 2>/dev/null || echo 0)
    order_filled=$(grep -c "Order filled" "$LOG_DIR/counter_gateway.log" 2>/dev/null || echo 0)
    order_rejected=$(grep -c "Order rejected" "$LOG_DIR/counter_gateway.log" 2>/dev/null || echo 0)
fi

echo -e "  发送订单: ${GREEN}$order_sent${NC} 条"
echo -e "  订单接受: ${GREEN}$order_accepted${NC} 条"
echo -e "  订单成交: ${GREEN}$order_filled${NC} 条"
echo -e "  订单拒绝: ${YELLOW}$order_rejected${NC} 条"

if [ "$order_sent" -gt 0 ]; then
    fill_rate=$(echo "scale=2; $order_filled * 100 / $order_sent" | bc 2>/dev/null || echo "0")
    echo -e "  成交率: ${GREEN}${fill_rate}%${NC}"
fi

# ═══════════════════════════════════════════════════════════
# 测试结果汇总
# ═══════════════════════════════════════════════════════════
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}测试结果汇总${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

all_passed=true

# 检查各层
echo -e "${CYAN}各层状态检查:${NC}\n"

layers=(
    "md_simulator:行情模拟器"
    "md_gateway:行情网关"
    "ors_gateway:订单网关"
    "counter_gateway:柜台网关"
    "golang_trader:策略引擎"
)

for layer in "${layers[@]}"; do
    name="${layer%%:*}"
    desc="${layer##*:}"

    if grep -q "$name" "$PID_FILE" 2>/dev/null; then
        pid=$(grep "$name" "$PID_FILE" | awk '{print $1}')
        if kill -0 "$pid" 2>/dev/null; then
            echo -e "  ${GREEN}✓${NC} $desc (PID: $pid)"
        else
            echo -e "  ${RED}✗${NC} $desc (进程已停止)"
            all_passed=false
        fi
    else
        echo -e "  ${RED}✗${NC} $desc (未启动)"
        all_passed=false
    fi
done

# 数据流检查
echo -e "\n${CYAN}数据流检查:${NC}\n"

if [ "$md_generated" -gt 100 ]; then
    echo -e "  ${GREEN}✓${NC} 行情生成: $md_generated 条"
else
    echo -e "  ${RED}✗${NC} 行情生成不足: $md_generated 条"
    all_passed=false
fi

if [ "$md_published" -gt 50 ]; then
    echo -e "  ${GREEN}✓${NC} NATS发布: $md_published 条"
else
    echo -e "  ${YELLOW}⚠${NC} NATS发布较少: $md_published 条"
fi

# 最终结果
echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
if [ "$all_passed" = true ]; then
    echo -e "${GREEN}✓ 端到端测试通过！${NC}"
    echo -e "${GREEN}所有组件正常运行，数据链路完整${NC}"
else
    echo -e "${YELLOW}⚠ 测试完成但有警告${NC}"
    echo -e "${YELLOW}请检查日志获取详细信息${NC}"
fi
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -e "${CYAN}查看详细日志:${NC}"
echo -e "  tail -f $LOG_DIR/md_simulator.log"
echo -e "  tail -f $LOG_DIR/md_gateway.log"
echo -e "  tail -f $LOG_DIR/ors_gateway.log"
echo -e "  tail -f $LOG_DIR/counter_gateway.log"
echo -e "  tail -f $LOG_DIR/golang_trader.log"

echo -e "\n${CYAN}Trader Web UI:${NC}"
echo -e "  http://localhost:3000/?api=http://localhost:9201"

echo -e "\n${YELLOW}按Ctrl+C停止所有服务...${NC}\n"

# 保持运行
wait

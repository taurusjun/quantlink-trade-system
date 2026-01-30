#!/bin/bash
# 多策略端到端测试 + 热加载验证
# Multi-Strategy E2E Test with Hot Reload

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$PROJECT_ROOT"

LOG_DIR="$PROJECT_ROOT/test_logs"
mkdir -p "$LOG_DIR"

PID_FILE="$LOG_DIR/e2e_hotreload_pids.txt"
rm -f "$PID_FILE"

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║   多策略端到端测试 + 热加载验证                            ║${NC}"
echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}\n"

# 清理函数
cleanup() {
    echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
    echo -e "${YELLOW}清理测试环境...${NC}"
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
    pkill -9 -f "trader.*hot_reload" 2>/dev/null || true
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
        echo -e "${GREEN}✓ $name 启动成功 (PID: $pid)${NC}\n"
        return 0
    else
        echo -e "${RED}✗ $name 启动失败${NC}"
        return 1
    fi
}

# Step 1: 检查NATS
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 1: 检查NATS服务${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

if ! lsof -i :4222 >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠ NATS未运行，启动中...${NC}"
    nats-server > /dev/null 2>&1 &
    sleep 2
fi
echo -e "${GREEN}✓ NATS服务运行中${NC}\n"

# Step 2: 启动MD Simulator
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 2: 启动行情模拟器${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process "md_simulator" "./gateway/build/md_simulator 100 queue" "$LOG_DIR/md_simulator.log"

# Step 3: 启动MD Gateway
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 3: 启动MD Gateway${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process "md_gateway" "./gateway/build/md_gateway queue" "$LOG_DIR/md_gateway.log"

# Step 4: 启动ORS Gateway
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 4: 启动ORS Gateway${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process "ors_gateway" "./gateway/build/ors_gateway" "$LOG_DIR/ors_gateway.log"

# Step 5: 启动Counter Gateway
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 5: 启动Counter Gateway${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process "counter_gateway" "./gateway/build/counter_gateway" "$LOG_DIR/counter_gateway.log"

# Step 6: 准备Model文件
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 6: 准备Model文件${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

mkdir -p golang/models

cat > golang/models/model.ag_pairwise.txt << 'EOF'
BEGIN_PLACE 2.0
BEGIN_REMOVE 0.5
SIZE 4
MAX_SIZE 16
STOP_LOSS 50000
MAX_LOSS 100000
LOOKBACK_PERIOD 100
EOF
echo -e "${GREEN}✓ 创建 model.ag_pairwise.txt${NC}"

cat > golang/models/model.ag_passive.txt << 'EOF'
BEGIN_PLACE 1.5
BEGIN_REMOVE 0.3
SIZE 2
MAX_SIZE 10
STOP_LOSS 30000
MAX_LOSS 60000
EOF
echo -e "${GREEN}✓ 创建 model.ag_passive.txt${NC}\n"

# Step 7: 启动Trader
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 7: 启动多策略Trader${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

start_process "trader" "./bin/trader -config config/trader.hot_reload.test.yaml" "$LOG_DIR/trader.log"

sleep 3

# Step 8: 验证API
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 8: 验证API和策略状态${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 健康检查${NC}"
curl -s http://localhost:9301/api/v1/health | python3 -m json.tool
echo ""

echo -e "${CYAN}[2] 策略列表${NC}"
curl -s http://localhost:9301/api/v1/strategies | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f\"策略总数: {d['data']['count']}\")
for s in d['data']['strategies']:
    print(f\"  - {s['id']}: {s['type']} (running={s['running']}, active={s['active']})\")
"
echo ""

# Step 9: 激活策略
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 9: 激活策略${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 激活 ag_pairwise${NC}"
curl -s -X POST http://localhost:9301/api/v1/strategies/ag_pairwise/activate | python3 -m json.tool
sleep 2

echo -e "\n${CYAN}[2] 激活 ag_passive${NC}"
curl -s -X POST http://localhost:9301/api/v1/strategies/ag_passive/activate | python3 -m json.tool
sleep 2

# Step 10: 观察运行15秒
echo -e "\n${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 10: 观察策略运行（15秒）${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

for i in {1..3}; do
    echo -e "${BLUE}[观察点 $i/3]${NC}"
    
    md_count=$(grep -c "Pushed:" "$LOG_DIR/md_simulator.log" 2>/dev/null || echo 0)
    echo -e "  行情生成: ${GREEN}$md_count${NC} 条"
    
    if [ -f "$LOG_DIR/ors_gateway.log" ]; then
        order_count=$(grep -c "Received order request" "$LOG_DIR/ors_gateway.log" 2>/dev/null || echo 0)
        echo -e "  订单发送: ${GREEN}$order_count${NC} 条"
    fi
    
    echo ""
    sleep 5
done

# Step 11: 热加载测试 - ag_pairwise
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 11: 热加载测试 - ag_pairwise${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 查看当前参数${NC}"
curl -s http://localhost:9301/api/v1/strategies/ag_pairwise | python3 -c "
import sys, json
d = json.load(sys.stdin)
if 'data' in d:
    print(f\"  策略: {d['data']['id']}\")
    print(f\"  类型: {d['data']['type']}\")
    print(f\"  激活: {d['data']['active']}\")
    print(f\"  运行: {d['data']['running']}\")
"
echo ""

echo -e "${CYAN}[2] 修改Model文件（降低阈值，增大手数）${NC}"
echo -e "  修改前: entry_zscore=2.0, size=4"
echo -e "  修改后: entry_zscore=1.0, size=8"

cat > golang/models/model.ag_pairwise.txt << 'EOF'
BEGIN_PLACE 1.0
BEGIN_REMOVE 0.3
SIZE 8
MAX_SIZE 16
STOP_LOSS 50000
MAX_LOSS 100000
LOOKBACK_PERIOD 100
EOF

echo -e "\n${CYAN}[3] 触发热加载${NC}"
curl -s -X POST http://localhost:9301/api/v1/strategies/ag_pairwise/model/reload | python3 -m json.tool
echo ""

sleep 1

echo -e "${CYAN}[4] 验证参数更新${NC}"
curl -s http://localhost:9301/api/v1/strategies/ag_pairwise/model/status | python3 -m json.tool
echo ""

# Step 12: 热加载测试 - ag_passive  
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 12: 热加载测试 - ag_passive${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 修改Model文件（增大手数）${NC}"
echo -e "  修改前: size=2"
echo -e "  修改后: size=4"

cat > golang/models/model.ag_passive.txt << 'EOF'
BEGIN_PLACE 1.5
BEGIN_REMOVE 0.3
SIZE 4
MAX_SIZE 10
STOP_LOSS 30000
MAX_LOSS 60000
EOF

echo -e "\n${CYAN}[2] 触发热加载${NC}"
curl -s -X POST http://localhost:9301/api/v1/strategies/ag_passive/model/reload | python3 -m json.tool
echo ""

# Step 13: 观察热加载后的运行（15秒）
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 13: 观察热加载后的运行（15秒）${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}观察参数更新后的订单行为...${NC}\n"

for i in {1..3}; do
    echo -e "${BLUE}[观察点 $i/3]${NC}"
    
    if [ -f "$LOG_DIR/ors_gateway.log" ]; then
        order_count=$(grep -c "Received order request" "$LOG_DIR/ors_gateway.log" 2>/dev/null || echo 0)
        echo -e "  总订单数: ${GREEN}$order_count${NC} 条"
        
        # 显示最近3个订单（如果有）
        echo -e "  最近订单:"
        tail -20 "$LOG_DIR/ors_gateway.log" | grep "Received order request" | tail -3 | while read line; do
            echo -e "    ${GREEN}└─${NC} ${line##*order request}"
        done
    fi
    
    echo ""
    sleep 5
done

# Step 14: 最终统计
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 14: 测试结果统计${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}行情数据链路:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

md_generated=$(grep -c "Pushed:" "$LOG_DIR/md_simulator.log" 2>/dev/null || echo 0)
md_published=$(grep "Published.*messages to NATS" "$LOG_DIR/md_gateway.log" 2>/dev/null | tail -1 | grep -oE "[0-9]+" | head -1 || echo 0)

echo -e "  生成行情: ${GREEN}$md_generated${NC} 条"
echo -e "  NATS发布: ${GREEN}$md_published${NC} 条"

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}订单处理链路:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

order_sent=0
order_accepted=0
order_filled=0

if [ -f "$LOG_DIR/ors_gateway.log" ]; then
    order_sent=$(grep -c "Received order request" "$LOG_DIR/ors_gateway.log" 2>/dev/null || echo 0)
fi

if [ -f "$LOG_DIR/counter_gateway.log" ]; then
    order_accepted=$(grep -c "Order accepted" "$LOG_DIR/counter_gateway.log" 2>/dev/null || echo 0)
    order_filled=$(grep -c "Order filled" "$LOG_DIR/counter_gateway.log" 2>/dev/null || echo 0)
fi

echo -e "  发送订单: ${GREEN}$order_sent${NC} 条"
echo -e "  订单接受: ${GREEN}$order_accepted${NC} 条"  
echo -e "  订单成交: ${GREEN}$order_filled${NC} 条"

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}热加载统计:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

reload_count=$(grep -c "Model reloaded\|Parameters updated" "$LOG_DIR/trader.log" 2>/dev/null || echo 0)
echo -e "  热加载次数: ${GREEN}$reload_count${NC}"

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ 多策略端到端测试 + 热加载验证完成！${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -e "${CYAN}查看详细日志:${NC}"
echo -e "  tail -f $LOG_DIR/trader.log | grep -E 'Parameters|Model'"
echo -e "  tail -f $LOG_DIR/ors_gateway.log | grep 'Received order'"

echo -e "\n${YELLOW}按Ctrl+C停止所有服务...${NC}\n"

# 保持运行
wait

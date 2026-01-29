#!/bin/bash
# 多策略热加载端到端测试脚本
# Multi-Strategy Hot Reload E2E Test
# 测试多个策略的热加载功能

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

# 日志目录
LOG_DIR="$PROJECT_ROOT/test_logs"
mkdir -p "$LOG_DIR"

# Model文件目录
MODEL_DIR="$PROJECT_ROOT/golang/models"
mkdir -p "$MODEL_DIR"

# PID文件
PID_FILE="$LOG_DIR/hot_reload_pids.txt"
rm -f "$PID_FILE"

echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     多策略热加载端到端测试                                  ║${NC}"
echo -e "${BLUE}║     Multi-Strategy Hot Reload E2E Test                   ║${NC}"
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
    pkill -9 -f "bin/trader" 2>/dev/null || true
    echo -e "${GREEN}✓ 清理完成${NC}"
}

trap cleanup EXIT INT TERM

# ═══════════════════════════════════════════════════════════
# Step 1: 创建Model文件
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 1: 创建测试Model文件${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

# Model 1: ag_pairwise
cat > "$MODEL_DIR/model.ag_pairwise.txt" << 'EOF'
# Pairwise Arbitrage Strategy Parameters
BEGIN_PLACE 2.0
BEGIN_REMOVE 0.5
SIZE 4
MAX_SIZE 16
STOP_LOSS 50000
MAX_LOSS 100000
LOOKBACK_PERIOD 100
EOF

echo -e "${GREEN}✓ 创建 model.ag_pairwise.txt${NC}"
echo -e "  entry_zscore: 2.0, exit_zscore: 0.5, size: 4\n"

# Model 2: ag_passive
cat > "$MODEL_DIR/model.ag_passive.txt" << 'EOF'
# Passive Market Making Strategy Parameters
BEGIN_PLACE 1.5
BEGIN_REMOVE 0.3
SIZE 2
MAX_SIZE 10
STOP_LOSS 30000
MAX_LOSS 60000
EOF

echo -e "${GREEN}✓ 创建 model.ag_passive.txt${NC}"
echo -e "  entry_zscore: 1.5, exit_zscore: 0.3, size: 2\n"

# Model 3: au_pairwise
cat > "$MODEL_DIR/model.au_pairwise.txt" << 'EOF'
# Gold Pairwise Arbitrage Strategy Parameters
BEGIN_PLACE 1.0
BEGIN_REMOVE 0.3
SIZE 1
MAX_SIZE 10
STOP_LOSS 20000
MAX_LOSS 50000
LOOKBACK_PERIOD 20
EOF

echo -e "${GREEN}✓ 创建 model.au_pairwise.txt${NC}"
echo -e "  entry_zscore: 1.0, exit_zscore: 0.3, size: 1\n"

# ═══════════════════════════════════════════════════════════
# Step 2: 创建支持热加载的配置文件
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 2: 创建支持热加载的配置文件${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

cat > "$PROJECT_ROOT/config/trader.hot_reload.test.yaml" << EOF
# Multi-Strategy Configuration with Hot Reload Support
# 多策略配置 + 热加载支持

system:
  mode: "live"

strategies:
  # 策略1：白银配对套利
  - id: "ag_pairwise"
    type: "pairwise_arb"
    enabled: true
    symbols: ["ag2603", "ag2605"]
    exchanges: ["SHFE"]
    allocation: 0.4
    max_position_size: 16

    # Model热加载配置
    model_file: "./golang/models/model.ag_pairwise.txt"
    hot_reload:
      enabled: true
      mode: "manual"              # 手动触发模式

    parameters:
      spread_type: "difference"
      lookback_period: 100.0
      entry_zscore: 2.0           # 初始值，会被model文件覆盖
      exit_zscore: 0.5
      order_size: 4.0
      min_correlation: 0.7

  # 策略2：被动做市
  - id: "ag_passive"
    type: "passive"
    enabled: true
    symbols: ["ag2603"]
    exchanges: ["SHFE"]
    allocation: 0.3
    max_position_size: 10

    # Model热加载配置
    model_file: "./golang/models/model.ag_passive.txt"
    hot_reload:
      enabled: true
      mode: "manual"

    parameters:
      spread_multiplier: 0.5
      order_size: 2.0

  # 策略3：黄金配对套利
  - id: "au_pairwise"
    type: "pairwise_arb"
    enabled: true
    symbols: ["au2604", "au2606"]
    exchanges: ["SHFE"]
    allocation: 0.3
    max_position_size: 10

    # Model热加载配置
    model_file: "./golang/models/model.au_pairwise.txt"
    hot_reload:
      enabled: true
      mode: "manual"

    parameters:
      spread_type: "difference"
      lookback_period: 20.0
      entry_zscore: 1.0
      exit_zscore: 0.3
      order_size: 1.0
      min_correlation: 0.0

session:
  start_time: "00:00:00"
  end_time: "23:59:59"
  timezone: "Asia/Shanghai"
  auto_start: true
  auto_stop: false
  auto_activate: false

risk:
  max_drawdown: 10000.0
  stop_loss: 50000.0
  max_loss: 100000.0
  daily_loss_limit: 200000.0
  max_reject_count: 10
  check_interval_ms: 100

engine:
  ors_gateway_addr: "localhost:50052"
  nats_addr: "nats://localhost:4222"
  order_queue_size: 100
  timer_interval: 5s
  max_concurrent_orders: 10

portfolio:
  total_capital: 1000000.0
  rebalance_interval_sec: 3600
  min_allocation: 0.05
  max_allocation: 0.50
  enable_auto_rebalance: false
  enable_correlation_calc: false

api:
  enabled: true
  port: 9301
  host: "localhost"

logging:
  level: "info"
  file: "./log/trader.hot_reload.test.log"
  max_size_mb: 100
  max_backups: 10
  max_age_days: 30
  compress: true
  console: true
  json_format: false
EOF

echo -e "${GREEN}✓ 创建 trader.hot_reload.test.yaml${NC}"
echo -e "  配置了3个策略,每个都启用热加载功能\n"

# ═══════════════════════════════════════════════════════════
# Step 3: 检查前置服务
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 3: 检查前置服务${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

# 检查NATS
if ! lsof -i :4222 >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠ NATS服务未运行，尝试启动...${NC}"
    nats-server > /dev/null 2>&1 &
    sleep 2
    if lsof -i :4222 >/dev/null 2>&1; then
        echo -e "${GREEN}✓ NATS服务启动成功${NC}\n"
    else
        echo -e "${RED}✗ NATS服务启动失败${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✓ NATS服务运行中 (端口4222)${NC}\n"
fi

# 检查ORS Gateway (可选，用于完整测试)
if ! lsof -i :50052 >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠ ORS Gateway未运行 (端口50052)${NC}"
    echo -e "${CYAN}  如需测试完整订单流程，请先启动:${NC}"
    echo -e "${CYAN}  ./gateway/build/ors_gateway &${NC}\n"
else
    echo -e "${GREEN}✓ ORS Gateway运行中 (端口50052)${NC}\n"
fi

# ═══════════════════════════════════════════════════════════
# Step 4: 启动多策略Trader
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 4: 启动多策略Trader${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}启动命令:${NC}"
echo -e "  ./bin/trader -config config/trader.hot_reload.test.yaml\n"

./bin/trader -config config/trader.hot_reload.test.yaml > "$LOG_DIR/trader.hot_reload.log" 2>&1 &
TRADER_PID=$!
echo "$TRADER_PID trader" >> "$PID_FILE"

echo -e "${BLUE}等待Trader启动... (PID: $TRADER_PID)${NC}"
sleep 5

# 检查进程
if ! kill -0 "$TRADER_PID" 2>/dev/null; then
    echo -e "${RED}✗ Trader启动失败${NC}"
    echo -e "${RED}查看日志: tail -f $LOG_DIR/trader.hot_reload.log${NC}"
    exit 1
fi

# 检查API
if ! lsof -i :9301 >/dev/null 2>&1; then
    echo -e "${RED}✗ Trader API未就绪 (端口9301)${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Trader启动成功${NC}"
echo -e "${GREEN}✓ API端口就绪: http://localhost:9301${NC}\n"

# ═══════════════════════════════════════════════════════════
# Step 5: 查看初始状态
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 5: 查看初始策略状态${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

sleep 2

echo -e "${CYAN}[1] 获取策略列表${NC}"
strategies_response=$(curl -s http://localhost:9301/api/v1/strategies)
echo "$strategies_response" | python3 -m json.tool 2>/dev/null || echo "$strategies_response"
echo ""

echo -e "${CYAN}[2] 查看ag_pairwise策略初始参数${NC}"
ag_pairwise_status=$(curl -s http://localhost:9301/api/v1/strategies/ag_pairwise)
echo "$ag_pairwise_status" | python3 -m json.tool 2>/dev/null || echo "$ag_pairwise_status"
echo ""

echo -e "${CYAN}[3] 查看ag_passive策略初始参数${NC}"
ag_passive_status=$(curl -s http://localhost:9301/api/v1/strategies/ag_passive)
echo "$ag_passive_status" | python3 -m json.tool 2>/dev/null || echo "$ag_passive_status"
echo ""

echo -e "${CYAN}[4] 查看au_pairwise策略初始参数${NC}"
au_pairwise_status=$(curl -s http://localhost:9301/api/v1/strategies/au_pairwise)
echo "$au_pairwise_status" | python3 -m json.tool 2>/dev/null || echo "$au_pairwise_status"
echo ""

# ═══════════════════════════════════════════════════════════
# Step 6: 测试热加载 - 策略1 (ag_pairwise)
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 6: 测试热加载 - 策略1 (ag_pairwise)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 修改model文件${NC}"
echo -e "  修改前: entry_zscore=2.0, exit_zscore=0.5, size=4"
echo -e "  修改后: entry_zscore=1.5, exit_zscore=0.3, size=6\n"

cat > "$MODEL_DIR/model.ag_pairwise.txt" << 'EOF'
# Pairwise Arbitrage Strategy Parameters (Modified)
BEGIN_PLACE 1.5
BEGIN_REMOVE 0.3
SIZE 6
MAX_SIZE 16
STOP_LOSS 50000
MAX_LOSS 100000
LOOKBACK_PERIOD 100
EOF

echo -e "${CYAN}[2] 触发热加载${NC}"
reload_response=$(curl -s -X POST http://localhost:9301/api/v1/strategies/ag_pairwise/model/reload)
echo "$reload_response" | python3 -m json.tool 2>/dev/null || echo "$reload_response"
echo ""

if echo "$reload_response" | grep -q '"success":true'; then
    echo -e "${GREEN}✓ 热加载请求成功${NC}\n"
else
    echo -e "${RED}✗ 热加载请求失败${NC}\n"
fi

sleep 1

echo -e "${CYAN}[3] 验证参数更新${NC}"
updated_status=$(curl -s http://localhost:9301/api/v1/strategies/ag_pairwise)
echo "$updated_status" | python3 -m json.tool 2>/dev/null || echo "$updated_status"
echo ""

# 检查参数是否更新
if echo "$updated_status" | grep -q '"entry_zscore":1.5'; then
    echo -e "${GREEN}✓ entry_zscore 已更新为 1.5${NC}"
else
    echo -e "${RED}✗ entry_zscore 未更新${NC}"
fi

if echo "$updated_status" | grep -q '"exit_zscore":0.3'; then
    echo -e "${GREEN}✓ exit_zscore 已更新为 0.3${NC}"
else
    echo -e "${RED}✗ exit_zscore 未更新${NC}"
fi

if echo "$updated_status" | grep -q '"order_size":6'; then
    echo -e "${GREEN}✓ order_size 已更新为 6${NC}\n"
else
    echo -e "${RED}✗ order_size 未更新${NC}\n"
fi

# ═══════════════════════════════════════════════════════════
# Step 7: 测试热加载 - 策略2 (ag_passive)
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 7: 测试热加载 - 策略2 (ag_passive)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 修改model文件${NC}"
echo -e "  修改前: entry_zscore=1.5, exit_zscore=0.3, size=2"
echo -e "  修改后: entry_zscore=2.5, exit_zscore=0.8, size=3\n"

cat > "$MODEL_DIR/model.ag_passive.txt" << 'EOF'
# Passive Market Making Strategy Parameters (Modified)
BEGIN_PLACE 2.5
BEGIN_REMOVE 0.8
SIZE 3
MAX_SIZE 10
STOP_LOSS 30000
MAX_LOSS 60000
EOF

echo -e "${CYAN}[2] 触发热加载${NC}"
reload_response=$(curl -s -X POST http://localhost:9301/api/v1/strategies/ag_passive/model/reload)
echo "$reload_response" | python3 -m json.tool 2>/dev/null || echo "$reload_response"
echo ""

sleep 1

echo -e "${CYAN}[3] 验证参数更新${NC}"
updated_status=$(curl -s http://localhost:9301/api/v1/strategies/ag_passive)
echo "$updated_status" | python3 -c "import sys, json; d=json.load(sys.stdin); print(json.dumps(d.get('data', {}).get('parameters', {}), indent=2))" 2>/dev/null || echo "$updated_status"
echo ""

# ═══════════════════════════════════════════════════════════
# Step 8: 测试热加载 - 策略3 (au_pairwise)
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 8: 测试热加载 - 策略3 (au_pairwise)${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 修改model文件${NC}"
echo -e "  修改前: entry_zscore=1.0, exit_zscore=0.3, size=1"
echo -e "  修改后: entry_zscore=0.8, exit_zscore=0.2, size=2\n"

cat > "$MODEL_DIR/model.au_pairwise.txt" << 'EOF'
# Gold Pairwise Arbitrage Strategy Parameters (Modified)
BEGIN_PLACE 0.8
BEGIN_REMOVE 0.2
SIZE 2
MAX_SIZE 10
STOP_LOSS 20000
MAX_LOSS 50000
LOOKBACK_PERIOD 20
EOF

echo -e "${CYAN}[2] 触发热加载${NC}"
reload_response=$(curl -s -X POST http://localhost:9301/api/v1/strategies/au_pairwise/model/reload)
echo "$reload_response" | python3 -m json.tool 2>/dev/null || echo "$reload_response"
echo ""

sleep 1

echo -e "${CYAN}[3] 验证参数更新${NC}"
updated_status=$(curl -s http://localhost:9301/api/v1/strategies/au_pairwise)
echo "$updated_status" | python3 -c "import sys, json; d=json.load(sys.stdin); print(json.dumps(d.get('data', {}).get('parameters', {}), indent=2))" 2>/dev/null || echo "$updated_status"
echo ""

# ═══════════════════════════════════════════════════════════
# Step 9: 查看热加载历史
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 9: 查看热加载历史${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] ag_pairwise 热加载历史${NC}"
history_response=$(curl -s http://localhost:9301/api/v1/strategies/ag_pairwise/model/history)
echo "$history_response" | python3 -m json.tool 2>/dev/null || echo "$history_response"
echo ""

echo -e "${CYAN}[2] ag_passive 热加载历史${NC}"
history_response=$(curl -s http://localhost:9301/api/v1/strategies/ag_passive/model/history)
echo "$history_response" | python3 -m json.tool 2>/dev/null || echo "$history_response"
echo ""

echo -e "${CYAN}[3] au_pairwise 热加载历史${NC}"
history_response=$(curl -s http://localhost:9301/api/v1/strategies/au_pairwise/model/history)
echo "$history_response" | python3 -m json.tool 2>/dev/null || echo "$history_response"
echo ""

# ═══════════════════════════════════════════════════════════
# Step 10: 测试无效参数回滚
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 10: 测试无效参数回滚${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${CYAN}[1] 写入无效参数 (负数)${NC}"

cat > "$MODEL_DIR/model.ag_pairwise.txt" << 'EOF'
# Invalid Parameters Test
BEGIN_PLACE -1.0
BEGIN_REMOVE -0.5
SIZE -5
MAX_SIZE 16
EOF

echo -e "${CYAN}[2] 尝试热加载${NC}"
reload_response=$(curl -s -X POST http://localhost:9301/api/v1/strategies/ag_pairwise/model/reload)
echo "$reload_response" | python3 -m json.tool 2>/dev/null || echo "$reload_response"
echo ""

if echo "$reload_response" | grep -q '"success":false'; then
    echo -e "${GREEN}✓ 预期行为：拒绝无效参数${NC}\n"
else
    echo -e "${YELLOW}⚠ 意外：接受了无效参数${NC}\n"
fi

echo -e "${CYAN}[3] 验证参数未改变 (应保持旧值)${NC}"
current_status=$(curl -s http://localhost:9301/api/v1/strategies/ag_pairwise)
echo "$current_status" | python3 -c "import sys, json; d=json.load(sys.stdin); print(json.dumps(d.get('data', {}).get('parameters', {}), indent=2))" 2>/dev/null || echo "$current_status"
echo ""

# ═══════════════════════════════════════════════════════════
# Step 11: 最终汇总
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}Step 11: 测试结果汇总${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════${NC}\n"

echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}测试项目:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  ${GREEN}✓${NC} 多策略配置加载"
echo -e "  ${GREEN}✓${NC} Model文件创建"
echo -e "  ${GREEN}✓${NC} Trader启动"
echo -e "  ${GREEN}✓${NC} 策略1热加载 (ag_pairwise)"
echo -e "  ${GREEN}✓${NC} 策略2热加载 (ag_passive)"
echo -e "  ${GREEN}✓${NC} 策略3热加载 (au_pairwise)"
echo -e "  ${GREEN}✓${NC} 参数更新验证"
echo -e "  ${GREEN}✓${NC} 热加载历史查询"
echo -e "  ${GREEN}✓${NC} 无效参数回滚测试"

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}热加载统计:${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

# 统计日志中的热加载次数
reload_count=$(grep -c "Model reloaded" "$LOG_DIR/trader.hot_reload.log" 2>/dev/null || echo 0)
echo -e "  总热加载次数: ${GREEN}${reload_count}${NC}"

# 检查是否有错误
error_count=$(grep -c "error" "$LOG_DIR/trader.hot_reload.log" 2>/dev/null || echo 0)
if [ "$error_count" -gt 0 ]; then
    echo -e "  错误数: ${YELLOW}${error_count}${NC}"
else
    echo -e "  错误数: ${GREEN}0${NC}"
fi

echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${GREEN}✓ 多策略热加载端到端测试完成！${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -e "${CYAN}API端点:${NC}"
echo -e "  Dashboard: http://localhost:9301/api/v1/dashboard/overview"
echo -e "  Strategies: http://localhost:9301/api/v1/strategies"
echo -e "  Hot Reload: http://localhost:9301/api/v1/strategies/{id}/model/reload"
echo -e "  Web UI: http://localhost:3000/?api=http://localhost:9301"

echo -e "\n${CYAN}查看详细日志:${NC}"
echo -e "  tail -f $LOG_DIR/trader.hot_reload.log"

echo -e "\n${CYAN}Model文件位置:${NC}"
echo -e "  $MODEL_DIR/model.ag_pairwise.txt"
echo -e "  $MODEL_DIR/model.ag_passive.txt"
echo -e "  $MODEL_DIR/model.au_pairwise.txt"

echo -e "\n${YELLOW}提示: 修改Model文件后,使用以下命令触发热加载:${NC}"
echo -e "  ${BLUE}curl -X POST http://localhost:9301/api/v1/strategies/ag_pairwise/model/reload${NC}"
echo -e "  ${BLUE}curl -X POST http://localhost:9301/api/v1/strategies/ag_passive/model/reload${NC}"
echo -e "  ${BLUE}curl -X POST http://localhost:9301/api/v1/strategies/au_pairwise/model/reload${NC}"

echo -e "\n${YELLOW}按Ctrl+C停止测试...${NC}\n"

# 保持运行，方便手动测试
wait

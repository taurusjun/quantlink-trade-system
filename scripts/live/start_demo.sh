#!/bin/bash
# ============================================
# 脚本名称: start_demo.sh
# 用途: 快速启动模拟交易系统（演示模式）
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - @docs/核心文档/QUICKSTART.md
#   - @docs/实盘/订单回报链路修复报告_2026-01-30-16_59.md
# ============================================

set -e

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 配置
LOG_DIR="log"
CONFIG_FILE="config/trader.demo.yaml"
POSITION_DIR="data/positions"

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

# 检查函数
check_binary() {
    local name=$1
    local path=$2
    if [ ! -f "$path" ]; then
        log_error "$name not found at $path"
        log_error "Please run: cd gateway/build && cmake .. && make"
        exit 1
    fi
}

# 清理函数
cleanup() {
    log_warn "Caught signal, stopping services..."
    ./scripts/live/stop_demo.sh
    exit 0
}

trap cleanup EXIT INT TERM

# ============================================
# 主逻辑
# ============================================

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║  QuantlinkTrader - Demo Mode                              ║"
echo "║  模拟交易系统快速启动                                     ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

# 0. 预检查
log_step "[0/7] Pre-flight checks..."
mkdir -p "$LOG_DIR"
mkdir -p "$POSITION_DIR"
check_binary "md_simulator" "gateway/build/md_simulator"
check_binary "md_gateway" "gateway/build/md_gateway"
check_binary "ors_gateway" "gateway/build/ors_gateway"
check_binary "counter_bridge" "gateway/build/counter_bridge"
check_binary "trader" "bin/trader"
log_info "✓ All binaries found"

if [ ! -f "$CONFIG_FILE" ]; then
    log_error "Config file not found: $CONFIG_FILE"
    exit 1
fi
log_info "✓ Config file found"
echo ""

# 1. 清理旧进程和数据
log_step "[1/7] Cleaning up old processes and data..."
pkill -9 -f "nats-server" 2>/dev/null || true
pkill -9 -f "md_simulator" 2>/dev/null || true
pkill -9 -f "md_gateway" 2>/dev/null || true
pkill -9 -f "ors_gateway" 2>/dev/null || true
pkill -9 -f "counter_bridge.*simulator" 2>/dev/null || true
pkill -9 -f "trader.*demo" 2>/dev/null || true
sleep 1
log_info "✓ Old processes cleaned"

# 清理历史持仓数据
if [ -d "$POSITION_DIR" ]; then
    POSITION_COUNT=$(find "$POSITION_DIR" -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
    if [ "$POSITION_COUNT" -gt 0 ]; then
        log_warn "Found $POSITION_COUNT position snapshot files"
        rm -f "$POSITION_DIR"/*.json 2>/dev/null || true
        log_info "✓ Historical positions cleaned"
    else
        log_info "✓ No historical positions to clean"
    fi
fi

# 清理共享内存
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs ipcrm -m 2>/dev/null || true
log_info "✓ Shared memory cleaned"
echo ""

# 2. 启动 NATS
log_step "[2/7] Starting NATS server..."
nats-server > "$LOG_DIR/nats.log" 2>&1 &
NATS_PID=$!
sleep 2
if ! ps -p $NATS_PID > /dev/null 2>&1; then
    log_error "NATS failed to start"
    tail -10 "$LOG_DIR/nats.log"
    exit 1
fi
log_info "✓ NATS started (PID: $NATS_PID)"
echo ""

# 3. 启动行情组件
log_step "[3/7] Starting market data components..."
./gateway/build/md_simulator > "$LOG_DIR/md_simulator.log" 2>&1 &
MD_SIM_PID=$!
sleep 1
./gateway/build/md_gateway > "$LOG_DIR/md_gateway.log" 2>&1 &
MD_GW_PID=$!
sleep 1
log_info "✓ md_simulator started (PID: $MD_SIM_PID)"
log_info "✓ md_gateway started (PID: $MD_GW_PID)"
echo ""

# 4. 启动订单路由
log_step "[4/7] Starting order routing..."
./gateway/build/ors_gateway > "$LOG_DIR/ors_gateway.log" 2>&1 &
ORS_PID=$!
sleep 2
if ! ps -p $ORS_PID > /dev/null 2>&1; then
    log_error "ORS Gateway failed to start"
    tail -10 "$LOG_DIR/ors_gateway.log"
    exit 1
fi
log_info "✓ ors_gateway started (PID: $ORS_PID)"
echo ""

# 5. 启动模拟成交
log_step "[5/7] Starting simulator (counter bridge)..."
./gateway/build/counter_bridge simulator:config/simulator/simulator.yaml > "$LOG_DIR/counter_bridge.log" 2>&1 &
CB_PID=$!
sleep 2
if ! ps -p $CB_PID > /dev/null 2>&1; then
    log_error "Counter Bridge failed to start"
    tail -10 "$LOG_DIR/counter_bridge.log"
    exit 1
fi
log_info "✓ counter_bridge started (PID: $CB_PID)"
echo ""

# 6. 启动 Trader
log_step "[6/7] Starting Trader..."
./bin/trader -config "$CONFIG_FILE" > "$LOG_DIR/trader.demo.log" 2>&1 &
TRADER_PID=$!
sleep 3
if ! ps -p $TRADER_PID > /dev/null 2>&1; then
    log_error "Trader failed to start"
    tail -20 "$LOG_DIR/trader.demo.log"
    exit 1
fi
log_info "✓ Trader started (PID: $TRADER_PID)"
echo ""

# 7. 等待系统初始化
log_step "[7/7] Waiting for system initialization..."
sleep 2

# 检查所有进程
echo ""
log_info "═══════════════════════════════════════════════════════════"
log_info "System Status"
log_info "═══════════════════════════════════════════════════════════"
ps -p $NATS_PID > /dev/null 2>&1 && echo "  ✓ nats-server      (PID: $NATS_PID)" || echo "  ✗ nats-server"
ps -p $MD_SIM_PID > /dev/null 2>&1 && echo "  ✓ md_simulator     (PID: $MD_SIM_PID)" || echo "  ✗ md_simulator"
ps -p $MD_GW_PID > /dev/null 2>&1 && echo "  ✓ md_gateway       (PID: $MD_GW_PID)" || echo "  ✗ md_gateway"
ps -p $ORS_PID > /dev/null 2>&1 && echo "  ✓ ors_gateway      (PID: $ORS_PID)" || echo "  ✗ ors_gateway"
ps -p $CB_PID > /dev/null 2>&1 && echo "  ✓ counter_bridge   (PID: $CB_PID)" || echo "  ✗ counter_bridge"
ps -p $TRADER_PID > /dev/null 2>&1 && echo "  ✓ trader           (PID: $TRADER_PID)" || echo "  ✗ trader"
echo ""

# 显示 Dashboard 和 API 信息
log_info "═══════════════════════════════════════════════════════════"
log_info "Access Information"
log_info "═══════════════════════════════════════════════════════════"
echo "  📊 Dashboard:  http://localhost:9201/dashboard"
echo "  🔌 API:        http://localhost:9201/api/v1/"
echo "  📝 Logs:       tail -f $LOG_DIR/trader.demo.log"
echo ""

# 激活策略提示
log_info "═══════════════════════════════════════════════════════════"
log_info "Next Steps"
log_info "═══════════════════════════════════════════════════════════"
echo "  1. 查看策略状态:"
echo "     curl http://localhost:9201/api/v1/strategy/status | jq ."
echo ""
echo "  2. 激活策略:"
echo "     curl -X POST http://localhost:9201/api/v1/strategy/activate"
echo ""
echo "  3. 查看实时日志:"
echo "     tail -f $LOG_DIR/trader.demo.log | grep -E 'Order|Trade|Signal'"
echo ""
echo "  4. 停止系统:"
echo "     ./scripts/live/stop_demo.sh"
echo ""

log_info "═══════════════════════════════════════════════════════════"
log_info "✓ System started successfully!"
log_info "═══════════════════════════════════════════════════════════"
echo ""
echo "Press Ctrl+C to stop..."

# 保持运行
trap - EXIT
wait

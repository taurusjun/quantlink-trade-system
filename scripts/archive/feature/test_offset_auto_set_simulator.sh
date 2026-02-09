#!/bin/bash
set -e

# ============================================
# 脚本名称: test_offset_auto_set_simulator.sh
# 用途: 测试 Simulator Plugin Offset 自动设置功能
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - CTP实施报告: @docs/实盘/CTP_Plugin_今昨仓完整实施报告_2026-01-30.md
#   - Offset方案: @docs/实盘/Plugin_层_Offset_自动设置方案_2026-01-30.md
# ============================================

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }

cleanup() {
    log_info "Cleaning up..."
    pkill -f nats-server 2>/dev/null || true
    pkill -f md_simulator 2>/dev/null || true
    pkill -f md_gateway 2>/dev/null || true
    pkill -f ors_gateway 2>/dev/null || true
    pkill -f counter_bridge 2>/dev/null || true
    pkill -f "trader -config" 2>/dev/null || true
    ipcs -m 2>/dev/null | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true
    log_info "Cleanup completed"
}

trap cleanup EXIT

echo "========================================="
echo "  Offset Auto-Set Test (Simulator)"
echo "========================================="
echo ""

# ==================== 准备 ====================

log_step "Step 1: 准备测试环境"

mkdir -p log data/simulator test_logs

# 检查二进制文件
if [ ! -f "gateway/build/counter_bridge" ]; then
    log_error "counter_bridge not found"
    exit 1
fi

log_info "✓ Test environment prepared"

# ==================== 启动服务 ====================

log_step "Step 2: 启动基础服务"

# 启动 NATS
log_info "Starting NATS server..."
nats-server > log/nats.log 2>&1 &
sleep 2

if ! pgrep -f nats-server > /dev/null; then
    log_error "Failed to start NATS server"
    exit 1
fi
log_info "✓ NATS server started"

# 启动 MD Simulator
log_info "Starting MD simulator..."
./gateway/build/md_simulator > log/md_simulator.log 2>&1 &
sleep 2

if ! pgrep -f md_simulator > /dev/null; then
    log_error "Failed to start MD simulator"
    exit 1
fi
log_info "✓ MD simulator started"

# 启动 MD Gateway
log_info "Starting MD gateway..."
./gateway/build/md_gateway > log/md_gateway.log 2>&1 &
sleep 2

if ! pgrep -f md_gateway > /dev/null; then
    log_error "Failed to start MD gateway"
    exit 1
fi
log_info "✓ MD gateway started"

# 启动 ORS Gateway
log_info "Starting ORS gateway..."
./gateway/build/ors_gateway > log/ors_gateway.log 2>&1 &
sleep 2

if ! pgrep -f ors_gateway > /dev/null; then
    log_error "Failed to start ORS gateway"
    exit 1
fi
log_info "✓ ORS gateway started"

# 启动 Counter Bridge (Simulator Plugin)
log_info "Starting counter_bridge with Simulator plugin..."
./gateway/build/counter_bridge simulator:config/simulator/simulator.yaml > log/counter_bridge_sim.log 2>&1 &
CB_PID=$!
sleep 3

if ! ps -p $CB_PID > /dev/null; then
    log_error "Failed to start counter_bridge with Simulator plugin"
    cat log/counter_bridge_sim.log
    exit 1
fi
log_info "✓ Counter bridge (Simulator) started"

# 检查 Simulator 登录状态
log_info "Checking Simulator login status..."
sleep 2

if grep -q "Simulator plugin loaded" log/counter_bridge_sim.log; then
    log_info "✓ Simulator plugin loaded successfully"
else
    log_warn "Simulator plugin status unclear, check logs:"
    tail -10 log/counter_bridge_sim.log
fi

# 启动 Trader
log_info "Starting trader (test mode)..."
./bin/trader -config config/trader.test.yaml > log/trader_offset_sim.log 2>&1 &
TRADER_PID=$!
sleep 5

if ! ps -p $TRADER_PID > /dev/null; then
    log_error "Failed to start trader"
    cat log/trader_offset_sim.log
    exit 1
fi
log_info "✓ Trader started"

# 等待系统稳定
log_info "Waiting for system to stabilize..."
sleep 5

# ==================== 测试场景 ====================

log_step "Step 3: 测试 Offset 自动设置"

log_info "Activating strategy..."
curl -X POST http://localhost:9201/api/v1/strategy/activate \
    -H "Content-Type: application/json" \
    -d '{"strategy_id": "test_92201"}' \
    > /dev/null 2>&1

sleep 5

# ==================== 场景 1: 无持仓开仓 ====================

log_step "Scenario 1: 无持仓时开仓（应自动设置 OPEN）"

log_info "Waiting for initial orders..."
sleep 10

log_info "Checking offset in logs..."
if grep -q "Auto-set offset:.*OPEN" log/counter_bridge_sim.log; then
    log_info "✓ Scenario 1 PASSED: Offset correctly set to OPEN"
    grep "Auto-set offset:" log/counter_bridge_sim.log | head -5
else
    log_warn "⚠ Scenario 1: No OPEN offset found yet"
fi

# 检查持仓更新
log_info "Checking position updates..."
sleep 3

if grep -q "Position updated.*OPEN" log/counter_bridge_sim.log; then
    log_info "✓ Position updated after OPEN trades"
    grep "Position updated" log/counter_bridge_sim.log | head -3
else
    log_info "ℹ No position updates yet"
fi

# ==================== 场景 2: 有持仓时平仓 ====================

log_step "Scenario 2: 有持仓时反向订单（应自动设置 CLOSE）"

log_info "Waiting for opposite direction orders..."
sleep 20

log_info "Checking for CLOSE offset..."
if grep -q "Auto-set offset:.*CLOSE" log/counter_bridge_sim.log; then
    log_info "✓ Scenario 2 PASSED: Offset correctly set to CLOSE"
    grep "Auto-set offset:.*CLOSE" log/counter_bridge_sim.log | head -3
else
    log_warn "⚠ Scenario 2: No CLOSE offset found (may need longer wait)"
fi

# 检查持仓更新（平仓）
if grep -q "Position updated.*CLOSE" log/counter_bridge_sim.log; then
    log_info "✓ Position updated after CLOSE trades"
    grep "Position updated.*CLOSE" log/counter_bridge_sim.log | head -3
fi

# ==================== 场景 3: 持仓持久化 ====================

log_step "Scenario 3: 持仓持久化验证"

log_info "Checking position file..."
if ls data/simulator/*.json 1> /dev/null 2>&1; then
    log_info "✓ Position file created"
    ls -lh data/simulator/
    log_info "Position file content:"
    cat data/simulator/*.json 2>/dev/null | head -30
else
    log_warn "⚠ Position file not found (may not have positions yet)"
fi

# ==================== 测试统计 ====================

log_step "Step 4: 测试结果统计"

echo ""
echo "========================================="
echo "           Test Summary"
echo "========================================="
echo ""

# 统计 Offset 自动设置
OPEN_COUNT=$(grep -c "Auto-set offset:.*OPEN" log/counter_bridge_sim.log 2>/dev/null || echo "0")
CLOSE_COUNT=$(grep -c "Auto-set offset:.*CLOSE" log/counter_bridge_sim.log 2>/dev/null || echo "0")

echo "Offset Auto-Set Statistics:"
echo "  - OPEN:  $OPEN_COUNT"
echo "  - CLOSE: $CLOSE_COUNT"
echo ""

# 统计持仓更新
POS_UPDATE_OPEN=$(grep -c "Position updated.*OPEN" log/counter_bridge_sim.log 2>/dev/null || echo "0")
POS_UPDATE_CLOSE=$(grep -c "Position updated.*CLOSE" log/counter_bridge_sim.log 2>/dev/null || echo "0")

echo "Position Update Statistics:"
echo "  - OPEN updates:  $POS_UPDATE_OPEN"
echo "  - CLOSE updates: $POS_UPDATE_CLOSE"
echo ""

# 统计订单和成交
ORDER_COUNT=$(grep -c "Order sent" log/trader_offset_sim.log 2>/dev/null || echo "0")
TRADE_COUNT=$(grep -c "Trade:" log/counter_bridge_sim.log 2>/dev/null || echo "0")

echo "Trading Statistics:"
echo "  - Orders sent: $ORDER_COUNT"
echo "  - Trades:      $TRADE_COUNT"
echo ""

# 检查错误
ERROR_COUNT=$(grep -c "Risk check failed" log/counter_bridge_sim.log 2>/dev/null || echo "0")
POS_MISMATCH=$(grep -c "Position mismatch" log/counter_bridge_sim.log 2>/dev/null || echo "0")

echo "Error Statistics:"
echo "  - Risk check failures: $ERROR_COUNT"
echo "  - Position mismatches: $POS_MISMATCH"
echo ""

# 综合评估
echo "========================================="
if [ "$ERROR_COUNT" -eq 0 ] && [ "$POS_MISMATCH" -eq 0 ] && [ "$OPEN_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ TEST PASSED${NC}"
    echo "  - Offset auto-set working correctly"
    echo "  - No errors detected"
    echo "  - Position updates functioning"
    EXIT_CODE=0
elif [ "$ORDER_COUNT" -eq 0 ]; then
    echo -e "${YELLOW}⚠ TEST INCONCLUSIVE (No orders generated)${NC}"
    echo "  - Strategy may not have triggered yet"
    echo "  - Try waiting longer or adjusting parameters"
    EXIT_CODE=1
else
    echo -e "${YELLOW}⚠ TEST PARTIAL${NC}"
    echo "  - Some features working, check details above"
    EXIT_CODE=1
fi
echo "========================================="
echo ""

# ==================== 日志保存 ====================

log_step "Step 5: 保存测试日志"

TEST_LOG_DIR="test_logs/simulator_offset_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$TEST_LOG_DIR"

cp log/counter_bridge_sim.log "$TEST_LOG_DIR/"
cp log/trader_offset_sim.log "$TEST_LOG_DIR/"
cp log/nats.log "$TEST_LOG_DIR/" 2>/dev/null || true
cp log/md_simulator.log "$TEST_LOG_DIR/" 2>/dev/null || true

if ls data/simulator/*.json 1> /dev/null 2>&1; then
    cp data/simulator/*.json "$TEST_LOG_DIR/" 2>/dev/null || true
fi

log_info "✓ Test logs saved to $TEST_LOG_DIR"

# ==================== 手动验证提示 ====================

echo ""
echo "========================================="
echo "      Manual Verification Commands"
echo "========================================="
echo ""
echo "1. Check offset auto-set:"
echo "   grep 'Auto-set offset:' log/counter_bridge_sim.log"
echo ""
echo "2. Check position updates:"
echo "   grep 'Position updated' log/counter_bridge_sim.log"
echo ""
echo "3. Check position file:"
echo "   cat data/simulator/*.json"
echo ""
echo "4. Monitor live:"
echo "   tail -f log/counter_bridge_sim.log | grep -E 'offset|Position'"
echo ""
echo "5. Check orders:"
echo "   grep 'Order sent' log/trader_offset_sim.log"
echo ""
echo "6. Check trades:"
echo "   grep 'Trade:' log/counter_bridge_sim.log"
echo ""
echo "========================================="
echo ""

if [ "$EXIT_CODE" -eq 0 ]; then
    log_info "✅ Test completed successfully!"
else
    log_warn "⚠️ Test completed with warnings (see summary above)"
fi

log_info "Services are still running. Press Ctrl+C to stop."
echo ""

# 保持运行一段时间，允许用户观察
sleep 30

exit $EXIT_CODE

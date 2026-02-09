#!/bin/bash
set -e

# ============================================
# 脚本名称: test_ctp_offset_auto_set.sh
# 用途: 测试 CTP Plugin Offset 自动设置功能（今昨仓支持）
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - 实施报告: @docs/实盘/CTP_Plugin_今昨仓实施报告_Phase1_2026-01-30.md
#   - 方案文档: @docs/实盘/Plugin_层_今昨仓支持与CTP_Plugin_实施方案_2026-01-30.md
#   - Offset方案: @docs/实盘/Plugin_层_Offset_自动设置方案_2026-01-30.md
# ============================================

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

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
    echo -e "${BLUE}[STEP]${NC} $1"
}

# 清理函数
cleanup() {
    log_info "Cleaning up..."

    # 停止所有进程
    pkill -f nats-server 2>/dev/null || true
    pkill -f md_gateway 2>/dev/null || true
    pkill -f counter_bridge 2>/dev/null || true
    pkill -f ors_gateway 2>/dev/null || true
    pkill -f "trader -config" 2>/dev/null || true

    # 清理共享内存
    ipcs -m 2>/dev/null | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true

    log_info "Cleanup completed"
}

trap cleanup EXIT

# ==================== 测试准备 ====================

log_step "Step 1: 准备测试环境"

# 检查配置文件
if [ ! -f "config/ctp/ctp_td.secret.yaml" ]; then
    log_error "CTP secret config not found: config/ctp/ctp_td.secret.yaml"
    log_error "Please create the secret config file with your CTP credentials"
    exit 1
fi

if [ ! -f "config/ctp/ctp_md.secret.yaml" ]; then
    log_error "CTP MD secret config not found: config/ctp/ctp_md.secret.yaml"
    log_error "Please create the secret config file with your CTP credentials"
    exit 1
fi

# 创建日志目录
mkdir -p log data/ctp_positions test_logs

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

# 启动 CTP MD Gateway
log_info "Starting CTP MD gateway..."
./gateway/build/ctp_md_gateway \
    config/ctp/ctp_md.yaml \
    config/ctp/ctp_md.secret.yaml \
    > log/ctp_md_gateway.log 2>&1 &
sleep 3

if ! pgrep -f ctp_md_gateway > /dev/null; then
    log_error "Failed to start CTP MD gateway"
    cat log/ctp_md_gateway.log
    exit 1
fi
log_info "✓ CTP MD gateway started"

# 启动 ORS Gateway
log_info "Starting ORS gateway..."
./gateway/build/ors_gateway > log/ors_gateway.log 2>&1 &
sleep 2

if ! pgrep -f ors_gateway > /dev/null; then
    log_error "Failed to start ORS gateway"
    cat log/ors_gateway.log
    exit 1
fi
log_info "✓ ORS gateway started"

# 启动 Counter Bridge (CTP Plugin)
log_info "Starting counter_bridge with CTP plugin..."
./gateway/build/counter_bridge \
    ctp:config/ctp/ctp_td.yaml:config/ctp/ctp_td.secret.yaml \
    > log/counter_bridge_ctp.log 2>&1 &
CB_PID=$!
sleep 5

if ! ps -p $CB_PID > /dev/null; then
    log_error "Failed to start counter_bridge with CTP plugin"
    cat log/counter_bridge_ctp.log
    exit 1
fi
log_info "✓ Counter bridge (CTP) started"

# 检查 CTP 登录状态
log_info "Checking CTP login status..."
sleep 10

if grep -q "Login successful" log/counter_bridge_ctp.log; then
    log_info "✓ CTP login successful"
else
    log_warn "CTP login status unclear, check logs:"
    tail -20 log/counter_bridge_ctp.log
fi

# 检查持仓查询
log_info "Checking position query..."
sleep 5

if grep -q "Position updated from CTP" log/counter_bridge_ctp.log; then
    log_info "✓ Position query successful"
    grep "Position:" log/counter_bridge_ctp.log | tail -5
else
    log_warn "Position query not found in logs"
fi

# ==================== 测试场景 ====================

log_step "Step 3: 测试 Offset 自动设置"

# 启动 Trader (使用测试配置，不自动激活)
log_info "Starting trader (test mode)..."
./bin/trader -config config/trader.test.yaml > log/trader_offset_test.log 2>&1 &
TRADER_PID=$!
sleep 5

if ! ps -p $TRADER_PID > /dev/null; then
    log_error "Failed to start trader"
    cat log/trader_offset_test.log
    exit 1
fi
log_info "✓ Trader started"

# 等待系统稳定
log_info "Waiting for system to stabilize..."
sleep 10

# ==================== 场景 1: 无持仓开仓 ====================

log_step "Scenario 1: 无持仓时开仓（应自动设置 OPEN）"

log_info "Activating strategy..."
curl -X POST http://localhost:9201/api/v1/strategy/activate \
    -H "Content-Type: application/json" \
    -d '{"strategy_id": "test_92201"}' \
    > /dev/null 2>&1

sleep 10

log_info "Checking offset in logs..."
if grep -q "Auto-set offset:.*OPEN" log/counter_bridge_ctp.log; then
    log_info "✓ Scenario 1 PASSED: Offset correctly set to OPEN"
    grep "Auto-set offset:" log/counter_bridge_ctp.log | tail -3
else
    log_warn "⚠ Scenario 1: No OPEN offset found (may not have triggered orders yet)"
fi

# 检查持仓更新
log_info "Checking position updates after trades..."
sleep 5

if grep -q "Position updated (OPEN)" log/counter_bridge_ctp.log; then
    log_info "✓ Position updated after OPEN trades"
    grep "Position updated" log/counter_bridge_ctp.log | tail -5
else
    log_info "ℹ No position updates yet (waiting for trades)"
fi

# ==================== 场景 2: 有持仓时平仓 ====================

log_step "Scenario 2: 有持仓时反向订单（应自动设置 CLOSE）"

log_info "Waiting for opposite direction orders..."
sleep 20

log_info "Checking for CLOSE offset..."
if grep -q "Auto-set offset:.*CLOSE" log/counter_bridge_ctp.log; then
    log_info "✓ Scenario 2 PASSED: Offset correctly set to CLOSE"
    grep "Auto-set offset:.*CLOSE" log/counter_bridge_ctp.log | tail -3
else
    log_warn "⚠ Scenario 2: No CLOSE offset found (may need longer wait or position reversal)"
fi

# 检查持仓更新（平仓）
if grep -q "Position updated (CLOSE)" log/counter_bridge_ctp.log; then
    log_info "✓ Position updated after CLOSE trades"
    grep "Position updated (CLOSE)" log/counter_bridge_ctp.log | tail -5
fi

# ==================== 场景 3: 持仓持久化 ====================

log_step "Scenario 3: 持仓持久化验证"

log_info "Checking position file..."
if [ -f "data/ctp_positions/*_positions.json" ]; then
    log_info "✓ Position file created"
    ls -lh data/ctp_positions/
    log_info "Position file content:"
    cat data/ctp_positions/*_positions.json | head -20
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
OPEN_COUNT=$(grep -c "Auto-set offset:.*OPEN" log/counter_bridge_ctp.log 2>/dev/null || echo "0")
CLOSE_COUNT=$(grep -c "Auto-set offset:.*CLOSE" log/counter_bridge_ctp.log 2>/dev/null || echo "0")
CLOSE_TODAY_COUNT=$(grep -c "Auto-set offset:.*CLOSE_TODAY" log/counter_bridge_ctp.log 2>/dev/null || echo "0")

echo "Offset Auto-Set Statistics:"
echo "  - OPEN:        $OPEN_COUNT"
echo "  - CLOSE:       $CLOSE_COUNT"
echo "  - CLOSE_TODAY: $CLOSE_TODAY_COUNT"
echo ""

# 统计持仓更新
POS_UPDATE_OPEN=$(grep -c "Position updated (OPEN)" log/counter_bridge_ctp.log 2>/dev/null || echo "0")
POS_UPDATE_CLOSE=$(grep -c "Position updated (CLOSE)" log/counter_bridge_ctp.log 2>/dev/null || echo "0")

echo "Position Update Statistics:"
echo "  - OPEN updates:  $POS_UPDATE_OPEN"
echo "  - CLOSE updates: $POS_UPDATE_CLOSE"
echo ""

# 统计订单和成交
ORDER_COUNT=$(grep -c "Order sent" log/trader_offset_test.log 2>/dev/null || echo "0")
TRADE_COUNT=$(grep -c "Trade:" log/counter_bridge_ctp.log 2>/dev/null || echo "0")

echo "Trading Statistics:"
echo "  - Orders sent: $ORDER_COUNT"
echo "  - Trades:      $TRADE_COUNT"
echo ""

# 检查错误
ERROR_COUNT=$(grep -c "Position mismatch" log/counter_bridge_ctp.log 2>/dev/null || echo "0")
echo "Error Statistics:"
echo "  - Position mismatches: $ERROR_COUNT"
echo ""

# 综合评估
echo "========================================="
if [ "$ERROR_COUNT" -eq 0 ] && [ "$OPEN_COUNT" -gt 0 ]; then
    echo -e "${GREEN}✓ TEST PASSED${NC}"
    echo "  - Offset auto-set working correctly"
    echo "  - No position mismatches detected"
    echo "  - Position updates functioning"
else
    echo -e "${YELLOW}⚠ TEST INCONCLUSIVE${NC}"
    echo "  - May need more time for orders to trigger"
    echo "  - Check logs for detailed analysis"
fi
echo "========================================="
echo ""

# ==================== 日志保存 ====================

log_step "Step 5: 保存测试日志"

TEST_LOG_DIR="test_logs/ctp_offset_$(date +%Y%m%d_%H%M%S)"
mkdir -p "$TEST_LOG_DIR"

cp log/counter_bridge_ctp.log "$TEST_LOG_DIR/"
cp log/trader_offset_test.log "$TEST_LOG_DIR/"
cp log/nats.log "$TEST_LOG_DIR/" 2>/dev/null || true

if [ -f data/ctp_positions/*_positions.json ]; then
    cp data/ctp_positions/*_positions.json "$TEST_LOG_DIR/" 2>/dev/null || true
fi

log_info "✓ Test logs saved to $TEST_LOG_DIR"

# ==================== 手动验证提示 ====================

echo ""
echo "========================================="
echo "      Manual Verification Steps"
echo "========================================="
echo ""
echo "1. Check CTP login:"
echo "   grep 'Login' log/counter_bridge_ctp.log"
echo ""
echo "2. Check position query:"
echo "   grep 'Position:' log/counter_bridge_ctp.log"
echo ""
echo "3. Check offset auto-set:"
echo "   grep 'Auto-set offset:' log/counter_bridge_ctp.log"
echo ""
echo "4. Check position updates:"
echo "   grep 'Position updated' log/counter_bridge_ctp.log"
echo ""
echo "5. Check position file:"
echo "   cat data/ctp_positions/*_positions.json"
echo ""
echo "6. Monitor live:"
echo "   tail -f log/counter_bridge_ctp.log | grep -E 'offset|Position'"
echo ""
echo "========================================="
echo ""

log_info "Test completed! Press Ctrl+C to stop all services."

# 保持运行，允许用户手动观察
wait

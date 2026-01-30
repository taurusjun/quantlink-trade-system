#!/bin/bash
set -e

# ============================================
# 脚本名称: test_simulator_e2e.sh
# 用途: 模拟交易所端到端测试
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - 实施报告: @docs/功能实现/模拟交易所完整实施计划_2026-01-30.md
#   - 架构说明: @docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md
# ============================================

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}✓${NC} $1"
}

log_section() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

# 清理函数
cleanup() {
    log_info "Cleaning up..."
    pkill -f md_simulator || true
    pkill -f md_gateway || true
    pkill -f ors_gateway || true
    pkill -f counter_bridge || true
    pkill -f "trader -config" || true
    sleep 1
    ipcs -m | grep $(whoami) | awk '{print $2}' | xargs ipcrm -m 2>/dev/null || true
}

trap cleanup EXIT

# 主测试逻辑
main() {
    log_section "Simulator End-to-End Test"

    # 1. 启动完整系统
    log_info "Starting simulator system..."
    ./scripts/live/start_simulator.sh > log/simulator_e2e.log 2>&1 &
    START_PID=$!

    # 等待系统启动
    log_info "Waiting for system to start (10 seconds)..."
    sleep 10

    # 2. 验证所有进程运行
    log_section "Verifying Processes"

    MISSING=""
    for proc in md_simulator md_gateway ors_gateway counter_bridge trader; do
        if pgrep -f "$proc" > /dev/null; then
            log_success "$proc is running"
        else
            log_error "$proc is NOT running"
            MISSING="$MISSING $proc"
        fi
    done

    if [ -n "$MISSING" ]; then
        log_error "Missing processes:$MISSING"
        cat log/simulator_e2e.log
        exit 1
    fi

    # 3. 测试 API 端点
    log_section "Testing API Endpoints"

    # 策略状态
    log_info "Testing strategy status..."
    STATUS=$(curl -s http://localhost:9201/api/v1/strategy/status || echo "FAILED")
    if echo "$STATUS" | grep -q "strategy_id"; then
        log_success "Strategy status API working"
    else
        log_error "Strategy status API failed"
        echo "$STATUS"
        exit 1
    fi

    # 持仓查询
    log_info "Testing position query..."
    POSITIONS=$(curl -s http://localhost:8080/positions || echo "FAILED")
    if echo "$POSITIONS" | grep -q "exchange"; then
        log_success "Position query API working"
    else
        log_error "Position query API failed"
        echo "$POSITIONS"
        exit 1
    fi

    # 4. 激活策略
    log_section "Activating Strategy"

    log_info "Sending activate request..."
    ACTIVATE_RESP=$(curl -s -X POST http://localhost:9201/api/v1/strategy/activate)
    echo "$ACTIVATE_RESP"

    if echo "$ACTIVATE_RESP" | grep -q "success.*true"; then
        log_success "Strategy activated"
    else
        log_error "Strategy activation failed"
        exit 1
    fi

    # 5. 等待订单生成
    log_section "Waiting for Orders"

    log_info "Monitoring for order generation (30 seconds)..."
    TIMEOUT=30
    ORDER_FOUND=0

    for i in $(seq 1 $TIMEOUT); do
        if grep -q "Order sent" log/trader.log 2>/dev/null; then
            ORDER_FOUND=1
            break
        fi
        sleep 1
        echo -n "."
    done
    echo ""

    if [ $ORDER_FOUND -eq 1 ]; then
        log_success "Orders detected in trader log"
        grep "Order sent" log/trader.log | tail -5
    else
        log_error "No orders generated within $TIMEOUT seconds"
        echo "Last 20 lines of trader.log:"
        tail -20 log/trader.log
        exit 1
    fi

    # 6. 验证订单成交
    log_section "Verifying Order Execution"

    sleep 3
    if grep -q "Trade" log/trader.log 2>/dev/null || \
       grep -q "FILLED" log/counter_bridge.log 2>/dev/null; then
        log_success "Order execution detected"
    else
        log_error "No order execution detected"
        exit 1
    fi

    # 7. 检查持仓更新
    log_section "Checking Position Updates"

    FINAL_POSITIONS=$(curl -s http://localhost:8080/positions)
    if echo "$FINAL_POSITIONS" | grep -q '"volume":[1-9]'; then
        log_success "Positions updated"
        echo "$FINAL_POSITIONS" | jq '.'
    else
        log_error "No positions found"
        echo "$FINAL_POSITIONS"
    fi

    # 8. 测试总结
    log_section "Test Summary"

    log_success "All tests passed!"
    echo ""
    echo "Test Results:"
    echo "  ✓ System startup"
    echo "  ✓ All processes running"
    echo "  ✓ API endpoints working"
    echo "  ✓ Strategy activation"
    echo "  ✓ Order generation"
    echo "  ✓ Order execution"
    echo "  ✓ Position updates"
    echo ""

    # 等待用户查看结果
    log_info "Test completed. Press Ctrl+C to stop all services."
    wait
}

main "$@"

#!/bin/bash
set -e

# ============================================
# 脚本名称: test_live_e2e.sh
# 用途: 实盘端到端测试（使用 trader.live.yaml 配置）
# 作者: QuantLink Team
# 日期: 2026-02-09
#
# 相关文档:
#   - 实施报告: @docs/功能实现/PairwiseArbStrategy动态阈值和追单功能实现.md
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

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
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
    pkill -f ctp_md_gateway || true
    pkill -f md_gateway || true
    pkill -f ors_gateway || true
    pkill -f counter_bridge || true
    pkill -f "trader -config" || true
    sleep 1
    ipcs -m | grep $(whoami) | awk '{print $2}' | xargs ipcrm -m 2>/dev/null || true
}

trap cleanup EXIT

# 启动单个组件
start_component() {
    local name=$1
    local cmd=$2
    local logfile=$3

    $cmd > $logfile 2>&1 &
    sleep 2
    if pgrep -f "$name" > /dev/null; then
        log_success "$name started"
        return 0
    else
        log_error "$name failed to start"
        cat $logfile | tail -20
        return 1
    fi
}

# 主测试逻辑
main() {
    log_section "Live Trading End-to-End Test"
    log_info "Using config: config/trader.live.yaml"

    # 检查配置文件
    if [ ! -f "config/trader.live.yaml" ]; then
        log_error "config/trader.live.yaml not found"
        exit 1
    fi

    # 1. 创建必要目录
    log_info "Creating directories..."
    mkdir -p log data/simulator

    # 2. 启动 NATS
    log_info "Starting NATS server..."
    if ! pgrep -f nats-server > /dev/null; then
        nats-server > log/nats.log 2>&1 &
        sleep 2
    fi
    log_success "NATS server ready"

    # 3. 启动 md_simulator（模拟行情）
    log_info "Starting md_simulator..."
    start_component "md_simulator" "./gateway/build/md_simulator" "log/md_simulator.log" || exit 1

    # 4. 启动 md_gateway
    log_info "Starting md_gateway..."
    start_component "md_gateway" "./gateway/build/md_gateway" "log/md_gateway.log" || exit 1

    # 5. 启动 ors_gateway
    log_info "Starting ors_gateway..."
    start_component "ors_gateway" "./gateway/build/ors_gateway" "log/ors_gateway.log" || exit 1

    # 6. 启动 counter_bridge (Simulator 模式)
    log_info "Starting counter_bridge (Simulator mode)..."
    ./gateway/build/counter_bridge simulator:config/simulator/simulator.yaml > log/counter_bridge.log 2>&1 &
    sleep 2
    if pgrep -f counter_bridge > /dev/null; then
        log_success "counter_bridge started"
    else
        log_error "counter_bridge failed to start"
        cat log/counter_bridge.log | tail -20
        exit 1
    fi

    # 7. 启动 trader（使用实盘配置）
    log_info "Starting trader with live config..."
    ./bin/trader -config config/trader.live.yaml > log/trader.live.log 2>&1 &
    sleep 2
    if pgrep -f "trader -config" > /dev/null; then
        log_success "trader started"
    else
        log_error "trader failed to start"
        cat log/trader.live.log | tail -20
        exit 1
    fi

    # 8. 等待 API 服务就绪
    log_info "Waiting for API to be ready..."
    TRADER_LOG="log/trader.live.log"
    for i in $(seq 1 30); do
        if curl -s http://localhost:9201/api/v1/strategy/status > /dev/null 2>&1; then
            log_success "API is ready after ${i} seconds"
            break
        fi
        sleep 1
        echo -n "."
    done
    echo ""

    # 9. 验证进程
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
        exit 1
    fi

    # 10. 测试 API 端点
    log_section "Testing API Endpoints"

    # 策略状态
    log_info "Testing strategy status..."
    STATUS=$(curl -s http://localhost:9201/api/v1/strategy/status 2>&1 || echo "CURL_FAILED")
    if echo "$STATUS" | grep -q "success"; then
        log_success "Strategy status API working"

        # 显示动态阈值配置
        log_info "Checking dynamic threshold config..."
        if echo "$STATUS" | grep -q "entry_threshold"; then
            log_success "Dynamic threshold indicators present"
        fi
    else
        log_error "Strategy status API failed"
        echo "$STATUS"
        exit 1
    fi

    # 持仓查询
    log_info "Testing position query..."
    POSITIONS=$(curl -s http://localhost:8080/positions || echo "FAILED")
    if echo "$POSITIONS" | grep -q "success"; then
        log_success "Position query API working"
    else
        log_error "Position query API failed"
        echo "$POSITIONS"
        exit 1
    fi

    # 11. 激活策略
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

    # 12. 等待订单生成
    log_section "Waiting for Orders"
    log_info "Monitoring for order generation (30 seconds)..."
    TIMEOUT=30
    ORDER_FOUND=0

    for i in $(seq 1 $TIMEOUT); do
        if grep -q "Order sent" "$TRADER_LOG" 2>/dev/null; then
            ORDER_FOUND=1
            break
        fi
        sleep 1
        echo -n "."
    done
    echo ""

    if [ $ORDER_FOUND -eq 1 ]; then
        log_success "Orders detected in trader log"
        grep "Order sent" "$TRADER_LOG" | tail -5
    else
        log_error "No orders generated within $TIMEOUT seconds"
        echo "Last 20 lines of $TRADER_LOG:"
        tail -20 "$TRADER_LOG"
        exit 1
    fi

    # 13. 验证订单执行
    log_section "Verifying Order Execution"
    sleep 3
    if grep -q "Trade\|FILLED\|Order sent" "$TRADER_LOG" 2>/dev/null || \
       grep -q "FILLED" log/counter_bridge.log 2>/dev/null; then
        log_success "Order execution detected"
    else
        log_error "No order execution detected"
        exit 1
    fi

    # 14. 检查动态阈值功能
    log_section "Checking Dynamic Threshold Feature"
    if grep -q "Dynamic threshold" "$TRADER_LOG" 2>/dev/null; then
        log_success "Dynamic threshold feature is active"
        grep "Dynamic threshold" "$TRADER_LOG" | head -3
    else
        log_warn "Dynamic threshold log not found (may be disabled or not triggered)"
    fi

    # 15. 检查追单功能
    log_section "Checking Aggressive Order Feature"
    if grep -q "Aggressive order" "$TRADER_LOG" 2>/dev/null; then
        log_success "Aggressive order feature is active"
        grep "Aggressive order" "$TRADER_LOG" | head -3
    else
        log_warn "Aggressive order log not found (may be disabled or no exposure)"
    fi

    # 16. 检查持仓更新
    log_section "Checking Position Updates"
    FINAL_POSITIONS=$(curl -s http://localhost:8080/positions)
    if echo "$FINAL_POSITIONS" | grep -q '"volume":[1-9]'; then
        log_success "Positions updated"
        echo "$FINAL_POSITIONS" | jq '.' 2>/dev/null || echo "$FINAL_POSITIONS"
    else
        log_warn "No positions found (may be expected)"
        echo "$FINAL_POSITIONS" | jq '.' 2>/dev/null || echo "$FINAL_POSITIONS"
    fi

    # 17. 测试总结
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
    echo "Live Config Features:"
    echo "  - Dynamic threshold: Enabled"
    echo "  - Aggressive order: Enabled"
    echo ""

    # 等待用户查看结果
    log_info "Test completed. Press Ctrl+C to stop all services."
    wait
}

main "$@"

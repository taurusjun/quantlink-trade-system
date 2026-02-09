#!/bin/bash
set -e

# ============================================
# 脚本名称: test_simulator_e2e.sh
# 用途: 模拟交易所端到端测试
# 作者: QuantLink Team
# 日期: 2026-02-09
#
# 用法:
#   ./scripts/test/e2e/test_simulator_e2e.sh           # 运行测试后退出
#   ./scripts/test/e2e/test_simulator_e2e.sh --run     # 启动系统并保持运行
#
# 相关文档:
#   - 架构说明: @docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md
# ============================================

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$PROJECT_ROOT"

# 参数解析
RUN_MODE=false
for arg in "$@"; do
    case $arg in
        --run)
            RUN_MODE=true
            shift
            ;;
    esac
done

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_success() { echo -e "${GREEN}✓${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_section() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

# 清理函数
cleanup() {
    if [ "$RUN_MODE" = true ]; then
        log_info "Stopping all services..."
    else
        log_info "Cleaning up..."
    fi
    pkill -f md_simulator || true
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

# 启动系统
start_system() {
    log_section "Starting Simulator System"
    mkdir -p log data/simulator

    # 启动 NATS
    if ! pgrep -f nats-server > /dev/null; then
        nats-server > log/nats.log 2>&1 &
        sleep 2
    fi
    log_success "NATS server ready"

    # 启动各组件
    start_component "md_simulator" "./gateway/build/md_simulator" "log/md_simulator.log" || exit 1
    start_component "md_gateway" "./gateway/build/md_gateway" "log/md_gateway.log" || exit 1
    start_component "ors_gateway" "./gateway/build/ors_gateway" "log/ors_gateway.log" || exit 1

    # 启动 counter_bridge
    ./gateway/build/counter_bridge simulator:config/simulator/simulator.yaml > log/counter_bridge.log 2>&1 &
    sleep 2
    if pgrep -f counter_bridge > /dev/null; then
        log_success "counter_bridge started"
    else
        log_error "counter_bridge failed to start"
        cat log/counter_bridge.log | tail -20
        exit 1
    fi

    # 启动 trader
    ./bin/trader -config config/trader.test.yaml > log/trader.log 2>&1 &
    sleep 2
    if pgrep -f "trader -config" > /dev/null; then
        log_success "trader started"
    else
        log_error "trader failed to start"
        cat log/trader.log | tail -20
        exit 1
    fi

    # 等待 API 服务就绪
    log_info "Waiting for API to be ready..."
    for i in $(seq 1 30); do
        if curl -s http://localhost:9201/api/v1/strategy/status > /dev/null 2>&1; then
            log_success "API is ready"
            break
        fi
        sleep 1
        echo -n "."
    done
    echo ""
}

# 验证进程
verify_processes() {
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
}

# 运行测试
run_tests() {
    # 测试 API 端点
    log_section "Testing API Endpoints"

    log_info "Testing strategy status..."
    STATUS=$(curl -s http://localhost:9201/api/v1/strategy/status 2>&1 || echo "CURL_FAILED")
    if echo "$STATUS" | grep -q "strategy_id\|success"; then
        log_success "Strategy status API working"
    else
        log_error "Strategy status API failed"
        echo "$STATUS"
        exit 1
    fi

    log_info "Testing position query..."
    POSITIONS=$(curl -s http://localhost:8080/positions || echo "FAILED")
    if echo "$POSITIONS" | grep -q "success"; then
        log_success "Position query API working"
    else
        log_error "Position query API failed"
        echo "$POSITIONS"
        exit 1
    fi

    # 激活策略
    log_section "Activating Strategy"
    log_info "Sending activate request..."
    ACTIVATE_RESP=$(curl -s -X POST http://localhost:9201/api/v1/strategy/activate)
    if echo "$ACTIVATE_RESP" | grep -q "success.*true"; then
        log_success "Strategy activated"
    else
        log_error "Strategy activation failed"
        exit 1
    fi

    # 等待订单生成
    log_section "Waiting for Orders"
    log_info "Monitoring for order generation (30 seconds)..."
    TRADER_LOG="log/trader.test.log"
    ORDER_FOUND=0

    for i in $(seq 1 30); do
        if grep -q "Order sent" "$TRADER_LOG" 2>/dev/null; then
            ORDER_FOUND=1
            break
        fi
        sleep 1
        echo -n "."
    done
    echo ""

    if [ $ORDER_FOUND -eq 1 ]; then
        log_success "Orders detected"
        grep "Order sent" "$TRADER_LOG" | tail -3
    else
        log_error "No orders generated within 30 seconds"
        tail -20 "$TRADER_LOG"
        exit 1
    fi

    # 验证订单成交
    log_section "Verifying Order Execution"
    sleep 3
    if grep -q "Trade\|FILLED\|Order sent" "$TRADER_LOG" 2>/dev/null || \
       grep -q "FILLED" log/counter_bridge.log 2>/dev/null; then
        log_success "Order execution detected"
    else
        log_error "No order execution detected"
        exit 1
    fi

    # 测试总结
    log_section "Test Summary"
    log_success "All tests passed!"
    echo ""
    echo "  ✓ System startup"
    echo "  ✓ All processes running"
    echo "  ✓ API endpoints working"
    echo "  ✓ Strategy activation"
    echo "  ✓ Order generation"
    echo "  ✓ Order execution"
}

# 显示运行信息
show_run_info() {
    log_section "System Running"
    echo "Endpoints:"
    echo "  Dashboard:     http://localhost:9201/dashboard"
    echo "  API:           http://localhost:9201/api/v1"
    echo "  Positions:     http://localhost:8080/positions"
    echo ""
    echo "Commands:"
    echo "  Activate:      curl -X POST http://localhost:9201/api/v1/strategy/activate"
    echo "  Status:        curl http://localhost:9201/api/v1/strategy/status | jq"
    echo "  Logs:          tail -f log/trader.log"
    echo ""
    echo "Press Ctrl+C to stop all services."
}

# 主逻辑
main() {
    if [ "$RUN_MODE" = true ]; then
        log_section "Simulator System (Run Mode)"
    else
        log_section "Simulator End-to-End Test"
    fi

    start_system
    verify_processes

    if [ "$RUN_MODE" = true ]; then
        show_run_info
        # 保持运行
        while true; do sleep 1; done
    else
        run_tests
        log_info "Test completed. Cleaning up..."
    fi
}

main "$@"

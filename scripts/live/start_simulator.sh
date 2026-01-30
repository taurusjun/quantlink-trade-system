#!/bin/bash
set -e

# ============================================
# 脚本名称: start_simulator.sh
# 用途: 启动模拟交易系统（完整链路测试）
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - 实施报告: @docs/功能实现/模拟交易所完整实施计划_2026-01-30.md
#   - 架构说明: @docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md
#   - 使用指南: @docs/核心文档/USAGE.md
# ============================================

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
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

log_section() {
    echo -e "\n${BLUE}========================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}========================================${NC}\n"
}

# 清理函数
cleanup() {
    log_info "Stopping all services..."

    # 停止所有相关进程
    pkill -f md_simulator || true
    pkill -f md_gateway || true
    pkill -f ors_gateway || true
    pkill -f counter_bridge || true
    pkill -f "trader -config" || true
    pkill -f nats-server || true

    # 清理共享内存
    ipcs -m | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null || true

    log_info "Cleanup completed"
}

trap cleanup EXIT

# 主逻辑
main() {
    log_section "Starting Simulator Trading System"

    # 1. 创建必要的目录
    log_info "[1/8] Creating directories..."
    mkdir -p log data/simulator bin

    # 2. 启动 NATS
    log_info "[2/8] Starting NATS server..."
    if pgrep -f nats-server > /dev/null; then
        log_warn "NATS server already running"
    else
        nats-server > log/nats.log 2>&1 &
        sleep 2
        if pgrep -f nats-server > /dev/null; then
            log_info "✓ NATS server started"
        else
            log_error "Failed to start NATS server"
            cat log/nats.log
            exit 1
        fi
    fi

    # 3. 启动 md_simulator
    log_info "[3/8] Starting md_simulator..."
    if [ ! -f gateway/build/md_simulator ]; then
        log_error "md_simulator not found. Please run: cd gateway/build && cmake .. && make"
        exit 1
    fi
    ./gateway/build/md_simulator > log/md_simulator.log 2>&1 &
    sleep 1
    if pgrep -f md_simulator > /dev/null; then
        log_info "✓ md_simulator started"
    else
        log_error "Failed to start md_simulator"
        cat log/md_simulator.log
        exit 1
    fi

    # 4. 启动 md_gateway
    log_info "[4/8] Starting md_gateway..."
    if [ ! -f gateway/build/md_gateway ]; then
        log_error "md_gateway not found. Please run: cd gateway/build && cmake .. && make"
        exit 1
    fi
    ./gateway/build/md_gateway > log/md_gateway.log 2>&1 &
    sleep 1
    if pgrep -f md_gateway > /dev/null; then
        log_info "✓ md_gateway started"
    else
        log_error "Failed to start md_gateway"
        cat log/md_gateway.log
        exit 1
    fi

    # 5. 启动 ors_gateway
    log_info "[5/8] Starting ors_gateway..."
    if [ ! -f gateway/build/ors_gateway ]; then
        log_error "ors_gateway not found. Please run: cd gateway/build && cmake .. && make"
        exit 1
    fi
    ./gateway/build/ors_gateway > log/ors_gateway.log 2>&1 &
    sleep 1
    if pgrep -f ors_gateway > /dev/null; then
        log_info "✓ ors_gateway started"
    else
        log_error "Failed to start ors_gateway"
        cat log/ors_gateway.log
        exit 1
    fi

    # 6. 启动 counter_bridge (Simulator 模式)
    log_info "[6/8] Starting counter_bridge (Simulator mode)..."
    if [ ! -f gateway/build/counter_bridge ]; then
        log_error "counter_bridge not found. Please run: cd gateway/build && cmake .. && make"
        exit 1
    fi
    if [ ! -f config/simulator/simulator.yaml ]; then
        log_error "Simulator config not found: config/simulator/simulator.yaml"
        exit 1
    fi
    ./gateway/build/counter_bridge simulator:config/simulator/simulator.yaml > log/counter_bridge.log 2>&1 &
    CB_PID=$!
    sleep 2

    # 检查 counter_bridge 是否成功启动
    if ! ps -p $CB_PID > /dev/null; then
        log_error "Counter Bridge failed to start"
        cat log/counter_bridge.log
        exit 1
    fi
    log_info "✓ counter_bridge started (Simulator plugin)"

    # 7. 启动 Trader
    log_info "[7/8] Starting trader..."
    if [ ! -f bin/trader ]; then
        log_error "trader not found. Please run: cd golang && go build -o ../bin/trader cmd/trader/main.go"
        exit 1
    fi
    if [ ! -f config/trader.test.yaml ]; then
        log_error "Trader config not found: config/trader.test.yaml"
        exit 1
    fi
    ./bin/trader -config config/trader.test.yaml > log/trader.log 2>&1 &
    TRADER_PID=$!
    sleep 3

    # 检查 trader 是否成功启动
    if ! ps -p $TRADER_PID > /dev/null; then
        log_error "Trader failed to start"
        cat log/trader.log
        exit 1
    fi
    log_info "✓ trader started"

    # 8. 验证系统状态
    log_info "[8/8] Verifying system status..."
    sleep 2

    # 检查所有进程
    MISSING_PROCS=""
    for proc in md_simulator md_gateway ors_gateway counter_bridge trader nats-server; do
        if ! pgrep -f "$proc" > /dev/null; then
            MISSING_PROCS="$MISSING_PROCS $proc"
        fi
    done

    if [ -n "$MISSING_PROCS" ]; then
        log_error "Missing processes:$MISSING_PROCS"
        exit 1
    fi

    log_section "✓ Simulator Trading System Started Successfully"

    echo ""
    echo "System Information:"
    echo "  - Dashboard:        http://localhost:9201/dashboard"
    echo "  - API Endpoint:     http://localhost:9201/api/v1"
    echo "  - Position Query:   http://localhost:8080/positions"
    echo "  - Simulator Stats:  http://localhost:8080/simulator/stats"
    echo ""
    echo "Quick Commands:"
    echo "  - Activate strategy:  curl -X POST http://localhost:9201/api/v1/strategy/activate"
    echo "  - View strategy status: curl http://localhost:9201/api/v1/strategy/status"
    echo "  - View positions:     curl http://localhost:8080/positions | jq ."
    echo "  - View logs:          tail -f log/trader.log"
    echo "  - View orders:        tail -f log/trader.log | grep 'Order sent'"
    echo ""
    echo "Press Ctrl+C to stop all services"
    echo ""

    # 等待用户中断
    while true; do
        sleep 1
    done
}

main "$@"

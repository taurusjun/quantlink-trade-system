#!/bin/bash
set -e

# ============================================
# 脚本名称: test_ctp_live_e2e.sh
# 用途: CTP实盘端到端测试（连接真实CTP服务器）
# 作者: QuantLink Team
# 日期: 2026-02-09
#
# 用法:
#   ./scripts/test/e2e/test_ctp_live_e2e.sh              # 运行测试后退出
#   ./scripts/test/e2e/test_ctp_live_e2e.sh --run        # 前台运行（Ctrl+C停止）
#   ./scripts/test/e2e/test_ctp_live_e2e.sh --background # 后台运行
#
# 停止后台服务:
#   ./scripts/live/stop_all.sh
#
# 相关文档:
#   - 架构说明: @docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md
# ============================================

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../../.. && pwd)"
cd "$PROJECT_ROOT"

# 参数解析
RUN_MODE=false
BACKGROUND_MODE=false
for arg in "$@"; do
    case $arg in
        --run)
            RUN_MODE=true
            ;;
        --background)
            BACKGROUND_MODE=true
            ;;
    esac
done

# 后台模式：重新执行自己
if [ "$BACKGROUND_MODE" = true ]; then
    echo "Starting in background mode..."
    mkdir -p log
    nohup "$0" --run > log/ctp_live_system.log 2>&1 &
    BG_PID=$!
    sleep 3
    if ps -p $BG_PID > /dev/null 2>&1; then
        echo "✓ System started in background (PID: $BG_PID)"
        echo "  Log: log/ctp_live_system.log"
        echo "  Stop: ./scripts/live/stop_all.sh"
    else
        echo "✗ Failed to start in background"
        tail -20 log/ctp_live_system.log
        exit 1
    fi
    exit 0
fi

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
    pkill -f ctp_md_gateway || true
    pkill -f md_gateway || true
    pkill -f ors_gateway || true
    pkill -f counter_bridge || true
    pkill -f "trader -config" || true
    sleep 1
    ipcs -m | grep $(whoami) | awk '{print $2}' | xargs ipcrm -m 2>/dev/null || true
}

trap cleanup EXIT

# 检查配置文件
check_configs() {
    log_info "Checking configuration files..."
    local missing=0

    for f in "config/trader.live.yaml" "config/ctp/ctp_md.yaml" "config/ctp/ctp_md.secret.yaml" \
             "config/ctp/ctp_td.yaml" "config/ctp/ctp_td.secret.yaml"; do
        if [ ! -f "$f" ]; then
            log_error "Missing: $f"
            missing=1
        fi
    done

    if [ $missing -eq 1 ]; then
        exit 1
    fi
    log_success "All configuration files exist"
}

# 检查二进制文件
check_binaries() {
    log_info "Checking binaries..."
    local missing=0

    for f in "gateway/build/ctp_md_gateway" "gateway/build/md_gateway" \
             "gateway/build/ors_gateway" "gateway/build/counter_bridge" "bin/trader"; do
        if [ ! -f "$f" ]; then
            log_error "Missing: $f"
            missing=1
        fi
    done

    if [ $missing -eq 1 ]; then
        log_info "Please run: cd gateway/build && cmake .. && make"
        exit 1
    fi
    log_success "All binaries exist"
}

# 启动系统
start_system() {
    log_section "Starting CTP Live System"
    mkdir -p log ctp_flow data/positions

    # 启动 NATS
    if ! pgrep -f nats-server > /dev/null; then
        nats-server > log/nats.log 2>&1 &
        sleep 2
    fi
    log_success "NATS server ready"

    # 启动 CTP 行情网关
    log_info "Starting CTP Market Data Gateway..."
    ./gateway/build/ctp_md_gateway -c config/ctp/ctp_md.yaml -s config/ctp/ctp_md.secret.yaml > log/ctp_md_gateway.log 2>&1 &

    for i in $(seq 1 30); do
        if grep -q "Login successful" log/ctp_md_gateway.log 2>/dev/null; then
            log_success "CTP MD login successful"
            break
        fi
        if grep -q "Login failed" log/ctp_md_gateway.log 2>/dev/null; then
            log_error "CTP MD login failed"
            cat log/ctp_md_gateway.log | tail -20
            exit 1
        fi
        echo -n "."
        sleep 1
    done
    echo ""

    # 启动 md_gateway
    log_info "Starting md_gateway..."
    ./gateway/build/md_gateway > log/md_gateway.log 2>&1 &
    sleep 2
    if pgrep -f md_gateway > /dev/null; then
        log_success "md_gateway started"
    else
        log_error "md_gateway failed"
        exit 1
    fi

    # 启动 ors_gateway
    log_info "Starting ors_gateway..."
    ./gateway/build/ors_gateway > log/ors_gateway.log 2>&1 &
    sleep 2
    if pgrep -f ors_gateway > /dev/null; then
        log_success "ors_gateway started"
    else
        log_error "ors_gateway failed"
        exit 1
    fi

    # 启动 counter_bridge (CTP 模式)
    log_info "Starting counter_bridge (CTP mode)..."
    ./gateway/build/counter_bridge "ctp:config/ctp/ctp_td.yaml" > log/counter_bridge.log 2>&1 &

    for i in $(seq 1 30); do
        if grep -q "CTP plugin initialized" log/counter_bridge.log 2>/dev/null; then
            log_success "CTP TD login successful"
            break
        fi
        echo -n "."
        sleep 1
    done
    echo ""

    if ! pgrep -f counter_bridge > /dev/null; then
        log_error "counter_bridge failed"
        cat log/counter_bridge.log | tail -20
        exit 1
    fi

    # 启动 trader
    log_info "Starting trader..."
    ./bin/trader -config config/trader.live.yaml > log/trader.live.log 2>&1 &
    sleep 3
    if pgrep -f "trader -config" > /dev/null; then
        log_success "trader started"
    else
        log_error "trader failed"
        cat log/trader.live.log | tail -20
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
    for proc in ctp_md_gateway md_gateway ors_gateway counter_bridge trader; do
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
    TRADER_LOG="log/trader.live.log"

    # 测试 API 端点
    log_section "Testing API Endpoints"

    log_info "Testing strategy status..."
    STATUS=$(curl -s http://localhost:9201/api/v1/strategy/status 2>&1 || echo "CURL_FAILED")
    if echo "$STATUS" | grep -q "success"; then
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
        log_warn "Position query returned: $POSITIONS"
    fi

    # 检查市场数据接收
    log_section "Checking Market Data"
    log_info "Waiting for market data (30 seconds)..."
    MD_FOUND=0
    for i in $(seq 1 30); do
        if grep -q "Received market data" "$TRADER_LOG" 2>/dev/null; then
            MD_FOUND=1
            break
        fi
        sleep 1
        echo -n "."
    done
    echo ""

    if [ $MD_FOUND -eq 1 ]; then
        log_success "Market data received"
        grep "Received market data" "$TRADER_LOG" | head -3
    else
        log_warn "No market data (outside trading hours?)"
    fi

    # 测试总结
    log_section "Test Summary"
    log_success "System verification passed!"
    echo ""
    echo "  ✓ All processes running"
    echo "  ✓ API endpoints working"
    if [ $MD_FOUND -eq 1 ]; then
        echo "  ✓ Market data received"
    else
        echo "  ⚠ No market data (may be outside trading hours)"
    fi
}

# 显示运行信息
show_run_info() {
    log_section "CTP Live System Running"
    log_warn "Connected to REAL CTP servers (SimNow)"
    echo ""
    echo "Endpoints:"
    echo "  Dashboard:     http://localhost:9201/dashboard"
    echo "  API:           http://localhost:9201/api/v1"
    echo "  Positions:     http://localhost:8080/positions"
    echo ""
    echo "Commands:"
    echo "  Activate:      curl -X POST http://localhost:9201/api/v1/strategy/activate"
    echo "  Status:        curl http://localhost:9201/api/v1/strategy/status | jq"
    echo "  Positions:     curl http://localhost:8080/positions | jq"
    echo "  Logs:          tail -f log/trader.live.log"
    echo ""
    echo "Press Ctrl+C to stop all services."
}

# 主逻辑
main() {
    if [ "$RUN_MODE" = true ]; then
        log_section "CTP Live System (Run Mode)"
    else
        log_section "CTP Live End-to-End Test"
    fi

    log_warn "This connects to REAL CTP servers (SimNow)"

    check_configs
    check_binaries
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

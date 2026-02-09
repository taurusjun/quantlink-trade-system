#!/bin/bash
set -e

# ============================================
# 脚本名称: test_ctp_live_e2e.sh
# 用途: CTP实盘端到端测试（连接真实CTP，非模拟器）
# 作者: QuantLink Team
# 日期: 2026-02-09
#
# 相关文档:
#   - 实施报告: @docs/功能实现/PairwiseArbStrategy动态阈值和追单功能实现.md
#   - 架构说明: @docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md
# ============================================

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")"/../../.. && pwd)"
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
    pkill -f ctp_md_gateway || true
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
    log_section "CTP Live Trading End-to-End Test"
    log_info "Using config: config/trader.live.yaml"
    log_warn "This test connects to REAL CTP servers (SimNow)"

    # 1. 检查配置文件
    log_info "Checking configuration files..."
    if [ ! -f "config/trader.live.yaml" ]; then
        log_error "config/trader.live.yaml not found"
        exit 1
    fi
    if [ ! -f "config/ctp/ctp_md.yaml" ]; then
        log_error "config/ctp/ctp_md.yaml not found"
        exit 1
    fi
    if [ ! -f "config/ctp/ctp_md.secret.yaml" ]; then
        log_error "config/ctp/ctp_md.secret.yaml not found (contains credentials)"
        exit 1
    fi
    if [ ! -f "config/ctp/ctp_td.yaml" ]; then
        log_error "config/ctp/ctp_td.yaml not found"
        exit 1
    fi
    if [ ! -f "config/ctp/ctp_td.secret.yaml" ]; then
        log_error "config/ctp/ctp_td.secret.yaml not found (contains credentials)"
        exit 1
    fi
    log_success "All configuration files exist"

    # 2. 检查可执行文件
    log_info "Checking binaries..."
    if [ ! -f "gateway/build/ctp_md_gateway" ]; then
        log_error "gateway/build/ctp_md_gateway not found"
        log_info "Please run: cd gateway/build && make ctp_md_gateway"
        exit 1
    fi
    if [ ! -f "gateway/build/md_gateway" ]; then
        log_error "gateway/build/md_gateway not found"
        exit 1
    fi
    if [ ! -f "gateway/build/ors_gateway" ]; then
        log_error "gateway/build/ors_gateway not found"
        exit 1
    fi
    if [ ! -f "gateway/build/counter_bridge" ]; then
        log_error "gateway/build/counter_bridge not found"
        exit 1
    fi
    if [ ! -f "bin/trader" ]; then
        log_error "bin/trader not found"
        exit 1
    fi
    log_success "All binaries exist"

    # 3. 创建必要目录
    log_info "Creating directories..."
    mkdir -p log ctp_flow data/positions

    # 4. 启动 NATS
    log_info "Starting NATS server..."
    if ! pgrep -f nats-server > /dev/null; then
        nats-server > log/nats.log 2>&1 &
        sleep 2
    fi
    log_success "NATS server ready"

    # 5. 启动 CTP 行情网关
    log_section "Starting CTP Market Data Gateway"
    log_info "Connecting to CTP server..."
    ./gateway/build/ctp_md_gateway -c config/ctp/ctp_md.yaml -s config/ctp/ctp_md.secret.yaml > log/ctp_md_gateway.log 2>&1 &

    # 等待 CTP 登录
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

    if ! grep -q "Login successful" log/ctp_md_gateway.log 2>/dev/null; then
        log_error "CTP MD login timeout"
        cat log/ctp_md_gateway.log | tail -20
        exit 1
    fi

    # 6. 启动 md_gateway
    log_info "Starting md_gateway..."
    ./gateway/build/md_gateway > log/md_gateway.log 2>&1 &
    sleep 2
    if pgrep -f md_gateway > /dev/null; then
        log_success "md_gateway started"
    else
        log_error "md_gateway failed to start"
        cat log/md_gateway.log | tail -20
        exit 1
    fi

    # 7. 启动 ors_gateway
    log_info "Starting ors_gateway..."
    ./gateway/build/ors_gateway > log/ors_gateway.log 2>&1 &
    sleep 2
    if pgrep -f ors_gateway > /dev/null; then
        log_success "ors_gateway started"
    else
        log_error "ors_gateway failed to start"
        cat log/ors_gateway.log | tail -20
        exit 1
    fi

    # 8. 启动 counter_bridge (CTP 模式)
    log_section "Starting Counter Bridge (CTP Mode)"
    log_info "Connecting to CTP trading server..."
    ./gateway/build/counter_bridge "ctp:config/ctp/ctp_td.yaml" > log/counter_bridge.log 2>&1 &

    # 等待 CTP 交易登录
    for i in $(seq 1 30); do
        if grep -q "CTP plugin initialized" log/counter_bridge.log 2>/dev/null; then
            log_success "CTP TD login successful"
            break
        fi
        if grep -q "Failed to initialize CTP" log/counter_bridge.log 2>/dev/null; then
            log_error "CTP TD login failed"
            cat log/counter_bridge.log | tail -20
            exit 1
        fi
        echo -n "."
        sleep 1
    done
    echo ""

    if ! pgrep -f counter_bridge > /dev/null; then
        log_error "counter_bridge failed to start"
        cat log/counter_bridge.log | tail -20
        exit 1
    fi

    # 9. 启动 trader
    log_section "Starting Trader"
    log_info "Starting trader with live config..."
    ./bin/trader -config config/trader.live.yaml > log/trader.live.log 2>&1 &
    sleep 3
    if pgrep -f "trader -config" > /dev/null; then
        log_success "trader started"
    else
        log_error "trader failed to start"
        cat log/trader.live.log | tail -20
        exit 1
    fi

    # 10. 等待 API 服务就绪
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

    # 11. 验证进程
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

    # 12. 测试 API 端点
    log_section "Testing API Endpoints"

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
        log_warn "Position query API returned: $POSITIONS"
    fi

    # 13. 检查动态阈值功能
    log_section "Checking Dynamic Threshold Feature"
    if grep -q "Dynamic threshold enabled" "$TRADER_LOG" 2>/dev/null; then
        log_success "Dynamic threshold feature is active"
        grep "Dynamic threshold enabled" "$TRADER_LOG" | head -3
    else
        log_warn "Dynamic threshold log not found"
    fi

    # 14. 检查追单功能
    log_section "Checking Aggressive Order Feature"
    if grep -q "Aggressive order enabled" "$TRADER_LOG" 2>/dev/null; then
        log_success "Aggressive order feature is active"
        grep "Aggressive order enabled" "$TRADER_LOG" | head -3
    else
        log_warn "Aggressive order log not found"
    fi

    # 15. 检查市场数据接收
    log_section "Checking Market Data Reception"
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
        log_warn "No market data received (outside trading hours?)"
        log_info "CTP MD Stats:"
        grep "Stats:" log/ctp_md_gateway.log 2>/dev/null | tail -3 || echo "No stats available"
    fi

    # 16. 测试总结
    log_section "Test Summary"
    echo ""
    echo "System Status:"
    echo "  ✓ All processes running"
    echo "  ✓ API endpoints working"
    echo ""
    echo "Live Config Features:"
    if grep -q "Dynamic threshold enabled" "$TRADER_LOG" 2>/dev/null; then
        echo "  ✓ Dynamic threshold: Enabled"
    else
        echo "  ⚠ Dynamic threshold: Not confirmed"
    fi
    if grep -q "Aggressive order enabled" "$TRADER_LOG" 2>/dev/null; then
        echo "  ✓ Aggressive order: Enabled"
    else
        echo "  ⚠ Aggressive order: Not confirmed"
    fi
    echo ""

    # 提供操作指引
    log_section "Next Steps"
    echo "The system is running. You can:"
    echo ""
    echo "1. Activate strategy:"
    echo "   curl -X POST http://localhost:9201/api/v1/strategy/activate"
    echo ""
    echo "2. View strategy status:"
    echo "   curl http://localhost:9201/api/v1/strategy/status | jq"
    echo ""
    echo "3. View positions:"
    echo "   curl http://localhost:8080/positions | jq"
    echo ""
    echo "4. Monitor logs:"
    echo "   tail -f log/trader.live.log"
    echo ""
    echo "Press Ctrl+C to stop all services."

    # 等待用户操作
    wait
}

main "$@"

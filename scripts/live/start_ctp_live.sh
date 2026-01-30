#!/bin/bash
set -e

# ============================================
# 脚本名称: start_ctp_live.sh
# 用途: 启动CTP实盘交易系统
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - 使用指南: @docs/实盘/使用实盘配置启动.md
#   - 快速参考: @docs/实盘/实盘测试快速参考.md
# ============================================

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_section() { echo -e "\n${BLUE}========================================${NC}"; echo -e "${BLUE}$1${NC}"; echo -e "${BLUE}========================================${NC}\n"; }

cleanup() {
    log_warn "Received interrupt signal"
    log_info "Stopping all services..."
    pkill -f ctp_md_gateway 2>/dev/null || true
    pkill -f md_gateway 2>/dev/null || true
    pkill -f ors_gateway 2>/dev/null || true
    pkill -f counter_bridge 2>/dev/null || true
    pkill -f "trader -config config/trader.live.yaml" 2>/dev/null || true
    log_info "Services stopped"
    exit 0
}

trap cleanup INT TERM

log_section "启动CTP实盘交易系统"

# 检查配置文件
log_info "[1/8] 检查配置文件..."
if [ ! -f "config/ctp/ctp_md.secret.yaml" ]; then
    log_error "CTP行情配置文件不存在: config/ctp/ctp_md.secret.yaml"
    exit 1
fi

if [ ! -f "config/ctp/ctp_td.secret.yaml" ]; then
    log_error "CTP交易配置文件不存在: config/ctp/ctp_td.secret.yaml"
    exit 1
fi

if [ ! -f "config/trader.live.yaml" ]; then
    log_error "Trader实盘配置文件不存在: config/trader.live.yaml"
    exit 1
fi

log_info "✓ 配置文件检查通过"

# 创建目录
log_info "[2/8] 创建日志目录..."
mkdir -p log data/ctp_positions test_logs

# 启动 NATS
log_info "[3/8] 启动 NATS server..."
if pgrep -f nats-server > /dev/null; then
    log_warn "NATS server 已在运行"
else
    nats-server > log/nats.log 2>&1 &
    sleep 2
    if pgrep -f nats-server > /dev/null; then
        log_info "✓ NATS server 启动成功"
    else
        log_error "NATS server 启动失败"
        exit 1
    fi
fi

# 启动 CTP MD Gateway
log_info "[4/8] 启动 CTP 行情网关..."
./gateway/build/ctp_md_gateway \
    -c config/ctp/ctp_md.yaml \
    -s config/ctp/ctp_md.secret.yaml \
    > log/ctp_md_gateway.log 2>&1 &
CTP_MD_PID=$!
sleep 5

if ps -p $CTP_MD_PID > /dev/null; then
    log_info "✓ CTP 行情网关启动成功 (PID: $CTP_MD_PID)"
else
    log_error "CTP 行情网关启动失败"
    tail -20 log/ctp_md_gateway.log
    exit 1
fi

# 检查CTP登录状态
log_info "等待CTP行情登录..."
sleep 5
if grep -q "Login successful" log/ctp_md_gateway.log; then
    log_info "✓ CTP 行情登录成功"
else
    log_warn "CTP 行情登录状态不明确，检查日志："
    tail -10 log/ctp_md_gateway.log
fi

# 启动 MD Gateway
log_info "[5/8] 启动 MD Gateway..."
./gateway/build/md_gateway > log/md_gateway.log 2>&1 &
MD_GW_PID=$!
sleep 2

if ps -p $MD_GW_PID > /dev/null; then
    log_info "✓ MD Gateway 启动成功 (PID: $MD_GW_PID)"
else
    log_error "MD Gateway 启动失败"
    tail -20 log/md_gateway.log
    exit 1
fi

# 启动 ORS Gateway
log_info "[6/8] 启动 ORS Gateway..."
./gateway/build/ors_gateway > log/ors_gateway.log 2>&1 &
ORS_GW_PID=$!
sleep 2

if ps -p $ORS_GW_PID > /dev/null; then
    log_info "✓ ORS Gateway 启动成功 (PID: $ORS_GW_PID)"
else
    log_error "ORS Gateway 启动失败"
    tail -20 log/ors_gateway.log
    exit 1
fi

# 启动 Counter Bridge (CTP Plugin)
log_info "[7/8] 启动 Counter Bridge (CTP Plugin)..."
./gateway/build/counter_bridge \
    ctp:config/ctp/ctp_td.yaml:config/ctp/ctp_td.secret.yaml \
    > log/counter_bridge_ctp.log 2>&1 &
CB_PID=$!
sleep 8

if ps -p $CB_PID > /dev/null; then
    log_info "✓ Counter Bridge (CTP) 启动成功 (PID: $CB_PID)"
else
    log_error "Counter Bridge 启动失败"
    tail -30 log/counter_bridge_ctp.log
    exit 1
fi

# 检查CTP交易登录状态
log_info "等待CTP交易登录..."
sleep 5
if grep -q "Login successful" log/counter_bridge_ctp.log; then
    log_info "✓ CTP 交易登录成功"
    # 检查持仓查询
    if grep -q "Position updated from CTP" log/counter_bridge_ctp.log; then
        log_info "✓ 持仓查询成功"
        grep "Position:" log/counter_bridge_ctp.log | tail -5
    fi
else
    log_warn "CTP 交易登录状态不明确，检查日志："
    tail -20 log/counter_bridge_ctp.log
fi

# 启动 Trader (实盘配置)
log_info "[8/8] 启动 Trader (实盘配置)..."
./bin/trader -config config/trader.live.yaml > log/trader.live.log 2>&1 &
TRADER_PID=$!
sleep 5

if ps -p $TRADER_PID > /dev/null; then
    log_info "✓ Trader 启动成功 (PID: $TRADER_PID)"
else
    log_error "Trader 启动失败"
    tail -30 log/trader.live.log
    exit 1
fi

# 验证系统状态
log_info "[9/9] 验证系统状态..."
sleep 3

log_section "✓ CTP实盘交易系统启动成功"

echo ""
echo "系统信息："
echo "  - Dashboard:        http://localhost:9201/dashboard"
echo "  - API Endpoint:     http://localhost:9201/api/v1"
echo "  - Position Query:   http://localhost:8080/positions"
echo ""
echo "进程信息："
echo "  - CTP MD Gateway:   PID $CTP_MD_PID"
echo "  - MD Gateway:       PID $MD_GW_PID"
echo "  - ORS Gateway:      PID $ORS_GW_PID"
echo "  - Counter Bridge:   PID $CB_PID"
echo "  - Trader:           PID $TRADER_PID"
echo ""
echo "快速命令："
echo "  - 查看策略状态: curl http://localhost:9201/api/v1/strategies | jq ."
echo "  - 激活策略:     curl -X POST http://localhost:9201/api/v1/strategies/<策略ID>/activate"
echo "  - 查看持仓:     curl http://localhost:9201/api/v1/positions | jq ."
echo "  - 实时订单:     tail -f log/trader.live.log | grep 'Order sent'"
echo "  - 实时行情:     tail -f log/trader.live.log | grep 'market data'"
echo "  - 策略统计:     tail -f log/trader.live.log | grep 'Stats:'"
echo ""
echo "⚠️  重要提示："
echo "  1. 策略默认未激活，需要手动激活"
echo "  2. 实盘配置更保守 (entry_zscore=2.5)，信号较少"
echo "  3. 首次使用请小仓位测试"
echo "  4. 密切监控前几笔订单"
echo "  5. 按 Ctrl+C 停止所有服务"
echo ""

# 等待用户中断
wait

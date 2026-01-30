#!/bin/bash
# ============================================
# 脚本名称: stop_demo.sh
# 用途: 停止模拟交易系统（演示模式）
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - @docs/核心文档/QUICKSTART.md
# ============================================

set -e

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# ============================================
# 主逻辑
# ============================================

echo ""
echo "╔═══════════════════════════════════════════════════════════╗"
echo "║  停止模拟交易系统                                         ║"
echo "╚═══════════════════════════════════════════════════════════╝"
echo ""

log_info "Stopping all services..."

# 停止各个组件
pkill -9 -f "trader.*demo" 2>/dev/null && log_info "✓ Trader stopped" || log_warn "Trader not running"
pkill -9 -f "counter_bridge.*simulator" 2>/dev/null && log_info "✓ Counter Bridge stopped" || log_warn "Counter Bridge not running"
pkill -9 -f "ors_gateway" 2>/dev/null && log_info "✓ ORS Gateway stopped" || log_warn "ORS Gateway not running"
pkill -9 -f "md_gateway" 2>/dev/null && log_info "✓ MD Gateway stopped" || log_warn "MD Gateway not running"
pkill -9 -f "md_simulator" 2>/dev/null && log_info "✓ MD Simulator stopped" || log_warn "MD Simulator not running"
pkill -9 -f "nats-server" 2>/dev/null && log_info "✓ NATS stopped" || log_warn "NATS not running"

sleep 2

# 清理共享内存
log_info "Cleaning shared memory..."
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs ipcrm -m 2>/dev/null || true
log_info "✓ Shared memory cleaned"

# 验证
REMAINING=$(ps aux | grep -E "nats-server|md_simulator|md_gateway|ors_gateway|counter_bridge.*simulator|trader.*demo" | grep -v grep | wc -l)

echo ""
if [ $REMAINING -eq 0 ]; then
    log_info "═══════════════════════════════════════════════════════════"
    log_info "✓ All services stopped successfully"
    log_info "═══════════════════════════════════════════════════════════"
else
    log_warn "═══════════════════════════════════════════════════════════"
    log_warn "⚠ Warning: $REMAINING processes still running"
    log_warn "═══════════════════════════════════════════════════════════"
    ps aux | grep -E "nats-server|md_simulator|md_gateway|ors_gateway|counter_bridge.*simulator|trader.*demo" | grep -v grep
fi
echo ""

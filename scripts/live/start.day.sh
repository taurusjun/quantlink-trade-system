#!/bin/bash
# ============================================
# 脚本名称: start.day.sh
# 用途: 日盘启动脚本 - 对齐 C++ TradeBot_China/bin/start.day.sh
# 作者: QuantLink Team
# 日期: 2026-02-10
#
# 相关文档:
#   - @docs/核心文档/USAGE.md
#   - @docs/实盘/
#
# C++ 原始启动命令：
# ./TradeBot --Live \
#   --controlFile ./controls/day/control.ag2602.ag2604.par.txt.92201 \
#   --strategyID 92201 \
#   --configFile ./config/config_CHINA.92201.cfg \
#   --logFile ./log/log.92201.20241226
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

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# 设置系统限制
ulimit -c unlimited

# 创建必要目录
mkdir -p log
mkdir -p data/positions

DATE=$(date +%Y%m%d)

log_info "=========================================="
log_info "日盘启动脚本 - $(date)"
log_info "=========================================="

# ========================================
# 策略 92201 - ag2602/ag2604 配对套利
# ========================================
STRATEGY_ID="92201"
CONTROL_FILE="./bin/controls/day/control.ag2602.ag2604.par.txt.92201"
CONFIG_FILE="./bin/config/trader.92201.yaml"
LOG_FILE="./log/trader.${STRATEGY_ID}.${DATE}.log"

if [ -f "$CONTROL_FILE" ]; then
    log_info "Starting strategy ${STRATEGY_ID}..."
    log_info "  Control: ${CONTROL_FILE}"
    log_info "  Config:  ${CONFIG_FILE}"
    log_info "  Log:     ${LOG_FILE}"

    nohup ./bin/trader \
        --Live \
        --controlFile "$CONTROL_FILE" \
        --strategyID "$STRATEGY_ID" \
        --config "$CONFIG_FILE" \
        --log-file "$LOG_FILE" \
        >> "nohup.out.${STRATEGY_ID}" 2>&1 &

    PID=$!
    echo $PID > "trader.${STRATEGY_ID}.pid"
    log_info "Strategy ${STRATEGY_ID} started with PID: ${PID}"
else
    log_warn "Control file not found: ${CONTROL_FILE}, skipping strategy ${STRATEGY_ID}"
fi

# ========================================
# 添加更多策略...
# ========================================
# 复制上面的模板，修改 STRATEGY_ID, CONTROL_FILE, CONFIG_FILE

log_info "=========================================="
log_info "日盘启动完成"
log_info "=========================================="
log_info "查看日志: tail -f log/trader.*.${DATE}.log"
log_info "停止所有: ./scripts/live/stop_all.sh"

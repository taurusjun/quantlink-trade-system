#!/bin/bash
# ============================================
# 脚本名称: start_strategy.sh
# 用途: 通用策略启动脚本 - 对齐 C++ TradeBot 启动方式
# 作者: QuantLink Team
# 日期: 2026-02-10
#
# 相关文档:
#   - @docs/核心文档/USAGE.md
#   - @docs/实盘/
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

# 使用说明
usage() {
    echo "Usage: $0 <strategy_id> <control_file> [config_file]"
    echo ""
    echo "Arguments:"
    echo "  strategy_id   策略ID (例如: 92201)"
    echo "  control_file  Control 文件路径 (例如: ./bin/controls/day/control.ag2502.ag2504.par.txt.92201)"
    echo "  config_file   配置文件路径 (可选, 默认: config/trader.yaml)"
    echo ""
    echo "Examples:"
    echo "  $0 92201 ./bin/controls/day/control.ag2502.ag2504.par.txt.92201"
    echo "  $0 92201 ./bin/controls/day/control.ag2502.ag2504.par.txt.92201 config/trader.92201.yaml"
    exit 1
}

# 参数检查
if [ $# -lt 2 ]; then
    usage
fi

STRATEGY_ID=$1
CONTROL_FILE=$2
CONFIG_FILE=${3:-"config/trader.yaml"}

# 验证文件存在
if [ ! -f "$CONTROL_FILE" ]; then
    log_error "Control file not found: $CONTROL_FILE"
    exit 1
fi

if [ ! -f "$CONFIG_FILE" ]; then
    log_warn "Config file not found: $CONFIG_FILE, using default settings"
fi

# 生成日志文件路径
LOG_DIR="./log"
mkdir -p "$LOG_DIR"
DATE=$(date +%Y%m%d)
LOG_FILE="${LOG_DIR}/trader.${STRATEGY_ID}.${DATE}.log"

log_info "Starting strategy ${STRATEGY_ID}..."
log_info "  Control file: ${CONTROL_FILE}"
log_info "  Config file:  ${CONFIG_FILE}"
log_info "  Log file:     ${LOG_FILE}"

# 启动 trader
# 对齐 C++ TradeBot 命令行格式：
# ./TradeBot --Live --controlFile <control> --strategyID <id> --configFile <config> --logFile <log>
./bin/trader \
    --Live \
    --controlFile "$CONTROL_FILE" \
    --strategyID "$STRATEGY_ID" \
    --config "$CONFIG_FILE" \
    --log-file "$LOG_FILE"

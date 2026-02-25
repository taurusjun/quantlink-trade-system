#!/bin/bash
# ============================================
# 脚本名称: build_deploy_new.sh
# 用途: 一键编译部署到 deploy_new 目录（代码与数据分离）
# 日期: 2026-02-14
#
# 使用方式:
#   ./scripts/build_deploy_new.sh                    # 完整编译（默认 sim 模式）
#   ./scripts/build_deploy_new.sh --mode live        # 完整编译（实盘模式）
#   ./scripts/build_deploy_new.sh --go --mode sim    # 仅 Go + 模拟盘配置
#   ./scripts/build_deploy_new.sh --cpp --mode live  # 仅 C++ + 实盘配置
#   ./scripts/build_deploy_new.sh --clean            # 清理后重新编译
#
# 目录设计:
#   deploy_new/  - 编译产物 + 脚本 + Web资源（代码变动时重建）
#   data_new/    - 配置 + 模型 + 数据（三层结构: common/sim/live）
#     common/    - 两种模式共享（config_CHINA.*.cfg, controls, models）
#     sim/       - 模拟盘专用（simulator.yaml, sim daily_init）
#     live/      - 实盘专用（ctp/*.yaml, live daily_init, ctp_flow）
#
#   --mode 参数控制 data_new 合并: common + sim|live → deploy_new
#   deploy_new/ 可直接运行或部署到新服务器
#
# 启动方式（模拟与实盘一致）:
#   cd deploy_new
#   ./scripts/start_gateway.sh sim         # 模拟网关
#   ./scripts/start_gateway.sh ctp         # CTP实盘网关
#   ./scripts/start_strategy.sh 92201 day  # 启动策略（日盘）
#   ./scripts/stop_all.sh                  # 停止所有
#
# 相关文档:
#   - @docs/核心文档/BUILD_GUIDE.md
#   - @docs/核心文档/USAGE.md
# ============================================

set -e

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_section() {
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
}

# 配置
DEPLOY_DIR="${PROJECT_ROOT}/deploy_new"
DATA_DIR="${PROJECT_ROOT}/data_new"
BUILD_GO=true
BUILD_CPP=true
CLEAN_BUILD=false
DEPLOY_MODE="sim"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --go)    BUILD_GO=true; BUILD_CPP=false; shift ;;
        --cpp)   BUILD_GO=false; BUILD_CPP=true; shift ;;
        --clean) CLEAN_BUILD=true; shift ;;
        --mode)
            if [[ -z "$2" || "$2" == --* ]]; then
                log_error "--mode 需要参数: sim 或 live"
                exit 1
            fi
            if [[ "$2" != "sim" && "$2" != "live" ]]; then
                log_error "--mode 只接受 sim 或 live（当前: $2）"
                exit 1
            fi
            DEPLOY_MODE="$2"; shift 2 ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --mode sim|live  部署模式（默认: sim）"
            echo "  --go             仅编译 Go 组件"
            echo "  --cpp            仅编译 C++ 组件"
            echo "  --clean          清理后重新编译"
            echo "  --help           显示帮助"
            echo ""
            echo "目录结构: data_new/{common,sim,live} → deploy_new/"
            echo "  common/ 始终复制，sim/ 或 live/ 按 --mode 选择"
            exit 0
            ;;
        *) log_error "Unknown option: $1"; exit 1 ;;
    esac
done

# 显示构建信息
log_section "QuantLink Trade System - 编译部署 (deploy_new)"
log_info "项目根目录: ${PROJECT_ROOT}"
log_info "部署目录:   ${DEPLOY_DIR}"
log_info "数据目录:   ${DATA_DIR}"
log_info "部署模式:   ${DEPLOY_MODE}"
log_info "编译 Go:    ${BUILD_GO}"
log_info "编译 C++:   ${BUILD_CPP}"

# ==================== 清理 ====================
if [ "$CLEAN_BUILD" = true ]; then
    log_info "清理 deploy_new 目录..."
    rm -rf "${DEPLOY_DIR}"
fi

# ==================== 创建 deploy_new 目录结构 ====================
log_section "创建 deploy_new 目录结构"
mkdir -p "${DEPLOY_DIR}/bin"
mkdir -p "${DEPLOY_DIR}/web"
mkdir -p "${DEPLOY_DIR}/scripts"
mkdir -p "${DEPLOY_DIR}/lib"
mkdir -p "${DEPLOY_DIR}/log"
log_info "目录结构创建完成"

# ==================== 复制 Web 资源 ====================
if [ -d "${PROJECT_ROOT}/tbsrc-golang/web" ]; then
    cp "${PROJECT_ROOT}/tbsrc-golang/web/"*.html "${DEPLOY_DIR}/web/" 2>/dev/null || true
    log_info "  Web 资源: $(ls "${DEPLOY_DIR}/web/" 2>/dev/null | wc -l | tr -d ' ') 个 HTML 文件"
fi

# ==================== 编译 C++ ====================
if [ "$BUILD_CPP" = true ]; then
    log_section "编译 C++ 网关组件"

    GATEWAY_DIR="${PROJECT_ROOT}/gateway"
    GATEWAY_BUILD="${GATEWAY_DIR}/build"

    if [ "$CLEAN_BUILD" = true ]; then
        log_info "清理 gateway/build 目录..."
        rm -rf "${GATEWAY_BUILD}"
    fi

    mkdir -p "${GATEWAY_BUILD}"
    cd "${GATEWAY_BUILD}"

    log_info "运行 CMake..."
    cmake .. -DCMAKE_BUILD_TYPE=Release

    log_info "编译中..."
    make -j$(sysctl -n hw.ncpu 2>/dev/null || nproc) \
        md_shm_feeder md_gateway md_simulator ors_gateway counter_bridge ctp_md_gateway md_benchmark 2>&1 || true

    # 复制编译产物
    log_info "复制 C++ 可执行文件..."
    CORE_COMPONENTS="md_shm_feeder md_gateway md_simulator ors_gateway counter_bridge ctp_md_gateway"
    for comp in $CORE_COMPONENTS; do
        if [ -f "$comp" ]; then
            cp "$comp" "${DEPLOY_DIR}/bin/"
            log_info "  $comp"
        else
            log_error "  $comp 编译失败！"
            exit 1
        fi
    done
    [ -f "md_benchmark" ] && cp md_benchmark "${DEPLOY_DIR}/bin/" && log_info "  md_benchmark"

    cd "${PROJECT_ROOT}"

    # 复制 CTP 动态库（macOS）
    if [ "$(uname)" = "Darwin" ]; then
        CTP_FRAMEWORK_DIR="${GATEWAY_DIR}/third_party/ctp"
        if [ -d "${CTP_FRAMEWORK_DIR}" ]; then
            log_info "复制 CTP Framework..."
            mkdir -p "${DEPLOY_DIR}/lib/ctp"
            cp -R "${CTP_FRAMEWORK_DIR}"/*.framework "${DEPLOY_DIR}/lib/ctp/" 2>/dev/null || true
        fi
    fi

    log_info "C++ 组件编译完成"
fi

# ==================== 编译 Go ====================
if [ "$BUILD_GO" = true ]; then
    log_section "编译 Go 策略组件"

    cd "${PROJECT_ROOT}/tbsrc-golang"

    log_info "编译 trader..."
    go build -o "${DEPLOY_DIR}/bin/trader" ./cmd/trader/main.go
    log_info "  trader"

    log_info "编译 webserver..."
    go build -o "${DEPLOY_DIR}/bin/webserver" ./cmd/webserver/main.go
    log_info "  webserver"

    log_info "编译 backtest..."
    go build -o "${DEPLOY_DIR}/bin/backtest" ./cmd/backtest/main.go 2>/dev/null || log_warn "  backtest (可选，跳过)"

    log_info "编译 backtest_optimize..."
    go build -o "${DEPLOY_DIR}/bin/backtest_optimize" ./cmd/backtest_optimize/main.go 2>/dev/null || log_warn "  backtest_optimize (可选，跳过)"

    cd "${PROJECT_ROOT}"
    log_info "Go 组件编译完成"
fi

# ==================== 生成启动脚本 ====================
log_section "生成启动脚本"

# --- start_gateway.sh ---
cat > "${DEPLOY_DIR}/scripts/start_gateway.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 启动网关层（tbsrc-golang 架构：SysV SHM 直连）
# Usage: ./scripts/start_gateway.sh [sim|ctp]
#
# sim  - 模拟环境（md_shm_feeder simulator + counter_bridge simulator）
# ctp  - CTP实盘（md_shm_feeder ctp + counter_bridge ctp）
#
# 数据流:
#   md_shm_feeder → [SysV SHM 0x1001] → trader
#   trader → [SysV SHM 0x2001/0x3001] → counter_bridge
# ============================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

MODE=${1:-sim}
DATE=$(date +%Y%m%d)

if [ "$MODE" != "sim" ] && [ "$MODE" != "ctp" ]; then
    echo "Usage: $0 [sim|ctp]"
    echo "  sim  - 模拟环境（默认）"
    echo "  ctp  - CTP实盘"
    exit 1
fi

# 记录当前运行模式（供 start_strategy.sh 读取）
echo "$MODE" > .gateway_mode

echo ""
echo "════════════════════════════════════════════════════════════"
if [ "$MODE" = "sim" ]; then
    echo "  QuantLink Trade System - 模拟环境 (SHM Direct)"
else
    echo -e "  QuantLink Trade System - ${RED}CTP 实盘环境${NC} (SHM Direct)"
fi
echo "════════════════════════════════════════════════════════════"
echo ""

# CTP 实盘确认
if [ "$MODE" = "ctp" ]; then
    CTP_MD_CONFIG="config/ctp/ctp_md.secret.yaml"
    CTP_TD_CONFIG="config/ctp/ctp_td.secret.yaml"

    if [ ! -f "$CTP_MD_CONFIG" ] || [ ! -f "$CTP_TD_CONFIG" ]; then
        echo -e "${RED}[ERROR]${NC} CTP 配置文件不存在"
        echo "  需要: $CTP_MD_CONFIG"
        echo "  需要: $CTP_TD_CONFIG"
        echo "  请先从模板创建: cp config/ctp/ctp_md.yaml $CTP_MD_CONFIG"
        exit 1
    fi

    read -p "确认启动 CTP 实盘? (y/N): " confirm
    [ "$confirm" != "y" ] && [ "$confirm" != "Y" ] && exit 0
fi

# 清理共享内存
ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true

mkdir -p log ctp_flow

# 1. MD SHM Feeder (行情 → SysV SHM 0x1001)
if [ "$MODE" = "sim" ]; then
    # 从 control 文件提取合约列表（C++ 格式: token0=baseName, token7=secondName）
    # baseName 格式: ag_F_3_SFE → 提取 control 文件名中的合约: control.ag2603.ag2605.par.txt.92201
    SYMBOLS=""
    for ctrl in controls/day/control.*.par.txt.*; do
        [ -f "$ctrl" ] || continue
        # 从 control 文件名提取合约: control.<sym1>.<sym2>.par.txt.<id>
        fname=$(basename "$ctrl")
        # 去掉 "control." 前缀和 ".par.txt.NNNNN" 后缀
        syms="${fname#control.}"
        syms="${syms%.par.txt.*}"
        # syms = "ag2603.ag2605" → 替换 . 为 ,
        syms=$(echo "$syms" | tr '.' ',')
        [ -n "$SYMBOLS" ] && SYMBOLS="${SYMBOLS},"
        SYMBOLS="${SYMBOLS}${syms}"
    done
    if [ -z "$SYMBOLS" ]; then
        SYMBOLS="ag2603,ag2605,au2604,au2606"
    fi
    # macOS SHM limit: use smaller queue (4096), Linux can use 65536
    QUEUE_SIZE=2048
    if [ "$(uname)" = "Linux" ]; then
        QUEUE_SIZE=65536
    fi
    ./bin/md_shm_feeder "simulator:${SYMBOLS}" --queue-size "$QUEUE_SIZE" > "log/md_shm_feeder.${DATE}.log" 2>&1 &
    sleep 1
    echo -e "${GREEN}[INFO]${NC} MD SHM Feeder (Simulator: ${SYMBOLS}, queue=${QUEUE_SIZE})"
else
    QUEUE_SIZE=2048
    if [ "$(uname)" = "Linux" ]; then
        QUEUE_SIZE=65536
    fi
    ./bin/md_shm_feeder "ctp:${CTP_MD_CONFIG}" --queue-size "$QUEUE_SIZE" > "log/md_shm_feeder.${DATE}.log" 2>&1 &
    sleep 2
    echo -e "${GREEN}[INFO]${NC} MD SHM Feeder (CTP, queue=${QUEUE_SIZE})"
fi

# 2. Counter Bridge (SysV MWMR 0x2001/0x3001/0x4001)
if [ "$MODE" = "sim" ]; then
    ./bin/counter_bridge simulator:config/simulator.yaml > "log/counter_bridge.${DATE}.log" 2>&1 &
    sleep 1
    echo -e "${GREEN}[INFO]${NC} Counter Bridge (Simulator)"
else
    ./bin/counter_bridge ctp:"$CTP_TD_CONFIG" > "log/counter_bridge.${DATE}.log" 2>&1 &
    sleep 2
    echo -e "${GREEN}[INFO]${NC} Counter Bridge (CTP)"
fi

# 4. Web Server (Overview Dashboard on port 8080)
if [ -f ./bin/webserver ]; then
    pkill -f "webserver.*-port 8080" 2>/dev/null || true
    sleep 0.5
    ./bin/webserver -port 8080 > "log/webserver.${DATE}.log" 2>&1 &
    sleep 0.5
    echo -e "${GREEN}[INFO]${NC} Web Server (Overview: http://localhost:8080)"
fi

echo ""
echo -e "${GREEN}[INFO]${NC} 网关层启动完成 (${MODE})"
echo -e "${GREEN}[INFO]${NC} Overview:  http://localhost:8080"
echo -e "${GREEN}[INFO]${NC} 启动策略: ./scripts/start_strategy.sh <strategy_id> [day|night]"
echo ""
SCRIPT_EOF

# --- start_strategy.sh ---
cat > "${DEPLOY_DIR}/scripts/start_strategy.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 启动策略（对齐 C++ TradeBot 启动方式）
#
# Usage:
#   ./scripts/start_strategy.sh <strategy_id> [day|night] [--fg]
#
# C++ 原方式:
#   ./TradeBot --Live --controlFile ./controls/xxx --strategyID 92201 \
#              --configFile ./config/config_CHINA.92201.cfg \
#              --adjustLTP 1 --printMod 1 --updateInterval 300000
#
# Examples:
#   ./scripts/start_strategy.sh 92201              # 自动检测 day/night
#   ./scripts/start_strategy.sh 92201 day          # 日盘
#   ./scripts/start_strategy.sh 92201 night --fg   # 夜盘前台调试
#   ./scripts/start_strategy.sh 92202
#
# 自动查找:
#   controlFile: controls/{session}/control.*.par.txt.{strategyID}
#   configFile:  config/config_CHINA.{strategyID}.cfg
# ============================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

if [ $# -lt 1 ]; then
    echo "Usage: $0 <strategy_id> [day|night] [--fg]"
    echo ""
    echo "  strategy_id  策略ID，如 92201"
    echo "  day|night    交易时段（默认自动检测）"
    echo "  --fg         前台运行（调试用）"
    echo ""
    echo "Examples:"
    echo "  $0 92201"
    echo "  $0 92201 day"
    echo "  $0 92202 night --fg"
    echo ""
    echo "可用策略:"
    for f in controls/day/control.*.par.txt.*; do
        [ -f "$f" ] || continue
        sid="${f##*.}"
        fname=$(basename "$f")
        echo "  ${sid}  (${fname})"
    done
    exit 1
fi

STRATEGY_ID=$1
SESSION=""
FOREGROUND=false
shift
for arg in "$@"; do
    case "$arg" in
        day|night) SESSION="$arg" ;;
        --fg)      FOREGROUND=true ;;
    esac
done

# 自动检测 session: 20:00~04:00 → night, 其他 → day
if [ -z "$SESSION" ]; then
    HOUR=$(date +%H)
    if [ "$HOUR" -ge 20 ] || [ "$HOUR" -lt 4 ]; then
        SESSION="night"
    else
        SESSION="day"
    fi
fi

DATE=$(date +%Y%m%d)
# 年份前两位 → yearPrefix (C++ baseName→symbol 映射需要)
YEAR_PREFIX=$(date +%y)

# 自动查找 controlFile: controls/{session}/control.*.par.txt.{strategyID}
CONTROL_FILE=""
for f in controls/${SESSION}/control.*.par.txt.${STRATEGY_ID}; do
    [ -f "$f" ] && CONTROL_FILE="$f" && break
done

if [ -z "$CONTROL_FILE" ]; then
    echo -e "${RED}[ERROR]${NC} 找不到 controlFile: controls/${SESSION}/control.*.par.txt.${STRATEGY_ID}"
    echo ""
    echo "可用 control 文件 (${SESSION}):"
    ls controls/${SESSION}/control.*.par.txt.* 2>/dev/null || echo "  (无)"
    exit 1
fi

# configFile: config/config_CHINA.{strategyID}.cfg
CONFIG_FILE="config/config_CHINA.${STRATEGY_ID}.cfg"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}[ERROR]${NC} 找不到 configFile: ${CONFIG_FILE}"
    exit 1
fi

LOG_FILE="./log/trader.${STRATEGY_ID}.${DATE}.log"

# 读取网关运行模式，确定数据目录
if [ ! -f .gateway_mode ]; then
    echo -e "${RED}[ERROR]${NC} .gateway_mode 文件不存在，请先启动网关: ./scripts/start_gateway.sh [sim|ctp]"
    exit 1
fi
GATEWAY_MODE=$(cat .gateway_mode)
case "$GATEWAY_MODE" in
    sim) DATA_DIR="./data/sim" ;;
    ctp) DATA_DIR="./data/live" ;;
    *)   echo -e "${RED}[ERROR]${NC} .gateway_mode 内容无效: ${GATEWAY_MODE}"; exit 1 ;;
esac

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  启动策略 ${STRATEGY_ID} (${SESSION})"
echo "════════════════════════════════════════════════════════════"
echo -e "${GREEN}[INFO]${NC} Strategy ID:  ${STRATEGY_ID}"
echo -e "${GREEN}[INFO]${NC} Session:      ${SESSION}"
echo -e "${GREEN}[INFO]${NC} Gateway Mode: ${GATEWAY_MODE}"
echo -e "${GREEN}[INFO]${NC} Data Dir:     ${DATA_DIR}"
echo -e "${GREEN}[INFO]${NC} ControlFile:  ${CONTROL_FILE}"
echo -e "${GREEN}[INFO]${NC} ConfigFile:   ${CONFIG_FILE}"
echo -e "${GREEN}[INFO]${NC} YearPrefix:   ${YEAR_PREFIX}"
echo -e "${GREEN}[INFO]${NC} Log:          ${LOG_FILE}"
echo ""

mkdir -p log "${DATA_DIR}"

if [ "$FOREGROUND" = true ]; then
    echo -e "${YELLOW}[INFO]${NC} 前台模式 (Ctrl+C 停止)"
    ./bin/trader --Live \
        -controlFile "$CONTROL_FILE" \
        -strategyID "$STRATEGY_ID" \
        -configFile "$CONFIG_FILE" \
        -dataDir "$DATA_DIR" \
        -yearPrefix "$YEAR_PREFIX" \
        -adjustLTP 1 \
        -printMod 1 \
        -updateInterval 300000 \
        -logFile "$LOG_FILE" \
        2>&1 | tee -a "$LOG_FILE"
else
    ulimit -c unlimited 2>/dev/null || true
    nohup ./bin/trader --Live \
        -controlFile "$CONTROL_FILE" \
        -strategyID "$STRATEGY_ID" \
        -configFile "$CONFIG_FILE" \
        -dataDir "$DATA_DIR" \
        -yearPrefix "$YEAR_PREFIX" \
        -adjustLTP 1 \
        -printMod 1 \
        -updateInterval 300000 \
        -logFile "$LOG_FILE" \
        >> "nohup.out.${STRATEGY_ID}" 2>&1 &

    PID=$!
    echo -e "${GREEN}[INFO]${NC} 策略已在后台启动 (PID: ${PID})"
    echo -e "${GREEN}[INFO]${NC} 查看日志: tail -f nohup.out.${STRATEGY_ID}"

    # 等待几秒检查进程是否存活
    sleep 2
    if kill -0 $PID 2>/dev/null; then
        echo -e "${GREEN}[INFO]${NC} 策略进程运行正常"
    else
        echo -e "${RED}[ERROR]${NC} 策略进程已退出，请检查日志"
        tail -20 "nohup.out.${STRATEGY_ID}" 2>/dev/null
        exit 1
    fi
fi
SCRIPT_EOF

# --- start_all.sh ---
cat > "${DEPLOY_DIR}/scripts/start_all.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 一键启动所有（网关 + 所有策略）
#
# Usage:
#   ./scripts/start_all.sh sim [day|night]  # 模拟环境，启动所有策略
#   ./scripts/start_all.sh ctp [day|night]  # CTP实盘，启动所有策略
# ============================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
NC='\033[0m'

MODE=${1:-sim}
SESSION=${2:-}

# 自动检测 session
if [ -z "$SESSION" ]; then
    HOUR=$(date +%H)
    if [ "$HOUR" -ge 20 ] || [ "$HOUR" -lt 4 ]; then
        SESSION="night"
    else
        SESSION="day"
    fi
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  一键启动: 模式=${MODE}  时段=${SESSION}"
echo "════════════════════════════════════════════════════════════"
echo ""

# 1. 启动网关
./scripts/start_gateway.sh "$MODE"

sleep 2

# 2. 启动所有策略（从 config/config_CHINA.*.cfg 发现策略 ID）
FOUND=0
for cfg_file in config/config_CHINA.*.cfg; do
    [ -f "$cfg_file" ] || continue
    # 提取策略ID: config/config_CHINA.92201.cfg -> 92201
    fname=$(basename "$cfg_file")
    STRATEGY_ID="${fname#config_CHINA.}"
    STRATEGY_ID="${STRATEGY_ID%.cfg}"
    [ -z "$STRATEGY_ID" ] && continue
    # 检查对应的 control 文件是否存在
    CTRL_FOUND=false
    for ctrl in controls/${SESSION}/control.*.par.txt.${STRATEGY_ID}; do
        [ -f "$ctrl" ] && CTRL_FOUND=true && break
    done
    if [ "$CTRL_FOUND" = false ]; then
        echo -e "${GREEN}[INFO]${NC} 跳过策略 ${STRATEGY_ID}（无 ${SESSION} control 文件）"
        continue
    fi
    FOUND=1
    echo -e "${GREEN}[INFO]${NC} 启动策略 ${STRATEGY_ID} (${SESSION})..."
    ./scripts/start_strategy.sh "$STRATEGY_ID" "$SESSION"
    sleep 1
done

if [ "$FOUND" -eq 0 ]; then
    echo -e "${GREEN}[INFO]${NC} 未找到策略配置 (config/config_CHINA.*.cfg + controls/${SESSION}/)"
fi

echo ""
echo -e "${GREEN}[INFO]${NC} 所有组件启动完成"
echo -e "${GREEN}[INFO]${NC} 查看状态: ps aux | grep -E 'trader|gateway|bridge'"
echo -e "${GREEN}[INFO]${NC} 停止所有: ./scripts/stop_all.sh"
echo ""
SCRIPT_EOF

# --- stop_all.sh ---
cat > "${DEPLOY_DIR}/scripts/stop_all.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 停止所有组件
# ============================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  停止 QuantLink Trade System"
echo "════════════════════════════════════════════════════════════"
echo ""

# 先停 trader（触发 graceful shutdown 保存 daily_init）
for proc in trader; do
    pids=$(pgrep -f "$proc" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        echo -e "${YELLOW}[INFO]${NC} 发送 SIGTERM 到 $proc (等待 graceful shutdown)..."
        pkill -TERM -f "$proc" 2>/dev/null || true
    fi
done

# 等待 trader 退出（最多 10 秒）
for i in $(seq 1 10); do
    if ! pgrep -f "trader" > /dev/null 2>&1; then
        break
    fi
    sleep 1
done

# 如果 trader 还没退出，强制 kill
if pgrep -f "trader" > /dev/null 2>&1; then
    echo -e "${YELLOW}[WARN]${NC} trader 未在 10 秒内退出，强制终止..."
    pkill -9 -f "trader" 2>/dev/null || true
fi
echo -e "${GREEN}[INFO]${NC} trader 已停止"

# 停止网关组件
for proc in counter_bridge ors_gateway md_gateway ctp_md_gateway md_simulator webserver; do
    if pgrep -f "$proc" > /dev/null 2>&1; then
        echo -e "${GREEN}[INFO]${NC} 停止 $proc..."
        pkill -f "$proc" 2>/dev/null || true
    fi
done

# 清理共享内存
ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true

# 清理模式标记
rm -f .gateway_mode

echo ""
echo -e "${GREEN}[INFO]${NC} 所有组件已停止"
echo ""
SCRIPT_EOF

# 设置脚本可执行权限
chmod +x "${DEPLOY_DIR}/scripts/"*.sh
log_info "  start_gateway.sh  (模拟/CTP 统一网关启动)"
log_info "  start_strategy.sh (策略启动，自动查找 control 文件)"
log_info "  start_all.sh      (一键启动网关+所有策略)"
log_info "  stop_all.sh       (停止所有，graceful shutdown)"

# ==================== 合并 data_new → deploy_new (模式: ${DEPLOY_MODE}) ====================
log_section "合并 data_new → deploy_new (模式: ${DEPLOY_MODE})"

if [ ! -d "${DATA_DIR}" ]; then
    log_error "data_new 目录不存在: ${DATA_DIR}"
    log_error "请先创建 data_new 目录并放入配置文件"
    exit 1
fi

COMMON_DIR="${DATA_DIR}/common"
MODE_DIR="${DATA_DIR}/${DEPLOY_MODE}"

if [ ! -d "${COMMON_DIR}" ]; then
    log_error "data_new/common 目录不存在: ${COMMON_DIR}"
    exit 1
fi

if [ ! -d "${MODE_DIR}" ]; then
    log_error "data_new/${DEPLOY_MODE} 目录不存在: ${MODE_DIR}"
    exit 1
fi

# 1. 复制 common（config/controls/models 总是覆盖）
for dir in config controls models; do
    if [ -d "${COMMON_DIR}/${dir}" ]; then
        cp -R "${COMMON_DIR}/${dir}" "${DEPLOY_DIR}/"
        log_info "  common/${dir}/"
    fi
done

# 2. 复制模式配置（overlay 合并到 deploy_new/config/）
if [ -d "${MODE_DIR}/config" ]; then
    cp -R "${MODE_DIR}/config/"* "${DEPLOY_DIR}/config/" 2>/dev/null || true
    log_info "  ${DEPLOY_MODE}/config/"
fi

# 3. 复制两种模式的数据到 data/sim/ 和 data/live/（保留已有运行时数据）
for data_mode in sim live; do
    src_data="${DATA_DIR}/${data_mode}/data"
    dst_data="${DEPLOY_DIR}/data/${data_mode}"
    if [ -d "${src_data}" ]; then
        mkdir -p "${dst_data}"
        find "${src_data}" -type f | while read file; do
            filename=$(basename "$file")
            target="${dst_data}/${filename}"
            if [ ! -f "$target" ]; then
                cp "$file" "$target"
            fi
        done
        log_info "  data/${data_mode}/ (保留已有数据)"
    fi
done

# 4. live 模式: 创建 ctp_flow 目录
if [ "$DEPLOY_MODE" = "live" ]; then
    mkdir -p "${DEPLOY_DIR}/ctp_flow"
    if [ -d "${DATA_DIR}/live/ctp_flow" ]; then
        cp -R "${DATA_DIR}/live/ctp_flow/"* "${DEPLOY_DIR}/ctp_flow/" 2>/dev/null || true
    fi
    log_info "  ctp_flow/"
fi

log_info "data_new 合并完成 (模式: ${DEPLOY_MODE})"

# ==================== 完成 ====================
log_section "构建完成"

echo ""
log_info "deploy_new 目录结构:"
echo ""
# 用 find 展示目录树（排除 log、lib 细节）
(
    echo "  deploy_new/"
    echo "  ├── bin/"
    ls "${DEPLOY_DIR}/bin/" 2>/dev/null | while read f; do
        echo "  │   ├── $f"
    done
    echo "  ├── config/"
    ls "${DEPLOY_DIR}/config/" 2>/dev/null | while read f; do
        if [ -d "${DEPLOY_DIR}/config/$f" ]; then
            echo "  │   ├── $f/"
        else
            echo "  │   ├── $f"
        fi
    done
    echo "  ├── controls/"
    for session in day night; do
        if [ -d "${DEPLOY_DIR}/controls/${session}" ]; then
            echo "  │   ├── ${session}/"
            ls "${DEPLOY_DIR}/controls/${session}/" 2>/dev/null | while read f; do
                echo "  │   │   ├── $f"
            done
        fi
    done
    echo "  ├── models/"
    ls "${DEPLOY_DIR}/models/" 2>/dev/null | while read f; do
        echo "  │   ├── $f"
    done
    echo "  ├── data/"
    echo "  ├── scripts/"
    ls "${DEPLOY_DIR}/scripts/" 2>/dev/null | while read f; do
        echo "  │   ├── $f"
    done
    echo "  ├── web/"
    ls "${DEPLOY_DIR}/web/" 2>/dev/null | while read f; do
        echo "  │   ├── $f"
    done
    echo "  ├── lib/"
    echo "  ├── log/"
    echo "  └── ctp_flow/"
)

echo ""
log_info "可执行文件:"
ls -lh "${DEPLOY_DIR}/bin/" 2>/dev/null | grep -v "^total" | awk '{print "  " $NF " (" $5 ")"}'

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  使用方式${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo ""
echo "  cd deploy_new"
echo ""
echo "  # 模拟测试"
echo "  ./scripts/start_gateway.sh sim"
echo "  ./scripts/start_strategy.sh 92201 day"
echo "  ./scripts/start_strategy.sh 92202 day"
echo ""
echo "  # 或一键启动"
echo "  ./scripts/start_all.sh sim day"
echo ""
echo "  # CTP 实盘"
echo "  ./scripts/start_gateway.sh ctp"
echo "  ./scripts/start_strategy.sh 92201 day"
echo ""
echo "  # 停止"
echo "  ./scripts/stop_all.sh"
echo ""
echo "  # Dashboard"
echo "  http://localhost:9201/overview"
echo "  http://localhost:9201/dashboard"
echo ""
echo "  # 部署到新服务器"
echo "  scp -r deploy_new/ user@server:/opt/quantlink/"
echo ""

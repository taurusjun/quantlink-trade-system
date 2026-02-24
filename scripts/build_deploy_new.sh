#!/bin/bash
# ============================================
# 脚本名称: build_deploy_new.sh
# 用途: 一键编译部署到 deploy_new 目录（代码与数据分离）
# 日期: 2026-02-14
#
# 使用方式:
#   ./scripts/build_deploy_new.sh          # 完整编译
#   ./scripts/build_deploy_new.sh --go     # 仅编译 Go 组件
#   ./scripts/build_deploy_new.sh --cpp    # 仅编译 C++ 组件
#   ./scripts/build_deploy_new.sh --clean  # 清理后重新编译
#
# 目录设计:
#   deploy_new/ - 编译产物 + 脚本 + Web资源（代码变动时重建）
#   data_new/   - 配置 + 模型 + 数据（配置变动时修改，持久化）
#
#   脚本结束时自动将 data_new/ 复制到 deploy_new/ 中
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

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --go)    BUILD_GO=true; BUILD_CPP=false; shift ;;
        --cpp)   BUILD_GO=false; BUILD_CPP=true; shift ;;
        --clean) CLEAN_BUILD=true; shift ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --go      仅编译 Go 组件"
            echo "  --cpp     仅编译 C++ 组件"
            echo "  --clean   清理后重新编译"
            echo "  --help    显示帮助"
            echo ""
            echo "编译产物 → deploy_new/  配置数据 → data_new/"
            echo "脚本结束时自动合并 data_new → deploy_new"
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
        md_gateway md_simulator ors_gateway counter_bridge ctp_md_gateway md_benchmark 2>&1 || true

    # 复制编译产物
    log_info "复制 C++ 可执行文件..."
    CORE_COMPONENTS="md_gateway md_simulator ors_gateway counter_bridge ctp_md_gateway"
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
# 启动网关层（模拟/CTP 统一入口）
# Usage: ./scripts/start_gateway.sh [sim|ctp]
#
# sim  - 模拟环境（md_simulator + counter_bridge simulator）
# ctp  - CTP实盘（ctp_md_gateway + counter_bridge ctp）
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

echo ""
echo "════════════════════════════════════════════════════════════"
if [ "$MODE" = "sim" ]; then
    echo "  QuantLink Trade System - 模拟环境"
else
    echo -e "  QuantLink Trade System - ${RED}CTP 实盘环境${NC}"
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

mkdir -p log

# 1. NATS
if ! pgrep -f "nats-server" > /dev/null 2>&1; then
    if command -v nats-server &> /dev/null; then
        nats-server -p 4222 > log/nats.log 2>&1 &
        sleep 1
        echo -e "${GREEN}[INFO]${NC} NATS Server"
    else
        echo -e "${RED}[ERROR]${NC} NATS 未安装: brew install nats-server"
        exit 1
    fi
else
    echo -e "${GREEN}[INFO]${NC} NATS Server (已运行)"
fi

# 2. 行情源
if [ "$MODE" = "sim" ]; then
    ./bin/md_simulator > "log/md_simulator.${DATE}.log" 2>&1 &
    sleep 1
    echo -e "${GREEN}[INFO]${NC} MD Simulator"
else
    ./bin/ctp_md_gateway --config "$CTP_MD_CONFIG" > "log/ctp_md_gateway.${DATE}.log" 2>&1 &
    sleep 2
    echo -e "${GREEN}[INFO]${NC} CTP MD Gateway"
fi

# 3. MD Gateway
./bin/md_gateway > "log/md_gateway.${DATE}.log" 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} MD Gateway"

# 4. ORS Gateway
./bin/ors_gateway > "log/ors_gateway.${DATE}.log" 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} ORS Gateway"

# 5. Counter Bridge
if [ "$MODE" = "sim" ]; then
    ./bin/counter_bridge simulator:config/simulator.yaml > "log/counter_bridge.${DATE}.log" 2>&1 &
    sleep 1
    echo -e "${GREEN}[INFO]${NC} Counter Bridge (Simulator)"
else
    ./bin/counter_bridge ctp:"$CTP_TD_CONFIG" > "log/counter_bridge.${DATE}.log" 2>&1 &
    sleep 2
    echo -e "${GREEN}[INFO]${NC} Counter Bridge (CTP)"
fi

# 6. Web Server (Overview Dashboard on port 8080)
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
echo -e "${GREEN}[INFO]${NC} 启动策略: ./scripts/start_strategy.sh <strategy_id> <session>"
echo ""
SCRIPT_EOF

# --- start_strategy.sh ---
cat > "${DEPLOY_DIR}/scripts/start_strategy.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 启动策略（模拟/实盘 统一命令）
#
# Usage:
#   ./scripts/start_strategy.sh <strategy_id> [session] [--fg]
#
# Examples:
#   ./scripts/start_strategy.sh 92201 day        # 日盘，后台运行
#   ./scripts/start_strategy.sh 92201 night      # 夜盘，后台运行
#   ./scripts/start_strategy.sh 92201 day --fg   # 日盘，前台运行（调试）
#   ./scripts/start_strategy.sh 92202 night
#
# 自动查找:
#   controls/{session}/control.*.{strategy_id}
#   models/model.*.{strategy_id}
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
    echo "Usage: $0 <strategy_id> [session] [--fg]"
    echo ""
    echo "  strategy_id  策略ID，如 92201"
    echo "  session      交易时段: day (默认) 或 night"
    echo "  --fg         前台运行（调试用）"
    echo ""
    echo "Examples:"
    echo "  $0 92201 day"
    echo "  $0 92201 night"
    echo "  $0 92202 day --fg"
    echo ""
    echo "可用策略:"
    for f in controls/day/control.*.* controls/night/control.*.*; do
        [ -f "$f" ] || continue
        sid="${f##*.}"
        session_dir="$(basename "$(dirname "$f")")"
        symbols="${f#*control.}"
        symbols="${symbols%.par.txt.*}"
        echo "  ${sid}  ${symbols}  (${session_dir})"
    done
    exit 1
fi

STRATEGY_ID=$1
SESSION=${2:-day}
FOREGROUND=false
for arg in "$@"; do
    [ "$arg" = "--fg" ] && FOREGROUND=true
done

DATE=$(date +%Y%m%d)
CONFIG_FILE="config/trader.yaml"
LOG_FILE="./log/trader.${STRATEGY_ID}.${DATE}.log"

# 查找 control 文件
CONTROL_FILE=$(ls controls/${SESSION}/control.*.${STRATEGY_ID} 2>/dev/null | head -1)
if [ -z "$CONTROL_FILE" ]; then
    echo -e "${RED}[ERROR]${NC} 找不到 control 文件: controls/${SESSION}/control.*.${STRATEGY_ID}"
    echo ""
    echo "可用文件:"
    ls controls/day/control.* controls/night/control.* 2>/dev/null || echo "  (无)"
    exit 1
fi

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  启动策略 ${STRATEGY_ID}"
echo "════════════════════════════════════════════════════════════"
echo -e "${GREEN}[INFO]${NC} Strategy ID:  ${STRATEGY_ID}"
echo -e "${GREEN}[INFO]${NC} Session:      ${SESSION}"
echo -e "${GREEN}[INFO]${NC} Control:      ${CONTROL_FILE}"
echo -e "${GREEN}[INFO]${NC} Config:       ${CONFIG_FILE}"
echo -e "${GREEN}[INFO]${NC} Log:          ${LOG_FILE}"
echo ""

mkdir -p log

if [ "$FOREGROUND" = true ]; then
    echo -e "${YELLOW}[INFO]${NC} 前台模式 (Ctrl+C 停止)"
    ./bin/trader \
        --Live \
        --controlFile "$CONTROL_FILE" \
        --strategyID "$STRATEGY_ID" \
        --config "$CONFIG_FILE" \
        --log-file "$LOG_FILE"
else
    ulimit -c unlimited 2>/dev/null || true
    nohup ./bin/trader \
        --Live \
        --controlFile "$CONTROL_FILE" \
        --strategyID "$STRATEGY_ID" \
        --config "$CONFIG_FILE" \
        --log-file "$LOG_FILE" \
        >> "nohup.out.${STRATEGY_ID}" 2>&1 &

    PID=$!
    echo -e "${GREEN}[INFO]${NC} 策略已在后台启动 (PID: ${PID})"
    echo -e "${GREEN}[INFO]${NC} 查看日志: tail -f ${LOG_FILE}"
    echo -e "${GREEN}[INFO]${NC} 查看输出: tail -f nohup.out.${STRATEGY_ID}"

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
#   ./scripts/start_all.sh sim day       # 模拟环境，日盘所有策略
#   ./scripts/start_all.sh sim night     # 模拟环境，夜盘所有策略
#   ./scripts/start_all.sh ctp day       # CTP实盘，日盘所有策略
#   ./scripts/start_all.sh ctp night     # CTP实盘，夜盘所有策略
# ============================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
NC='\033[0m'

MODE=${1:-sim}
SESSION=${2:-day}

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  一键启动: 模式=${MODE}  时段=${SESSION}"
echo "════════════════════════════════════════════════════════════"
echo ""

# 1. 启动网关
./scripts/start_gateway.sh "$MODE"

sleep 2

# 2. 启动所有策略
CONTROL_DIR="controls/${SESSION}"
if [ ! -d "$CONTROL_DIR" ]; then
    echo -e "${GREEN}[INFO]${NC} 无 ${SESSION} 时段策略"
    exit 0
fi

for control_file in ${CONTROL_DIR}/control.*; do
    [ -f "$control_file" ] || continue
    STRATEGY_ID="${control_file##*.}"
    echo -e "${GREEN}[INFO]${NC} 启动策略 ${STRATEGY_ID}..."
    ./scripts/start_strategy.sh "$STRATEGY_ID" "$SESSION"
    sleep 1
done

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

# ==================== 合并 data_new → deploy_new ====================
log_section "合并 data_new → deploy_new"

if [ ! -d "${DATA_DIR}" ]; then
    log_error "data_new 目录不存在: ${DATA_DIR}"
    log_error "请先创建 data_new 目录并放入配置文件"
    exit 1
fi

# 使用 rsync 合并（保留已有文件，如 daily_init、positions）
# -a: 归档模式  -v: 详细  --ignore-existing: 不覆盖已存在的数据文件
# 但配置文件需要覆盖，所以分两步：
# 1. 复制配置文件（覆盖）
# 2. 复制数据文件（不覆盖）

# 复制配置（总是覆盖）
for dir in config controls models; do
    if [ -d "${DATA_DIR}/${dir}" ]; then
        cp -R "${DATA_DIR}/${dir}" "${DEPLOY_DIR}/"
        log_info "  ${dir}/"
    fi
done

# 复制数据目录（保留已有的 daily_init 和 positions）
if [ -d "${DATA_DIR}/data" ]; then
    # 创建目标目录结构
    find "${DATA_DIR}/data" -type d | while read dir; do
        target="${DEPLOY_DIR}/${dir#${DATA_DIR}/}"
        mkdir -p "$target"
    done
    # 复制文件（不覆盖已有的）
    find "${DATA_DIR}/data" -type f | while read file; do
        target="${DEPLOY_DIR}/${file#${DATA_DIR}/}"
        if [ ! -f "$target" ]; then
            cp "$file" "$target"
        fi
    done
    log_info "  data/ (保留已有数据)"
fi

# CTP flow 目录
mkdir -p "${DEPLOY_DIR}/ctp_flow"
if [ -d "${DATA_DIR}/ctp_flow" ]; then
    cp -R "${DATA_DIR}/ctp_flow/"* "${DEPLOY_DIR}/ctp_flow/" 2>/dev/null || true
fi

log_info "data_new 合并完成"

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
echo "  ./scripts/start_strategy.sh 92201 night"
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

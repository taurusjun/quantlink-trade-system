#!/bin/bash
# ============================================
# 脚本名称: build_deploy_java.sh
# 用途: 一键编译部署 C++ 网关 + Java 策略引擎（无 Go 组件）
# 日期: 2026-02-26
#
# 使用方式:
#   ./scripts/build_deploy_java.sh                    # 完整编译 C++ + Java
#   ./scripts/build_deploy_java.sh --cpp              # 仅编译 C++ 网关
#   ./scripts/build_deploy_java.sh --java             # 仅编译 Java 策略
#   ./scripts/build_deploy_java.sh --mode live        # 实盘配置
#   ./scripts/build_deploy_java.sh --clean            # 清理后重编译
#
# 部署目录:  deploy_java/
# 配置来源:  data_new/{common,sim,live}
#
# 启动方式:
#   cd deploy_java
#   ./scripts/start_gateway.sh sim          # 模拟网关 (C++)
#   ./scripts/start_gateway.sh ctp          # CTP 实盘网关 (C++)
#   ./scripts/start_strategy.sh 92201 day   # Java 策略
#   ./scripts/stop_all.sh                   # 停止所有
# ============================================

set -e

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色
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
DEPLOY_DIR="${PROJECT_ROOT}/deploy_java"
DATA_DIR="${PROJECT_ROOT}/data_new"
BUILD_CPP=true
BUILD_JAVA=true
CLEAN_BUILD=false
DEPLOY_MODE="sim"

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --cpp)   BUILD_CPP=true; BUILD_JAVA=false; shift ;;
        --java)  BUILD_CPP=false; BUILD_JAVA=true; shift ;;
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
            echo "  --cpp            仅编译 C++ 网关"
            echo "  --java           仅编译 Java 策略"
            echo "  --clean          清理后重新编译"
            echo "  --help           显示帮助"
            exit 0
            ;;
        *) log_error "Unknown option: $1"; exit 1 ;;
    esac
done

# Java 环境
JAVA_HOME="${JAVA_HOME:-/Users/user/Library/Java/JavaVirtualMachines/openjdk-25.0.1/Contents/Home}"
export JAVA_HOME
MVN_CMD="${MVN_CMD:-/opt/homebrew/bin/mvn}"
if [ ! -f "$MVN_CMD" ]; then
    MVN_CMD="$(which mvn 2>/dev/null || true)"
fi

log_section "QuantLink Trade System - C++ 网关 + Java 策略 (deploy_java)"
log_info "项目根目录: ${PROJECT_ROOT}"
log_info "部署目录:   ${DEPLOY_DIR}"
log_info "部署模式:   ${DEPLOY_MODE}"
log_info "编译 C++:   ${BUILD_CPP}"
log_info "编译 Java:  ${BUILD_JAVA}"

# ==================== 清理 ====================
if [ "$CLEAN_BUILD" = true ]; then
    log_info "清理 deploy_java 目录（保留 data/）..."
    for d in bin lib scripts config live sim log; do
        rm -rf "${DEPLOY_DIR}/${d}"
    done
fi

# ==================== 创建目录结构 ====================
log_section "创建 deploy_java 目录结构"
mkdir -p "${DEPLOY_DIR}/bin"
mkdir -p "${DEPLOY_DIR}/lib"
mkdir -p "${DEPLOY_DIR}/scripts"
mkdir -p "${DEPLOY_DIR}/log"
log_info "目录结构创建完成"

# ==================== 编译 C++ 网关 ====================
if [ "$BUILD_CPP" = true ]; then
    log_section "编译 C++ 网关组件"

    GATEWAY_DIR="${PROJECT_ROOT}/gateway"
    GATEWAY_BUILD="${GATEWAY_DIR}/build"

    if [ "$CLEAN_BUILD" = true ]; then
        log_info "清理 gateway/build..."
        rm -rf "${GATEWAY_BUILD}"
    fi

    mkdir -p "${GATEWAY_BUILD}"
    cd "${GATEWAY_BUILD}"

    log_info "运行 CMake..."
    cmake .. -DCMAKE_BUILD_TYPE=Release

    log_info "编译中..."
    make -j$(sysctl -n hw.ncpu 2>/dev/null || nproc) \
        md_shm_feeder counter_bridge md_benchmark 2>&1 || true

    log_info "复制 C++ 可执行文件..."
    for comp in md_shm_feeder counter_bridge; do
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

    # CTP 动态库（macOS）
    if [ "$(uname)" = "Darwin" ]; then
        CTP_FRAMEWORK_DIR="${GATEWAY_DIR}/third_party/ctp"
        if [ -d "${CTP_FRAMEWORK_DIR}" ]; then
            log_info "复制 CTP Framework..."
            mkdir -p "${DEPLOY_DIR}/lib/ctp"
            cp -R "${CTP_FRAMEWORK_DIR}"/*.framework "${DEPLOY_DIR}/lib/ctp/" 2>/dev/null || true
        fi
    fi

    log_info "C++ 网关编译完成"
fi

# ==================== 编译 Java ====================
if [ "$BUILD_JAVA" = true ]; then
    log_section "编译 Java 策略组件"

    if [ -z "$MVN_CMD" ] || [ ! -f "$MVN_CMD" ]; then
        log_error "Maven 未找到，请安装 Maven 或设置 MVN_CMD 环境变量"
        exit 1
    fi

    log_info "JAVA_HOME: ${JAVA_HOME}"
    log_info "Maven: ${MVN_CMD}"

    if [ "$CLEAN_BUILD" = true ]; then
        log_info "清理 Java 构建..."
        "$MVN_CMD" -f "${PROJECT_ROOT}/tbsrc-java/pom.xml" clean -q 2>&1 || true
    fi

    log_info "编译 + 打包..."
    "$MVN_CMD" -f "${PROJECT_ROOT}/tbsrc-java/pom.xml" package -DskipTests -q 2>&1

    log_info "部署 JAR..."
    # 清理旧 jar（避免残留）
    rm -f "${DEPLOY_DIR}/lib/trader-1.0-SNAPSHOT.jar"
    cp "${PROJECT_ROOT}/tbsrc-java/target/trader-1.0-SNAPSHOT.jar" "${DEPLOY_DIR}/lib/"
    cp "${PROJECT_ROOT}/tbsrc-java/target/lib/"*.jar "${DEPLOY_DIR}/lib/" 2>/dev/null || true
    log_info "  trader-1.0-SNAPSHOT.jar + $(ls "${PROJECT_ROOT}/tbsrc-java/target/lib/"*.jar 2>/dev/null | wc -l | tr -d ' ') 个依赖"

    log_info "Java 策略编译完成"
fi

# ==================== 生成启动脚本 ====================
log_section "生成启动脚本"

# --- start_gateway.sh (C++ 网关 + Java OverviewServer) ---
cat > "${DEPLOY_DIR}/scripts/start_gateway.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 启动 C++ 网关层（md_shm_feeder + counter_bridge）+ Java OverviewServer
# Usage: ./scripts/start_gateway.sh [sim|ctp]
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
    exit 1
fi

echo "$MODE" > .gateway_mode

echo ""
echo "════════════════════════════════════════════════════════════"
if [ "$MODE" = "sim" ]; then
    echo "  QuantLink (C++ + Java) - 模拟环境"
else
    echo -e "  QuantLink (C++ + Java) - ${RED}CTP 实盘环境${NC}"
fi
echo "════════════════════════════════════════════════════════════"
echo ""

# CTP 配置检查
if [ "$MODE" = "ctp" ]; then
    CTP_MD_CONFIG="config/ctp/ctp_md.secret.yaml"
    CTP_TD_CONFIG="config/ctp/ctp_td.secret.yaml"
    if [ ! -f "$CTP_MD_CONFIG" ] || [ ! -f "$CTP_TD_CONFIG" ]; then
        echo -e "${RED}[ERROR]${NC} CTP 配置文件不存在"
        echo "  需要: $CTP_MD_CONFIG"
        echo "  需要: $CTP_TD_CONFIG"
        exit 1
    fi
fi

# 清理共享内存
ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true

mkdir -p log ctp_flow

# 1. MD SHM Feeder
if [ "$MODE" = "sim" ]; then
    SYMBOLS=""
    for ctrl in sim/controls/day/control.*.par.txt.*; do
        [ -f "$ctrl" ] || continue
        fname=$(basename "$ctrl")
        syms="${fname#control.}"
        syms="${syms%.par.txt.*}"
        syms=$(echo "$syms" | tr '.' ',')
        [ -n "$SYMBOLS" ] && SYMBOLS="${SYMBOLS},"
        SYMBOLS="${SYMBOLS}${syms}"
    done
    [ -z "$SYMBOLS" ] && SYMBOLS="ag2603,ag2605"
    QUEUE_SIZE=2048
    [ "$(uname)" = "Linux" ] && QUEUE_SIZE=65536
    ./bin/md_shm_feeder "simulator:${SYMBOLS}" --queue-size "$QUEUE_SIZE" > "log/md_shm_feeder.${DATE}.log" 2>&1 &
    sleep 1
    echo -e "${GREEN}[INFO]${NC} MD SHM Feeder (Simulator: ${SYMBOLS}, queue=${QUEUE_SIZE})"
else
    QUEUE_SIZE=2048
    [ "$(uname)" = "Linux" ] && QUEUE_SIZE=65536
    ./bin/md_shm_feeder "ctp:${CTP_MD_CONFIG}" --queue-size "$QUEUE_SIZE" > "log/md_shm_feeder.${DATE}.log" 2>&1 &
    sleep 2
    echo -e "${GREEN}[INFO]${NC} MD SHM Feeder (CTP, queue=${QUEUE_SIZE})"
fi

# 2. Counter Bridge (HTTP :8082 — /account, /health)
if [ "$MODE" = "sim" ]; then
    ./bin/counter_bridge simulator:config/simulator.yaml > "log/counter_bridge.${DATE}.log" 2>&1 &
    sleep 1
    echo -e "${GREEN}[INFO]${NC} Counter Bridge (Simulator, HTTP :8082)"
else
    ./bin/counter_bridge ctp:"$CTP_TD_CONFIG" > "log/counter_bridge.${DATE}.log" 2>&1 &
    sleep 2
    echo -e "${GREEN}[INFO]${NC} Counter Bridge (CTP, HTTP :8082)"
fi

# 3. Java OverviewServer (端口 8080)
JAVA_HOME="${JAVA_HOME:-/Users/user/Library/Java/JavaVirtualMachines/openjdk-25.0.1/Contents/Home}"
export JAVA_HOME

CLASSPATH="$DEPLOY_ROOT/lib/trader-1.0-SNAPSHOT.jar"
for jar in "$DEPLOY_ROOT/lib/"*.jar; do
    [ "$jar" = "$DEPLOY_ROOT/lib/trader-1.0-SNAPSHOT.jar" ] && continue
    echo "$jar" | grep -q "/ctp/" && continue
    CLASSPATH="$CLASSPATH:$jar"
done

OVERVIEW_PORT=8080
nohup "$JAVA_HOME/bin/java" --enable-native-access=ALL-UNNAMED \
    -cp "$CLASSPATH" \
    com.quantlink.trader.api.overview.OverviewServer "$OVERVIEW_PORT" \
    >> "log/overview.${DATE}.log" 2>&1 &
echo $! > overview.pid
sleep 1
echo -e "${GREEN}[INFO]${NC} Java OverviewServer (port ${OVERVIEW_PORT})"

echo ""
echo -e "${GREEN}[INFO]${NC} C++ 网关 + Overview 启动完成 (${MODE})"
echo -e "${GREEN}[INFO]${NC} Overview:  http://localhost:${OVERVIEW_PORT}/"
echo -e "${GREEN}[INFO]${NC} 启动策略: ./scripts/start_strategy.sh <strategy_id> [day|night]"
echo ""
SCRIPT_EOF

# --- start_strategy.sh (Java Trader) ---
cat > "${DEPLOY_DIR}/scripts/start_strategy.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 启动 Java 策略引擎
# Usage: ./scripts/start_strategy.sh <strategy_id> [day|night] [--fg]
# ============================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

JAVA_HOME="${JAVA_HOME:-/Users/user/Library/Java/JavaVirtualMachines/openjdk-25.0.1/Contents/Home}"
export JAVA_HOME

if [ $# -lt 1 ]; then
    echo "Usage: $0 <strategy_id> [day|night] [--fg]"
    echo ""
    echo "可用策略:"
    for f in live/controls/day/control.*.par.txt.* sim/controls/day/control.*.par.txt.*; do
        [ -f "$f" ] || continue
        sid="${f##*.}"
        env="${f%%/*}"
        echo "  ${sid}  [${env}] ($(basename "$f"))"
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

# 自动检测 session
if [ -z "$SESSION" ]; then
    HOUR=$(date +%H)
    if [ "$HOUR" -ge 20 ] || [ "$HOUR" -lt 4 ]; then
        SESSION="night"
    else
        SESSION="day"
    fi
fi

DATE=$(date +%Y%m%d)
YEAR_PREFIX=$(date +%y)

# 读取网关模式
if [ ! -f .gateway_mode ]; then
    echo -e "${RED}[ERROR]${NC} 请先启动网关: ./scripts/start_gateway.sh [sim|ctp]"
    exit 1
fi
GATEWAY_MODE=$(cat .gateway_mode)
case "$GATEWAY_MODE" in
    sim) ENV_DIR="sim" ;;
    ctp) ENV_DIR="live" ;;
    *)   echo -e "${RED}[ERROR]${NC} .gateway_mode 无效: ${GATEWAY_MODE}"; exit 1 ;;
esac
DATA_DIR="./${ENV_DIR}/data"

# 查找 controlFile（从环境目录）
CONTROL_FILE=""
for f in ${ENV_DIR}/controls/${SESSION}/control.*.par.txt.${STRATEGY_ID}; do
    [ -f "$f" ] && CONTROL_FILE="$f" && break
done
if [ -z "$CONTROL_FILE" ]; then
    echo -e "${RED}[ERROR]${NC} 找不到 controlFile: ${ENV_DIR}/controls/${SESSION}/control.*.par.txt.${STRATEGY_ID}"
    exit 1
fi

CONFIG_FILE="config/config_CHINA.${STRATEGY_ID}.cfg"
if [ ! -f "$CONFIG_FILE" ]; then
    echo -e "${RED}[ERROR]${NC} 找不到 configFile: ${CONFIG_FILE}"
    exit 1
fi

LOG_FILE="./log/trader.${STRATEGY_ID}.${DATE}.log"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  启动 Java 策略 ${STRATEGY_ID} (${SESSION})"
echo "════════════════════════════════════════════════════════════"
echo -e "${GREEN}[INFO]${NC} Strategy ID:  ${STRATEGY_ID}"
echo -e "${GREEN}[INFO]${NC} Session:      ${SESSION}"
echo -e "${GREEN}[INFO]${NC} Gateway Mode: ${GATEWAY_MODE}"
echo -e "${GREEN}[INFO]${NC} Data Dir:     ${DATA_DIR}"
echo -e "${GREEN}[INFO]${NC} ControlFile:  ${CONTROL_FILE}"
echo -e "${GREEN}[INFO]${NC} ConfigFile:   ${CONFIG_FILE}"
echo -e "${GREEN}[INFO]${NC} JAVA_HOME:    ${JAVA_HOME}"
echo ""

mkdir -p log "${DATA_DIR}"

# 构建 classpath
CLASSPATH="$DEPLOY_ROOT/lib/trader-1.0-SNAPSHOT.jar"
for jar in "$DEPLOY_ROOT/lib/"*.jar; do
    [ "$jar" = "$DEPLOY_ROOT/lib/trader-1.0-SNAPSHOT.jar" ] && continue
    # 跳过 CTP 目录
    echo "$jar" | grep -q "/ctp/" && continue
    CLASSPATH="$CLASSPATH:$jar"
done

JAVA_OPTS="--enable-native-access=ALL-UNNAMED"

if [ "$FOREGROUND" = true ]; then
    echo -e "${YELLOW}[INFO]${NC} 前台模式 (Ctrl+C 停止)"
    "$JAVA_HOME/bin/java" $JAVA_OPTS -cp "$CLASSPATH" \
        com.quantlink.trader.TraderMain \
        --Live \
        -controlFile "$CONTROL_FILE" \
        -strategyID "$STRATEGY_ID" \
        -configFile "$CONFIG_FILE" \
        -dataDir "$DATA_DIR" \
        -yearPrefix "$YEAR_PREFIX" \
        -logFile "$LOG_FILE" \
        -printMod 1 \
        2>&1 | tee -a "$LOG_FILE"
else
    nohup "$JAVA_HOME/bin/java" $JAVA_OPTS -cp "$CLASSPATH" \
        com.quantlink.trader.TraderMain \
        --Live \
        -controlFile "$CONTROL_FILE" \
        -strategyID "$STRATEGY_ID" \
        -configFile "$CONFIG_FILE" \
        -dataDir "$DATA_DIR" \
        -yearPrefix "$YEAR_PREFIX" \
        -logFile "$LOG_FILE" \
        -printMod 1 \
        >> "nohup.out.${STRATEGY_ID}" 2>&1 &

    PID=$!
    echo "$PID" > "trader.${STRATEGY_ID}.pid"
    echo -e "${GREEN}[INFO]${NC} Java Trader 已启动 (PID: ${PID})"
    echo -e "${GREEN}[INFO]${NC} Dashboard: http://localhost:9201/dashboard.html"
    echo -e "${GREEN}[INFO]${NC} 查看日志: tail -f nohup.out.${STRATEGY_ID}"

    sleep 3
    if kill -0 $PID 2>/dev/null; then
        echo -e "${GREEN}[INFO]${NC} 进程运行正常"
    else
        echo -e "${RED}[ERROR]${NC} 进程已退出，检查日志:"
        tail -20 "nohup.out.${STRATEGY_ID}" 2>/dev/null
        exit 1
    fi
fi
SCRIPT_EOF

# --- start_all.sh ---
cat > "${DEPLOY_DIR}/scripts/start_all.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 一键启动: C++ 网关 + 所有 Java 策略
# Usage: ./scripts/start_all.sh [sim|ctp] [day|night]
# ============================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
NC='\033[0m'

MODE=${1:-sim}
SESSION=${2:-}

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
echo "  一键启动 (C++ + Java): 模式=${MODE}  时段=${SESSION}"
echo "════════════════════════════════════════════════════════════"
echo ""

# 1. 启动 C++ 网关
./scripts/start_gateway.sh "$MODE"
sleep 2

# 2. 启动所有 Java 策略
FOUND=0
for cfg_file in config/config_CHINA.*.cfg; do
    [ -f "$cfg_file" ] || continue
    fname=$(basename "$cfg_file")
    STRATEGY_ID="${fname#config_CHINA.}"
    STRATEGY_ID="${STRATEGY_ID%.cfg}"
    [ -z "$STRATEGY_ID" ] && continue

    ENV_DIR="sim"
    [ "$MODE" = "ctp" ] && ENV_DIR="live"
    CTRL_FOUND=false
    for ctrl in ${ENV_DIR}/controls/${SESSION}/control.*.par.txt.${STRATEGY_ID}; do
        [ -f "$ctrl" ] && CTRL_FOUND=true && break
    done
    if [ "$CTRL_FOUND" = false ]; then
        echo -e "${GREEN}[INFO]${NC} 跳过策略 ${STRATEGY_ID}（无 ${SESSION} control 文件）"
        continue
    fi

    FOUND=1
    ./scripts/start_strategy.sh "$STRATEGY_ID" "$SESSION"
    sleep 1
done

[ "$FOUND" -eq 0 ] && echo -e "${GREEN}[INFO]${NC} 未找到可启动的策略"

echo ""
echo -e "${GREEN}[INFO]${NC} 所有组件启动完成"
echo -e "${GREEN}[INFO]${NC} Overview:  http://localhost:8080/"
echo -e "${GREEN}[INFO]${NC} 停止: ./scripts/stop_all.sh"
echo ""
SCRIPT_EOF

# --- stop_all.sh ---
cat > "${DEPLOY_DIR}/scripts/stop_all.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 停止所有: Java 策略 + Java OverviewServer + C++ 网关
# ============================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  停止 QuantLink (C++ + Java)"
echo "════════════════════════════════════════════════════════════"
echo ""

# 1. 停止 Java Trader（graceful shutdown 保存 daily_init）
for pid_file in trader.*.pid; do
    [ -f "$pid_file" ] || continue
    PID=$(cat "$pid_file")
    SID=$(basename "$pid_file" .pid | sed 's/trader\.//')
    if kill -0 "$PID" 2>/dev/null; then
        echo -e "${YELLOW}[INFO]${NC} 发送 SIGTERM 到 Java Trader ${SID} (PID=${PID})..."
        kill -TERM "$PID" 2>/dev/null || true
    fi
    rm -f "$pid_file"
done

# 等待 Java 进程退出（最多 10 秒）
for i in $(seq 1 10); do
    if ! pgrep -f "TraderMain" > /dev/null 2>&1; then
        break
    fi
    sleep 1
done
if pgrep -f "TraderMain" > /dev/null 2>&1; then
    echo -e "${YELLOW}[WARN]${NC} Java Trader 未在 10 秒内退出，强制终止..."
    pkill -9 -f "TraderMain" 2>/dev/null || true
fi
echo -e "${GREEN}[INFO]${NC} Java Trader 已停止"

# 2. 停止 Java OverviewServer
if [ -f overview.pid ]; then
    OV_PID=$(cat overview.pid)
    if kill -0 "$OV_PID" 2>/dev/null; then
        echo -e "${GREEN}[INFO]${NC} 停止 OverviewServer (PID=${OV_PID})..."
        kill -TERM "$OV_PID" 2>/dev/null || true
    fi
    rm -f overview.pid
fi
if pgrep -f "OverviewServer" > /dev/null 2>&1; then
    pkill -f "OverviewServer" 2>/dev/null || true
fi
echo -e "${GREEN}[INFO]${NC} OverviewServer 已停止"

# 3. 停止 C++ 网关
for proc in md_shm_feeder counter_bridge; do
    if pgrep -f "$proc" > /dev/null 2>&1; then
        echo -e "${GREEN}[INFO]${NC} 停止 $proc..."
        pkill -f "$proc" 2>/dev/null || true
    fi
done

# 清理 SysV IPC（共享内存 + 信号量 + 消息队列，防止残留订单）
ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true
ipcs -s 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -s {} 2>/dev/null || true
ipcs -q 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -q {} 2>/dev/null || true

rm -f .gateway_mode

echo ""
echo -e "${GREEN}[INFO]${NC} 所有组件已停止"
echo ""
SCRIPT_EOF

# restart_live.sh — 实盘重启（停止→清理SHM→重启）
cat > "${DEPLOY_DIR}/scripts/restart_live.sh" << 'SCRIPT_EOF'
#!/bin/bash
# ============================================
# 实盘重启脚本: 停止所有 → 清理 SHM → 重启网关+策略
# Usage: ./scripts/restart_live.sh <strategy_id> [day|night]
# ============================================
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

if [ $# -lt 1 ]; then
    echo "Usage: $0 <strategy_id> [day|night]"
    echo ""
    echo "此脚本执行完整的实盘重启流程:"
    echo "  1. 停止所有进程 (Java Trader + OverviewServer + C++ 网关)"
    echo "  2. 彻底清理 SysV 共享内存和信号量"
    echo "  3. 启动 CTP 网关 (md_shm_feeder + counter_bridge + OverviewServer)"
    echo "  4. 启动 Java 策略"
    exit 1
fi

STRATEGY_ID=$1
SESSION=${2:-""}

echo ""
echo "════════════════════════════════════════════════════════════"
echo -e "  ${RED}QuantLink 实盘重启${NC}"
echo "  策略: ${STRATEGY_ID}"
echo "════════════════════════════════════════════════════════════"
echo ""

# Step 1: 停止所有进程
echo -e "${YELLOW}[Step 1/4]${NC} 停止所有进程..."

for pid_file in trader.*.pid; do
    [ -f "$pid_file" ] || continue
    PID=$(cat "$pid_file")
    SID=$(basename "$pid_file" .pid | sed 's/trader\.//')
    if kill -0 "$PID" 2>/dev/null; then
        echo -e "  ${YELLOW}→${NC} 发送 SIGTERM 到 Trader ${SID} (PID=${PID})"
        kill -TERM "$PID" 2>/dev/null || true
    fi
    rm -f "$pid_file"
done

for i in $(seq 1 15); do
    if ! pgrep -f "TraderMain" > /dev/null 2>&1; then
        break
    fi
    sleep 1
done
if pgrep -f "TraderMain" > /dev/null 2>&1; then
    echo -e "  ${YELLOW}[WARN]${NC} Trader 未在 15 秒内退出，强制终止..."
    pkill -9 -f "TraderMain" 2>/dev/null || true
    sleep 1
fi
echo -e "  ${GREEN}✓${NC} Java Trader 已停止"

if [ -f overview.pid ]; then
    OV_PID=$(cat overview.pid)
    if kill -0 "$OV_PID" 2>/dev/null; then
        kill -TERM "$OV_PID" 2>/dev/null || true
    fi
    rm -f overview.pid
fi
pkill -f "OverviewServer" 2>/dev/null || true
echo -e "  ${GREEN}✓${NC} OverviewServer 已停止"

for proc in counter_bridge md_shm_feeder; do
    if pgrep -f "$proc" > /dev/null 2>&1; then
        pkill -f "$proc" 2>/dev/null || true
    fi
done
sleep 1
for proc in counter_bridge md_shm_feeder; do
    if pgrep -f "$proc" > /dev/null 2>&1; then
        pkill -9 -f "$proc" 2>/dev/null || true
    fi
done
echo -e "  ${GREEN}✓${NC} C++ 网关已停止"

# Step 2: 彻底清理 SysV IPC
echo ""
echo -e "${YELLOW}[Step 2/4]${NC} 清理 SysV 共享内存和信号量..."

SHM_COUNT=$(ipcs -m 2>/dev/null | grep "$(whoami)" | wc -l | tr -d ' ')
if [ "$SHM_COUNT" -gt 0 ]; then
    ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true
    echo -e "  ${GREEN}✓${NC} 已清理 ${SHM_COUNT} 个共享内存段"
else
    echo -e "  ${GREEN}✓${NC} 无共享内存段需要清理"
fi

SEM_COUNT=$(ipcs -s 2>/dev/null | grep "$(whoami)" | wc -l | tr -d ' ')
if [ "$SEM_COUNT" -gt 0 ]; then
    ipcs -s 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -s {} 2>/dev/null || true
    echo -e "  ${GREEN}✓${NC} 已清理 ${SEM_COUNT} 个信号量"
else
    echo -e "  ${GREEN}✓${NC} 无信号量需要清理"
fi

MQ_COUNT=$(ipcs -q 2>/dev/null | grep "$(whoami)" | wc -l | tr -d ' ')
if [ "$MQ_COUNT" -gt 0 ]; then
    ipcs -q 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -q {} 2>/dev/null || true
    echo -e "  ${GREEN}✓${NC} 已清理 ${MQ_COUNT} 个消息队列"
else
    echo -e "  ${GREEN}✓${NC} 无消息队列需要清理"
fi

rm -f .gateway_mode

REMAINING=$(ipcs -m 2>/dev/null | grep "$(whoami)" | wc -l | tr -d ' ')
if [ "$REMAINING" -gt 0 ]; then
    echo -e "  ${RED}[WARN]${NC} 仍有 ${REMAINING} 个共享内存段未清理！"
    ipcs -m 2>/dev/null | grep "$(whoami)"
fi

# Step 3: 启动 CTP 网关
echo ""
echo -e "${YELLOW}[Step 3/4]${NC} 启动 CTP 网关..."
"$SCRIPT_DIR/start_gateway.sh" ctp

# Step 4: 启动策略
echo ""
echo -e "${YELLOW}[Step 4/4]${NC} 启动策略 ${STRATEGY_ID}..."
STRAT_ARGS="$STRATEGY_ID"
[ -n "$SESSION" ] && STRAT_ARGS="$STRAT_ARGS $SESSION"
"$SCRIPT_DIR/start_strategy.sh" $STRAT_ARGS

echo ""
echo "════════════════════════════════════════════════════════════"
echo -e "  ${GREEN}实盘重启完成${NC}"
echo "════════════════════════════════════════════════════════════"
echo ""
echo -e "${GREEN}[INFO]${NC} 查看策略日志:  tail -f nohup.out.${STRATEGY_ID}"
echo -e "${GREEN}[INFO]${NC} Overview:      http://localhost:8080/"
echo -e "${GREEN}[INFO]${NC} Dashboard:     http://localhost:9201/dashboard.html"
echo -e "${GREEN}[INFO]${NC} 停止所有:      ./scripts/stop_all.sh"
echo ""
SCRIPT_EOF

chmod +x "${DEPLOY_DIR}/scripts/"*.sh
log_info "  start_gateway.sh   (C++ 网关: sim/ctp)"
log_info "  start_strategy.sh  (Java 策略引擎)"
log_info "  start_all.sh       (一键启动)"
log_info "  stop_all.sh        (停止所有)"
log_info "  restart_live.sh    (实盘重启: 停止→清理SHM→重启)"

# ==================== 合并 data_new → deploy_java ====================
log_section "合并 data_new → deploy_java (模式: ${DEPLOY_MODE})"

if [ ! -d "${DATA_DIR}" ]; then
    log_error "data_new 目录不存在: ${DATA_DIR}"
    exit 1
fi

COMMON_DIR="${DATA_DIR}/common"
MODE_DIR="${DATA_DIR}/${DEPLOY_MODE}"

if [ ! -d "${COMMON_DIR}" ]; then
    log_error "data_new/common 目录不存在: ${COMMON_DIR}"
    exit 1
fi

# 1. 复制 common/config
if [ -d "${COMMON_DIR}/config" ]; then
    cp -R "${COMMON_DIR}/config" "${DEPLOY_DIR}/"
    log_info "  common/config/"
fi

# 2. 复制 live 和 sim 环境（controls/models/data 各自独立）
for env_name in live sim; do
    env_src="${DATA_DIR}/${env_name}"
    env_dst="${DEPLOY_DIR}/${env_name}"
    [ -d "${env_src}" ] || continue

    # controls
    if [ -d "${env_src}/controls" ]; then
        mkdir -p "${env_dst}/controls"
        cp -R "${env_src}/controls/"* "${env_dst}/controls/"
        log_info "  ${env_name}/controls/"
    fi

    # models
    if [ -d "${env_src}/models" ]; then
        mkdir -p "${env_dst}/models"
        cp "${env_src}/models/"*.par.txt.* "${env_dst}/models/" 2>/dev/null || true
        log_info "  ${env_name}/models/"
    fi

    # data（保留运行时数据，不覆盖已有文件）
    if [ -d "${env_src}/data" ]; then
        mkdir -p "${env_dst}/data"
        find "${env_src}/data" -type f | while read file; do
            filename=$(basename "$file")
            target="${env_dst}/data/${filename}"
            [ ! -f "$target" ] && cp "$file" "$target"
        done || true
        log_info "  ${env_name}/data/"
    fi

    # config（环境专属配置，合并到 deploy_java/config/）
    if [ -d "${env_src}/config" ]; then
        cp -R "${env_src}/config/"* "${DEPLOY_DIR}/config/" 2>/dev/null || true
        log_info "  ${env_name}/config/ → config/"
    fi
done

# 3. live: ctp_flow
mkdir -p "${DEPLOY_DIR}/ctp_flow"
[ -d "${DATA_DIR}/live/ctp_flow" ] && cp -R "${DATA_DIR}/live/ctp_flow/"* "${DEPLOY_DIR}/ctp_flow/" 2>/dev/null || true

log_info "data_new 合并完成 (模式: ${DEPLOY_MODE})"

# ==================== 完成 ====================
log_section "构建完成"

echo ""
log_info "deploy_java/ 目录结构:"
echo ""
echo "  deploy_java/"
echo "  ├── bin/          (C++ 网关)"
ls "${DEPLOY_DIR}/bin/" 2>/dev/null | while read f; do echo "  │   ├── $f"; done
echo "  ├── lib/          (Java JARs)"
echo "  │   ├── trader-1.0-SNAPSHOT.jar"
echo "  │   └── ... $(ls "${DEPLOY_DIR}/lib/"*.jar 2>/dev/null | wc -l | tr -d ' ') 个 JAR"
echo "  ├── config/"
echo "  ├── live/          (controls, models, data)"
echo "  ├── sim/           (controls, models, data)"
echo "  ├── scripts/"
ls "${DEPLOY_DIR}/scripts/" 2>/dev/null | while read f; do echo "  │   ├── $f"; done
echo "  └── log/"

echo ""
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}  使用方式${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo ""
echo "  cd deploy_java"
echo ""
echo "  # 模拟测试"
echo "  ./scripts/start_gateway.sh sim"
echo "  ./scripts/start_strategy.sh 92201 day"
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
echo "  http://localhost:9201/dashboard.html"
echo ""

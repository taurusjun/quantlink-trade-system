#!/bin/bash
# ============================================
# 脚本名称: build_deploy.sh
# 用途: 一键编译所有组件并打包到 deploy 目录
# 作者: QuantLink Team
# 日期: 2026-02-10
#
# 使用方式:
#   ./scripts/build_deploy.sh          # 完整编译
#   ./scripts/build_deploy.sh --go     # 仅编译 Go 组件
#   ./scripts/build_deploy.sh --cpp    # 仅编译 C++ 组件
#   ./scripts/build_deploy.sh --clean  # 清理后重新编译
#
# 部署到服务器:
#   scp -r deploy/ user@server:/opt/quantlink/
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
    echo ""
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
}

# 配置
DEPLOY_DIR="${PROJECT_ROOT}/deploy"
BUILD_GO=true
BUILD_CPP=true
CLEAN_BUILD=false

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
        --go)
            BUILD_GO=true
            BUILD_CPP=false
            shift
            ;;
        --cpp)
            BUILD_GO=false
            BUILD_CPP=true
            shift
            ;;
        --clean)
            CLEAN_BUILD=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --go      仅编译 Go 组件"
            echo "  --cpp     仅编译 C++ 组件"
            echo "  --clean   清理后重新编译"
            echo "  --help    显示帮助"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            exit 1
            ;;
    esac
done

# 显示构建信息
log_section "QuantLink Trade System - 构建部署"
log_info "项目根目录: ${PROJECT_ROOT}"
log_info "部署目录:   ${DEPLOY_DIR}"
log_info "编译 Go:    ${BUILD_GO}"
log_info "编译 C++:   ${BUILD_CPP}"
log_info "清理构建:   ${CLEAN_BUILD}"

# 清理旧的 deploy 目录
if [ "$CLEAN_BUILD" = true ]; then
    log_info "清理 deploy 目录..."
    rm -rf "${DEPLOY_DIR}"
fi

# 创建 deploy 目录结构
log_section "创建目录结构"
mkdir -p "${DEPLOY_DIR}/bin"
mkdir -p "${DEPLOY_DIR}/config"
mkdir -p "${DEPLOY_DIR}/config/ctp"
mkdir -p "${DEPLOY_DIR}/controls/day"
mkdir -p "${DEPLOY_DIR}/controls/night"
mkdir -p "${DEPLOY_DIR}/models"
mkdir -p "${DEPLOY_DIR}/data/positions"
mkdir -p "${DEPLOY_DIR}/log"
mkdir -p "${DEPLOY_DIR}/scripts"
mkdir -p "${DEPLOY_DIR}/lib"
mkdir -p "${DEPLOY_DIR}/ctp_flow"  # CTP API flow文件目录
mkdir -p "${DEPLOY_DIR}/golang/web"  # Web 资源目录（dashboard）
log_info "目录结构创建完成"

# 复制 Web 资源（dashboard）
if [ -d "${PROJECT_ROOT}/golang/web" ]; then
    cp "${PROJECT_ROOT}/golang/web/"*.html "${DEPLOY_DIR}/golang/web/" 2>/dev/null || true
    log_info "  ✓ Dashboard HTML 文件"
fi

# ==================== 编译 C++ 网关组件 ====================
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
    # 编译核心组件（忽略测试文件编译错误）
    make -j$(sysctl -n hw.ncpu 2>/dev/null || nproc) md_gateway md_simulator ors_gateway counter_bridge ctp_md_gateway md_benchmark 2>&1 || true

    # 复制编译产物到 deploy/bin
    log_info "复制 C++ 可执行文件到 deploy/bin..."

    # 核心组件（必须存在）
    CORE_COMPONENTS="md_gateway md_simulator ors_gateway counter_bridge ctp_md_gateway"
    for comp in $CORE_COMPONENTS; do
        if [ -f "$comp" ]; then
            cp "$comp" "${DEPLOY_DIR}/bin/"
            log_info "  ✓ $comp"
        else
            log_error "  ✗ $comp 编译失败！"
            exit 1
        fi
    done

    # 工具（可选）
    [ -f "md_benchmark" ] && cp md_benchmark "${DEPLOY_DIR}/bin/" && log_info "  ✓ md_benchmark"

    cd "${PROJECT_ROOT}"
    log_info "C++ 组件编译完成"

    # 复制 CTP 动态库（macOS）
    if [ "$(uname)" = "Darwin" ]; then
        CTP_FRAMEWORK_DIR="${GATEWAY_DIR}/third_party/ctp"
        if [ -d "${CTP_FRAMEWORK_DIR}" ]; then
            log_info "复制 CTP Framework..."
            mkdir -p "${DEPLOY_DIR}/lib/ctp"
            cp -R "${CTP_FRAMEWORK_DIR}"/*.framework "${DEPLOY_DIR}/lib/ctp/" 2>/dev/null || true
        fi
    fi
fi

# ==================== 编译 Go 策略组件 ====================
if [ "$BUILD_GO" = true ]; then
    log_section "编译 Go 策略组件"

    GOLANG_DIR="${PROJECT_ROOT}/golang"
    cd "${GOLANG_DIR}"

    log_info "编译 trader..."
    go build -o "${DEPLOY_DIR}/bin/trader" ./cmd/trader/main.go
    log_info "  ✓ trader"

    log_info "编译 backtest..."
    go build -o "${DEPLOY_DIR}/bin/backtest" ./cmd/backtest/main.go 2>/dev/null || log_warn "  ✗ backtest (可选)"

    log_info "编译 backtest_optimize..."
    go build -o "${DEPLOY_DIR}/bin/backtest_optimize" ./cmd/backtest_optimize/main.go 2>/dev/null || log_warn "  ✗ backtest_optimize (可选)"

    cd "${PROJECT_ROOT}"
    log_info "Go 组件编译完成"
fi

# ==================== 复制配置文件 ====================
log_section "复制配置文件"

# 主配置
if [ -f "config/trader.yaml" ]; then
    cp config/trader.yaml "${DEPLOY_DIR}/config/"
    log_info "  ✓ trader.yaml"
fi

if [ -f "config/trader.test.yaml" ]; then
    cp config/trader.test.yaml "${DEPLOY_DIR}/config/"
    log_info "  ✓ trader.test.yaml"
fi

# CTP 配置
if [ -d "config/ctp" ]; then
    # 复制模板文件
    cp config/ctp/*.yaml "${DEPLOY_DIR}/config/ctp/" 2>/dev/null || true
    # 复制 secret 文件（如果存在）
    if ls config/ctp/*.secret.yaml 1> /dev/null 2>&1; then
        cp config/ctp/*.secret.yaml "${DEPLOY_DIR}/config/ctp/" 2>/dev/null || true
        log_info "  ✓ CTP 配置（含 secret）"
    else
        log_info "  ✓ CTP 配置模板（需手动创建 secret 文件）"
    fi
fi

# 示例 model/control 文件
if [ -d "bin/models" ]; then
    cp bin/models/*.sample "${DEPLOY_DIR}/models/" 2>/dev/null || true
    log_info "  ✓ Model 示例文件"
fi

if [ -d "bin/controls/day" ]; then
    cp bin/controls/day/*.sample "${DEPLOY_DIR}/controls/day/" 2>/dev/null || true
    log_info "  ✓ Control 示例文件 (day)"
fi

if [ -d "bin/config" ]; then
    cp bin/config/*.sample "${DEPLOY_DIR}/config/" 2>/dev/null || true
    log_info "  ✓ 策略配置示例"
fi

# ==================== 创建启动脚本 ====================
log_section "创建启动脚本"

# 模拟环境启动脚本
cat > "${DEPLOY_DIR}/scripts/start_simulator.sh" << 'SCRIPT_EOF'
#!/bin/bash
# 启动模拟环境 - 用于开发测试
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  QuantLink Trade System - 模拟环境"
echo "════════════════════════════════════════════════════════════"
echo ""

# 清理共享内存
ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true

mkdir -p log

# 1. NATS
if ! pgrep -f "nats-server" > /dev/null 2>&1; then
    if command -v nats-server &> /dev/null; then
        nats-server -p 4222 > log/nats.log 2>&1 &
        sleep 1
        echo -e "${GREEN}[INFO]${NC} ✓ NATS Server"
    else
        echo -e "${RED}[ERROR]${NC} NATS 未安装: brew install nats-server"
        exit 1
    fi
fi

# 2. MD Simulator
./bin/md_simulator > log/md_simulator.log 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} ✓ MD Simulator"

# 3. MD Gateway
./bin/md_gateway > log/md_gateway.log 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} ✓ MD Gateway"

# 4. ORS Gateway
./bin/ors_gateway > log/ors_gateway.log 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} ✓ ORS Gateway"

# 5. Counter Bridge (Simulator)
./bin/counter_bridge simulator:config/simulator.yaml > log/counter_bridge.log 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} ✓ Counter Bridge (Simulator)"

echo ""
echo -e "${GREEN}[INFO]${NC} 模拟环境启动完成"
echo -e "${GREEN}[INFO]${NC} 启动策略: ./scripts/start_strategy.sh 92201 ./controls/day/..."
echo ""
SCRIPT_EOF

# CTP 实盘启动脚本
cat > "${DEPLOY_DIR}/scripts/start_ctp.sh" << 'SCRIPT_EOF'
#!/bin/bash
# 启动 CTP 实盘环境
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

CTP_MD_CONFIG="config/ctp/ctp_md.secret.yaml"
CTP_TD_CONFIG="config/ctp/ctp_td.secret.yaml"

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  QuantLink Trade System - CTP 实盘环境"
echo "════════════════════════════════════════════════════════════"
echo ""
echo -e "${RED}  ⚠️  警告: 这是实盘环境！${NC}"
echo ""

# 检查配置
if [ ! -f "$CTP_MD_CONFIG" ] || [ ! -f "$CTP_TD_CONFIG" ]; then
    echo -e "${RED}[ERROR]${NC} CTP 配置文件不存在"
    echo "请先配置: cp config/ctp/ctp_md.yaml $CTP_MD_CONFIG"
    exit 1
fi

read -p "确认启动 CTP 实盘? (y/N): " confirm
[ "$confirm" != "y" ] && [ "$confirm" != "Y" ] && exit 0

# 清理共享内存
ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true

mkdir -p log
DATE=$(date +%Y%m%d)

# 1. NATS
if ! pgrep -f "nats-server" > /dev/null 2>&1; then
    nats-server -p 4222 > log/nats.log 2>&1 &
    sleep 1
fi
echo -e "${GREEN}[INFO]${NC} ✓ NATS Server"

# 2. CTP MD Gateway
./bin/ctp_md_gateway --config "$CTP_MD_CONFIG" > "log/ctp_md_gateway.${DATE}.log" 2>&1 &
sleep 2
echo -e "${GREEN}[INFO]${NC} ✓ CTP MD Gateway"

# 3. MD Gateway
./bin/md_gateway > "log/md_gateway.${DATE}.log" 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} ✓ MD Gateway"

# 4. ORS Gateway
./bin/ors_gateway > "log/ors_gateway.${DATE}.log" 2>&1 &
sleep 1
echo -e "${GREEN}[INFO]${NC} ✓ ORS Gateway"

# 5. Counter Bridge (CTP)
./bin/counter_bridge ctp:"$CTP_TD_CONFIG" > "log/counter_bridge.${DATE}.log" 2>&1 &
sleep 2
echo -e "${GREEN}[INFO]${NC} ✓ Counter Bridge (CTP)"

echo ""
echo -e "${GREEN}[INFO]${NC} CTP 实盘环境启动完成"
echo ""
SCRIPT_EOF

# 停止脚本
cat > "${DEPLOY_DIR}/scripts/stop_all.sh" << 'SCRIPT_EOF'
#!/bin/bash
# 停止所有组件
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$(dirname "$SCRIPT_DIR")"

GREEN='\033[0;32m'
NC='\033[0m'

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  停止 QuantLink Trade System"
echo "════════════════════════════════════════════════════════════"
echo ""

for proc in trader counter_bridge ors_gateway md_gateway ctp_md_gateway md_simulator; do
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

# 策略启动脚本
cat > "${DEPLOY_DIR}/scripts/start_strategy.sh" << 'SCRIPT_EOF'
#!/bin/bash
# 启动策略
# Usage: ./scripts/start_strategy.sh <strategy_id> <control_file> [config_file]

set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

if [ $# -lt 2 ]; then
    echo "Usage: $0 <strategy_id> <control_file> [config_file]"
    echo "Example: $0 92201 ./controls/day/control.ag2602.ag2604.par.txt.92201"
    exit 1
fi

STRATEGY_ID=$1
CONTROL_FILE=$2
CONFIG_FILE=${3:-"config/trader.yaml"}
DATE=$(date +%Y%m%d)
LOG_FILE="./log/trader.${STRATEGY_ID}.${DATE}.log"

echo "Starting strategy ${STRATEGY_ID}..."
./bin/trader \
    --Live \
    --controlFile "$CONTROL_FILE" \
    --strategyID "$STRATEGY_ID" \
    --config "$CONFIG_FILE" \
    --log-file "$LOG_FILE"
SCRIPT_EOF

# 日盘启动脚本
cat > "${DEPLOY_DIR}/scripts/start.day.sh" << 'SCRIPT_EOF'
#!/bin/bash
# 日盘启动脚本
set -e
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$DEPLOY_ROOT"

ulimit -c unlimited
DATE=$(date +%Y%m%d)

echo "=========================================="
echo "日盘启动 - $(date)"
echo "=========================================="

# 策略 92201 - ag2602/ag2604
STRATEGY_ID="92201"
CONTROL_FILE="./controls/day/control.ag2602.ag2604.par.txt.92201"

if [ -f "$CONTROL_FILE" ]; then
    nohup ./bin/trader \
        --Live \
        --controlFile "$CONTROL_FILE" \
        --strategyID "$STRATEGY_ID" \
        --config "./config/trader.yaml" \
        --log-file "./log/trader.${STRATEGY_ID}.${DATE}.log" \
        >> "nohup.out.${STRATEGY_ID}" 2>&1 &
    echo "✓ Strategy ${STRATEGY_ID} started (PID: $!)"
else
    echo "⚠ Control file not found: ${CONTROL_FILE}"
fi

echo ""
echo "日盘启动完成"
echo "查看日志: tail -f log/trader.*.${DATE}.log"
SCRIPT_EOF

# 设置脚本可执行权限
chmod +x "${DEPLOY_DIR}/scripts/"*.sh
log_info "  ✓ start_simulator.sh (模拟环境)"
log_info "  ✓ start_ctp.sh (CTP 实盘)"
log_info "  ✓ stop_all.sh"
log_info "  ✓ start_strategy.sh"
log_info "  ✓ start.day.sh"

# ==================== 创建 README ====================
log_section "创建部署说明"

cat > "${DEPLOY_DIR}/README.md" << 'README_EOF'
# QuantLink Trade System - 部署包

## 目录结构

```
deploy/
├── bin/                    # 可执行文件
│   ├── trader              # Go 策略引擎
│   ├── md_gateway          # 行情网关
│   ├── md_simulator        # 行情模拟器
│   ├── ors_gateway         # 订单路由网关
│   ├── counter_bridge      # 成交网关
│   └── ctp_md_gateway      # CTP 行情网关
├── config/                 # 配置文件
├── controls/               # Control 文件
├── models/                 # Model 文件
├── data/positions/         # 持仓快照
├── log/                    # 日志目录
└── scripts/                # 启动脚本
    ├── start_simulator.sh  # 模拟环境
    ├── start_ctp.sh        # CTP 实盘
    ├── start_strategy.sh   # 启动策略
    └── stop_all.sh         # 停止服务
```

## 快速开始

### 模拟环境（开发测试）

```bash
cd deploy
./scripts/start_simulator.sh
./scripts/start_strategy.sh 92201 ./controls/day/control.ag2602.ag2604.par.txt.92201
./scripts/stop_all.sh
```

### CTP 实盘环境

```bash
cd deploy
# 1. 配置 CTP 账号
cp config/ctp/ctp_md.yaml config/ctp/ctp_md.secret.yaml
cp config/ctp/ctp_td.yaml config/ctp/ctp_td.secret.yaml

# 2. 启动
./scripts/start_ctp.sh
./scripts/start_strategy.sh 92201 ./controls/day/control.ag2602.ag2604.par.txt.92201
./scripts/stop_all.sh
```

## 日志查看

```bash
tail -f log/*.log
```

## 构建时间

本部署包构建于: $(date)
README_EOF

log_info "  ✓ README.md"

# ==================== 完成 ====================
log_section "构建完成"

# 显示 deploy 目录内容
echo ""
log_info "deploy 目录结构:"
find "${DEPLOY_DIR}" -type f | head -30 | while read f; do
    echo "  ${f#${DEPLOY_DIR}/}"
done

# 显示文件大小
echo ""
log_info "可执行文件:"
ls -lh "${DEPLOY_DIR}/bin/" 2>/dev/null | grep -v "^total" | awk '{print "  " $NF " (" $5 ")"}'

echo ""
log_info "部署包位置: ${DEPLOY_DIR}"
log_info "部署到服务器: scp -r ${DEPLOY_DIR}/ user@server:/opt/quantlink/"
echo ""

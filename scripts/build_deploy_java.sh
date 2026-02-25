#!/bin/bash
# ============================================
# 脚本名称: build_deploy_java.sh
# 用途: 编译 Java 策略引擎并部署到 deploy_java 目录
# 日期: 2026-02-25
#
# 使用方式:
#   ./scripts/build_deploy_java.sh              # 编译 + 测试 + 部署
#   ./scripts/build_deploy_java.sh --skip-test  # 编译 + 部署（跳过测试）
#   ./scripts/build_deploy_java.sh --clean      # 清理后重新编译
#   ./scripts/build_deploy_java.sh --test-only  # 仅运行测试
#
# 目录设计:
#   deploy_java/
#   ├── lib/              - JAR 包 + 依赖
#   │   ├── trader-1.0-SNAPSHOT.jar
#   │   └── snakeyaml-2.2.jar
#   ├── config/           - 配置文件（从 data_new 复制）
#   ├── data/             - 运行时数据（daily_init 等）
#   ├── log/              - 日志输出
#   └── scripts/          - 启停脚本
#
# 相关文档:
#   - @docs/java迁移/2026-02-25-10_00_java_迁移可行性评估.md
# ============================================

set -e

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
log_section() { echo -e "\n${BLUE}=== $1 ===${NC}"; }

# 参数解析
SKIP_TEST=false
CLEAN=false
TEST_ONLY=false

for arg in "$@"; do
    case "$arg" in
        --skip-test) SKIP_TEST=true ;;
        --clean)     CLEAN=true ;;
        --test-only) TEST_ONLY=true ;;
        *) log_error "未知参数: $arg"; exit 1 ;;
    esac
done

# 检测 Java 和 Maven
JAVA_HOME="${JAVA_HOME:-/Users/user/Library/Java/JavaVirtualMachines/openjdk-25.0.1/Contents/Home}"
MVN="${MVN:-/opt/homebrew/bin/mvn}"

if [ ! -d "$JAVA_HOME" ]; then
    log_error "JAVA_HOME 不存在: $JAVA_HOME"
    exit 1
fi

if [ ! -x "$MVN" ]; then
    log_error "Maven 不可用: $MVN"
    exit 1
fi

export JAVA_HOME
JAVA_VERSION=$("$JAVA_HOME/bin/java" -version 2>&1 | head -1)
log_info "Java: $JAVA_VERSION"
log_info "Maven: $MVN"

JAVA_DIR="$PROJECT_ROOT/tbsrc-java"
DEPLOY_DIR="$PROJECT_ROOT/deploy_java"

# ---- 清理 ----
if $CLEAN; then
    log_section "清理编译产物"
    "$MVN" -f "$JAVA_DIR/pom.xml" clean -q
    log_info "Maven clean 完成"
fi

# ---- 仅测试 ----
if $TEST_ONLY; then
    log_section "运行测试"
    "$MVN" -f "$JAVA_DIR/pom.xml" test
    log_info "测试完成"
    exit 0
fi

# ---- 编译 ----
log_section "编译 Java 策略引擎"
if $SKIP_TEST; then
    "$MVN" -f "$JAVA_DIR/pom.xml" package -DskipTests -q
    log_info "编译完成（跳过测试）"
else
    "$MVN" -f "$JAVA_DIR/pom.xml" package
    log_info "编译 + 测试完成"
fi

# ---- 部署 ----
log_section "部署到 $DEPLOY_DIR"

# 创建部署目录
mkdir -p "$DEPLOY_DIR"/{lib,config,data,log,scripts}

# 复制 JAR
cp "$JAVA_DIR/target/trader-1.0-SNAPSHOT.jar" "$DEPLOY_DIR/lib/"
log_info "主 JAR 已复制"

# 复制依赖
if [ -d "$JAVA_DIR/target/lib" ]; then
    cp "$JAVA_DIR/target/lib/"*.jar "$DEPLOY_DIR/lib/" 2>/dev/null || true
    log_info "依赖 JAR 已复制"
fi

# 复制配置（从 data_new/common 和 data_new/sim）
if [ -d "$PROJECT_ROOT/data_new/common/config" ]; then
    cp -r "$PROJECT_ROOT/data_new/common/config/"* "$DEPLOY_DIR/config/" 2>/dev/null || true
fi
if [ -d "$PROJECT_ROOT/data_new/sim/config" ]; then
    cp -r "$PROJECT_ROOT/data_new/sim/config/"* "$DEPLOY_DIR/config/" 2>/dev/null || true
fi

# 创建启动脚本
cat > "$DEPLOY_DIR/scripts/run_tests.sh" << 'SCRIPT'
#!/bin/bash
# 运行 Java 策略引擎测试
set -e
DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT_ROOT="$(cd "$DEPLOY_DIR/.." && pwd)"
JAVA_HOME="${JAVA_HOME:-/Users/user/Library/Java/JavaVirtualMachines/openjdk-25.0.1/Contents/Home}"
MVN="${MVN:-/opt/homebrew/bin/mvn}"
export JAVA_HOME

echo "[INFO] 运行 Java 策略引擎测试..."
"$MVN" -f "$PROJECT_ROOT/tbsrc-java/pom.xml" test
echo "[INFO] 测试完成"
SCRIPT
chmod +x "$DEPLOY_DIR/scripts/run_tests.sh"

# 创建验证脚本
cat > "$DEPLOY_DIR/scripts/verify_deploy.sh" << 'SCRIPT'
#!/bin/bash
# 验证部署完整性
set -e
DEPLOY_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
JAVA_HOME="${JAVA_HOME:-/Users/user/Library/Java/JavaVirtualMachines/openjdk-25.0.1/Contents/Home}"
export JAVA_HOME

echo "=== 部署验证 ==="

# 检查 JAR 存在
if [ -f "$DEPLOY_DIR/lib/trader-1.0-SNAPSHOT.jar" ]; then
    echo "[OK] 主 JAR 存在: $(ls -lh "$DEPLOY_DIR/lib/trader-1.0-SNAPSHOT.jar" | awk '{print $5}')"
else
    echo "[FAIL] 主 JAR 不存在"
    exit 1
fi

# 检查依赖
JAR_COUNT=$(ls "$DEPLOY_DIR/lib/"*.jar 2>/dev/null | wc -l)
echo "[OK] JAR 文件数: $JAR_COUNT"

# 列出类
echo ""
echo "=== 策略类 ==="
"$JAVA_HOME/bin/jar" tf "$DEPLOY_DIR/lib/trader-1.0-SNAPSHOT.jar" | grep "strategy/" | sort
echo ""
echo "=== 核心类 ==="
"$JAVA_HOME/bin/jar" tf "$DEPLOY_DIR/lib/trader-1.0-SNAPSHOT.jar" | grep "core/" | sort
echo ""
echo "=== SHM 类 ==="
"$JAVA_HOME/bin/jar" tf "$DEPLOY_DIR/lib/trader-1.0-SNAPSHOT.jar" | grep "shm/" | sort
echo ""
echo "=== Connector 类 ==="
"$JAVA_HOME/bin/jar" tf "$DEPLOY_DIR/lib/trader-1.0-SNAPSHOT.jar" | grep "connector/" | sort

echo ""
echo "=== 验证完成 ==="
SCRIPT
chmod +x "$DEPLOY_DIR/scripts/verify_deploy.sh"

# ---- 汇总 ----
log_section "部署完成"
echo ""
log_info "部署目录: $DEPLOY_DIR"
log_info "目录结构:"
echo "  deploy_java/"
echo "  ├── lib/           $(ls "$DEPLOY_DIR/lib/"*.jar 2>/dev/null | wc -l | tr -d ' ') 个 JAR"
echo "  ├── config/        配置文件"
echo "  ├── data/          运行时数据"
echo "  ├── log/           日志输出"
echo "  └── scripts/       启停脚本"
echo ""
log_info "验证: ./deploy_java/scripts/verify_deploy.sh"
log_info "测试: ./deploy_java/scripts/run_tests.sh"

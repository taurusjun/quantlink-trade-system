#!/bin/bash
# QuantlinkTrader 一键编译打包部署脚本
# 用途: 从零开始编译、打包、准备部署包
# Usage: ./scripts/quick_deploy.sh [deploy|remote]
#   deploy: 本地编译并准备部署包
#   remote: 编译、打包并上传到远程服务器

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

DEPLOY_MODE="${1:-deploy}"
DEPLOY_DIR="deploy"
PACKAGE_NAME="quantlink-trader-$(date +%Y%m%d-%H%M%S).tar.gz"

# ═══════════════════════════════════════════════════════════
# 打印横幅
# ═══════════════════════════════════════════════════════════
print_banner() {
    echo -e "${BLUE}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${BLUE}║     QuantlinkTrader 一键编译打包部署工具                  ║${NC}"
    echo -e "${BLUE}║                                                           ║${NC}"
    echo -e "${BLUE}║     快速构建生产环境部署包                                 ║${NC}"
    echo -e "${BLUE}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# ═══════════════════════════════════════════════════════════
# 检查依赖
# ═══════════════════════════════════════════════════════════
check_dependencies() {
    echo -e "${YELLOW}[1/6] 检查依赖环境...${NC}"

    local missing_deps=()

    # 检查 Go
    if ! command -v go &> /dev/null; then
        missing_deps+=("go (brew install go)")
    else
        echo -e "${GREEN}  ✓ Go $(go version | awk '{print $3}')${NC}"
    fi

    # 检查 CMake
    if ! command -v cmake &> /dev/null; then
        missing_deps+=("cmake (brew install cmake)")
    else
        echo -e "${GREEN}  ✓ CMake $(cmake --version | head -1 | awk '{print $3}')${NC}"
    fi

    # 检查 protoc
    if ! command -v protoc &> /dev/null; then
        missing_deps+=("protobuf (brew install protobuf)")
    else
        echo -e "${GREEN}  ✓ protoc $(protoc --version | awk '{print $2}')${NC}"
    fi

    # 检查 gRPC
    if ! pkg-config --exists grpc++ 2>/dev/null; then
        missing_deps+=("grpc (brew install grpc)")
    else
        echo -e "${GREEN}  ✓ gRPC $(pkg-config --modversion grpc++)${NC}"
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        echo -e "${RED}✗ 缺少以下依赖:${NC}"
        for dep in "${missing_deps[@]}"; do
            echo -e "${RED}  - $dep${NC}"
        done
        echo ""
        echo -e "${YELLOW}请先安装缺失的依赖，然后重新运行此脚本${NC}"
        exit 1
    fi

    echo -e "${GREEN}  ✓ 所有依赖已就绪${NC}\n"
}

# ═══════════════════════════════════════════════════════════
# 生成 Protobuf 代码
# ═══════════════════════════════════════════════════════════
generate_proto() {
    echo -e "${YELLOW}[2/6] 生成 Protobuf 代码...${NC}"

    if [ -f "scripts/generate_proto.sh" ]; then
        ./scripts/generate_proto.sh > /tmp/proto_gen.log 2>&1
        if [ $? -eq 0 ]; then
            echo -e "${GREEN}  ✓ Protobuf 代码生成成功${NC}\n"
        else
            echo -e "${RED}  ✗ Protobuf 代码生成失败${NC}"
            cat /tmp/proto_gen.log
            exit 1
        fi
    else
        echo -e "${YELLOW}  ⚠ scripts/generate_proto.sh 不存在，跳过${NC}\n"
    fi
}

# ═══════════════════════════════════════════════════════════
# 编译 C++ Gateway
# ═══════════════════════════════════════════════════════════
build_gateway() {
    echo -e "${YELLOW}[3/6] 编译 C++ Gateway 组件...${NC}"

    # 清理旧的构建
    if [ -d "gateway/build" ]; then
        echo -e "${CYAN}  清理旧的构建目录...${NC}"
        rm -rf gateway/build
    fi

    # 创建构建目录
    mkdir -p gateway/build
    cd gateway/build

    # CMake 配置
    echo -e "${CYAN}  运行 CMake 配置...${NC}"
    cmake .. -DCMAKE_BUILD_TYPE=Release > /tmp/cmake.log 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${RED}  ✗ CMake 配置失败${NC}"
        cat /tmp/cmake.log
        exit 1
    fi

    # 编译
    echo -e "${CYAN}  编译中（使用 $(sysctl -n hw.ncpu 2>/dev/null || nproc) 个 CPU 核心）...${NC}"
    make -j$(sysctl -n hw.ncpu 2>/dev/null || nproc) > /tmp/make.log 2>&1
    if [ $? -ne 0 ]; then
        echo -e "${RED}  ✗ 编译失败${NC}"
        tail -50 /tmp/make.log
        exit 1
    fi

    cd "$PROJECT_ROOT"

    # 验证可执行文件
    local executables=("md_simulator" "md_gateway" "ors_gateway" "counter_gateway")
    for exe in "${executables[@]}"; do
        if [ -f "gateway/build/$exe" ]; then
            echo -e "${GREEN}  ✓ $exe${NC}"
        else
            echo -e "${RED}  ✗ $exe 未生成${NC}"
            exit 1
        fi
    done

    echo ""
}

# ═══════════════════════════════════════════════════════════
# 编译 Golang Trader
# ═══════════════════════════════════════════════════════════
build_golang() {
    echo -e "${YELLOW}[4/6] 编译 Golang Trader...${NC}"

    cd "$PROJECT_ROOT/golang"

    # 更新依赖
    echo -e "${CYAN}  更新 Go 模块依赖...${NC}"
    go mod tidy > /tmp/go_mod.log 2>&1

    # 编译
    echo -e "${CYAN}  编译 Trader...${NC}"
    go build -o ../bin/trader cmd/trader/main.go 2>&1 | tee /tmp/go_build.log
    if [ $? -ne 0 ]; then
        echo -e "${RED}  ✗ Trader 编译失败${NC}"
        cat /tmp/go_build.log
        exit 1
    fi

    cd "$PROJECT_ROOT"

    # 验证
    if [ -f "bin/trader" ]; then
        local size=$(ls -lh bin/trader | awk '{print $5}')
        echo -e "${GREEN}  ✓ bin/trader (大小: $size)${NC}\n"
    else
        echo -e "${RED}  ✗ bin/trader 未生成${NC}"
        exit 1
    fi
}

# ═══════════════════════════════════════════════════════════
# 准备部署包
# ═══════════════════════════════════════════════════════════
prepare_deployment() {
    echo -e "${YELLOW}[5/6] 准备部署包...${NC}"

    # 清理旧的部署目录
    if [ -d "$DEPLOY_DIR" ]; then
        rm -rf "$DEPLOY_DIR"
    fi

    # 创建目录结构
    mkdir -p "$DEPLOY_DIR"/{bin,config,log}

    # 复制可执行文件
    echo -e "${CYAN}  复制可执行文件...${NC}"
    cp bin/trader "$DEPLOY_DIR/bin/" && echo -e "${GREEN}    ✓ trader${NC}"
    cp gateway/build/md_simulator "$DEPLOY_DIR/bin/" && echo -e "${GREEN}    ✓ md_simulator${NC}"
    cp gateway/build/md_gateway "$DEPLOY_DIR/bin/" && echo -e "${GREEN}    ✓ md_gateway${NC}"
    cp gateway/build/ors_gateway "$DEPLOY_DIR/bin/" && echo -e "${GREEN}    ✓ ors_gateway${NC}"
    cp gateway/build/counter_gateway "$DEPLOY_DIR/bin/" && echo -e "${GREEN}    ✓ counter_gateway${NC}"

    # 复制配置文件
    echo -e "${CYAN}  复制配置文件...${NC}"
    if [ -d "config" ]; then
        cp config/*.yaml "$DEPLOY_DIR/config/" 2>/dev/null && echo -e "${GREEN}    ✓ 配置文件${NC}"
    fi

    # 创建启动脚本
    echo -e "${CYAN}  创建启动脚本...${NC}"
    cat > "$DEPLOY_DIR/start.sh" << 'EOFSTART'
#!/bin/bash
# QuantlinkTrader 启动脚本

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${YELLOW}启动 QuantlinkTrader 系统...${NC}\n"

# 检查 NATS
if ! lsof -i :4222 >/dev/null 2>&1; then
    echo -e "${YELLOW}警告: NATS 服务未运行 (端口 4222)${NC}"
    echo "请先启动: nats-server &"
    exit 1
fi

# 创建日志目录
mkdir -p log

# 启动组件
echo "启动行情模拟器..."
./bin/md_simulator 100 queue > log/md_simulator.log 2>&1 &
echo "  PID: $!"

sleep 1
echo "启动行情网关..."
./bin/md_gateway queue > log/md_gateway.log 2>&1 &
echo "  PID: $!"

sleep 1
echo "启动订单路由服务..."
./bin/ors_gateway > log/ors_gateway.log 2>&1 &
echo "  PID: $!"

sleep 1
echo "启动模拟成交网关..."
./bin/counter_gateway > log/counter_gateway.log 2>&1 &
echo "  PID: $!"

sleep 2
echo "启动交易策略引擎..."
./bin/trader -config config/trader.yaml > log/trader.log 2>&1 &
echo "  PID: $!"

sleep 3

echo ""
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}所有组件已启动！${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo "查看日志:"
echo "  tail -f log/trader.log"
echo ""
echo "激活策略 (需手动执行):"
echo "  curl -X POST http://localhost:9201/api/v1/strategy/activate \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"strategy_id\": \"prod_92201\"}'"
echo ""
echo "停止系统:"
echo "  ./stop.sh"
EOFSTART

    chmod +x "$DEPLOY_DIR/start.sh"
    echo -e "${GREEN}    ✓ start.sh${NC}"

    # 创建停止脚本
    cat > "$DEPLOY_DIR/stop.sh" << 'EOFSTOP'
#!/bin/bash
# QuantlinkTrader 停止脚本

echo "停止 QuantlinkTrader 系统..."

pkill -f md_simulator && echo "  ✓ md_simulator 已停止"
pkill -f md_gateway && echo "  ✓ md_gateway 已停止"
pkill -f ors_gateway && echo "  ✓ ors_gateway 已停止"
pkill -f counter_gateway && echo "  ✓ counter_gateway 已停止"
pkill -f "trader -config" && echo "  ✓ trader 已停止"

sleep 2

# 清理共享内存
ipcs -m 2>/dev/null | grep $(whoami) | awk '{print $2}' | xargs -r ipcrm -m 2>/dev/null
echo "  ✓ 共享内存已清理"

echo ""
echo "所有组件已停止"
EOFSTOP

    chmod +x "$DEPLOY_DIR/stop.sh"
    echo -e "${GREEN}    ✓ stop.sh${NC}"

    # 创建 README
    cat > "$DEPLOY_DIR/README.md" << 'EOFREADME'
# QuantlinkTrader 部署包

## 快速启动

1. 确保 NATS 服务已启动:
   ```bash
   nats-server &
   ```

2. 启动系统:
   ```bash
   ./start.sh
   ```

3. 激活策略:
   ```bash
   curl -X POST http://localhost:9201/api/v1/strategy/activate \
     -H 'Content-Type: application/json' \
     -d '{"strategy_id": "prod_92201"}'
   ```

4. 停止系统:
   ```bash
   ./stop.sh
   ```

## 目录结构

```
deploy/
├── bin/              # 可执行文件
├── config/           # 配置文件
├── log/              # 日志文件
├── start.sh          # 启动脚本
├── stop.sh           # 停止脚本
└── README.md         # 本文件
```

## 监控

- 查看主日志: `tail -f log/trader.log`
- 查看订单: `grep "Order sent" log/trader.log`
- 查看策略统计: `grep "Stats:" log/trader.log`
- 查看状态: `curl http://localhost:9201/api/v1/strategy/status`

## 配置

编辑 `config/trader.yaml` 修改策略参数。

重要参数:
- `strategy.parameters.entry_zscore` - 入场阈值
- `risk.stop_loss` - 止损金额
- `session.start_time/end_time` - 交易时段
EOFREADME

    echo -e "${GREEN}    ✓ README.md${NC}"

    echo -e "${GREEN}  ✓ 部署包准备完成: $DEPLOY_DIR/${NC}\n"
}

# ═══════════════════════════════════════════════════════════
# 打包部署包
# ═══════════════════════════════════════════════════════════
package_deployment() {
    echo -e "${YELLOW}[6/6] 打包部署包...${NC}"

    if [ "$DEPLOY_MODE" == "local" ]; then
        echo -e "${CYAN}  跳过打包（本地部署模式）${NC}\n"
        return
    fi

    echo -e "${CYAN}  压缩部署包...${NC}"
    tar czf "$PACKAGE_NAME" "$DEPLOY_DIR/" 2>&1
    if [ $? -eq 0 ]; then
        local size=$(ls -lh "$PACKAGE_NAME" | awk '{print $5}')
        echo -e "${GREEN}  ✓ $PACKAGE_NAME (大小: $size)${NC}\n"
    else
        echo -e "${RED}  ✗ 打包失败${NC}"
        exit 1
    fi
}

# ═══════════════════════════════════════════════════════════
# 上传到远程服务器
# ═══════════════════════════════════════════════════════════
upload_to_remote() {
    if [ "$DEPLOY_MODE" != "remote" ]; then
        return
    fi

    echo -e "${YELLOW}上传到远程服务器...${NC}\n"

    # 提示输入服务器信息
    read -p "输入远程服务器地址 (例如: user@192.168.1.100): " REMOTE_SERVER

    if [ -z "$REMOTE_SERVER" ]; then
        echo -e "${YELLOW}未输入服务器地址，跳过上传${NC}"
        return
    fi

    echo -e "${CYAN}上传 $PACKAGE_NAME 到 $REMOTE_SERVER:~/${NC}"
    scp "$PACKAGE_NAME" "$REMOTE_SERVER:~/"

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ 上传成功${NC}\n"
        echo -e "${CYAN}在远程服务器上执行以下命令部署:${NC}"
        echo -e "  ssh $REMOTE_SERVER"
        echo -e "  tar xzf $PACKAGE_NAME"
        echo -e "  cd deploy"
        echo -e "  ./start.sh"
    else
        echo -e "${RED}✗ 上传失败${NC}"
    fi
}

# ═══════════════════════════════════════════════════════════
# 显示完成信息
# ═══════════════════════════════════════════════════════════
show_completion() {
    echo ""
    echo -e "${GREEN}╔═══════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║               编译打包完成！                               ║${NC}"
    echo -e "${GREEN}╚═══════════════════════════════════════════════════════════╝${NC}"
    echo ""

    echo -e "${CYAN}部署包位置:${NC}"
    echo -e "  目录: $PROJECT_ROOT/$DEPLOY_DIR/"
    if [ -f "$PACKAGE_NAME" ]; then
        echo -e "  压缩包: $PROJECT_ROOT/$PACKAGE_NAME"
    fi
    echo ""

    echo -e "${CYAN}本地测试:${NC}"
    echo -e "  cd $DEPLOY_DIR"
    echo -e "  nats-server &        # 启动 NATS"
    echo -e "  ./start.sh           # 启动系统"
    echo ""

    if [ -f "$PACKAGE_NAME" ]; then
        echo -e "${CYAN}部署到服务器:${NC}"
        echo -e "  scp $PACKAGE_NAME user@server:~/"
        echo -e "  ssh user@server"
        echo -e "  tar xzf $PACKAGE_NAME"
        echo -e "  cd deploy && ./start.sh"
        echo ""
    fi

    echo -e "${CYAN}查看详细文档:${NC}"
    echo -e "  docs/系统_编译部署启动指南_2026-01-24-16_15.md"
    echo ""
}

# ═══════════════════════════════════════════════════════════
# 主流程
# ═══════════════════════════════════════════════════════════
main() {
    print_banner

    # 解析参数
    case "$DEPLOY_MODE" in
        deploy|local)
            DEPLOY_MODE="deploy"
            echo -e "${CYAN}模式: 本地编译并准备部署包${NC}\n"
            ;;
        remote)
            echo -e "${CYAN}模式: 编译、打包并上传到远程服务器${NC}\n"
            ;;
        *)
            echo -e "${RED}错误: 未知模式 '$DEPLOY_MODE'${NC}"
            echo "用法: $0 [deploy|remote]"
            echo "  deploy: 本地编译并准备部署包 (默认)"
            echo "  remote: 编译、打包并上传到远程服务器"
            exit 1
            ;;
    esac

    # 执行构建流程
    check_dependencies
    generate_proto
    build_gateway
    build_golang
    prepare_deployment
    package_deployment
    upload_to_remote
    show_completion
}

# 运行主流程
main "$@"

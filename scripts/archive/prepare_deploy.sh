#!/bin/bash
# 准备部署包 - 收集所有可执行文件和配置
# Usage: ./scripts/prepare_deploy.sh [output_dir]

set -e

# 颜色定义
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

DEPLOY_DIR="${1:-deploy}"

echo -e "${YELLOW}准备部署包...${NC}"
echo -e "输出目录: $DEPLOY_DIR"
echo ""

# 清理并创建部署目录
rm -rf "$DEPLOY_DIR"
mkdir -p "$DEPLOY_DIR/bin"
mkdir -p "$DEPLOY_DIR/config"
mkdir -p "$DEPLOY_DIR/log"

# ═══════════════════════════════════════════════════════════
# 复制可执行文件
# ═══════════════════════════════════════════════════════════
echo -e "${YELLOW}[1/3] 复制可执行文件...${NC}"

# Golang Trader
if [ -f "bin/trader" ]; then
    cp bin/trader "$DEPLOY_DIR/bin/"
    echo -e "${GREEN}✓ bin/trader${NC}"
else
    echo "警告: bin/trader 不存在，请先编译"
fi

# C++ Gateways
if [ -f "gateway/build/md_simulator" ]; then
    cp gateway/build/md_simulator "$DEPLOY_DIR/bin/"
    echo -e "${GREEN}✓ gateway/build/md_simulator${NC}"
fi

if [ -f "gateway/build/md_gateway" ]; then
    cp gateway/build/md_gateway "$DEPLOY_DIR/bin/"
    echo -e "${GREEN}✓ gateway/build/md_gateway${NC}"
fi

if [ -f "gateway/build/ors_gateway" ]; then
    cp gateway/build/ors_gateway "$DEPLOY_DIR/bin/"
    echo -e "${GREEN}✓ gateway/build/ors_gateway${NC}"
fi

if [ -f "gateway/build/counter_gateway" ]; then
    cp gateway/build/counter_gateway "$DEPLOY_DIR/bin/"
    echo -e "${GREEN}✓ gateway/build/counter_gateway${NC}"
fi

# ═══════════════════════════════════════════════════════════
# 复制配置文件
# ═══════════════════════════════════════════════════════════
echo ""
echo -e "${YELLOW}[2/3] 复制配置文件...${NC}"

if [ -d "config" ]; then
    cp config/*.yaml "$DEPLOY_DIR/config/" 2>/dev/null || true
    echo -e "${GREEN}✓ 配置文件已复制${NC}"
fi

# ═══════════════════════════════════════════════════════════
# 创建启动脚本
# ═══════════════════════════════════════════════════════════
echo ""
echo -e "${YELLOW}[3/3] 创建启动脚本...${NC}"

cat > "$DEPLOY_DIR/start.sh" << 'EOFSCRIPT'
#!/bin/bash
# 快速启动脚本

# 检查 NATS
if ! lsof -i :4222 >/dev/null 2>&1; then
    echo "警告: NATS 服务未运行 (端口 4222)"
    echo "请先启动: nats-server &"
    exit 1
fi

# 启动组件
echo "启动行情模拟器..."
./bin/md_simulator 100 queue > log/md_simulator.log 2>&1 &

sleep 1
echo "启动行情网关..."
./bin/md_gateway queue > log/md_gateway.log 2>&1 &

sleep 1
echo "启动订单路由服务..."
./bin/ors_gateway > log/ors_gateway.log 2>&1 &

sleep 1
echo "启动模拟成交网关..."
./bin/counter_gateway > log/counter_gateway.log 2>&1 &

sleep 2
echo "启动交易策略..."
./bin/trader -config config/trader.yaml > log/trader.log 2>&1 &

echo ""
echo "所有组件已启动！"
echo "查看日志: tail -f log/trader.log"
EOFSCRIPT

chmod +x "$DEPLOY_DIR/start.sh"
echo -e "${GREEN}✓ start.sh 已创建${NC}"

cat > "$DEPLOY_DIR/stop.sh" << 'EOFSCRIPT'
#!/bin/bash
# 停止所有组件

echo "停止所有组件..."
pkill -f md_simulator
pkill -f md_gateway
pkill -f ors_gateway
pkill -f counter_gateway
pkill -f "trader -config"

echo "所有组件已停止"
EOFSCRIPT

chmod +x "$DEPLOY_DIR/stop.sh"
echo -e "${GREEN}✓ stop.sh 已创建${NC}"

# ═══════════════════════════════════════════════════════════
# 完成
# ═══════════════════════════════════════════════════════════
echo ""
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}部署包准备完成！${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo ""
echo "位置: $DEPLOY_DIR/"
echo ""
echo "部署包内容:"
ls -lh "$DEPLOY_DIR/bin/"
echo ""
echo "部署到服务器:"
echo "  tar czf quantlink-trader.tar.gz $DEPLOY_DIR/"
echo "  scp quantlink-trader.tar.gz user@server:~/"
echo ""
echo "服务器上解压并运行:"
echo "  tar xzf quantlink-trader.tar.gz"
echo "  cd $DEPLOY_DIR"
echo "  ./start.sh"

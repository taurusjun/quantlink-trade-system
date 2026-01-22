#!/bin/bash
# Web UI 测试指南
# 这个脚本会帮助你测试 Web UI 的完整流程

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "════════════════════════════════════════════════════════════"
echo "QuantlinkTrader Web UI 测试指南"
echo "════════════════════════════════════════════════════════════"
echo ""

# Step 1: 检查编译
echo -e "${BLUE}[Step 1]${NC} 检查 QuantlinkTrader 是否已编译..."
if [ -f "bin/QuantlinkTrader" ]; then
    echo -e "${GREEN}✓${NC} bin/QuantlinkTrader 存在"
else
    echo -e "${RED}✗${NC} bin/QuantlinkTrader 不存在，正在编译..."
    go build -o bin/QuantlinkTrader ./cmd/trader
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓${NC} 编译成功"
    else
        echo -e "${RED}✗${NC} 编译失败"
        exit 1
    fi
fi
echo ""

# Step 2: 检查配置文件
echo -e "${BLUE}[Step 2]${NC} 检查配置文件..."
CONFIG_FILE="config/trader.ag2502.ag2504.yaml"
if [ -f "$CONFIG_FILE" ]; then
    echo -e "${GREEN}✓${NC} 配置文件存在: $CONFIG_FILE"

    # 检查 API 是否启用
    if grep -q "enabled: true" "$CONFIG_FILE"; then
        API_PORT=$(grep "port:" "$CONFIG_FILE" | grep -v "#" | awk '{print $2}')
        echo -e "${GREEN}✓${NC} API 已启用，端口: $API_PORT"
    else
        echo -e "${YELLOW}!${NC} API 未启用，请修改配置文件"
    fi
else
    echo -e "${RED}✗${NC} 配置文件不存在"
    exit 1
fi
echo ""

# Step 3: 检查是否有运行中的实例
echo -e "${BLUE}[Step 3]${NC} 检查是否已有运行中的 QuantlinkTrader..."
RUNNING_PIDS=$(pgrep -f "QuantlinkTrader" || true)

if [ -n "$RUNNING_PIDS" ]; then
    echo -e "${YELLOW}!${NC} 发现运行中的 QuantlinkTrader 进程:"
    ps aux | grep QuantlinkTrader | grep -v grep | awk '{print "  PID " $2 " - " $11}'
    echo ""
    read -p "是否停止现有进程并重新启动? [y/N]: " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "正在停止现有进程..."
        pkill -f "QuantlinkTrader"
        sleep 2
        echo -e "${GREEN}✓${NC} 已停止"
    else
        echo "保持现有进程运行"
    fi
else
    echo -e "${GREEN}✓${NC} 没有运行中的实例"
fi
echo ""

# Step 4: 启动 QuantlinkTrader（后台）
echo -e "${BLUE}[Step 4]${NC} 启动 QuantlinkTrader (后台运行)..."

# 创建日志目录
mkdir -p log

# 启动进程
nohup ./bin/QuantlinkTrader --config "$CONFIG_FILE" > log/test.out 2>&1 &
TRADER_PID=$!

echo -e "${GREEN}✓${NC} QuantlinkTrader 已启动 (PID: $TRADER_PID)"
echo "  日志文件: log/test.out"
echo ""

# 等待启动
echo "等待服务启动 (5秒)..."
sleep 5

# Step 5: 检查进程是否还在运行
echo -e "${BLUE}[Step 5]${NC} 验证进程状态..."
if ps -p $TRADER_PID > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} QuantlinkTrader 正在运行 (PID: $TRADER_PID)"
else
    echo -e "${RED}✗${NC} QuantlinkTrader 启动失败，查看日志:"
    tail -20 log/test.out
    exit 1
fi
echo ""

# Step 6: 测试 API 连接
echo -e "${BLUE}[Step 6]${NC} 测试 API 连接..."
sleep 2  # 再等待一下确保 API 完全启动

API_URL="http://localhost:9201/api/v1/health"
if curl -s -f "$API_URL" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} API 服务正常"

    # 显示健康检查结果
    echo ""
    echo "健康检查结果:"
    curl -s "$API_URL" | python3 -m json.tool 2>/dev/null || curl -s "$API_URL"
else
    echo -e "${RED}✗${NC} API 服务无响应"
    echo ""
    echo "检查日志:"
    tail -30 log/test.out
    echo ""
    echo "进程可能需要更多时间启动，请稍等片刻后手动测试:"
    echo "  curl http://localhost:9201/api/v1/health"
fi
echo ""
echo ""

# Step 7: 打开 Web UI
echo -e "${BLUE}[Step 7]${NC} 打开 Web UI..."
echo ""
echo "════════════════════════════════════════════════════════════"
echo "测试准备完成！"
echo "════════════════════════════════════════════════════════════"
echo ""
echo "QuantlinkTrader 信息:"
echo "  PID: $TRADER_PID"
echo "  配置: $CONFIG_FILE"
echo "  API 端口: 9201"
echo "  日志: log/test.out"
echo ""
echo "现在可以:"
echo ""
echo "  1. 打开 Web UI:"
echo "     ./openWebUI.sh"
echo ""
echo "  2. 或直接在浏览器打开:"
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "     open web/control.html"
elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
    echo "     xdg-open web/control.html"
else
    echo "     file://$SCRIPT_DIR/web/control.html"
fi
echo ""
echo "  3. 在 Web UI 中配置:"
echo "     API 地址: localhost"
echo "     端口: 9201"
echo "     点击 '连接并刷新状态'"
echo ""
echo "  4. 测试并发保护:"
echo "     - 疯狂点击 '激活策略' 按钮"
echo "     - 打开多个浏览器标签页同时点击"
echo "     - 查看日志确认只执行一次:"
echo "       tail -f log/test.out | grep 'Activating strategy'"
echo ""
echo "  5. 查看实时日志:"
echo "     tail -f log/test.out"
echo ""
echo "  6. 停止测试:"
echo "     kill $TRADER_PID"
echo "     或使用: pkill -f QuantlinkTrader"
echo ""
echo "════════════════════════════════════════════════════════════"

# 询问是否直接打开浏览器
echo ""
read -p "是否现在打开浏览器? [Y/n]: " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Nn]$ ]]; then
    if [[ "$OSTYPE" == "darwin"* ]]; then
        open web/control.html
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        xdg-open web/control.html
    fi
    echo -e "${GREEN}✓${NC} 浏览器已打开"
fi

echo ""
echo "测试愉快！"
echo ""

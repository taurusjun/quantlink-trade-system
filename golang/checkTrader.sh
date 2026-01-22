#!/bin/bash
# 检查 QuantlinkTrader 运行状态

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo "════════════════════════════════════════════════════════════"
echo "QuantlinkTrader 运行状态检查"
echo "════════════════════════════════════════════════════════════"
echo ""

# 1. 检查进程
echo -e "${BLUE}[1] 进程检查${NC}"
PIDS=$(pgrep -f "QuantlinkTrader" || true)

if [ -z "$PIDS" ]; then
    echo -e "${RED}✗${NC} QuantlinkTrader 未运行"
    echo ""
    echo "启动方法:"
    echo "  ./testWebUI.sh           # 测试模式（自动启动）"
    echo "  ./runTrade.sh 92201      # 生产模式"
    exit 1
else
    echo -e "${GREEN}✓${NC} QuantlinkTrader 正在运行"
    echo ""
    echo "进程列表:"
    ps aux | grep QuantlinkTrader | grep -v grep | awk '{print "  PID " $2 " - CPU " $3 "% - MEM " $4 "% - " $11 " " $12 " " $13}'
fi

echo ""

# 2. 检查 PID 文件
echo -e "${BLUE}[2] PID 文件检查${NC}"
PID_FILES=$(ls trader.*.pid 2>/dev/null || true)

if [ -z "$PID_FILES" ]; then
    echo -e "${YELLOW}!${NC} 没有找到 PID 文件"
else
    echo -e "${GREEN}✓${NC} 找到 PID 文件:"
    for pid_file in $PID_FILES; do
        STRATEGY_ID=$(echo $pid_file | sed 's/trader\.\(.*\)\.pid/\1/')
        PID=$(cat "$pid_file" 2>/dev/null || echo "无法读取")

        if ps -p $PID > /dev/null 2>&1; then
            echo -e "  ${GREEN}●${NC} $pid_file (PID: $PID, 策略: $STRATEGY_ID) - 运行中"
        else
            echo -e "  ${RED}●${NC} $pid_file (PID: $PID, 策略: $STRATEGY_ID) - 已停止"
        fi
    done
fi

echo ""

# 3. 检查 API 端口
echo -e "${BLUE}[3] API 服务检查${NC}"

# 常见端口列表
PORTS=(9201 9301 4101)

for port in "${PORTS[@]}"; do
    if curl -s -f "http://localhost:${port}/api/v1/health" > /dev/null 2>&1; then
        echo -e "${GREEN}✓${NC} 端口 $port - API 服务正常"

        # 显示策略信息
        HEALTH=$(curl -s "http://localhost:${port}/api/v1/health")
        STRATEGY_ID=$(echo $HEALTH | grep -o '"strategy_id":"[^"]*"' | cut -d'"' -f4)
        MODE=$(echo $HEALTH | grep -o '"mode":"[^"]*"' | cut -d'"' -f4)
        echo "    策略ID: $STRATEGY_ID, 模式: $MODE"
    else
        # 检查端口是否被占用
        if lsof -i :$port > /dev/null 2>&1; then
            echo -e "${YELLOW}!${NC} 端口 $port - 端口被占用但 API 无响应（可能正在启动）"
        else
            echo -e "  端口 $port - 未使用"
        fi
    fi
done

echo ""

# 4. 检查日志文件
echo -e "${BLUE}[4] 日志文件检查${NC}"
LOG_FILES=$(ls log/trader.*.log 2>/dev/null || ls log/test.out 2>/dev/null || true)

if [ -z "$LOG_FILES" ]; then
    echo -e "${YELLOW}!${NC} 没有找到日志文件"
else
    echo -e "${GREEN}✓${NC} 找到日志文件:"
    for log_file in $LOG_FILES; do
        SIZE=$(du -h "$log_file" | awk '{print $1}')
        MODIFIED=$(stat -f "%Sm" -t "%Y-%m-%d %H:%M:%S" "$log_file" 2>/dev/null || stat -c "%y" "$log_file" 2>/dev/null || echo "未知")
        echo "  - $log_file (大小: $SIZE, 修改: $MODIFIED)"
    done

    echo ""
    echo "查看最新日志:"
    echo "  tail -20 ${LOG_FILES[0]}"
    echo ""
    echo "实时跟踪日志:"
    echo "  tail -f ${LOG_FILES[0]}"
fi

echo ""
echo "════════════════════════════════════════════════════════════"

# 5. 快速操作提示
if [ -n "$PIDS" ]; then
    echo ""
    echo "快速操作:"
    echo "  查看日志:  tail -f log/trader.*.log"
    echo "  打开 UI:   ./openWebUI.sh"
    echo "  停止进程:  pkill -f QuantlinkTrader"
    echo "  激活策略:  curl -X POST http://localhost:9201/api/v1/strategy/activate"
    echo "  查询状态:  curl http://localhost:9201/api/v1/strategy/status | jq"
    echo ""
fi

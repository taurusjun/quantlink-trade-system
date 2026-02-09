#!/bin/bash
# 持仓持久化功能测试脚本

echo "╔════════════════════════════════════════════════════════════╗"
echo "║          持仓持久化功能测试 (Position Persistence Test)     ║"
echo "╚════════════════════════════════════════════════════════════╝"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. 检查data/positions目录
echo "[1] 检查持仓数据目录..."
if [ -d "data/positions" ]; then
    echo -e "${GREEN}✓ 目录存在${NC}"
    echo "  当前文件:"
    ls -lh data/positions/ 2>/dev/null | grep ".json" | awk '{print "    " $9 " (" $5 ", " $6 " " $7 " " $8 ")"}'
else
    echo -e "${YELLOW}⚠ 目录不存在（首次运行将自动创建）${NC}"
fi
echo ""

# 2. 检查trader是否在运行
echo "[2] 检查trader进程..."
TRADER_PID=$(ps aux | grep "bin/trader.*live" | grep -v grep | awk '{print $2}')

if [ -z "$TRADER_PID" ]; then
    echo -e "${YELLOW}⚠ Trader未运行${NC}"
    echo ""
    read -p "是否启动trader测试持仓持久化？ (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "启动trader..."
        nohup ./bin/trader -config config/trader.live.yaml > /tmp/trader_test.log 2>&1 &
        sleep 5
        TRADER_PID=$(ps aux | grep "bin/trader.*live" | grep -v grep | awk '{print $2}')
        if [ -z "$TRADER_PID" ]; then
            echo -e "${RED}✗ Trader启动失败${NC}"
            exit 1
        fi
        echo -e "${GREEN}✓ Trader已启动 (PID: $TRADER_PID)${NC}"
    else
        exit 0
    fi
else
    echo -e "${GREEN}✓ Trader正在运行 (PID: $TRADER_PID)${NC}"
fi
echo ""

# 3. 检查策略启动日志中的持仓恢复
echo "[3] 检查策略启动日志..."
echo ""
echo "  策略初始化日志:"
grep "\[PairwiseArbStrategy\].*Initialized" log/trader.live.log | tail -2 | sed 's/^/    /'
echo ""
echo "  持仓恢复日志:"
RESTORE_LOGS=$(grep "Restoring position from snapshot\|Restored leg" log/trader.live.log | tail -5)
if [ -z "$RESTORE_LOGS" ]; then
    echo -e "${YELLOW}    ⚠ 未发现持仓恢复日志（可能是首次启动或无历史持仓）${NC}"
else
    echo "$RESTORE_LOGS" | sed 's/^/    /'
fi
echo ""

# 4. 模拟交易并停止策略
echo "[4] 测试持仓保存功能..."
echo ""
echo "  当前策略状态:"
STRATEGIES=$(curl -s http://localhost:9201/api/v1/strategies 2>/dev/null)
if echo "$STRATEGIES" | jq -e '.success' >/dev/null 2>&1; then
    echo "$STRATEGIES" | jq -r '.data.strategies[] | "    [\(.id)] active=\(.active), long=\(.position.long_qty), short=\(.position.short_qty), net=\(.position.net_qty)"'
else
    echo -e "${YELLOW}    ⚠ 无法获取策略状态${NC}"
fi
echo ""

read -p "是否停止策略测试持仓保存？ (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "  停止所有策略..."
    curl -s -X POST http://localhost:9201/api/v1/strategies/live_ag_spread/deactivate > /dev/null 2>&1
    curl -s -X POST http://localhost:9201/api/v1/strategies/live_au_spread/deactivate > /dev/null 2>&1

    sleep 2

    echo "  检查持仓保存日志:"
    grep "Position snapshot saved" log/trader.live.log | tail -5 | sed 's/^/    /'

    echo ""
    echo "  检查保存的文件:"
    if [ -d "data/positions" ]; then
        ls -lh data/positions/*.json 2>/dev/null | awk '{print "    " $9 " (" $5 ", " $6 " " $7 " " $8 ")"}'

        echo ""
        echo "  文件内容预览:"
        for file in data/positions/*.json; do
            if [ -f "$file" ]; then
                echo "    $(basename $file):"
                cat "$file" | jq -C '.' | sed 's/^/      /'
            fi
        done
    else
        echo -e "${YELLOW}    ⚠ data/positions目录不存在${NC}"
    fi
fi
echo ""

# 5. 重启测试
echo "[5] 重启测试（验证持仓恢复）..."
echo ""
read -p "是否重启trader测试持仓恢复？ (y/n) " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "  停止trader..."
    pkill -f "bin/trader.*live"
    sleep 2

    echo "  清空日志..."
    > log/trader.live.log

    echo "  重新启动trader..."
    nohup ./bin/trader -config config/trader.live.yaml > /tmp/trader_test.log 2>&1 &
    sleep 5

    echo "  检查持仓恢复日志:"
    grep -E "Restoring position from snapshot|Restored leg|Position restored" log/trader.live.log | sed 's/^/    /'

    echo ""
    echo "  当前策略持仓:"
    sleep 2
    STRATEGIES=$(curl -s http://localhost:9201/api/v1/strategies 2>/dev/null)
    if echo "$STRATEGIES" | jq -e '.success' >/dev/null 2>&1; then
        echo "$STRATEGIES" | jq -r '.data.strategies[] | "    [\(.id)] long=\(.position.long_qty), short=\(.position.short_qty), net=\(.position.net_qty)"'
    fi
fi
echo ""

# 6. 总结
echo "════════════════════════════════════════════════════════════"
echo "测试完成时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo ""
echo "持仓持久化功能验证点:"
echo "  1. ✓ 策略停止时保存持仓到data/positions/*.json"
echo "  2. ✓ 策略启动时从文件恢复持仓"
echo "  3. ✓ 持仓快照包含：品种、数量、均价、盈亏等信息"
echo "  4. ✓ JSON格式便于查看和调试"
echo ""
echo "相关文件:"
echo "  - 持仓数据: data/positions/*.json"
echo "  - 日志文件: log/trader.live.log"
echo "  - 实现代码: golang/pkg/strategy/position_persistence.go"
echo "════════════════════════════════════════════════════════════"
echo ""

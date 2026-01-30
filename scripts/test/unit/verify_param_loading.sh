#!/bin/bash
# 参数加载验证脚本

echo "╔════════════════════════════════════════════════════════════════╗"
echo "║              参数加载验证脚本 (Parameter Loading Test)          ║"
echo "╚════════════════════════════════════════════════════════════════╝"
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 1. 检查trader是否在运行
echo "[1] 检查trader进程..."
TRADER_PID=$(ps aux | grep "bin/trader.*live" | grep -v grep | awk '{print $2}')

if [ -z "$TRADER_PID" ]; then
    echo -e "${RED}✗ Trader未运行${NC}"
    echo ""
    echo "请先启动trader:"
    echo "  ./bin/trader -config config/trader.live.yaml"
    echo ""
    exit 1
else
    echo -e "${GREEN}✓ Trader正在运行 (PID: $TRADER_PID)${NC}"
fi
echo ""

# 2. 等待初始化完成
echo "[2] 等待初始化完成（5秒）..."
sleep 5
echo ""

# 3. 检查初始化日志
echo "[3] 验证初始化日志..."
echo ""

# 检查白银策略
echo "  白银策略 (live_ag_spread):"
AG_INIT=$(grep "\[PairwiseArbStrategy:live_ag_spread\] Initialized" log/trader.live.log | tail -1)
if [ -z "$AG_INIT" ]; then
    echo -e "${RED}    ✗ 未找到初始化日志${NC}"
else
    echo "    $AG_INIT"

    # 验证参数
    if echo "$AG_INIT" | grep -q "entry_z=2.50"; then
        echo -e "${GREEN}    ✓ entry_zscore=2.50 正确${NC}"
    else
        echo -e "${RED}    ✗ entry_zscore错误${NC}"
    fi

    if echo "$AG_INIT" | grep -q "min_corr=0.70"; then
        echo -e "${GREEN}    ✓ min_correlation=0.70 正确${NC}"
    else
        echo -e "${RED}    ✗ min_correlation显示错误或缺失${NC}"
    fi

    if echo "$AG_INIT" | grep -q "slippage=2 ticks"; then
        echo -e "${GREEN}    ✓ slippage_ticks=2 正确${NC}"
    else
        echo -e "${RED}    ✗ slippage_ticks错误（可能为0）${NC}"
    fi
fi
echo ""

# 检查黄金策略
echo "  黄金策略 (live_au_spread):"
AU_INIT=$(grep "\[PairwiseArbStrategy:live_au_spread\] Initialized" log/trader.live.log | tail -1)
if [ -z "$AU_INIT" ]; then
    echo -e "${RED}    ✗ 未找到初始化日志${NC}"
else
    echo "    $AU_INIT"

    # 验证参数
    if echo "$AU_INIT" | grep -q "entry_z=2.80"; then
        echo -e "${GREEN}    ✓ entry_zscore=2.80 正确${NC}"
    else
        echo -e "${RED}    ✗ entry_zscore错误${NC}"
    fi

    if echo "$AU_INIT" | grep -q "min_corr=0.75"; then
        echo -e "${GREEN}    ✓ min_correlation=0.75 正确${NC}"
    else
        echo -e "${RED}    ✗ min_correlation显示错误或缺失${NC}"
    fi

    if echo "$AU_INIT" | grep -q "slippage=1 ticks"; then
        echo -e "${GREEN}    ✓ slippage_ticks=1 正确${NC}"
    else
        echo -e "${RED}    ✗ slippage_ticks错误（可能为0）${NC}"
    fi
fi
echo ""

# 4. 检查API状态
echo "[4] 检查API状态..."
API_STATUS=$(curl -s http://localhost:9201/api/v1/health 2>/dev/null)
if echo "$API_STATUS" | jq -e '.success' >/dev/null 2>&1; then
    echo -e "${GREEN}✓ API服务正常${NC}"
    MODE=$(echo "$API_STATUS" | jq -r '.data.mode')
    echo "  模式: $MODE"
else
    echo -e "${RED}✗ API服务异常${NC}"
fi
echo ""

# 5. 检查策略状态
echo "[5] 检查策略状态..."
STRATEGIES=$(curl -s http://localhost:9201/api/v1/strategies 2>/dev/null)

if echo "$STRATEGIES" | jq -e '.success' >/dev/null 2>&1; then
    echo "$STRATEGIES" | jq -r '.data.strategies[] | "  [\(.id)] active=\(.active), conditions_met=\(.conditions_met)"'
else
    echo -e "${RED}✗ 无法获取策略状态${NC}"
fi
echo ""

# 6. 查看最新统计
echo "[6] 最新策略统计 (最近5条)..."
STATS=$(tail -100 log/trader.live.log | grep "Stats:" | tail -5)
if [ -z "$STATS" ]; then
    echo -e "${YELLOW}⚠ 暂无统计数据（等待市场数据）${NC}"
else
    echo "$STATS" | sed 's/^/  /'
fi
echo ""

# 7. 检查订单
echo "[7] 订单统计..."
ORDER_COUNT=$(grep -c "Order sent" log/trader.live.log 2>/dev/null || echo "0")
echo "  总订单数: $ORDER_COUNT"

if [ "$ORDER_COUNT" -gt 0 ]; then
    echo ""
    echo "  最近5笔订单:"
    grep "Order sent" log/trader.live.log | tail -5 | sed 's/^/    /'
fi
echo ""

# 8. 总结
echo "════════════════════════════════════════════════════════════════"
echo "验证总结:"
echo ""

# 统计验证结果
SUCCESS=0
FAILED=0

if echo "$AG_INIT" | grep -q "min_corr=0.70"; then
    ((SUCCESS++))
else
    ((FAILED++))
fi

if echo "$AG_INIT" | grep -q "slippage=2"; then
    ((SUCCESS++))
else
    ((FAILED++))
fi

if echo "$AU_INIT" | grep -q "min_corr=0.75"; then
    ((SUCCESS++))
else
    ((FAILED++))
fi

if echo "$AU_INIT" | grep -q "slippage=1"; then
    ((SUCCESS++))
else
    ((FAILED++))
fi

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ 参数加载验证通过 ($SUCCESS/4)${NC}"
    echo ""
    echo "所有关键参数已正确加载："
    echo "  - min_correlation: ag=0.70, au=0.75 ✓"
    echo "  - slippage_ticks: ag=2, au=1 ✓"
    echo ""
    echo "可以继续激活策略进行测试："
    echo "  curl -X POST http://localhost:9201/api/v1/strategies/live_ag_spread/activate"
    echo "  curl -X POST http://localhost:9201/api/v1/strategies/live_au_spread/activate"
else
    echo -e "${RED}✗ 参数加载验证失败 ($SUCCESS/$((SUCCESS+FAILED)))${NC}"
    echo ""
    echo "请检查："
    echo "  1. 是否使用了最新编译的 bin/trader"
    echo "  2. 是否使用了 config/trader.live.yaml"
    echo "  3. 是否重启了trader进程"
fi
echo ""
echo "实时监控："
echo "  ./monitor_live.sh                     # 综合监控"
echo "  tail -f log/trader.live.log | grep \"Stats:\"  # 策略统计"
echo "════════════════════════════════════════════════════════════════"
echo ""

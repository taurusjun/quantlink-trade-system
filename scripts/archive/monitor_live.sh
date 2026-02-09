#!/bin/bash
# 实盘交易实时监控脚本

echo "╔═══════════════════════════════════════════════════════╗"
echo "║          实盘交易系统监控 (Live Trading)               ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 1. 进程状态
echo "[1] 系统进程状态:"
ps aux | grep -E "bin/trader.*live|ctp_md|md_gateway|ors_gateway|counter" | grep -v grep | awk '{printf "  %-25s PID: %-8s CPU: %5s%%\n", $11, $2, $3}'
echo ""

# 2. API健康检查
echo "[2] API健康状态:"
curl -s http://localhost:9201/api/v1/health | jq -r '.data | "  Mode: \(.mode), Status: \(.status)"' 2>/dev/null || echo "  API不可用"
echo ""

# 3. 策略状态
echo "[3] 策略状态:"
curl -s http://localhost:9201/api/v1/strategies 2>/dev/null | jq -r '.data.strategies[] | "  \(.id): active=\(.active), conditions_met=\(.conditions_met)"' || echo "  无法获取策略状态"
echo ""

# 4. 最新市场数据
echo "[4] 最新市场数据 (最近5条):"
tail -50 log/trader.live.log | grep "Received market data" | tail -5 | sed 's/^/  /'
echo ""

# 5. 策略统计 (最新)
echo "[5] 策略统计 (最新):"
tail -50 log/trader.live.log | grep "Stats:" | tail -4 | sed 's/^/  /'
echo ""

# 6. 订单活动
echo "[6] 订单活动 (最近5条):"
ORDER_COUNT=$(grep -c "Order sent" log/trader.live.log 2>/dev/null || echo "0")
echo "  总订单数: $ORDER_COUNT"
if [ "$ORDER_COUNT" -gt 0 ]; then
    grep "Order sent" log/trader.live.log | tail -5 | sed 's/^/  /'
else
    echo "  暂无订单"
fi
echo ""

# 7. 价格验证（检查tick size）
echo "[7] 最新订单价格验证:"
if [ "$ORDER_COUNT" -gt 0 ]; then
    LAST_ORDER=$(grep "Order sent" log/trader.live.log | tail -1)
    echo "  $LAST_ORDER"
    echo "  ✓ 检查: 白银价格应为整数，黄金价格应为0.02的倍数"
else
    echo "  暂无订单可验证"
fi
echo ""

# 8. 持仓信息
echo "[8] 持仓信息:"
POS_RESULT=$(curl -s http://localhost:9201/api/v1/positions 2>/dev/null)
if echo "$POS_RESULT" | jq -e '.data | length > 0' >/dev/null 2>&1; then
    echo "$POS_RESULT" | jq -r '.data | to_entries[] | "  \(.key): \(.value | length) positions"'
else
    echo "  当前无持仓"
fi
echo ""

# 9. 风控状态
echo "[9] 风控状态:"
grep -i "RISK ALERT\|STOP" log/trader.live.log 2>/dev/null | tail -3 | sed 's/^/  /' || echo "  无风控告警"
echo ""

echo "════════════════════════════════════════════════════════"
echo "监控时间: $(date '+%Y-%m-%d %H:%M:%S')"
echo "实时日志: tail -f log/trader.live.log"
echo "Dashboard: http://localhost:9201"
echo "════════════════════════════════════════════════════════"

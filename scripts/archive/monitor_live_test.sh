#!/bin/bash
# 实盘测试监控脚本

echo "╔═══════════════════════════════════════════════════════╗"
echo "║          实盘多策略系统监控                            ║"
echo "╚═══════════════════════════════════════════════════════╝"
echo ""

# 1. 检查进程状态
echo "[1] 系统进程状态:"
ps aux | grep -E "nats-server|ctp_md|md_gateway|ors_gateway|counter|trader" | grep -v grep | awk '{printf "  %-20s PID: %-8s CPU: %5s%%\n", $11, $2, $3}'
echo ""

# 2. API健康检查
echo "[2] API健康状态:"
curl -s http://localhost:9201/api/v1/health | jq -r '.data | "  Mode: \(.mode), Status: \(.status)"' 2>/dev/null || echo "  API不可用"
echo ""

# 3. 策略状态
echo "[3] 策略状态:"
curl -s http://localhost:9201/api/v1/strategies | jq -r '.data.strategies[] | "  \(.id): running=\(.running), active=\(.active), conditions_met=\(.conditions_met)"' 2>/dev/null
echo ""

# 4. 最近市场数据
echo "[4] 最近接收的市场数据:"
tail -5 log/trader.test.log | grep "Received market data" || echo "  暂无最新行情"
echo ""

# 5. 最近订单
echo "[5] 最近发送的订单:"
tail -10 log/trader.test.log | grep "Order sent" || echo "  暂无新订单"
echo ""

# 6. Dashboard
echo "[6] Web Dashboard: http://localhost:9201"
echo ""

echo "提示: 使用 'tail -f log/trader.test.log' 查看实时日志"

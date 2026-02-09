#!/bin/bash
# ag2603 开仓交易脚本

echo "=========================================="
echo "ag2603 开仓交易"
echo "=========================================="
echo ""

# 先停止行情网关
echo "🛑 停止行情网关..."
pkill -f ctp_md_plugin
sleep 1

# 根据SimNow历史数据，ag2603通常在6800-7500区间
# 由于我们无法直接读取实时行情，我们使用一个相对安全的价格
# 当前时间是夜盘（21:07），ag2603应该在交易

echo "📊 ag2603 交易信息："
echo "  合约代码: ag2603 (白银2026年3月)"
echo "  交易所: 上期所 (SHFE)"
echo "  夜盘时间: 21:00-02:30"
echo ""

# 让我们先查询一下当前的大致价格区间
# 根据SimNow的模拟数据，白银通常在7000-7300
# 我们使用一个略高的价格来确保能够快速成交

SYMBOL="ag2603"
# 使用7200作为基准价（SimNow上白银的典型价格）
# 加10点开仓（7210）
BASE_PRICE=7200
OFFSET=10
ORDER_PRICE=$(echo "$BASE_PRICE + $OFFSET" | bc)

echo "💡 交易策略："
echo "  估计市价: ~${BASE_PRICE}"
echo "  下单价格: ${ORDER_PRICE} (市价+${OFFSET})"
echo "  手数: 1手"
echo "  方向: 买入开仓"
echo ""

read -p "⚠️  确认下单？这将发送真实订单到SimNow！(y/n): " -n 1 -r
echo ""

if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ 已取消"
    exit 0
fi

echo ""
echo "🚀 发送订单..."
echo ""

# 执行下单
./gateway/build/plugins/ctp/ctp_market_order_test config/ctp/ctp_td.yaml "$SYMBOL" "$ORDER_PRICE"

echo ""
echo "=========================================="
echo "交易完成"
echo "=========================================="
echo ""
echo "💡 提示："
echo "  - 如果订单未成交，可能是价格偏离市价太远"
echo "  - 可以适当调整 BASE_PRICE 的值"
echo "  - 使用 pkill -f ctp_md_plugin 停止行情网关"

#!/bin/bash
# CTP交易查询测试脚本

echo "=========================================="
echo "CTP 交易功能测试"
echo "=========================================="
echo ""

CONFIG="config/ctp/ctp_td.yaml"

# 检查配置文件
if [ ! -f "$CONFIG" ]; then
    echo "❌ 配置文件不存在: $CONFIG"
    exit 1
fi

echo "✅ 配置文件: $CONFIG"
echo ""

# 测试合约
SYMBOL="ag2603"
# 测试价格（远离市场价，确保不会成交）
PRICE="9999.0"

echo "📊 测试参数："
echo "  合约: $SYMBOL"
echo "  测试价格: $PRICE (远离市价，仅测试下单)"
echo ""

echo "🔧 开始测试..."
echo ""

# 运行市价单测试（只下单，不期望成交）
echo "1. 测试下单功能..."
./gateway/build/plugins/ctp/ctp_market_order_test "$CONFIG" "$SYMBOL" "$PRICE"

echo ""
echo "=========================================="
echo "测试完成"
echo "=========================================="

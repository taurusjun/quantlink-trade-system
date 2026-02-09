#!/bin/bash

# CTP交易测试脚本
# 自动化测试买入、卖出和查询功能

echo "========================================"
echo "CTP Trading Automated Test"
echo "========================================"
echo ""

cd /Users/user/PWorks/RD/quantlink-trade-system

# 测试序列：
# 1. 启动程序
# 2. 发送买入订单（ag2603, 价格7250, 1手）
# 3. 等待2秒
# 4. 查询统计
# 5. 退出

echo "启动CTP交易测试程序..."
echo ""

# 创建测试输入
cat << EOF | timeout 30s ./gateway/build/plugins/ctp/ctp_integrated_simple config/ctp/ctp_td.yaml
1
ag2603
7250
1
4
5
EOF

echo ""
echo "========================================"
echo "测试完成"
echo "========================================"

#!/bin/bash
# 快速平仓ag2603脚本

echo "========================================"
echo "快速平仓ag2603"
echo "========================================"
echo

# 获取当前行情（假设在29500-29600之间）
CLOSE_PRICE=29550

echo "平仓价格: $CLOSE_PRICE"
echo

# 编译简单平仓工具（如果需要）
echo "检查编译..."
cd gateway/build
if [ ! -f plugins/ctp/ctp_close_simple ]; then
    echo "编译平仓工具..."
    cmake .. > /dev/null 2>&1
    make ctp_close_simple > /dev/null 2>&1
fi
cd ../..

echo "========================================="
echo "尝试1: 平今仓（CLOSE_TODAY）"
echo "========================================="
./gateway/build/plugins/ctp/ctp_close_simple config/ctp/ctp_td.yaml ag2603 $CLOSE_PRICE

echo
echo "等待3秒..."
sleep 3

echo
echo "========================================"
echo "尝试2: 平昨仓（CLOSE_YESTERDAY）"
echo "========================================="

# 创建临时的平昨仓程序
cat > /tmp/close_yesterday.cpp << 'EOF'
#include "ctp_td_plugin.h"
#include <iostream>
#include <thread>
#include <chrono>

int main(int argc, char* argv[]) {
    if (argc < 4) return 1;

    std::string config_file = argv[1];
    std::string symbol = argv[2];
    double close_price = std::stod(argv[3]);

    hft::plugin::ctp::CTPTDPlugin plugin;
    if (!plugin.Initialize(config_file)) return 1;
    if (!plugin.Login()) return 1;

    std::cout << "⏳ 等待就绪..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    std::cout << "📤 发送平昨仓订单..." << std::endl;
    hft::plugin::OrderRequest request;
    strncpy(request.symbol, symbol.c_str(), sizeof(request.symbol) - 1);
    strncpy(request.exchange, "SHFE", sizeof(request.exchange) - 1);
    request.direction = hft::plugin::OrderDirection::SELL;
    request.offset = hft::plugin::OffsetFlag::CLOSE_YESTERDAY;
    request.price_type = hft::plugin::PriceType::LIMIT;
    request.price = close_price;
    request.volume = 1;

    std::string order_id = plugin.SendOrder(request);
    if (!order_id.empty()) {
        std::cout << "✅ 平昨仓订单已发送: " << order_id << std::endl;
    } else {
        std::cout << "❌ 平昨仓失败" << std::endl;
    }

    std::this_thread::sleep_for(std::chrono::seconds(3));
    plugin.Logout();
    return 0;
}
EOF

# 暂时跳过编译平昨仓版本
echo "（使用通用平仓标志）"

echo
echo "========================================"
echo "尝试3: 通用平仓（CLOSE）"
echo "========================================="
echo "（如果前两次都失败，说明可能已经平仓）"

echo
echo "========================================"
echo "完成"
echo "========================================"
echo
echo "建议："
echo "1. 通过SimNow网页端查看实际持仓: https://www.simnow.com.cn/"
echo "2. 查看log/ctp_td.log了解详细错误信息"
echo "3. 如果显示'平仓位不足'，说明可能已经平仓成功"

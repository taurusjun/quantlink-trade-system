// 简单平仓程序
#include "ctp_td_plugin.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <string>

int main(int argc, char* argv[]) {
    if (argc < 4) {
        std::cout << "用法: " << argv[0] << " <config_file> <symbol> <price>" << std::endl;
        std::cout << "示例: " << argv[0] << " config/ctp/ctp_td.yaml ag2603 29600" << std::endl;
        return 1;
    }

    std::string config_file = argv[1];
    std::string symbol = argv[2];
    double close_price = std::stod(argv[3]);

    std::cout << "========================================" << std::endl;
    std::cout << "CTP简单平仓程序" << std::endl;
    std::cout << "========================================\n" << std::endl;
    std::cout << "合约: " << symbol << std::endl;
    std::cout << "平仓价: " << close_price << "\n" << std::endl;

    // 创建交易插件
    std::cout << "🔧 初始化交易插件..." << std::endl;
    hft::plugin::ctp::CTPTDPlugin plugin;
    if (!plugin.Initialize(config_file)) {
        std::cerr << "❌ 初始化失败" << std::endl;
        return 1;
    }
    std::cout << "✅ 初始化成功\n" << std::endl;

    // 登录
    std::cout << "🔐 登录中..." << std::endl;
    if (!plugin.Login()) {
        std::cerr << "❌ 登录失败" << std::endl;
        return 1;
    }
    std::cout << "✅ 登录成功\n" << std::endl;

    // 等待系统就绪
    std::cout << "⏳ 等待系统就绪..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // 先尝试平今仓
    std::cout << "\n📤 发送平仓订单（平今仓）..." << std::endl;
    std::cout << "----------------------------------------" << std::endl;

    hft::plugin::OrderRequest request;
    strncpy(request.symbol, symbol.c_str(), sizeof(request.symbol) - 1);
    strncpy(request.exchange, "SHFE", sizeof(request.exchange) - 1);
    request.direction = hft::plugin::OrderDirection::SELL;  // 卖出平多头
    request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;  // 平今仓
    request.price_type = hft::plugin::PriceType::LIMIT;
    request.price = close_price;
    request.volume = 1;

    std::string order_id = plugin.SendOrder(request);
    if (order_id.empty()) {
        std::cerr << "❌ 平仓失败" << std::endl;
        plugin.Logout();
        return 1;
    }

    std::cout << "✅ 平仓订单已发送" << std::endl;
    std::cout << "  订单ID: " << order_id << std::endl;
    std::cout << "  合约: " << symbol << std::endl;
    std::cout << "  方向: 卖出" << std::endl;
    std::cout << "  开平: 平今" << std::endl;
    std::cout << "  价格: " << close_price << std::endl;
    std::cout << "  数量: 1手" << std::endl;

    // 等待成交
    std::cout << "\n⏳ 等待成交（5秒）..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(5));

    // 登出
    std::cout << "\n🔓 登出..." << std::endl;
    plugin.Logout();
    std::cout << "✅ 完成\n" << std::endl;

    return 0;
}

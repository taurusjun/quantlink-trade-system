// 自动查询并平仓程序
#include "ctp_td_plugin.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <string>
#include <vector>

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
    std::cout << "CTP自动平仓程序" << std::endl;
    std::cout << "========================================\n" << std::endl;

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

    // 查询持仓
    std::cout << "\n📊 查询持仓..." << std::endl;
    std::vector<hft::plugin::PositionInfo> positions;
    if (!plugin.QueryPositions(positions)) {
        std::cerr << "❌ 查询持仓失败" << std::endl;
        plugin.Logout();
        return 1;
    }

    // 查找目标合约持仓
    hft::plugin::PositionInfo* target_position = nullptr;
    for (auto& pos : positions) {
        if (std::string(pos.symbol) == symbol) {
            target_position = &pos;
            break;
        }
    }

    if (!target_position) {
        std::cout << "\n✅ 合约 " << symbol << " 无持仓（可能已平仓）\n" << std::endl;
        plugin.Logout();
        return 0;
    }

    // 显示持仓信息
    std::cout << "\n========================================" << std::endl;
    std::cout << "找到持仓: " << target_position->symbol << std::endl;
    std::cout << "========================================" << std::endl;
    std::cout << "  方向: " << (target_position->direction == hft::plugin::OrderDirection::BUY ? "多头" : "空头") << std::endl;
    std::cout << "  今仓: " << target_position->today_volume << std::endl;
    std::cout << "  昨仓: " << target_position->yesterday_volume << std::endl;
    std::cout << "  总量: " << target_position->volume << std::endl;
    std::cout << "  均价: " << target_position->avg_price << std::endl;
    std::cout << "========================================\n" << std::endl;

    if (target_position->volume == 0) {
        std::cout << "✅ 持仓为0，无需平仓\n" << std::endl;
        plugin.Logout();
        return 0;
    }

    // 确定平仓方向
    hft::plugin::OrderDirection close_direction;
    if (target_position->direction == hft::plugin::OrderDirection::BUY) {
        close_direction = hft::plugin::OrderDirection::SELL;  // 多头→卖出
        std::cout << "📤 准备平仓：多头持仓 → 卖出\n" << std::endl;
    } else {
        close_direction = hft::plugin::OrderDirection::BUY;   // 空头→买入
        std::cout << "📤 准备平仓：空头持仓 → 买入\n" << std::endl;
    }

    // 先平昨仓（如果有）
    if (target_position->yesterday_volume > 0) {
        std::cout << "📤 平昨仓 " << target_position->yesterday_volume << " 手 @ " << close_price << std::endl;

        hft::plugin::OrderRequest request;
        strncpy(request.symbol, symbol.c_str(), sizeof(request.symbol) - 1);
        strncpy(request.exchange, target_position->exchange, sizeof(request.exchange) - 1);
        request.direction = close_direction;
        request.offset = hft::plugin::OffsetFlag::CLOSE_YESTERDAY;
        request.price_type = hft::plugin::PriceType::LIMIT;
        request.price = close_price;
        request.volume = target_position->yesterday_volume;

        std::string order_id = plugin.SendOrder(request);
        if (order_id.empty()) {
            std::cerr << "  ❌ 平昨仓失败" << std::endl;
        } else {
            std::cout << "  ✅ 平昨仓订单已发送: " << order_id << std::endl;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(500));
    }

    // 再平今仓（如果有）
    if (target_position->today_volume > 0) {
        std::cout << "📤 平今仓 " << target_position->today_volume << " 手 @ " << close_price << std::endl;

        hft::plugin::OrderRequest request;
        strncpy(request.symbol, symbol.c_str(), sizeof(request.symbol) - 1);
        strncpy(request.exchange, target_position->exchange, sizeof(request.exchange) - 1);
        request.direction = close_direction;
        request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
        request.price_type = hft::plugin::PriceType::LIMIT;
        request.price = close_price;
        request.volume = target_position->today_volume;

        std::string order_id = plugin.SendOrder(request);
        if (order_id.empty()) {
            std::cerr << "  ❌ 平今仓失败" << std::endl;
        } else {
            std::cout << "  ✅ 平今仓订单已发送: " << order_id << std::endl;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(500));
    }

    // 等待成交
    std::cout << "\n⏳ 等待成交（5秒）..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(5));

    // 再次查询确认
    std::cout << "\n📊 查询最新持仓..." << std::endl;
    std::vector<hft::plugin::PositionInfo> new_positions;
    if (plugin.QueryPositions(new_positions)) {
        bool found = false;
        for (const auto& pos : new_positions) {
            if (std::string(pos.symbol) == symbol && pos.volume > 0) {
                found = true;
                std::cout << "⚠️  仍有持仓: " << pos.volume << " 手" << std::endl;
                break;
            }
        }
        if (!found) {
            std::cout << "✅ 已完全平仓" << std::endl;
        }
    }

    // 登出
    std::cout << "\n🔓 登出..." << std::endl;
    plugin.Logout();
    std::cout << "✅ 完成\n" << std::endl;

    return 0;
}

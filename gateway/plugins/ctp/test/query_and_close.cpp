// CTP持仓查询和平仓程序
#include "ctp_td_plugin.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <iomanip>

void PrintPositions(const std::vector<hft::plugin::PositionInfo>& positions) {
    if (positions.empty()) {
        std::cout << "\n✅ 当前无持仓\n" << std::endl;
        return;
    }

    std::cout << "\n========================================" << std::endl;
    std::cout << "当前持仓列表 (" << positions.size() << ")" << std::endl;
    std::cout << "========================================" << std::endl;

    for (const auto& pos : positions) {
        std::cout << "\n合约: " << pos.symbol << std::endl;
        std::cout << "  方向: " << (pos.direction == hft::plugin::OrderDirection::BUY ? "多头" : "空头") << std::endl;
        std::cout << "  今仓: " << pos.today_volume << std::endl;
        std::cout << "  昨仓: " << pos.yesterday_volume << std::endl;
        std::cout << "  总持仓: " << pos.volume << std::endl;
        std::cout << "  持仓均价: " << std::fixed << std::setprecision(2) << pos.avg_price << std::endl;
        std::cout << "  浮动盈亏: " << pos.position_profit << std::endl;
        std::cout << "  保证金: " << pos.margin << std::endl;
    }
    std::cout << "========================================\n" << std::endl;
}

int main(int argc, char* argv[]) {
    std::cout << "========================================" << std::endl;
    std::cout << "CTP持仓查询和平仓工具" << std::endl;
    std::cout << "========================================\n" << std::endl;

    if (argc < 2) {
        std::cout << "用法: " << argv[0] << " <config_file> [symbol] [close]" << std::endl;
        std::cout << "\n示例:" << std::endl;
        std::cout << "  查询所有持仓: " << argv[0] << " config/ctp/ctp_td.yaml" << std::endl;
        std::cout << "  查询指定合约: " << argv[0] << " config/ctp/ctp_td.yaml ag2603" << std::endl;
        std::cout << "  平仓指定合约: " << argv[0] << " config/ctp/ctp_td.yaml ag2603 close" << std::endl;
        return 1;
    }

    std::string config_file = argv[1];
    std::string target_symbol = (argc >= 3) ? argv[2] : "";
    bool do_close = (argc >= 4) && (std::string(argv[3]) == "close");

    std::cout << "配置文件: " << config_file << std::endl;
    if (!target_symbol.empty()) {
        std::cout << "目标合约: " << target_symbol << std::endl;
        if (do_close) {
            std::cout << "操作: 平仓" << std::endl;
        }
    }
    std::cout << std::endl;

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
    std::cout << "📊 查询持仓..." << std::endl;
    std::vector<hft::plugin::PositionInfo> positions;
    if (!plugin.QueryPositions(positions)) {
        std::cerr << "❌ 查询持仓失败" << std::endl;
        plugin.Logout();
        return 1;
    }

    // 过滤目标合约
    std::vector<hft::plugin::PositionInfo> target_positions;
    if (!target_symbol.empty()) {
        for (const auto& pos : positions) {
            if (std::string(pos.symbol) == target_symbol) {
                target_positions.push_back(pos);
            }
        }
        PrintPositions(target_positions);
    } else {
        PrintPositions(positions);
        target_positions = positions;
    }

    // 平仓操作
    if (do_close && !target_positions.empty()) {
        std::cout << "\n⚠️  开始平仓操作..." << std::endl;
        std::cout << "========================================\n" << std::endl;

        // 先查询最新行情获取价格
        std::this_thread::sleep_for(std::chrono::seconds(1));

        for (const auto& pos : target_positions) {
            uint32_t total_volume = pos.volume;
            if (total_volume == 0) {
                std::cout << "跳过 " << pos.symbol << "（无持仓）\n" << std::endl;
                continue;
            }

            std::cout << "平仓 " << pos.symbol << ":" << std::endl;
            std::cout << "  方向: " << (pos.direction == hft::plugin::OrderDirection::BUY ? "多头→卖出" : "空头→买入") << std::endl;
            std::cout << "  数量: " << total_volume << std::endl;

            // 设置平仓价格（对手价）
            // 多头平仓：卖出，使用持仓价-50（快速成交）
            // 空头平仓：买入，使用持仓价+50（快速成交）
            double close_price;
            hft::plugin::OrderDirection close_direction;
            hft::plugin::OffsetFlag offset_flag;

            if (pos.direction == hft::plugin::OrderDirection::BUY) {
                // 多头平仓：卖出
                close_direction = hft::plugin::OrderDirection::SELL;
                close_price = pos.avg_price - 50; // 使用持仓均价-50作为平仓价（快速成交）
            } else {
                // 空头平仓：买入
                close_direction = hft::plugin::OrderDirection::BUY;
                close_price = pos.avg_price + 50; // 使用持仓均价+50作为平仓价（快速成交）
            }

            std::cout << "  平仓价: " << std::fixed << std::setprecision(2) << close_price << std::endl;

            // 先平昨仓
            if (pos.yesterday_volume > 0) {
                std::cout << "\n  [1/2] 平昨仓 " << pos.yesterday_volume << " 手..." << std::endl;

                hft::plugin::OrderRequest request;
                strncpy(request.symbol, pos.symbol, sizeof(request.symbol) - 1);
                strncpy(request.exchange, pos.exchange, sizeof(request.exchange) - 1);
                request.direction = close_direction;
                request.offset = hft::plugin::OffsetFlag::CLOSE_YESTERDAY;
                request.price_type = hft::plugin::PriceType::LIMIT;
                request.price = close_price;
                request.volume = pos.yesterday_volume;

                std::string order_id = plugin.SendOrder(request);
                if (order_id.empty()) {
                    std::cerr << "  ❌ 平昨仓失败" << std::endl;
                } else {
                    std::cout << "  ✅ 平昨仓订单已发送: " << order_id << std::endl;
                }
                std::this_thread::sleep_for(std::chrono::milliseconds(500));
            }

            // 再平今仓
            if (pos.today_volume > 0) {
                std::cout << "\n  [2/2] 平今仓 " << pos.today_volume << " 手..." << std::endl;

                hft::plugin::OrderRequest request;
                strncpy(request.symbol, pos.symbol, sizeof(request.symbol) - 1);
                strncpy(request.exchange, pos.exchange, sizeof(request.exchange) - 1);
                request.direction = close_direction;
                request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
                request.price_type = hft::plugin::PriceType::LIMIT;
                request.price = close_price;
                request.volume = pos.today_volume;

                std::string order_id = plugin.SendOrder(request);
                if (order_id.empty()) {
                    std::cerr << "  ❌ 平今仓失败" << std::endl;
                } else {
                    std::cout << "  ✅ 平今仓订单已发送: " << order_id << std::endl;
                }
                std::this_thread::sleep_for(std::chrono::milliseconds(500));
            }

            std::cout << std::endl;
        }

        // 等待成交
        std::cout << "⏳ 等待成交确认（5秒）..." << std::endl;
        std::this_thread::sleep_for(std::chrono::seconds(5));

        // 再次查询持仓确认
        std::cout << "\n📊 查询最新持仓..." << std::endl;
        std::vector<hft::plugin::PositionInfo> new_positions;
        if (plugin.QueryPositions(new_positions)) {
            std::vector<hft::plugin::PositionInfo> new_target_positions;
            if (!target_symbol.empty()) {
                for (const auto& pos : new_positions) {
                    if (std::string(pos.symbol) == target_symbol) {
                        new_target_positions.push_back(pos);
                    }
                }
                PrintPositions(new_target_positions);
            } else {
                PrintPositions(new_positions);
            }
        }
    }

    // 登出
    std::cout << "🔓 登出..." << std::endl;
    plugin.Logout();
    std::cout << "✅ 完成\n" << std::endl;

    return 0;
}

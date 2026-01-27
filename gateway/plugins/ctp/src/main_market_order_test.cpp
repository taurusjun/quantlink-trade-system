/**
 * @file main_market_order_test.cpp
 * @brief CTP市价单测试（自动化）
 *
 * 功能：
 * 1. 登录CTP交易系统
 * 2. 自动发送一个接近市价的买入订单
 * 3. 监控订单和成交回报
 * 4. 如果成交，自动平仓
 */

#include "ctp_td_plugin.h"
#include <iostream>
#include <csignal>
#include <thread>
#include <chrono>
#include <atomic>
#include <iomanip>

using namespace hft::plugin::ctp;
using namespace hft::plugin;

// 全局运行标志
std::atomic<bool> g_running{true};
std::atomic<int> g_order_count{0};
std::atomic<int> g_trade_count{0};
std::string g_last_order_id;

// 信号处理函数
void signal_handler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << ", shutting down..." << std::endl;
    g_running.store(false);
}

// 订单回报回调
void on_order_callback(const OrderInfo& order) {
    const char* status_str[] = {
        "UNKNOWN", "PENDING", "ACCEPTED", "PARTIALLY_FILLED",
        "FILLED", "CANCELING", "PARTIALLY_CANCELED", "CANCELED", "REJECTED"
    };

    int status_idx = static_cast<int>(order.status);
    const char* status_name = (status_idx >= 0 && status_idx < 9) ? status_str[status_idx] : "INVALID";

    std::cout << "\n[OrderCallback] "
              << "ID=" << order.order_id
              << " Symbol=" << order.symbol
              << " " << (order.direction == OrderDirection::BUY ? "BUY" : "SELL")
              << " Status=" << status_name
              << " Vol=" << order.volume
              << " Traded=" << order.traded_volume
              << " Price=" << std::fixed << std::setprecision(2) << order.price
              << std::endl;

    g_order_count++;
}

// 成交回报回调
void on_trade_callback(const TradeInfo& trade) {
    std::cout << "\n🎉🎉🎉 *** 成交通知 *** 🎉🎉🎉"
              << "\n  TradeID: " << trade.trade_id
              << "\n  OrderID: " << trade.order_id
              << "\n  Symbol: " << trade.symbol
              << "\n  Direction: " << (trade.direction == OrderDirection::BUY ? "BUY" : "SELL")
              << "\n  Price: " << std::fixed << std::setprecision(2) << trade.price
              << "\n  Volume: " << trade.volume
              << "\n  Time: " << trade.trade_time
              << "\n********************************\n" << std::endl;

    g_trade_count++;
}

// 错误回调
void on_error_callback(int error_id, const std::string& error_msg) {
    std::cerr << "[ErrorCallback] ErrorID=" << error_id
              << " Message=" << error_msg << std::endl;
}

// 打印分隔线
void print_section(const std::string& title) {
    std::cout << "\n========================================" << std::endl;
    std::cout << title << std::endl;
    std::cout << "========================================\n" << std::endl;
}

int main(int argc, char* argv[]) {
    print_section("CTP Market Order Test - Automated");

    // 检查命令行参数
    if (argc < 4) {
        std::cerr << "Usage: " << argv[0] << " <td_config> <symbol> <price>" << std::endl;
        std::cerr << "Example: " << argv[0] << " config/ctp/ctp_td.yaml ag2603 7250" << std::endl;
        std::cerr << "\n说明：" << std::endl;
        std::cerr << "  symbol: 合约代码（如 ag2603）" << std::endl;
        std::cerr << "  price: 委托价格（建议使用当前卖一价或更高）" << std::endl;
        return 1;
    }

    std::string td_config_file = argv[1];
    std::string symbol = argv[2];
    double price = std::stod(argv[3]);

    // 注册信号处理
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

    // ==================== 初始化交易插件 ====================
    print_section("Step 1: Initialize Trading");

    CTPTDPlugin td_plugin;
    std::cout << "[Main] Initializing with config: " << td_config_file << std::endl;
    if (!td_plugin.Initialize(td_config_file)) {
        std::cerr << "[Main] ❌ Failed to initialize" << std::endl;
        return 1;
    }
    std::cout << "[Main] ✅ Initialized" << std::endl;

    // 注册回调
    td_plugin.RegisterOrderCallback(on_order_callback);
    td_plugin.RegisterTradeCallback(on_trade_callback);
    td_plugin.RegisterErrorCallback(on_error_callback);

    // 登录
    std::cout << "[Main] Logging in..." << std::endl;
    if (!td_plugin.Login()) {
        std::cerr << "[Main] ❌ Login failed" << std::endl;
        return 1;
    }
    std::cout << "[Main] ✅ Logged in" << std::endl;

    // 等待就绪
    std::cout << "[Main] Waiting for system ready (5 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(5));

    // ==================== 发送测试订单 ====================
    print_section("Step 2: Send Test Order");

    std::cout << "⚠️ 发送买入订单：" << std::endl;
    std::cout << "  合约: " << symbol << std::endl;
    std::cout << "  价格: " << std::fixed << std::setprecision(2) << price << std::endl;
    std::cout << "  手数: 1" << std::endl;
    std::cout << "  ⚠️ 此订单可能会成交！\n" << std::endl;

    OrderRequest order_req;
    strncpy(order_req.symbol, symbol.c_str(), sizeof(order_req.symbol) - 1);
    strncpy(order_req.exchange, "SHFE", sizeof(order_req.exchange) - 1);
    order_req.direction = OrderDirection::BUY;
    order_req.offset = OffsetFlag::OPEN;
    order_req.price_type = PriceType::LIMIT;
    order_req.price = price;
    order_req.volume = 1;
    strncpy(order_req.client_order_id, "AUTO_TEST_BUY", sizeof(order_req.client_order_id) - 1);

    g_last_order_id = td_plugin.SendOrder(order_req);
    if (g_last_order_id.empty()) {
        std::cerr << "❌ Failed to send order" << std::endl;
        td_plugin.Logout();
        return 1;
    }

    std::cout << "✅ Order sent: " << g_last_order_id << std::endl;

    // 等待订单响应
    std::cout << "\n[Main] Waiting for order response (5 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(5));

    // ==================== 检查是否成交 ====================
    print_section("Step 3: Check Execution");

    if (g_trade_count.load() > 0) {
        std::cout << "🎉 订单已成交！准备平仓..." << std::endl;

        // 等待一下再平仓
        std::this_thread::sleep_for(std::chrono::seconds(2));

        // 发送平仓订单（使用稍低的价格快速成交）
        OrderRequest close_order;
        strncpy(close_order.symbol, symbol.c_str(), sizeof(close_order.symbol) - 1);
        strncpy(close_order.exchange, "SHFE", sizeof(close_order.exchange) - 1);
        close_order.direction = OrderDirection::SELL;
        close_order.offset = OffsetFlag::CLOSE_TODAY;
        close_order.price_type = PriceType::LIMIT;
        close_order.price = price - 5.0;  // 使用稍低价格快速平仓
        close_order.volume = 1;
        strncpy(close_order.client_order_id, "AUTO_TEST_CLOSE", sizeof(close_order.client_order_id) - 1);

        std::cout << "\n发送平仓订单（价格: " << (price - 5.0) << "）..." << std::endl;
        std::string close_id = td_plugin.SendOrder(close_order);

        if (!close_id.empty()) {
            std::cout << "✅ Close order sent: " << close_id << std::endl;

            // 等待平仓响应
            std::cout << "Waiting for close order response (5 seconds)..." << std::endl;
            std::this_thread::sleep_for(std::chrono::seconds(5));

            if (g_trade_count.load() >= 2) {
                std::cout << "\n🎉 平仓成功！所有测试完成。" << std::endl;
            } else {
                std::cout << "\n⚠️ 平仓订单尚未成交，可能在排队中。" << std::endl;
            }
        } else {
            std::cerr << "❌ Failed to send close order" << std::endl;
        }

    } else {
        std::cout << "订单未成交。可能原因：" << std::endl;
        std::cout << "  1. 价格未达到市场价" << std::endl;
        std::cout << "  2. 当前不在交易时段" << std::endl;
        std::cout << "  3. 合约不存在或已过期" << std::endl;
    }

    // ==================== 最终统计 ====================
    print_section("Final Statistics");

    std::cout << std::left << std::setw(30) << "Total Orders:" << td_plugin.GetOrderCount() << std::endl;
    std::cout << std::setw(30) << "Total Trades:" << td_plugin.GetTradeCount() << std::endl;
    std::cout << std::setw(30) << "Order Callbacks:" << g_order_count.load() << std::endl;
    std::cout << std::setw(30) << "Trade Callbacks:" << g_trade_count.load() << std::endl;

    if (g_trade_count.load() >= 2) {
        std::cout << "\n✅ 测试成功：完成开仓和平仓" << std::endl;
    } else if (g_trade_count.load() == 1) {
        std::cout << "\n⚠️ 部分成功：完成开仓，平仓可能仍在进行" << std::endl;
    } else {
        std::cout << "\n⚠️ 订单未成交，建议调整价格后重试" << std::endl;
    }

    // ==================== 清理 ====================
    std::cout << "\n[Main] Logging out..." << std::endl;
    td_plugin.Logout();
    std::this_thread::sleep_for(std::chrono::seconds(1));

    std::cout << "[Main] Test completed" << std::endl;

    return 0;
}

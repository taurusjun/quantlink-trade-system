/**
 * @file main_ctp_td.cpp
 * @brief CTP交易插件完整测试程序
 *
 * 提供全面的交易功能测试，包括多合约、多订单类型、批量测试等
 */

#include "ctp_td_plugin.h"
#include <iostream>
#include <csignal>
#include <thread>
#include <chrono>
#include <atomic>
#include <vector>
#include <iomanip>

using namespace hft::plugin::ctp;
using namespace hft::plugin;

// 全局运行标志
std::atomic<bool> g_running{true};
std::atomic<int> g_order_count{0};
std::atomic<int> g_trade_count{0};

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

    std::cout << "[OrderCallback] "
              << "ID=" << order.order_id
              << " Symbol=" << order.symbol
              << " " << (order.direction == OrderDirection::BUY ? "BUY" : "SELL")
              << " Status=" << status_name
              << " Vol=" << order.volume
              << " Traded=" << order.traded_volume
              << " Price=" << order.price
              << std::endl;

    g_order_count++;
}

// 成交回报回调
void on_trade_callback(const TradeInfo& trade) {
    std::cout << "\n🎉 [TradeCallback] *** 成交通知 ***"
              << "\n  TradeID: " << trade.trade_id
              << "\n  OrderID: " << trade.order_id
              << "\n  Symbol: " << trade.symbol
              << "\n  Direction: " << (trade.direction == OrderDirection::BUY ? "BUY" : "SELL")
              << "\n  Price: " << trade.price
              << "\n  Volume: " << trade.volume
              << "\n  Time: " << trade.trade_time
              << "\n*********************\n" << std::endl;

    g_trade_count++;
}

// 错误回调
void on_error_callback(int error_id, const std::string& error_msg) {
    std::cerr << "[ErrorCallback] ErrorID=" << error_id
              << " Message=" << error_msg << std::endl;
}

// 测试辅助函数：发送订单
std::string send_test_order(CTPTDPlugin& plugin,
                            const char* symbol,
                            const char* exchange,
                            OrderDirection direction,
                            double price,
                            int volume,
                            PriceType price_type = PriceType::LIMIT,
                            const char* test_id = nullptr) {
    OrderRequest order_req;
    strncpy(order_req.symbol, symbol, sizeof(order_req.symbol) - 1);
    strncpy(order_req.exchange, exchange, sizeof(order_req.exchange) - 1);
    order_req.direction = direction;
    order_req.offset = OffsetFlag::OPEN;
    order_req.price_type = price_type;
    order_req.price = price;
    order_req.volume = volume;

    if (test_id) {
        strncpy(order_req.client_order_id, test_id, sizeof(order_req.client_order_id) - 1);
    }

    std::string order_id = plugin.SendOrder(order_req);
    if (!order_id.empty()) {
        std::cout << "  ✅ Order sent: " << order_id
                  << " [" << symbol << " " << (direction == OrderDirection::BUY ? "BUY" : "SELL")
                  << " " << volume << "@" << price << "]" << std::endl;
    } else {
        std::cout << "  ❌ Failed to send order" << std::endl;
    }

    return order_id;
}

// 打印分隔线
void print_section(const std::string& title) {
    std::cout << "\n========================================" << std::endl;
    std::cout << title << std::endl;
    std::cout << "========================================\n" << std::endl;
}

int main(int argc, char* argv[]) {
    print_section("CTP Trading Plugin - Comprehensive Test");

    // 检查命令行参数
    if (argc < 2) {
        std::cerr << "Usage: " << argv[0] << " <config_file>" << std::endl;
        std::cerr << "Example: " << argv[0] << " config/ctp/ctp_td.yaml" << std::endl;
        return 1;
    }

    std::string config_file = argv[1];

    // 注册信号处理
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

    // 创建插件实例
    CTPTDPlugin plugin;

    // 初始化插件
    std::cout << "[Main] Initializing plugin with config: " << config_file << std::endl;
    if (!plugin.Initialize(config_file)) {
        std::cerr << "[Main] ❌ Failed to initialize plugin" << std::endl;
        return 1;
    }
    std::cout << "[Main] ✅ Plugin initialized successfully\n" << std::endl;

    // 注册回调
    plugin.RegisterOrderCallback(on_order_callback);
    plugin.RegisterTradeCallback(on_trade_callback);
    plugin.RegisterErrorCallback(on_error_callback);
    std::cout << "[Main] ✅ Callbacks registered\n" << std::endl;

    // 登录
    std::cout << "[Main] Logging in..." << std::endl;
    if (!plugin.Login()) {
        std::cerr << "[Main] ❌ Failed to login" << std::endl;
        return 1;
    }
    std::cout << "[Main] ✅ Logged in successfully\n" << std::endl;

    // 等待系统就绪
    std::cout << "[Main] Waiting for system ready (5 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(5));

    // ==================== 测试1: 限价单测试（不会成交） ====================
    print_section("Test 1: Limit Orders (Far from Market Price)");

    std::vector<std::string> order_ids;

    // 白银 - 买入（价格过低）
    order_ids.push_back(send_test_order(plugin, "ag2603", "SHFE", OrderDirection::BUY, 6000.0, 1, PriceType::LIMIT, "TEST_AG_BUY"));
    std::this_thread::sleep_for(std::chrono::milliseconds(500));

    // 白银 - 卖出（价格过高）
    order_ids.push_back(send_test_order(plugin, "ag2603", "SHFE", OrderDirection::SELL, 9000.0, 1, PriceType::LIMIT, "TEST_AG_SELL"));
    std::this_thread::sleep_for(std::chrono::milliseconds(500));

    // 螺纹钢 - 买入
    order_ids.push_back(send_test_order(plugin, "rb2505", "SHFE", OrderDirection::BUY, 3000.0, 1, PriceType::LIMIT, "TEST_RB_BUY"));
    std::this_thread::sleep_for(std::chrono::milliseconds(500));

    std::cout << "\n[Main] Waiting 3 seconds for order responses..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // ==================== 测试2: 尝试接近市价的订单 ====================
    print_section("Test 2: Near-Market Price Orders (May Execute)");

    std::cout << "⚠️ 警告：以下订单可能会成交！" << std::endl;
    std::cout << "使用接近市价的价格进行测试...\n" << std::endl;

    // 白银 - 买入（假设市价约7200，使用7300尝试成交）
    order_ids.push_back(send_test_order(plugin, "ag2603", "SHFE", OrderDirection::BUY, 7300.0, 1, PriceType::LIMIT, "TEST_AG_MARKET_BUY"));
    std::this_thread::sleep_for(std::chrono::seconds(2));

    // 如果成交了，立即平仓
    if (g_trade_count.load() > 0) {
        std::cout << "\n[Main] 检测到成交，准备平仓..." << std::endl;
        std::this_thread::sleep_for(std::chrono::seconds(1));

        // 卖出平仓（使用更低价格快速成交）
        OrderRequest close_order;
        strncpy(close_order.symbol, "ag2603", sizeof(close_order.symbol) - 1);
        strncpy(close_order.exchange, "SHFE", sizeof(close_order.exchange) - 1);
        close_order.direction = OrderDirection::SELL;
        close_order.offset = OffsetFlag::CLOSE_TODAY;  // 平今仓
        close_order.price_type = PriceType::LIMIT;
        close_order.price = 7100.0;  // 使用更低价格快速平仓
        close_order.volume = 1;
        strncpy(close_order.client_order_id, "TEST_AG_CLOSE", sizeof(close_order.client_order_id) - 1);

        std::string close_order_id = plugin.SendOrder(close_order);
        if (!close_order_id.empty()) {
            std::cout << "  ✅ Close order sent: " << close_order_id << std::endl;
        }

        std::this_thread::sleep_for(std::chrono::seconds(2));
    }

    // ==================== 测试3: 撤单测试 ====================
    print_section("Test 3: Order Cancellation");

    // 发送一个不会成交的订单，然后撤销
    std::string cancel_test_order = send_test_order(plugin, "cu2603", "SHFE", OrderDirection::BUY, 70000.0, 1, PriceType::LIMIT, "TEST_CANCEL");
    std::this_thread::sleep_for(std::chrono::seconds(1));

    if (!cancel_test_order.empty()) {
        std::cout << "\n[Main] Attempting to cancel order: " << cancel_test_order << std::endl;
        if (plugin.CancelOrder(cancel_test_order)) {
            std::cout << "  ✅ Cancel request sent" << std::endl;
        } else {
            std::cout << "  ⚠️ Cancel request failed (order may already be in final state)" << std::endl;
        }
        std::this_thread::sleep_for(std::chrono::seconds(2));
    }

    // ==================== 测试4: 批量订单测试 ====================
    print_section("Test 4: Batch Order Test (Stress Test)");

    std::cout << "发送5个批量订单（价格远离市场，不会成交）..." << std::endl;

    for (int i = 0; i < 5; i++) {
        char test_id[32];
        snprintf(test_id, sizeof(test_id), "BATCH_%d", i + 1);

        order_ids.push_back(send_test_order(
            plugin,
            "ag2603",
            "SHFE",
            (i % 2 == 0) ? OrderDirection::BUY : OrderDirection::SELL,
            (i % 2 == 0) ? 6000.0 : 9000.0,
            1,
            PriceType::LIMIT,
            test_id
        ));

        std::this_thread::sleep_for(std::chrono::milliseconds(300));
    }

    std::cout << "\n[Main] Waiting 3 seconds for all order responses..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // ==================== 测试5: 查询订单状态 ====================
    print_section("Test 5: Query Order Status");

    int valid_orders = 0;
    for (const auto& order_id : order_ids) {
        if (order_id.empty()) continue;

        OrderInfo order_info;
        if (plugin.GetOrder(order_id, order_info)) {
            valid_orders++;
            std::cout << "  Order " << order_id
                      << ": " << order_info.symbol
                      << " Status=" << static_cast<int>(order_info.status)
                      << " Traded=" << order_info.traded_volume << "/" << order_info.volume
                      << std::endl;
        }
    }
    std::cout << "\n[Main] Successfully queried " << valid_orders << " orders" << std::endl;

    // ==================== 测试总结 ====================
    print_section("Test Summary");

    std::cout << std::left << std::setw(30) << "Total Orders Sent:" << order_ids.size() << std::endl;
    std::cout << std::setw(30) << "Order Callbacks Received:" << g_order_count.load() << std::endl;
    std::cout << std::setw(30) << "Trade Callbacks Received:" << g_trade_count.load() << std::endl;
    std::cout << std::setw(30) << "Plugin Order Count:" << plugin.GetOrderCount() << std::endl;
    std::cout << std::setw(30) << "Plugin Trade Count:" << plugin.GetTradeCount() << std::endl;
    std::cout << std::setw(30) << "Connection Status:" << (plugin.IsConnected() ? "Connected" : "Disconnected") << std::endl;

    // ==================== 保持运行 ====================
    std::cout << "\n[Main] Test completed. Press Ctrl+C to exit..." << std::endl;
    std::cout << "[Main] Monitoring for any additional callbacks...\n" << std::endl;

    // 运行30秒以监听任何延迟的回调
    auto start_time = std::chrono::steady_clock::now();
    while (g_running.load()) {
        auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::steady_clock::now() - start_time
        ).count();

        if (elapsed >= 30) {
            std::cout << "\n[Main] Auto-exiting after 30 seconds monitoring period" << std::endl;
            break;
        }

        std::this_thread::sleep_for(std::chrono::seconds(1));

        // 检查连接状态
        if (!plugin.IsConnected()) {
            std::cerr << "\n[Main] ⚠️ Disconnected from server" << std::endl;
            break;
        }
    }

    // ==================== 清理 ====================
    std::cout << "\n[Main] Shutting down..." << std::endl;
    plugin.Logout();
    std::this_thread::sleep_for(std::chrono::seconds(1));

    print_section("Final Statistics");
    std::cout << "Total Orders: " << plugin.GetOrderCount() << std::endl;
    std::cout << "Total Trades: " << plugin.GetTradeCount() << std::endl;
    std::cout << "\n[Main] Plugin terminated successfully" << std::endl;

    return 0;
}

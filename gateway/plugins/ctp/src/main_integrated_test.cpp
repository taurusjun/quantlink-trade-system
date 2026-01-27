/**
 * @file main_integrated_test.cpp
 * @brief CTP行情+交易综合测试程序
 *
 * 功能：
 * 1. 启动CTP行情插件，订阅合约行情
 * 2. 启动CTP交易插件，准备交易
 * 3. 根据实时行情价格，发送测试订单
 * 4. 尝试实际成交和平仓
 */

#include "ctp_md_plugin.h"
#include "ctp_td_plugin.h"
#include "shm_queue.h"
#include <iostream>
#include <csignal>
#include <thread>
#include <chrono>
#include <atomic>
#include <iomanip>
#include <map>
#include <mutex>

using namespace hft::plugin::ctp;
using namespace hft::plugin;
using namespace hft::shm;

// 全局运行标志
std::atomic<bool> g_running{true};
std::atomic<int> g_order_count{0};
std::atomic<int> g_trade_count{0};

// 行情数据缓存
struct LatestMarketData {
    std::string symbol;
    double last_price = 0.0;
    double bid_price = 0.0;
    double ask_price = 0.0;
    int volume = 0;
    std::string update_time;
    bool valid = false;
};

std::map<std::string, LatestMarketData> g_market_data;
std::mutex g_md_mutex;

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

// 行情数据处理线程
void market_data_thread(CTPMDPlugin* md_plugin) {
    std::cout << "[MD Thread] Market data processing thread started" << std::endl;

    // 打开共享内存队列读取行情
    ShmManager shm_manager;
    if (!shm_manager.Init("md_queue", 1024, sizeof(MarketDataRaw))) {
        std::cerr << "[MD Thread] Failed to open shared memory queue" << std::endl;
        return;
    }

    auto queue = shm_manager.GetQueue("md_queue");
    if (!queue) {
        std::cerr << "[MD Thread] Failed to get queue" << std::endl;
        return;
    }

    int consecutive_empty = 0;
    while (g_running.load()) {
        MarketDataRaw md_raw;
        if (queue->TryPop(&md_raw)) {
            consecutive_empty = 0;

            // 更新行情缓存
            std::lock_guard<std::mutex> lock(g_md_mutex);
            LatestMarketData& md = g_market_data[md_raw.symbol];

            md.symbol = md_raw.symbol;
            md.last_price = md_raw.last_price;
            md.bid_price = md_raw.bid_price[0];  // 第一档买价
            md.ask_price = md_raw.ask_price[0];  // 第一档卖价
            md.volume = static_cast<int>(md_raw.total_volume);

            // 将纳秒时间戳转换为时间字符串
            char time_buf[32];
            time_t seconds = md_raw.timestamp / 1000000000;
            struct tm* tm_info = localtime(&seconds);
            strftime(time_buf, sizeof(time_buf), "%H:%M:%S", tm_info);
            md.update_time = time_buf;

            md.valid = true;

            // 只打印第一次收到的行情和每10秒更新
            static std::map<std::string, int> tick_count;
            if (++tick_count[md.symbol] % 100 == 1) {
                std::cout << "[MD] " << md.symbol
                          << " Last=" << std::fixed << std::setprecision(2) << md.last_price
                          << " Bid=" << md.bid_price
                          << " Ask=" << md.ask_price
                          << " Vol=" << md.volume
                          << " Time=" << md.update_time
                          << std::endl;
            }
        } else {
            consecutive_empty++;
            if (consecutive_empty > 10) {
                std::this_thread::sleep_for(std::chrono::milliseconds(100));
            } else {
                std::this_thread::yield();
            }
        }
    }

    std::cout << "[MD Thread] Market data processing thread stopped" << std::endl;
}

// 获取最新行情
bool get_latest_price(const std::string& symbol, double& last_price, double& bid, double& ask) {
    std::lock_guard<std::mutex> lock(g_md_mutex);
    auto it = g_market_data.find(symbol);
    if (it != g_market_data.end() && it->second.valid) {
        last_price = it->second.last_price;
        bid = it->second.bid_price;
        ask = it->second.ask_price;
        return true;
    }
    return false;
}

// 打印分隔线
void print_section(const std::string& title) {
    std::cout << "\n========================================" << std::endl;
    std::cout << title << std::endl;
    std::cout << "========================================\n" << std::endl;
}

int main(int argc, char* argv[]) {
    print_section("CTP Integrated Test - Market Data + Trading");

    // 检查命令行参数
    if (argc < 3) {
        std::cerr << "Usage: " << argv[0] << " <md_config> <td_config>" << std::endl;
        std::cerr << "Example: " << argv[0] << " config/ctp/ctp_md.yaml config/ctp/ctp_td.yaml" << std::endl;
        return 1;
    }

    std::string md_config_file = argv[1];
    std::string td_config_file = argv[2];

    // 注册信号处理
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

    // ==================== 初始化行情插件 ====================
    print_section("Step 1: Initialize Market Data Plugin");

    CTPMDPlugin md_plugin;
    std::cout << "[Main] Initializing MD plugin with config: " << md_config_file << std::endl;
    if (!md_plugin.Initialize(md_config_file)) {
        std::cerr << "[Main] ❌ Failed to initialize MD plugin" << std::endl;
        return 1;
    }
    std::cout << "[Main] ✅ MD plugin initialized" << std::endl;

    // 启动行情插件
    std::cout << "[Main] Starting MD plugin..." << std::endl;
    if (!md_plugin.Start()) {
        std::cerr << "[Main] ❌ Failed to start MD plugin" << std::endl;
        return 1;
    }
    std::cout << "[Main] ✅ MD plugin started" << std::endl;

    // 等待行情插件登录
    std::cout << "[Main] Waiting for MD plugin login..." << std::endl;
    for (int i = 0; i < 20; i++) {
        if (md_plugin.IsLoggedIn()) {
            break;
        }
        std::this_thread::sleep_for(std::chrono::milliseconds(500));
    }

    if (!md_plugin.IsLoggedIn()) {
        std::cerr << "[Main] ❌ MD plugin login timeout" << std::endl;
        return 1;
    }
    std::cout << "[Main] ✅ MD plugin logged in" << std::endl;

    // 订阅合约行情
    std::vector<std::string> symbols = {"ag2603", "ag2604", "ag2606"};
    std::cout << "[Main] Subscribing to: ";
    for (const auto& sym : symbols) {
        std::cout << sym << " ";
    }
    std::cout << std::endl;

    if (!md_plugin.Subscribe(symbols)) {
        std::cerr << "[Main] ⚠️ Failed to subscribe" << std::endl;
    }

    // 启动行情数据处理线程
    std::thread md_thread(market_data_thread, &md_plugin);

    // 等待接收行情数据
    std::cout << "[Main] Waiting for market data (10 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(10));

    // ==================== 初始化交易插件 ====================
    print_section("Step 2: Initialize Trading Plugin");

    CTPTDPlugin td_plugin;
    std::cout << "[Main] Initializing TD plugin with config: " << td_config_file << std::endl;
    if (!td_plugin.Initialize(td_config_file)) {
        std::cerr << "[Main] ❌ Failed to initialize TD plugin" << std::endl;
        g_running = false;
        md_thread.join();
        md_plugin.Stop();
        return 1;
    }
    std::cout << "[Main] ✅ TD plugin initialized" << std::endl;

    // 注册交易回调
    td_plugin.RegisterOrderCallback(on_order_callback);
    td_plugin.RegisterTradeCallback(on_trade_callback);
    td_plugin.RegisterErrorCallback(on_error_callback);

    // 登录
    std::cout << "[Main] Logging in to trading..." << std::endl;
    if (!td_plugin.Login()) {
        std::cerr << "[Main] ❌ Failed to login" << std::endl;
        g_running = false;
        md_thread.join();
        md_plugin.Stop();
        return 1;
    }
    std::cout << "[Main] ✅ Trading logged in successfully" << std::endl;

    // 等待系统就绪
    std::cout << "[Main] Waiting for trading system ready (3 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // ==================== 基于实时行情的交易测试 ====================
    print_section("Step 3: Market Data Based Trading Test");

    // 显示当前行情
    std::cout << "Current Market Data:\n" << std::endl;
    {
        std::lock_guard<std::mutex> lock(g_md_mutex);
        for (const auto& pair : g_market_data) {
            const auto& md = pair.second;
            std::cout << "  " << md.symbol
                      << ": Last=" << std::fixed << std::setprecision(2) << md.last_price
                      << " Bid=" << md.bid_price
                      << " Ask=" << md.ask_price
                      << " (Time: " << md.update_time << ")"
                      << std::endl;
        }
    }

    // 测试1: 发送对价单（可能成交）
    std::string test_symbol = "ag2603";
    double last_price, bid, ask;

    if (get_latest_price(test_symbol, last_price, bid, ask)) {
        std::cout << "\n[Test 1] Attempting market-making order on " << test_symbol << std::endl;
        std::cout << "Current: Last=" << last_price << " Bid=" << bid << " Ask=" << ask << std::endl;

        // 发送买单（使用卖一价，期望成交）
        if (ask > 0) {
            OrderRequest order_req;
            strncpy(order_req.symbol, test_symbol.c_str(), sizeof(order_req.symbol) - 1);
            strncpy(order_req.exchange, "SHFE", sizeof(order_req.exchange) - 1);
            order_req.direction = OrderDirection::BUY;
            order_req.offset = OffsetFlag::OPEN;
            order_req.price_type = PriceType::LIMIT;
            order_req.price = ask;  // 使用卖一价买入，期望成交
            order_req.volume = 1;
            strncpy(order_req.client_order_id, "MD_TEST_BUY", sizeof(order_req.client_order_id) - 1);

            std::cout << "\n⚠️ Sending BUY order at ASK price: " << ask << " (may execute!)" << std::endl;
            std::string order_id = td_plugin.SendOrder(order_req);

            if (!order_id.empty()) {
                std::cout << "✅ Order sent: " << order_id << std::endl;

                // 等待订单响应
                std::cout << "Waiting 5 seconds for order execution..." << std::endl;
                std::this_thread::sleep_for(std::chrono::seconds(5));

                // 如果成交了，立即平仓
                if (g_trade_count.load() > 0) {
                    std::cout << "\n🎉 Trade executed! Attempting to close position..." << std::endl;

                    // 查询当前价格
                    if (get_latest_price(test_symbol, last_price, bid, ask)) {
                        OrderRequest close_order;
                        strncpy(close_order.symbol, test_symbol.c_str(), sizeof(close_order.symbol) - 1);
                        strncpy(close_order.exchange, "SHFE", sizeof(close_order.exchange) - 1);
                        close_order.direction = OrderDirection::SELL;
                        close_order.offset = OffsetFlag::CLOSE_TODAY;
                        close_order.price_type = PriceType::LIMIT;
                        close_order.price = bid;  // 使用买一价卖出
                        close_order.volume = 1;
                        strncpy(close_order.client_order_id, "MD_TEST_CLOSE", sizeof(close_order.client_order_id) - 1);

                        std::cout << "Sending CLOSE order at BID price: " << bid << std::endl;
                        std::string close_id = td_plugin.SendOrder(close_order);
                        if (!close_id.empty()) {
                            std::cout << "✅ Close order sent: " << close_id << std::endl;
                        }

                        std::this_thread::sleep_for(std::chrono::seconds(3));
                    }
                } else {
                    std::cout << "No execution. Order may be pending or canceled." << std::endl;
                }
            }
        } else {
            std::cout << "❌ Invalid market data (ask price = 0)" << std::endl;
        }
    } else {
        std::cout << "❌ No market data available for " << test_symbol << std::endl;
    }

    // ==================== 测试总结 ====================
    print_section("Test Summary");

    std::cout << std::left << std::setw(30) << "MD Messages Received:" << md_plugin.GetMessageCount() << std::endl;
    std::cout << std::setw(30) << "MD Messages Dropped:" << md_plugin.GetDroppedCount() << std::endl;
    std::cout << std::setw(30) << "Total Orders Sent:" << td_plugin.GetOrderCount() << std::endl;
    std::cout << std::setw(30) << "Total Trades:" << td_plugin.GetTradeCount() << std::endl;
    std::cout << std::setw(30) << "Order Callbacks:" << g_order_count.load() << std::endl;
    std::cout << std::setw(30) << "Trade Callbacks:" << g_trade_count.load() << std::endl;

    // ==================== 保持运行30秒 ====================
    std::cout << "\n[Main] Monitoring for 30 seconds..." << std::endl;
    auto start_time = std::chrono::steady_clock::now();
    while (g_running.load()) {
        auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::steady_clock::now() - start_time
        ).count();

        if (elapsed >= 30) {
            std::cout << "\n[Main] Auto-exiting after 30 seconds" << std::endl;
            break;
        }

        std::this_thread::sleep_for(std::chrono::seconds(1));
    }

    // ==================== 清理 ====================
    print_section("Cleanup");

    std::cout << "[Main] Stopping trading plugin..." << std::endl;
    td_plugin.Logout();

    std::cout << "[Main] Stopping market data plugin..." << std::endl;
    g_running = false;
    md_thread.join();
    md_plugin.Stop();

    std::cout << "\n[Main] All plugins terminated successfully" << std::endl;

    return 0;
}

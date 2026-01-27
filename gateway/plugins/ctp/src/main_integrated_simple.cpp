/**
 * @file main_integrated_simple.cpp
 * @brief CTP行情+交易简化综合测试
 *
 * 策略：
 * 1. 订阅白银期货行情
 * 2. 等待接收行情数据（通过日志输出观察）
 * 3. 手动输入当前市价
 * 4. 基于市价发送测试订单
 */

#include "ctp_td_plugin.h"
#include <iostream>
#include <csignal>
#include <thread>
#include <chrono>
#include <atomic>
#include <iomanip>
#include <sstream>

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
    std::cout << "\n🎉🎉🎉 [TradeCallback] *** 成交通知 ***"
              << "\n  TradeID: " << trade.trade_id
              << "\n  OrderID: " << trade.order_id
              << "\n  Symbol: " << trade.symbol
              << "\n  Direction: " << (trade.direction == OrderDirection::BUY ? "BUY" : "SELL")
              << "\n  Price: " << std::fixed << std::setprecision(2) << trade.price
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

// 打印分隔线
void print_section(const std::string& title) {
    std::cout << "\n========================================" << std::endl;
    std::cout << title << std::endl;
    std::cout << "========================================\n" << std::endl;
}

int main(int argc, char* argv[]) {
    print_section("CTP Market-Based Trading Test");

    // 检查命令行参数
    if (argc < 2) {
        std::cerr << "Usage: " << argv[0] << " <td_config>" << std::endl;
        std::cerr << "Example: " << argv[0] << " config/ctp/ctp_td.yaml" << std::endl;
        return 1;
    }

    std::string td_config_file = argv[1];

    // 注册信号处理
    signal(SIGINT, signal_handler);
    signal(SIGTERM, signal_handler);

    // ==================== 初始化交易插件 ====================
    print_section("Step 1: Initialize Trading Plugin");

    CTPTDPlugin td_plugin;
    std::cout << "[Main] Initializing TD plugin with config: " << td_config_file << std::endl;
    if (!td_plugin.Initialize(td_config_file)) {
        std::cerr << "[Main] ❌ Failed to initialize TD plugin" << std::endl;
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
        return 1;
    }
    std::cout << "[Main] ✅ Trading logged in successfully" << std::endl;

    // 等待系统就绪
    std::cout << "[Main] Waiting for trading system ready (3 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // ==================== 交互式交易测试 ====================
    print_section("Step 2: Market Data Query (External)");

    std::cout << "请在SimNow网站或其他行情软件查询当前行情：" << std::endl;
    std::cout << "https://www.simnow.com.cn/" << std::endl;
    std::cout << "\n推荐合约: ag2603 (白银2026年3月)" << std::endl;
    std::cout << "\n当前约14:00-15:00为下午交易时段" << std::endl;
    std::cout << "或者使用CTP行情插件查看实时行情\n" << std::endl;

    print_section("Step 3: Interactive Trading");

    while (g_running.load()) {
        std::cout << "\n========================================" << std::endl;
        std::cout << "交易选项 (输入数字):" << std::endl;
        std::cout << "1. 买入开仓 (对价单，可能成交)" << std::endl;
        std::cout << "2. 卖出开仓 (对价单，可能成交)" << std::endl;
        std::cout << "3. 平今仓位" << std::endl;
        std::cout << "4. 查询统计" << std::endl;
        std::cout << "5. 退出程序" << std::endl;
        std::cout << "========================================" << std::endl;
        std::cout << "请选择: ";

        int choice;
        std::cin >> choice;

        if (std::cin.fail()) {
            std::cin.clear();
            std::cin.ignore(10000, '\n');
            std::cout << "无效输入，请输入数字" << std::endl;
            continue;
        }

        if (choice == 5) {
            std::cout << "退出程序..." << std::endl;
            break;
        }

        if (choice == 4) {
            print_section("Trading Statistics");
            std::cout << std::left << std::setw(30) << "Total Orders:" << td_plugin.GetOrderCount() << std::endl;
            std::cout << std::setw(30) << "Total Trades:" << td_plugin.GetTradeCount() << std::endl;
            std::cout << std::setw(30) << "Order Callbacks:" << g_order_count.load() << std::endl;
            std::cout << std::setw(30) << "Trade Callbacks:" << g_trade_count.load() << std::endl;
            std::cout << std::setw(30) << "Connection Status:" << (td_plugin.IsConnected() ? "Connected" : "Disconnected") << std::endl;
            continue;
        }

        if (choice < 1 || choice > 3) {
            std::cout << "无效选项" << std::endl;
            continue;
        }

        // 获取交易参数
        std::string symbol;
        double price;
        int volume;

        std::cout << "\n合约代码 (如 ag2603): ";
        std::cin >> symbol;

        if (choice <= 2) {
            std::cout << "委托价格: ";
            std::cin >> price;
        } else {
            price = 0;  // 平仓时需要输入价格
            std::cout << "平仓价格: ";
            std::cin >> price;
        }

        std::cout << "手数: ";
        std::cin >> volume;

        // 发送订单
        OrderRequest order_req;
        strncpy(order_req.symbol, symbol.c_str(), sizeof(order_req.symbol) - 1);
        strncpy(order_req.exchange, "SHFE", sizeof(order_req.exchange) - 1);
        order_req.price_type = PriceType::LIMIT;
        order_req.price = price;
        order_req.volume = volume;

        switch (choice) {
            case 1:  // 买入开仓
                order_req.direction = OrderDirection::BUY;
                order_req.offset = OffsetFlag::OPEN;
                strncpy(order_req.client_order_id, "MANUAL_BUY_OPEN", sizeof(order_req.client_order_id) - 1);
                break;
            case 2:  // 卖出开仓
                order_req.direction = OrderDirection::SELL;
                order_req.offset = OffsetFlag::OPEN;
                strncpy(order_req.client_order_id, "MANUAL_SELL_OPEN", sizeof(order_req.client_order_id) - 1);
                break;
            case 3:  // 平今仓
                // 需要用户指定方向
                std::cout << "平仓方向 (1=多头平仓/卖出, 2=空头平仓/买入): ";
                int close_direction;
                std::cin >> close_direction;
                order_req.direction = (close_direction == 1) ? OrderDirection::SELL : OrderDirection::BUY;
                order_req.offset = OffsetFlag::CLOSE_TODAY;
                strncpy(order_req.client_order_id, "MANUAL_CLOSE", sizeof(order_req.client_order_id) - 1);
                break;
        }

        std::cout << "\n⚠️ 发送订单: "
                  << symbol << " "
                  << (order_req.direction == OrderDirection::BUY ? "BUY" : "SELL") << " "
                  << volume << "@" << std::fixed << std::setprecision(2) << price
                  << " (可能成交！)" << std::endl;

        std::string order_id = td_plugin.SendOrder(order_req);
        if (!order_id.empty()) {
            std::cout << "✅ 订单已发送: " << order_id << std::endl;
            std::cout << "等待订单响应..." << std::endl;
            std::this_thread::sleep_for(std::chrono::seconds(2));
        } else {
            std::cout << "❌ 订单发送失败" << std::endl;
        }
    }

    // ==================== 清理 ====================
    print_section("Cleanup");

    std::cout << "[Main] Stopping trading plugin..." << std::endl;
    td_plugin.Logout();
    std::this_thread::sleep_for(std::chrono::seconds(1));

    print_section("Final Statistics");
    std::cout << "Total Orders: " << td_plugin.GetOrderCount() << std::endl;
    std::cout << "Total Trades: " << td_plugin.GetTradeCount() << std::endl;
    std::cout << "\n[Main] Program terminated successfully" << std::endl;

    return 0;
}

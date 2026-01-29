#include "simulated_counter.h"
#include <iostream>
#include <sstream>
#include <iomanip>
#include <chrono>
#include <cstring>

namespace hft {
namespace counter {

SimulatedCounter::SimulatedCounter()
    : m_callback(nullptr)
    , m_connected(false)
    , m_running(false)
    , m_order_counter(0)
{
    // 初始化随机数生成器
    m_rng.seed(std::chrono::system_clock::now().time_since_epoch().count());
}

SimulatedCounter::~SimulatedCounter() {
    Disconnect();
}

bool SimulatedCounter::Connect() {
    if (m_connected.load()) {
        return true;
    }

    std::cout << "[SimCounter] Connecting to simulated exchange..." << std::endl;

    // 模拟连接延迟
    std::this_thread::sleep_for(std::chrono::milliseconds(100));

    m_connected = true;
    m_running = true;

    // 启动处理线程
    m_process_thread = std::thread(&SimulatedCounter::ProcessOrdersThread, this);

    std::cout << "[SimCounter] Connected successfully" << std::endl;
    return true;
}

void SimulatedCounter::Disconnect() {
    if (!m_connected.load()) {
        return;
    }

    std::cout << "[SimCounter] Disconnecting..." << std::endl;

    m_running = false;
    m_connected = false;

    // 等待处理线程结束
    if (m_process_thread.joinable()) {
        m_process_thread.join();
    }

    std::cout << "[SimCounter] Disconnected" << std::endl;
}

int SimulatedCounter::SendOrder(const hft::ors::OrderRequestRaw& req,
                                std::string& order_id) {
    if (!m_connected.load()) {
        return -1;
    }

    // 生成交易所订单ID
    order_id = GenerateExchangeOrderID();

    // 创建订单
    Order order;
    order.order_id = order_id;
    order.request = req;
    order.submit_time = std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();
    order.accepted = false;
    order.filled = false;
    order.canceled = false;

    // 加入队列
    {
        std::lock_guard<std::mutex> lock(m_orders_mutex);
        m_pending_orders.push(order);
    }

    std::cout << "[SimCounter] Order received: " << order_id
              << " symbol=" << req.symbol
              << " side=" << (req.side == 1 ? "BUY" : "SELL")
              << " price=" << req.price
              << " qty=" << req.quantity
              << std::endl;

    return 0;
}

int SimulatedCounter::CancelOrder(const std::string& order_id) {
    if (!m_connected.load()) {
        return -1;
    }

    std::cout << "[SimCounter] Cancel order: " << order_id << std::endl;

    // 简单模拟：延迟后回调撤单确认
    // Note: In production, we'd need to look up strategy_id from order store
    if (m_callback) {
        std::thread([this, order_id]() {
            std::this_thread::sleep_for(std::chrono::milliseconds(20));
            m_callback->OnOrderCanceled("", order_id);  // Strategy ID not available in cancel
        }).detach();
    }

    return 0;
}

int SimulatedCounter::QueryPosition(const std::string& symbol,
                                   hft::ors::OrderResponseRaw* position) {
    // 模拟查询：返回空仓位
    if (position) {
        std::memset(position, 0, sizeof(hft::ors::OrderResponseRaw));
        std::strncpy(position->order_id, "POSITION_QUERY", sizeof(position->order_id) - 1);
    }
    return 0;
}

void SimulatedCounter::ProcessOrdersThread() {
    std::cout << "[SimCounter] Process thread started" << std::endl;

    while (m_running.load()) {
        Order order;
        bool has_order = false;

        // 从队列取出订单
        {
            std::lock_guard<std::mutex> lock(m_orders_mutex);
            if (!m_pending_orders.empty()) {
                order = m_pending_orders.front();
                m_pending_orders.pop();
                has_order = true;
            }
        }

        if (has_order) {
            // 随机决定是否拒绝
            std::uniform_real_distribution<double> dist(0.0, 1.0);
            double rand_val = dist(m_rng);

            if (rand_val < m_config.reject_probability) {
                // 拒绝订单
                SimulateReject(order);
            } else {
                // 接受订单
                SimulateAccept(order);

                // 模拟成交
                if (m_config.immediate_fill || dist(m_rng) < m_config.fill_probability) {
                    SimulateFill(order);
                }
            }
        } else {
            // 队列空，短暂休眠
            std::this_thread::sleep_for(std::chrono::milliseconds(1));
        }
    }

    std::cout << "[SimCounter] Process thread stopped" << std::endl;
}

std::string SimulatedCounter::GenerateExchangeOrderID() {
    uint64_t counter = m_order_counter.fetch_add(1);
    auto now = std::chrono::system_clock::now();
    auto timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()).count();

    std::ostringstream oss;
    oss << "EX_" << timestamp << "_" << std::setfill('0') << std::setw(6) << counter;
    return oss.str();
}

void SimulatedCounter::SimulateAccept(const Order& order) {
    // 模拟接受延迟
    std::this_thread::sleep_for(std::chrono::milliseconds(m_config.accept_delay_ms));

    if (m_callback) {
        std::cout << "[SimCounter] Order accepted: " << order.order_id << std::endl;
        std::string strategy_id(order.request.strategy_id);
        m_callback->OnOrderAccept(strategy_id, order.order_id, order.order_id + "_EX");
    }
}

void SimulatedCounter::SimulateFill(const Order& order) {
    // 模拟成交延迟
    std::this_thread::sleep_for(std::chrono::milliseconds(m_config.fill_delay_ms));

    if (m_callback) {
        // 生成成交ID
        std::string exec_id = "EXEC_" + order.order_id;

        // 模拟成交（全部成交）
        std::cout << "[SimCounter] Order filled: " << order.order_id
                  << " price=" << order.request.price
                  << " qty=" << order.request.quantity
                  << std::endl;

        std::string strategy_id(order.request.strategy_id);
        m_callback->OnOrderFilled(
            strategy_id,
            order.order_id,
            exec_id,
            order.request.price,
            order.request.quantity,
            order.request.quantity  // 全部成交
        );
    }
}

void SimulatedCounter::SimulateReject(const Order& order) {
    // 模拟拒绝延迟
    std::this_thread::sleep_for(std::chrono::milliseconds(m_config.accept_delay_ms));

    if (m_callback) {
        std::cout << "[SimCounter] Order rejected: " << order.order_id << std::endl;
        std::string strategy_id(order.request.strategy_id);
        m_callback->OnOrderReject(strategy_id, order.order_id, 99, "Simulated rejection");
    }
}

} // namespace counter
} // namespace hft

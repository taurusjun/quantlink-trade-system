#pragma once

#include "counter_api.h"
#include <thread>
#include <atomic>
#include <random>
#include <queue>
#include <mutex>

namespace hft {
namespace counter {

// ============================================================================
// 模拟 Counter（用于测试）
// ============================================================================

class SimulatedCounter : public ICounterAPI {
public:
    SimulatedCounter();
    ~SimulatedCounter() override;

    // ICounterAPI 实现
    bool Connect() override;
    void Disconnect() override;
    bool IsConnected() const override { return m_connected.load(); }

    int SendOrder(const hft::ors::OrderRequestRaw& req,
                  std::string& order_id) override;

    int CancelOrder(const std::string& order_id) override;

    int QueryPosition(const std::string& symbol,
                     hft::ors::OrderResponseRaw* position) override;

    void SetCallback(ICounterCallback* callback) override {
        m_callback = callback;
    }

    std::string GetCounterType() const override { return "SIMULATED"; }

    // 配置参数
    struct Config {
        uint32_t accept_delay_ms = 10;      // 接受延迟（毫秒）
        uint32_t fill_delay_ms = 50;        // 成交延迟（毫秒）
        double fill_probability = 0.9;      // 成交概率
        double reject_probability = 0.05;   // 拒绝概率
        bool immediate_fill = false;        // 是否立即成交
    };

    void SetConfig(const Config& config) { m_config = config; }

private:
    // 订单结构
    struct Order {
        std::string order_id;
        hft::ors::OrderRequestRaw request;
        uint64_t submit_time;
        bool accepted;
        bool filled;
        bool canceled;
    };

    // 处理线程
    void ProcessOrdersThread();

    // 生成交易所订单ID
    std::string GenerateExchangeOrderID();

    // 模拟订单接受
    void SimulateAccept(const Order& order);

    // 模拟订单成交
    void SimulateFill(const Order& order);

    // 模拟订单拒绝
    void SimulateReject(const Order& order);

    // 成员变量
    ICounterCallback* m_callback;
    Config m_config;

    // 状态
    std::atomic<bool> m_connected;
    std::atomic<bool> m_running;

    // 订单队列
    std::queue<Order> m_pending_orders;
    std::mutex m_orders_mutex;

    // 处理线程
    std::thread m_process_thread;

    // 随机数生成器
    std::mt19937 m_rng;
    std::atomic<uint64_t> m_order_counter;
};

} // namespace counter
} // namespace hft

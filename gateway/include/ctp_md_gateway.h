#pragma once

#include <atomic>
#include <memory>
#include <string>
#include <unordered_set>
#include <mutex>
#include <chrono>
#include "ThostFtdcMdApi.h"
#include "ctp_config.h"
#include "shm_queue.h"

namespace hft {
namespace gateway {

// CTP行情网关实现（回调处理）
class CTPMDGatewayImpl : public CThostFtdcMdSpi {
public:
    explicit CTPMDGatewayImpl(const CTPMDConfig& config);
    virtual ~CTPMDGatewayImpl();

    // 启动和停止
    void Start();
    void Stop();
    bool IsRunning() const { return m_running.load(); }

    // CTP API回调接口
    void OnFrontConnected() override;
    void OnFrontDisconnected(int nReason) override;

    void OnRspUserLogin(CThostFtdcRspUserLoginField* pRspUserLogin,
                        CThostFtdcRspInfoField* pRspInfo,
                        int nRequestID, bool bIsLast) override;

    void OnRspError(CThostFtdcRspInfoField* pRspInfo,
                   int nRequestID, bool bIsLast) override;

    void OnRspSubMarketData(CThostFtdcSpecificInstrumentField* pSpecificInstrument,
                           CThostFtdcRspInfoField* pRspInfo,
                           int nRequestID, bool bIsLast) override;

    void OnRtnDepthMarketData(CThostFtdcDepthMarketDataField* pDepthMarketData) override;

private:
    // 内部方法
    void Login();
    void SubscribeInstruments();
    void Reconnect();

    // 数据转换
    void ConvertMarketData(CThostFtdcDepthMarketDataField* ctp_md,
                          hft::shm::MarketDataRaw* raw_md);

    // 延迟监控
    void UpdateLatencyStats(uint64_t latency_ns);
    void PrintLatencyStats();

    // 检查是否是错误响应
    bool IsErrorResponse(CThostFtdcRspInfoField* pRspInfo);

    // 配置
    CTPMDConfig m_config;

    // CTP API
    CThostFtdcMdApi* m_api = nullptr;

    // 共享内存队列
    hft::shm::ShmManager::Queue* m_queue = nullptr;

    // 状态管理
    std::atomic<bool> m_running{false};
    std::atomic<bool> m_connected{false};
    std::atomic<bool> m_logged_in{false};

    // 请求ID（线程安全）
    std::atomic<int> m_request_id{0};

    // 订阅的合约
    std::unordered_set<std::string> m_subscribed_instruments;
    mutable std::mutex m_subscription_mutex;

    // 重连相关
    int m_reconnect_count = 0;
    std::chrono::steady_clock::time_point m_last_reconnect_time;

    // 统计
    std::atomic<uint64_t> m_md_count{0};
    std::atomic<uint64_t> m_md_dropped{0};  // 队列满导致的丢弃数

    // 延迟统计
    std::atomic<uint64_t> m_total_latency_ns{0};
    std::atomic<uint64_t> m_min_latency_ns{UINT64_MAX};
    std::atomic<uint64_t> m_max_latency_ns{0};
    std::chrono::steady_clock::time_point m_last_stats_time;
};

// CTP行情网关主类
class CTPMDGateway {
public:
    explicit CTPMDGateway(const CTPMDConfig& config);
    ~CTPMDGateway();

    // 运行网关（阻塞）
    void Run();

    // 停止网关
    void Shutdown();

private:
    CTPMDConfig m_config;
    std::unique_ptr<CTPMDGatewayImpl> m_impl;
    std::atomic<bool> m_running{false};
};

} // namespace gateway
} // namespace hft

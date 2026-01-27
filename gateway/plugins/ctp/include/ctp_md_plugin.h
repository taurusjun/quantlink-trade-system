#pragma once

#include "plugin/md_plugin_interface.h"
#include "ThostFtdcMdApi.h"
#include "ctp_config.h"
#include "shm_queue.h"
#include <atomic>
#include <memory>
#include <string>
#include <vector>
#include <unordered_set>
#include <mutex>
#include <chrono>

namespace hft {
namespace plugin {
namespace ctp {

/**
 * CTP行情插件实现
 * 实现IMDPlugin接口，对接CTP行情API
 */
class CTPMDPlugin : public IMDPlugin, public CThostFtdcMdSpi {
public:
    CTPMDPlugin();
    virtual ~CTPMDPlugin();

    // ==================== IMDPlugin接口实现 ====================

    bool Initialize(const std::string& config_file) override;
    bool Start() override;
    void Stop() override;
    bool IsRunning() const override { return m_running.load(); }

    bool Subscribe(const std::vector<std::string>& symbols) override;
    bool Unsubscribe(const std::vector<std::string>& symbols) override;

    bool IsConnected() const override { return m_connected.load(); }
    bool IsLoggedIn() const override { return m_logged_in.load(); }
    std::string GetPluginName() const override { return "CTP"; }
    std::string GetPluginVersion() const override { return "1.0.0"; }

    uint64_t GetMessageCount() const override { return m_md_count.load(); }
    uint64_t GetDroppedCount() const override { return m_md_dropped.load(); }

    // ==================== CTP API回调接口 ====================

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

    // 配置（使用gateway命名空间中的CTPMDConfig）
    hft::gateway::CTPMDConfig m_config;

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

} // namespace ctp
} // namespace plugin
} // namespace hft

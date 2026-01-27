#include "ctp_md_plugin.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <cstring>
#include <ctime>
#include <iomanip>
#include <sstream>

namespace hft {
namespace plugin {
namespace ctp {

// ==================== CTPMDPlugin ====================

CTPMDPlugin::CTPMDPlugin()
    : m_last_stats_time(std::chrono::steady_clock::now())
{
    std::cout << "[CTPMDPlugin] Constructor called" << std::endl;
}

CTPMDPlugin::~CTPMDPlugin() {
    Stop();

    if (m_api) {
        m_api->Release();
        m_api = nullptr;
    }

    if (m_queue) {
        hft::shm::ShmManager::Close(m_queue);
        m_queue = nullptr;
    }
}

bool CTPMDPlugin::Initialize(const std::string& config_file) {
    std::cout << "[CTPMDPlugin] Initializing with config: " << config_file << std::endl;

    try {
        // 加载配置（LoadFromYaml会自动加载secret文件）
        if (!m_config.LoadFromYaml(config_file)) {
            std::cerr << "[CTPMDPlugin] ❌ Failed to load config file: " << config_file << std::endl;
            return false;
        }

        // 验证配置
        std::string error;
        if (!m_config.Validate(&error)) {
            std::cerr << "[CTPMDPlugin] ❌ Invalid config: " << error << std::endl;
            return false;
        }

        // 打印配置
        m_config.Print();

        // 创建CTP API实例（流文件保存在./ctp_flow/目录）
        m_api = CThostFtdcMdApi::CreateFtdcMdApi("./ctp_flow/");
        if (!m_api) {
            std::cerr << "[CTPMDPlugin] ❌ Failed to create CTP MD API" << std::endl;
            return false;
        }

        // 注册回调
        m_api->RegisterSpi(this);

        // 打开或创建共享内存队列
        m_queue = hft::shm::ShmManager::CreateOrOpen(m_config.shm_queue_name);
        if (!m_queue) {
            std::cerr << "[CTPMDPlugin] ❌ Failed to open shared memory: "
                      << m_config.shm_queue_name << std::endl;
            m_api->Release();
            m_api = nullptr;
            return false;
        }

        std::cout << "[CTPMDPlugin] ✅ Shared memory queue opened: "
                  << m_config.shm_queue_name << std::endl;
        std::cout << "[CTPMDPlugin] ✅ Initialized successfully" << std::endl;
        return true;

    } catch (const std::exception& e) {
        std::cerr << "[CTPMDPlugin] ❌ Exception during initialization: "
                  << e.what() << std::endl;
        return false;
    }
}

bool CTPMDPlugin::Start() {
    if (m_running.load()) {
        std::cout << "[CTPMDPlugin] Already running" << std::endl;
        return true;
    }

    if (!m_api) {
        std::cerr << "[CTPMDPlugin] ❌ Cannot start: not initialized" << std::endl;
        return false;
    }

    m_running.store(true);
    std::cout << "[CTPMDPlugin] Starting..." << std::endl;
    std::cout << "[CTPMDPlugin] Connecting to " << m_config.front_addr << std::endl;

    // 注册前置地址
    m_api->RegisterFront(const_cast<char*>(m_config.front_addr.c_str()));

    // 初始化（会触发OnFrontConnected回调）
    m_api->Init();

    return true;
}

void CTPMDPlugin::Stop() {
    if (!m_running.load()) {
        return;
    }

    std::cout << "[CTPMDPlugin] Stopping..." << std::endl;
    m_running.store(false);
    m_connected.store(false);
    m_logged_in.store(false);

    // 打印最终统计
    PrintLatencyStats();
    std::cout << "[CTPMDPlugin] Total messages: " << m_md_count.load() << std::endl;
    std::cout << "[CTPMDPlugin] Dropped messages: " << m_md_dropped.load() << std::endl;
}

bool CTPMDPlugin::Subscribe(const std::vector<std::string>& symbols) {
    if (symbols.empty()) {
        std::cout << "[CTPMDPlugin] No symbols to subscribe" << std::endl;
        return true;
    }

    if (!m_logged_in.load()) {
        std::cerr << "[CTPMDPlugin] ❌ Cannot subscribe: not logged in" << std::endl;
        return false;
    }

    std::cout << "[CTPMDPlugin] Subscribing to " << symbols.size() << " symbols..." << std::endl;

    // CTP API要求传递char*数组
    std::vector<char*> instrument_ids;
    for (const auto& symbol : symbols) {
        instrument_ids.push_back(const_cast<char*>(symbol.c_str()));
    }

    int ret = m_api->SubscribeMarketData(
        instrument_ids.data(),
        static_cast<int>(instrument_ids.size())
    );

    if (ret == 0) {
        std::cout << "[CTPMDPlugin] ✅ Subscription request sent" << std::endl;
        // 记录订阅的合约
        std::lock_guard<std::mutex> lock(m_subscription_mutex);
        for (const auto& symbol : symbols) {
            m_subscribed_instruments.insert(symbol);
        }
        return true;
    } else {
        std::cerr << "[CTPMDPlugin] ❌ Failed to subscribe, error: " << ret << std::endl;
        return false;
    }
}

bool CTPMDPlugin::Unsubscribe(const std::vector<std::string>& symbols) {
    if (symbols.empty()) {
        return true;
    }

    if (!m_api) {
        return false;
    }

    std::cout << "[CTPMDPlugin] Unsubscribing from " << symbols.size() << " symbols..." << std::endl;

    // CTP API要求传递char*数组
    std::vector<char*> instrument_ids;
    for (const auto& symbol : symbols) {
        instrument_ids.push_back(const_cast<char*>(symbol.c_str()));
    }

    int ret = m_api->UnSubscribeMarketData(
        instrument_ids.data(),
        static_cast<int>(instrument_ids.size())
    );

    if (ret == 0) {
        // 从记录中移除
        std::lock_guard<std::mutex> lock(m_subscription_mutex);
        for (const auto& symbol : symbols) {
            m_subscribed_instruments.erase(symbol);
        }
        return true;
    } else {
        std::cerr << "[CTPMDPlugin] ❌ Failed to unsubscribe, error: " << ret << std::endl;
        return false;
    }
}

// ==================== CTP回调函数 ====================

void CTPMDPlugin::OnFrontConnected() {
    std::cout << "[CTPMDPlugin] ✅ Connected to front server" << std::endl;
    m_connected.store(true);
    m_reconnect_count = 0;  // 重置重连计数

    // 连接成功后自动登录
    Login();
}

void CTPMDPlugin::OnFrontDisconnected(int nReason) {
    std::cerr << "[CTPMDPlugin] ❌ Disconnected from front server, reason: " << nReason << std::endl;
    m_connected.store(false);
    m_logged_in.store(false);

    // 常见原因说明
    switch (nReason) {
        case 0x1001:
            std::cerr << "  Reason: Network read failure" << std::endl;
            break;
        case 0x1002:
            std::cerr << "  Reason: Network write failure" << std::endl;
            break;
        case 0x2001:
            std::cerr << "  Reason: Heartbeat timeout" << std::endl;
            break;
        case 0x2002:
            std::cerr << "  Reason: Server sent disconnect notification" << std::endl;
            break;
        case 0x2003:
            std::cerr << "  Reason: Repeat login" << std::endl;
            break;
        default:
            std::cerr << "  Reason: Unknown (" << std::hex << nReason << std::dec << ")" << std::endl;
    }

    // 触发重连
    if (m_running.load()) {
        Reconnect();
    }
}

void CTPMDPlugin::Login() {
    std::cout << "[CTPMDPlugin] Sending login request..." << std::endl;

    CThostFtdcReqUserLoginField req = {};
    strncpy(req.BrokerID, m_config.broker_id.c_str(), sizeof(req.BrokerID) - 1);
    strncpy(req.UserID, m_config.user_id.c_str(), sizeof(req.UserID) - 1);
    strncpy(req.Password, m_config.password.c_str(), sizeof(req.Password) - 1);

    int ret = m_api->ReqUserLogin(&req, ++m_request_id);
    if (ret != 0) {
        std::cerr << "[CTPMDPlugin] ❌ Failed to send login request, error: " << ret << std::endl;
    }
}

void CTPMDPlugin::OnRspUserLogin(CThostFtdcRspUserLoginField* pRspUserLogin,
                                  CThostFtdcRspInfoField* pRspInfo,
                                  int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPMDPlugin] ❌ Login failed: " << pRspInfo->ErrorMsg
                  << " (ErrorID: " << pRspInfo->ErrorID << ")" << std::endl;
        return;
    }

    std::cout << "[CTPMDPlugin] ✅ Login successful" << std::endl;
    if (pRspUserLogin) {
        std::cout << "  Trading Day: " << pRspUserLogin->TradingDay << std::endl;
        std::cout << "  Login Time: " << pRspUserLogin->LoginTime << std::endl;
        std::cout << "  System Name: " << pRspUserLogin->SystemName << std::endl;
    }

    m_logged_in.store(true);

    // 登录成功后订阅行情
    SubscribeInstruments();
}

void CTPMDPlugin::SubscribeInstruments() {
    if (m_config.instruments.empty()) {
        std::cout << "[CTPMDPlugin] No instruments configured for subscription" << std::endl;
        return;
    }

    // 使用统一的Subscribe接口
    Subscribe(m_config.instruments);
}

void CTPMDPlugin::OnRspSubMarketData(CThostFtdcSpecificInstrumentField* pSpecificInstrument,
                                      CThostFtdcRspInfoField* pRspInfo,
                                      int nRequestID, bool bIsLast) {
    if (IsErrorResponse(pRspInfo)) {
        std::cerr << "[CTPMDPlugin] ❌ Subscribe failed: " << pRspInfo->ErrorMsg << std::endl;
        if (pSpecificInstrument) {
            std::cerr << "  Instrument: " << pSpecificInstrument->InstrumentID << std::endl;
        }
        return;
    }

    if (pSpecificInstrument) {
        std::cout << "[CTPMDPlugin] ✅ Subscribed: " << pSpecificInstrument->InstrumentID << std::endl;
    }
}

// ==================== 行情数据处理（核心）====================

void CTPMDPlugin::OnRtnDepthMarketData(CThostFtdcDepthMarketDataField* pDepthMarketData) {
    if (!pDepthMarketData) {
        return;
    }

    auto receive_time = std::chrono::high_resolution_clock::now();

    // 转换为内部格式
    hft::shm::MarketDataRaw raw_md = {};
    ConvertMarketData(pDepthMarketData, &raw_md);

    // 推送到共享内存队列
    if (!m_queue->Push(raw_md)) {
        // 队列满，丢弃数据
        m_md_dropped.fetch_add(1);

        // 每丢弃1000条打印一次警告
        if (m_md_dropped.load() % 1000 == 1) {
            std::cerr << "[CTPMDPlugin] ⚠️  Queue full! Dropped " << m_md_dropped.load()
                      << " messages" << std::endl;
        }
        return;
    }

    // 统计
    m_md_count.fetch_add(1);

    // 计算延迟（从CTP时间戳到现在）
    auto process_time = std::chrono::high_resolution_clock::now();
    auto latency = std::chrono::duration_cast<std::chrono::microseconds>(
        process_time - receive_time
    ).count();

    if (m_config.enable_latency_monitor) {
        UpdateLatencyStats(latency * 1000);  // 转换为纳秒
    }

    // 定期打印统计
    if (m_config.enable_latency_monitor &&
        m_md_count.load() % m_config.latency_log_interval == 0) {
        PrintLatencyStats();
    }
}

void CTPMDPlugin::ConvertMarketData(CThostFtdcDepthMarketDataField* ctp_md,
                                     hft::shm::MarketDataRaw* raw_md) {
    // 基本信息
    strncpy(raw_md->symbol, ctp_md->InstrumentID, sizeof(raw_md->symbol) - 1);
    strncpy(raw_md->exchange, "CTP", sizeof(raw_md->exchange) - 1);

    // 时间戳（使用系统当前时间，因为CTP的时间戳格式不统一）
    auto now = std::chrono::system_clock::now();
    raw_md->timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
        now.time_since_epoch()
    ).count();

    // 序列号
    raw_md->seq_num = m_md_count.load() + 1;

    // 买盘（CTP只有5档）
    auto set_price_level = [](double price, int qty, double* out_price, uint32_t* out_qty) {
        // 检查价格有效性（CTP用DBL_MAX表示无效）
        if (price > 0 && price < 1e10) {
            *out_price = price;
            *out_qty = static_cast<uint32_t>(qty);
        } else {
            *out_price = 0.0;
            *out_qty = 0;
        }
    };

    set_price_level(ctp_md->BidPrice1, ctp_md->BidVolume1, &raw_md->bid_price[0], &raw_md->bid_qty[0]);
    set_price_level(ctp_md->BidPrice2, ctp_md->BidVolume2, &raw_md->bid_price[1], &raw_md->bid_qty[1]);
    set_price_level(ctp_md->BidPrice3, ctp_md->BidVolume3, &raw_md->bid_price[2], &raw_md->bid_qty[2]);
    set_price_level(ctp_md->BidPrice4, ctp_md->BidVolume4, &raw_md->bid_price[3], &raw_md->bid_qty[3]);
    set_price_level(ctp_md->BidPrice5, ctp_md->BidVolume5, &raw_md->bid_price[4], &raw_md->bid_qty[4]);

    // 卖盘（CTP只有5档）
    set_price_level(ctp_md->AskPrice1, ctp_md->AskVolume1, &raw_md->ask_price[0], &raw_md->ask_qty[0]);
    set_price_level(ctp_md->AskPrice2, ctp_md->AskVolume2, &raw_md->ask_price[1], &raw_md->ask_qty[1]);
    set_price_level(ctp_md->AskPrice3, ctp_md->AskVolume3, &raw_md->ask_price[2], &raw_md->ask_qty[2]);
    set_price_level(ctp_md->AskPrice4, ctp_md->AskVolume4, &raw_md->ask_price[3], &raw_md->ask_qty[3]);
    set_price_level(ctp_md->AskPrice5, ctp_md->AskVolume5, &raw_md->ask_price[4], &raw_md->ask_qty[4]);

    // 剩余5档填0（系统支持10档）
    for (int i = 5; i < 10; ++i) {
        raw_md->bid_price[i] = 0.0;
        raw_md->bid_qty[i] = 0;
        raw_md->ask_price[i] = 0.0;
        raw_md->ask_qty[i] = 0;
    }

    // 成交信息
    raw_md->last_price = (ctp_md->LastPrice > 0 && ctp_md->LastPrice < 1e10) ? ctp_md->LastPrice : 0.0;
    raw_md->last_qty = ctp_md->Volume;
    raw_md->total_volume = ctp_md->Volume;

    // 注意：MarketDataRaw不包含turnover和open_interest，CTP提供的这些字段被忽略
}

// ==================== 辅助方法 ====================

void CTPMDPlugin::OnRspError(CThostFtdcRspInfoField* pRspInfo,
                              int nRequestID, bool bIsLast) {
    if (pRspInfo && pRspInfo->ErrorID != 0) {
        std::cerr << "[CTPMDPlugin] Error Response: " << pRspInfo->ErrorMsg
                  << " (ErrorID: " << pRspInfo->ErrorID << ")" << std::endl;
    }
}

bool CTPMDPlugin::IsErrorResponse(CThostFtdcRspInfoField* pRspInfo) {
    return pRspInfo && pRspInfo->ErrorID != 0;
}

void CTPMDPlugin::Reconnect() {
    m_reconnect_count++;

    // 检查重连次数限制
    if (m_config.max_reconnect_attempts > 0 &&
        m_reconnect_count > m_config.max_reconnect_attempts) {
        std::cerr << "[CTPMDPlugin] ❌ Max reconnect attempts (" << m_config.max_reconnect_attempts
                  << ") reached, giving up" << std::endl;
        m_running.store(false);
        return;
    }

    // 限制重连频率（至少间隔reconnect_interval_sec秒）
    auto now = std::chrono::steady_clock::now();
    auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(
        now - m_last_reconnect_time
    ).count();

    if (elapsed < m_config.reconnect_interval_sec) {
        int wait_time = m_config.reconnect_interval_sec - elapsed;
        std::cout << "[CTPMDPlugin] Waiting " << wait_time << "s before reconnect..." << std::endl;
        std::this_thread::sleep_for(std::chrono::seconds(wait_time));
    }

    m_last_reconnect_time = now;

    std::cout << "[CTPMDPlugin] Reconnecting (attempt " << m_reconnect_count << ")..." << std::endl;

    // CTP API的重连需要重新初始化
    if (m_api) {
        m_api->Release();
        m_api = nullptr;
    }

    m_api = CThostFtdcMdApi::CreateFtdcMdApi("./ctp_flow/");
    m_api->RegisterSpi(this);
    m_api->RegisterFront(const_cast<char*>(m_config.front_addr.c_str()));
    m_api->Init();
}

void CTPMDPlugin::UpdateLatencyStats(uint64_t latency_ns) {
    m_total_latency_ns.fetch_add(latency_ns);

    // 更新最小值
    uint64_t current_min = m_min_latency_ns.load();
    while (latency_ns < current_min &&
           !m_min_latency_ns.compare_exchange_weak(current_min, latency_ns)) {
        // Retry
    }

    // 更新最大值
    uint64_t current_max = m_max_latency_ns.load();
    while (latency_ns > current_max &&
           !m_max_latency_ns.compare_exchange_weak(current_max, latency_ns)) {
        // Retry
    }
}

void CTPMDPlugin::PrintLatencyStats() {
    uint64_t count = m_md_count.load();
    if (count == 0) {
        return;
    }

    uint64_t total_ns = m_total_latency_ns.load();
    uint64_t min_ns = m_min_latency_ns.load();
    uint64_t max_ns = m_max_latency_ns.load();
    uint64_t avg_ns = total_ns / count;

    auto now = std::chrono::steady_clock::now();
    auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(
        now - m_last_stats_time
    ).count();

    uint64_t rate = (elapsed > 0) ? (count / elapsed) : 0;

    std::cout << "[CTPMDPlugin] Stats: "
              << "Count=" << count
              << ", Rate=" << rate << " msg/s"
              << ", Latency(μs): Min=" << (min_ns / 1000)
              << ", Avg=" << (avg_ns / 1000)
              << ", Max=" << (max_ns / 1000)
              << ", Dropped=" << m_md_dropped.load()
              << std::endl;
}

} // namespace ctp
} // namespace plugin
} // namespace hft

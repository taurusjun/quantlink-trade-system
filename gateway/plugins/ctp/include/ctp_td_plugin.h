#pragma once

#include "plugin/td_plugin_interface.h"
#include "ThostFtdcTraderApi.h"
#include "ctp_td_config.h"
#include <atomic>
#include <memory>
#include <string>
#include <vector>
#include <map>
#include <mutex>
#include <condition_variable>
#include <chrono>

namespace hft {
namespace plugin {
namespace ctp {

/**
 * CTP持仓信息（用于Offset自动判断）
 */
struct CTPPosition {
    std::string symbol;
    std::string exchange;

    // 多头持仓
    uint32_t long_position;           // 总持仓
    uint32_t long_today_position;     // 今仓
    uint32_t long_yesterday_position; // 昨仓

    // 空头持仓
    uint32_t short_position;           // 总持仓
    uint32_t short_today_position;     // 今仓
    uint32_t short_yesterday_position; // 昨仓

    // 持仓均价
    double long_avg_price;
    double short_avg_price;

    CTPPosition()
        : long_position(0), long_today_position(0), long_yesterday_position(0)
        , short_position(0), short_today_position(0), short_yesterday_position(0)
        , long_avg_price(0.0), short_avg_price(0.0) {}
};

/**
 * CTP交易插件实现
 * 实现ITDPlugin接口，对接CTP交易API
 */
class CTPTDPlugin : public ITDPlugin, public CThostFtdcTraderSpi {
public:
    CTPTDPlugin();
    virtual ~CTPTDPlugin();

    // ==================== ITDPlugin接口实现 ====================

    bool Initialize(const std::string& config_file) override;
    bool Login() override;
    void Logout() override;
    bool IsLoggedIn() const override { return m_logged_in.load(); }
    bool IsConnected() const override { return m_connected.load(); }

    std::string SendOrder(const OrderRequest& request) override;
    bool CancelOrder(const std::string& order_id) override;

    bool QueryAccount(AccountInfo& account_info) override;
    bool QueryPositions(std::vector<PositionInfo>& positions) override;
    bool QueryOrders(std::vector<OrderInfo>& orders) override;
    bool QueryTrades(std::vector<TradeInfo>& trades) override;
    bool GetOrder(const std::string& order_id, OrderInfo& order_info) override;

    void RegisterOrderCallback(OrderCallback callback) override;
    void RegisterTradeCallback(TradeCallback callback) override;
    void RegisterErrorCallback(ErrorCallback callback) override;

    std::string GetPluginName() const override { return "CTP"; }
    std::string GetPluginVersion() const override { return "1.0.0"; }
    double GetAvailableFund() const override;

    uint64_t GetOrderCount() const override { return m_order_count.load(); }
    uint64_t GetTradeCount() const override { return m_trade_count.load(); }

    // ==================== CTP API回调接口 ====================

    // 连接相关
    void OnFrontConnected() override;
    void OnFrontDisconnected(int nReason) override;

    // 认证相关
    void OnRspAuthenticate(CThostFtdcRspAuthenticateField* pRspAuthenticateField,
                          CThostFtdcRspInfoField* pRspInfo,
                          int nRequestID, bool bIsLast) override;

    // 登录相关
    void OnRspUserLogin(CThostFtdcRspUserLoginField* pRspUserLogin,
                        CThostFtdcRspInfoField* pRspInfo,
                        int nRequestID, bool bIsLast) override;

    void OnRspUserLogout(CThostFtdcUserLogoutField* pUserLogout,
                        CThostFtdcRspInfoField* pRspInfo,
                        int nRequestID, bool bIsLast) override;

    // 结算单确认
    void OnRspSettlementInfoConfirm(CThostFtdcSettlementInfoConfirmField* pSettlementInfoConfirm,
                                   CThostFtdcRspInfoField* pRspInfo,
                                   int nRequestID, bool bIsLast) override;

    // 报单相关
    void OnRspOrderInsert(CThostFtdcInputOrderField* pInputOrder,
                         CThostFtdcRspInfoField* pRspInfo,
                         int nRequestID, bool bIsLast) override;

    void OnRspOrderAction(CThostFtdcInputOrderActionField* pInputOrderAction,
                         CThostFtdcRspInfoField* pRspInfo,
                         int nRequestID, bool bIsLast) override;

    void OnRtnOrder(CThostFtdcOrderField* pOrder) override;

    void OnRtnTrade(CThostFtdcTradeField* pTrade) override;

    // 查询相关
    void OnRspQryTradingAccount(CThostFtdcTradingAccountField* pTradingAccount,
                               CThostFtdcRspInfoField* pRspInfo,
                               int nRequestID, bool bIsLast) override;

    void OnRspQryInvestorPosition(CThostFtdcInvestorPositionField* pInvestorPosition,
                                 CThostFtdcRspInfoField* pRspInfo,
                                 int nRequestID, bool bIsLast) override;

    void OnRspQryOrder(CThostFtdcOrderField* pOrder,
                      CThostFtdcRspInfoField* pRspInfo,
                      int nRequestID, bool bIsLast) override;

    void OnRspQryTrade(CThostFtdcTradeField* pTrade,
                      CThostFtdcRspInfoField* pRspInfo,
                      int nRequestID, bool bIsLast) override;

    // 错误相关
    void OnRspError(CThostFtdcRspInfoField* pRspInfo,
                   int nRequestID, bool bIsLast) override;

private:
    // ==================== 内部方法 ====================

    // 认证
    void Authenticate();

    // 登录
    void DoLogin();

    // 结算单确认
    void ConfirmSettlement();

    // 重连
    void Reconnect();

    // 检查是否是错误响应
    bool IsErrorResponse(CThostFtdcRspInfoField* pRspInfo);

    // 数据转换：CTP订单 → OrderInfo
    void ConvertOrder(CThostFtdcOrderField* ctp_order, OrderInfo& order_info);

    // 数据转换：CTP成交 → TradeInfo
    void ConvertTrade(CThostFtdcTradeField* ctp_trade, TradeInfo& trade_info);

    // 数据转换：CTP持仓 → PositionInfo
    void ConvertPosition(CThostFtdcInvestorPositionField* ctp_pos, PositionInfo& pos_info);

    // 数据转换：CTP账户 → AccountInfo
    void ConvertAccount(CThostFtdcTradingAccountField* ctp_account, AccountInfo& account_info);

    // 生成订单ID
    std::string GenerateOrderRef();

    // 保存订单到本地缓存
    void SaveOrder(const std::string& order_id, const OrderInfo& order_info);

    // 从本地缓存获取订单
    bool GetOrderFromCache(const std::string& order_id, OrderInfo& order_info);

    // Offset自动设置
    void SetOpenClose(OrderRequest& request);

    // 更新持仓信息（从CTP查询结果更新）
    void UpdatePositionFromCTP();

    // 根据成交更新持仓
    void UpdatePositionFromTrade(const TradeInfo& trade);

    // 持仓持久化
    bool SavePositionsToFile();
    bool LoadPositionsFromFile();

    // ==================== 成员变量 ====================

    // 配置
    hft::gateway::CTPTDConfig m_config;

    // CTP API
    CThostFtdcTraderApi* m_api = nullptr;

    // 状态管理
    std::atomic<bool> m_connected{false};
    std::atomic<bool> m_authenticated{false};
    std::atomic<bool> m_logged_in{false};
    std::atomic<bool> m_settlement_confirmed{false};

    // 请求ID（线程安全）
    std::atomic<int> m_request_id{0};

    // 报单引用（线程安全）
    std::atomic<int> m_order_ref{0};

    // 会话ID和前置ID
    int m_front_id = 0;
    int m_session_id = 0;

    // 重连相关
    int m_reconnect_count = 0;
    std::chrono::steady_clock::time_point m_last_reconnect_time;

    // 统计
    std::atomic<uint64_t> m_order_count{0};
    std::atomic<uint64_t> m_trade_count{0};

    // 订单缓存（order_id → OrderInfo）
    std::map<std::string, OrderInfo> m_order_cache;
    mutable std::mutex m_order_cache_mutex;

    // 持仓管理（symbol → CTPPosition）
    std::map<std::string, CTPPosition> m_positions;
    mutable std::mutex m_position_mutex;

    // 持仓持久化路径
    std::string m_position_file_path;

    // 回调函数
    OrderCallback m_order_callback;
    TradeCallback m_trade_callback;
    ErrorCallback m_error_callback;
    std::mutex m_callback_mutex;

    // 查询结果缓存和同步
    AccountInfo m_cached_account;
    std::vector<PositionInfo> m_cached_positions;
    std::vector<OrderInfo> m_cached_orders;
    std::vector<TradeInfo> m_cached_trades;

    std::mutex m_query_mutex;
    std::condition_variable m_query_cv;
    bool m_query_finished = false;

    // 可用资金缓存
    std::atomic<double> m_available_fund{0.0};
};

} // namespace ctp
} // namespace plugin
} // namespace hft

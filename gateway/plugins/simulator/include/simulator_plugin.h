#pragma once

#include "simulator_config.h"
#include "../../include/plugin/td_plugin_interface.h"
#include <atomic>
#include <map>
#include <mutex>
#include <string>
#include <vector>
#include <chrono>

namespace hft {
namespace plugin {
namespace simulator {

// Internal order structure
struct InternalOrder {
    std::string order_id;
    std::string client_order_id;
    hft::plugin::OrderRequest request;
    hft::plugin::OrderStatus status;
    uint32_t traded_volume;
    uint64_t insert_time;
    uint64_t update_time;
    char status_msg[256];  // Status message (for REJECTED orders)
};

// Internal position structure
struct InternalPosition {
    std::string symbol;
    std::string exchange;
    hft::plugin::OrderDirection direction;
    uint32_t volume;              // Total position
    uint32_t today_volume;        // Today position
    uint32_t yesterday_volume;    // Yesterday position
    double avg_price;             // Average open price
    double total_cost;            // Total cost
    double total_volume_traded;   // Total traded volume
    double margin;                // Used margin
    double unrealized_pnl;        // Unrealized P&L
};

class SimulatorPlugin : public hft::plugin::ITDPlugin {
public:
    SimulatorPlugin();
    virtual ~SimulatorPlugin();

    // ITDPlugin interface implementation
    bool Initialize(const std::string& config_file) override;
    bool Login() override;
    void Logout() override;
    bool IsConnected() const override;
    bool IsLoggedIn() const override;

    std::string SendOrder(const hft::plugin::OrderRequest& request) override;
    bool CancelOrder(const std::string& order_id) override;

    bool QueryAccount(hft::plugin::AccountInfo& account_info) override;
    bool QueryPositions(std::vector<hft::plugin::PositionInfo>& positions) override;
    bool QueryOrders(std::vector<hft::plugin::OrderInfo>& orders) override;
    bool QueryTrades(std::vector<hft::plugin::TradeInfo>& trades) override;
    bool GetOrder(const std::string& order_id, hft::plugin::OrderInfo& order_info) override;

    void RegisterOrderCallback(hft::plugin::OrderCallback callback) override;
    void RegisterTradeCallback(hft::plugin::TradeCallback callback) override;
    void RegisterErrorCallback(hft::plugin::ErrorCallback callback) override;

    std::string GetPluginName() const override { return "SimulatorPlugin"; }
    std::string GetPluginVersion() const override { return "1.0.0"; }
    double GetAvailableFund() const override { return m_available; }
    uint64_t GetOrderCount() const override { return m_order_count.load(); }
    uint64_t GetTradeCount() const override { return m_trade_count.load(); }

private:
    // Configuration
    SimulatorConfig m_config;

    // State
    std::atomic<bool> m_connected{false};
    std::atomic<bool> m_logged_in{false};
    std::atomic<uint64_t> m_order_count{0};
    std::atomic<uint64_t> m_trade_count{0};
    std::atomic<int64_t> m_order_ref{1};

    // Account
    double m_balance;
    double m_available;
    double m_margin;
    double m_commission;
    double m_close_profit;
    double m_daily_pnl;
    std::mutex m_account_mutex;

    // Positions
    std::map<std::string, InternalPosition> m_positions;  // key: symbol_direction
    std::mutex m_position_mutex;

    // Orders
    std::map<std::string, InternalOrder> m_orders;
    std::mutex m_order_mutex;

    // Trades
    std::vector<hft::plugin::TradeInfo> m_trades;
    std::mutex m_trade_mutex;

    // Callbacks
    hft::plugin::OrderCallback m_order_callback;
    hft::plugin::TradeCallback m_trade_callback;
    hft::plugin::ErrorCallback m_error_callback;

    // Internal methods
    std::string GenerateOrderID();
    std::string GenerateTradeID();
    void SetOpenClose(hft::plugin::OrderRequest& request);  // Auto-set offset based on position
    void ProcessOrderImmediate(const std::string& order_id, const hft::plugin::OrderRequest& request);
    void UpdatePosition(const hft::plugin::TradeInfo& trade);
    void UpdateAccount();
    double CalculateMargin(const std::string& symbol, double price, uint32_t volume);
    double CalculateCommission(const std::string& symbol, double price, uint32_t volume);
    bool CheckRisk(const hft::plugin::OrderRequest& request, std::string* error_msg);
    hft::plugin::OrderInfo ConvertToOrderInfo(const InternalOrder& order);
    uint64_t GetCurrentNanoTime();
};

} // namespace simulator
} // namespace plugin
} // namespace hft

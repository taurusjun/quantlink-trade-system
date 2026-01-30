#include "../include/simulator_plugin.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <cstring>
#include <iomanip>
#include <sstream>
#include <algorithm>

namespace hft {
namespace plugin {
namespace simulator {

// ==================== 构造和析构 ====================

SimulatorPlugin::SimulatorPlugin() {
    std::cout << "[SimulatorPlugin] Constructor called" << std::endl;

    // Initialize account state
    m_balance = 0.0;
    m_available = 0.0;
    m_margin = 0.0;
    m_commission = 0.0;
    m_close_profit = 0.0;
    m_daily_pnl = 0.0;
}

SimulatorPlugin::~SimulatorPlugin() {
    std::cout << "[SimulatorPlugin] Destructor called" << std::endl;
    Logout();
}

// ==================== ITDPlugin接口实现 - 生命周期管理 ====================

bool SimulatorPlugin::Initialize(const std::string& config_file) {
    std::cout << "[SimulatorPlugin] Initializing with config: " << config_file << std::endl;

    try {
        // Load configuration
        if (!m_config.LoadFromYaml(config_file)) {
            std::cerr << "[SimulatorPlugin] ❌ Failed to load config file: " << config_file << std::endl;
            return false;
        }

        // Validate configuration
        std::string error;
        if (!m_config.Validate(&error)) {
            std::cerr << "[SimulatorPlugin] ❌ Invalid config: " << error << std::endl;
            return false;
        }

        // Initialize account with initial balance
        m_balance = m_config.initial_balance;
        m_available = m_config.initial_balance;
        m_margin = 0.0;
        m_commission = 0.0;
        m_close_profit = 0.0;
        m_daily_pnl = 0.0;

        std::cout << "[SimulatorPlugin] ✅ Initialized successfully" << std::endl;
        std::cout << "[SimulatorPlugin] Initial balance: " << m_balance << std::endl;
        std::cout << "[SimulatorPlugin] Mode: " << m_config.mode << std::endl;
        return true;

    } catch (const std::exception& e) {
        std::cerr << "[SimulatorPlugin] ❌ Exception during initialization: "
                  << e.what() << std::endl;
        return false;
    }
}

bool SimulatorPlugin::Login() {
    if (m_logged_in.load()) {
        std::cout << "[SimulatorPlugin] Already logged in" << std::endl;
        return true;
    }

    std::cout << "[SimulatorPlugin] Logging in..." << std::endl;

    // Set connected and logged in state
    m_connected.store(true);
    m_logged_in.store(true);

    // Reset counters
    m_order_count.store(0);
    m_trade_count.store(0);
    m_order_ref.store(1);

    // Initialize balance
    {
        std::lock_guard<std::mutex> lock(m_account_mutex);
        m_balance = m_config.initial_balance;
        m_available = m_config.initial_balance;
        m_margin = 0.0;
        m_commission = 0.0;
        m_close_profit = 0.0;
        m_daily_pnl = 0.0;
    }

    std::cout << "[SimulatorPlugin] ✅ Login successful" << std::endl;
    std::cout << "[SimulatorPlugin] Available balance: " << m_available << std::endl;
    return true;
}

void SimulatorPlugin::Logout() {
    if (!m_logged_in.load()) {
        return;
    }

    std::cout << "[SimulatorPlugin] Logging out..." << std::endl;

    // Clear all data structures
    {
        std::lock_guard<std::mutex> lock(m_order_mutex);
        m_orders.clear();
    }

    {
        std::lock_guard<std::mutex> lock(m_trade_mutex);
        m_trades.clear();
    }

    {
        std::lock_guard<std::mutex> lock(m_position_mutex);
        m_positions.clear();
    }

    // Reset state
    m_logged_in.store(false);
    m_connected.store(false);

    std::cout << "[SimulatorPlugin] Logged out" << std::endl;
}

bool SimulatorPlugin::IsConnected() const {
    return m_connected.load();
}

bool SimulatorPlugin::IsLoggedIn() const {
    return m_logged_in.load();
}

// ==================== ITDPlugin接口实现 - 交易功能 ====================

std::string SimulatorPlugin::SendOrder(const hft::plugin::OrderRequest& request) {
    if (!m_logged_in.load()) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot send order: not logged in" << std::endl;
        if (m_error_callback) {
            m_error_callback(-1, "Not logged in");
        }
        return "";
    }

    // Check risk
    std::string error_msg;
    if (!CheckRisk(request, &error_msg)) {
        std::cerr << "[SimulatorPlugin] ❌ Risk check failed: " << error_msg << std::endl;
        if (m_error_callback) {
            m_error_callback(-2, error_msg);
        }
        return "";
    }

    // Generate order ID
    std::string order_id = GenerateOrderID();

    // Create internal order
    InternalOrder internal_order;
    internal_order.order_id = order_id;
    internal_order.client_order_id = request.client_order_id;
    internal_order.request = request;
    internal_order.status = hft::plugin::OrderStatus::SUBMITTING;
    internal_order.traded_volume = 0;
    internal_order.insert_time = GetCurrentNanoTime();
    internal_order.update_time = internal_order.insert_time;

    // Store order
    {
        std::lock_guard<std::mutex> lock(m_order_mutex);
        m_orders[order_id] = internal_order;
    }

    // Increment order count
    m_order_count.fetch_add(1);

    std::cout << "[SimulatorPlugin] Order submitted: " << order_id
              << " | " << request.symbol
              << " | " << (request.direction == hft::plugin::OrderDirection::BUY ? "BUY" : "SELL")
              << " | " << request.volume << "@" << request.price << std::endl;

    // Notify order callback (SUBMITTING)
    if (m_order_callback) {
        m_order_callback(ConvertToOrderInfo(internal_order));
    }

    // Process order based on mode
    if (m_config.mode == "immediate") {
        // Launch async thread to process order
        std::thread([this, order_id, request]() {
            ProcessOrderImmediate(order_id, request);
        }).detach();
    } else {
        // market_driven mode not implemented yet
        std::cerr << "[SimulatorPlugin] ⚠️ market_driven mode not implemented" << std::endl;
    }

    return order_id;
}

bool SimulatorPlugin::CancelOrder(const std::string& order_id) {
    if (!m_logged_in.load()) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot cancel order: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_order_mutex);

    auto it = m_orders.find(order_id);
    if (it == m_orders.end()) {
        std::cerr << "[SimulatorPlugin] ❌ Order not found: " << order_id << std::endl;
        return false;
    }

    InternalOrder& order = it->second;

    // Check if order can be canceled
    if (order.status == hft::plugin::OrderStatus::FILLED ||
        order.status == hft::plugin::OrderStatus::CANCELED ||
        order.status == hft::plugin::OrderStatus::REJECTED) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot cancel order in status: "
                  << static_cast<int>(order.status) << std::endl;
        return false;
    }

    // Update order status
    order.status = hft::plugin::OrderStatus::CANCELED;
    order.update_time = GetCurrentNanoTime();

    std::cout << "[SimulatorPlugin] Order canceled: " << order_id << std::endl;

    // Notify order callback
    if (m_order_callback) {
        m_order_callback(ConvertToOrderInfo(order));
    }

    return true;
}

// ==================== ITDPlugin接口实现 - 查询功能 ====================

bool SimulatorPlugin::QueryAccount(hft::plugin::AccountInfo& account_info) {
    if (!m_logged_in.load()) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot query account: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_account_mutex);

    // Update account before returning
    // (This is safe to call even with mutex held as UpdateAccount uses its own locks)

    std::strncpy(account_info.account_id, "SIMULATOR", sizeof(account_info.account_id) - 1);
    account_info.balance = m_balance;
    account_info.available = m_available;
    account_info.margin = m_margin;
    account_info.frozen_margin = 0.0;  // Simulator doesn't track frozen margin separately
    account_info.commission = m_commission;
    account_info.close_profit = m_close_profit;
    account_info.position_profit = 0.0;  // TODO: Calculate from positions

    return true;
}

bool SimulatorPlugin::QueryPositions(std::vector<hft::plugin::PositionInfo>& positions) {
    if (!m_logged_in.load()) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot query positions: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_position_mutex);

    positions.clear();
    for (const auto& pair : m_positions) {
        const InternalPosition& internal_pos = pair.second;

        hft::plugin::PositionInfo pos_info;
        std::strncpy(pos_info.symbol, internal_pos.symbol.c_str(), sizeof(pos_info.symbol) - 1);
        std::strncpy(pos_info.exchange, internal_pos.exchange.c_str(), sizeof(pos_info.exchange) - 1);
        pos_info.direction = internal_pos.direction;
        pos_info.volume = internal_pos.volume;
        pos_info.today_volume = internal_pos.today_volume;
        pos_info.yesterday_volume = internal_pos.yesterday_volume;
        pos_info.avg_price = internal_pos.avg_price;
        pos_info.position_profit = internal_pos.unrealized_pnl;
        pos_info.margin = internal_pos.margin;

        positions.push_back(pos_info);
    }

    return true;
}

bool SimulatorPlugin::QueryOrders(std::vector<hft::plugin::OrderInfo>& orders) {
    if (!m_logged_in.load()) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot query orders: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_order_mutex);

    orders.clear();
    for (const auto& pair : m_orders) {
        orders.push_back(ConvertToOrderInfo(pair.second));
    }

    return true;
}

bool SimulatorPlugin::QueryTrades(std::vector<hft::plugin::TradeInfo>& trades) {
    if (!m_logged_in.load()) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot query trades: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_trade_mutex);

    trades = m_trades;
    return true;
}

bool SimulatorPlugin::GetOrder(const std::string& order_id, hft::plugin::OrderInfo& order_info) {
    if (!m_logged_in.load()) {
        std::cerr << "[SimulatorPlugin] ❌ Cannot get order: not logged in" << std::endl;
        return false;
    }

    std::lock_guard<std::mutex> lock(m_order_mutex);

    auto it = m_orders.find(order_id);
    if (it == m_orders.end()) {
        std::cerr << "[SimulatorPlugin] ❌ Order not found: " << order_id << std::endl;
        return false;
    }

    order_info = ConvertToOrderInfo(it->second);
    return true;
}

// ==================== ITDPlugin接口实现 - 回调注册 ====================

void SimulatorPlugin::RegisterOrderCallback(hft::plugin::OrderCallback callback) {
    m_order_callback = callback;
    std::cout << "[SimulatorPlugin] Order callback registered" << std::endl;
}

void SimulatorPlugin::RegisterTradeCallback(hft::plugin::TradeCallback callback) {
    m_trade_callback = callback;
    std::cout << "[SimulatorPlugin] Trade callback registered" << std::endl;
}

void SimulatorPlugin::RegisterErrorCallback(hft::plugin::ErrorCallback callback) {
    m_error_callback = callback;
    std::cout << "[SimulatorPlugin] Error callback registered" << std::endl;
}

// ==================== 内部方法 - 订单处理 ====================

void SimulatorPlugin::ProcessOrderImmediate(const std::string& order_id,
                                           const hft::plugin::OrderRequest& request) {
    // Simulate accept delay
    if (m_config.accept_delay_ms > 0) {
        std::this_thread::sleep_for(std::chrono::milliseconds(m_config.accept_delay_ms));
    }

    // Update order status to ACCEPTED
    {
        std::lock_guard<std::mutex> lock(m_order_mutex);
        auto it = m_orders.find(order_id);
        if (it == m_orders.end()) {
            return;  // Order was removed
        }

        InternalOrder& order = it->second;
        if (order.status == hft::plugin::OrderStatus::CANCELED) {
            return;  // Order was canceled
        }

        order.status = hft::plugin::OrderStatus::ACCEPTED;
        order.update_time = GetCurrentNanoTime();

        if (m_order_callback) {
            m_order_callback(ConvertToOrderInfo(order));
        }
    }

    // Simulate fill delay
    if (m_config.fill_delay_ms > 0) {
        std::this_thread::sleep_for(std::chrono::milliseconds(m_config.fill_delay_ms));
    }

    // Check if order was canceled during delay
    {
        std::lock_guard<std::mutex> lock(m_order_mutex);
        auto it = m_orders.find(order_id);
        if (it == m_orders.end() || it->second.status == hft::plugin::OrderStatus::CANCELED) {
            return;
        }
    }

    // Calculate fill price with slippage
    double fill_price = request.price;
    if (request.price_type == hft::plugin::PriceType::MARKET || m_config.slippage_ticks > 0) {
        // Apply slippage based on direction
        // For simplicity, assume 1 tick = 1 unit (should be configurable per symbol)
        if (request.direction == hft::plugin::OrderDirection::BUY) {
            fill_price += m_config.slippage_ticks;
        } else {
            fill_price -= m_config.slippage_ticks;
        }
    }

    // Generate trade
    hft::plugin::TradeInfo trade;
    std::string trade_id = GenerateTradeID();
    std::strncpy(trade.trade_id, trade_id.c_str(), sizeof(trade.trade_id) - 1);
    std::strncpy(trade.order_id, order_id.c_str(), sizeof(trade.order_id) - 1);
    std::strncpy(trade.symbol, request.symbol, sizeof(trade.symbol) - 1);
    std::strncpy(trade.exchange, request.exchange, sizeof(trade.exchange) - 1);
    trade.direction = request.direction;
    trade.offset = request.offset;
    trade.price = fill_price;
    trade.volume = request.volume;
    trade.trade_time = GetCurrentNanoTime();

    // Update order status to FILLED
    {
        std::lock_guard<std::mutex> lock(m_order_mutex);
        auto it = m_orders.find(order_id);
        if (it == m_orders.end()) {
            return;
        }

        InternalOrder& order = it->second;
        order.status = hft::plugin::OrderStatus::FILLED;
        order.traded_volume = request.volume;
        order.update_time = GetCurrentNanoTime();

        if (m_order_callback) {
            m_order_callback(ConvertToOrderInfo(order));
        }
    }

    // Store trade
    {
        std::lock_guard<std::mutex> lock(m_trade_mutex);
        m_trades.push_back(trade);
    }

    // Increment trade count
    m_trade_count.fetch_add(1);

    std::cout << "[SimulatorPlugin] Trade executed: " << trade_id
              << " | " << request.symbol
              << " | " << request.volume << "@" << fill_price << std::endl;

    // Update position
    UpdatePosition(trade);

    // Update account
    UpdateAccount();

    // Notify trade callback
    if (m_trade_callback) {
        m_trade_callback(trade);
    }
}

// UpdatePosition updates position using Chinese futures net position model
// 参考 tbsrc 和 Golang BaseStrategy::UpdatePosition
// 中国期货市场规则：
// 1. 不允许同时持有多空持仓
// 2. 买入时先平空再开多
// 3. 卖出时先平多再开空
// 4. 模拟交易所自己判断开平，不依赖 offset 字段
void SimulatorPlugin::UpdatePosition(const hft::plugin::TradeInfo& trade) {
    std::lock_guard<std::mutex> lock(m_position_mutex);

    // Use symbol as position key (NOT symbol_direction, because we use net position model)
    std::string pos_key = std::string(trade.symbol);

    // Get or create position
    InternalPosition& pos = m_positions[pos_key];

    // Initialize position if new
    if (pos.volume == 0 && pos.total_volume_traded == 0) {
        pos.symbol = trade.symbol;
        pos.exchange = trade.exchange;
        pos.yesterday_volume = 0;
    }

    uint32_t qty = trade.volume;
    double price = trade.price;

    if (trade.direction == hft::plugin::OrderDirection::BUY) {
        // 买入逻辑：先平空再开多
        // 1. 检查是否有空头持仓需要平仓
        if (pos.direction == hft::plugin::OrderDirection::SELL && pos.volume > 0) {
            // 当前是空头持仓，买入平空
            uint32_t closedQty = std::min(qty, pos.volume);

            // 计算平空盈亏: (卖出均价 - 买入价) × 平仓数量
            double close_pnl = (pos.avg_price - price) * closedQty;
            {
                std::lock_guard<std::mutex> lock(m_account_mutex);
                m_close_profit += close_pnl;
                m_daily_pnl += close_pnl;
            }

            // 减少空头持仓
            pos.volume -= closedQty;

            // 更新今昨仓
            if (pos.today_volume >= closedQty) {
                pos.today_volume -= closedQty;
            } else {
                uint32_t from_yesterday = closedQty - pos.today_volume;
                pos.today_volume = 0;
                pos.yesterday_volume -= std::min(pos.yesterday_volume, from_yesterday);
            }

            std::cout << "[SimulatorPlugin] 平空: " << closedQty << " @ " << price
                      << ", 空头均价 " << pos.avg_price << ", 盈亏 " << close_pnl
                      << ", 剩余 " << pos.volume << std::endl;

            qty -= closedQty;

            // 如果全部平仓，重置相关数据
            if (pos.volume == 0) {
                pos.avg_price = 0;
                pos.total_cost = 0;
                pos.total_volume_traded = 0;
                pos.direction = hft::plugin::OrderDirection::BUY;  // Reset direction
            }
        }

        // 2. 剩余数量开多仓
        if (qty > 0) {
            // 更新多头持仓和平均价
            double oldCost = pos.avg_price * pos.volume;
            pos.total_cost = oldCost + price * qty;
            pos.volume += qty;
            pos.today_volume += qty;
            pos.total_volume_traded += qty;
            pos.avg_price = pos.total_cost / pos.total_volume_traded;
            pos.direction = hft::plugin::OrderDirection::BUY;

            std::cout << "[SimulatorPlugin] 开多: " << qty << " @ " << price
                      << ", 多头均价 " << pos.avg_price << ", 总持仓 " << pos.volume << std::endl;
        }

    } else {
        // 卖出逻辑：先平多再开空
        // 1. 检查是否有多头持仓需要平仓
        if (pos.direction == hft::plugin::OrderDirection::BUY && pos.volume > 0) {
            // 当前是多头持仓，卖出平多
            uint32_t closedQty = std::min(qty, pos.volume);

            // 计算平多盈亏: (卖出价 - 买入均价) × 平仓数量
            double close_pnl = (price - pos.avg_price) * closedQty;
            {
                std::lock_guard<std::mutex> lock(m_account_mutex);
                m_close_profit += close_pnl;
                m_daily_pnl += close_pnl;
            }

            // 减少多头持仓
            pos.volume -= closedQty;

            // 更新今昨仓
            if (pos.today_volume >= closedQty) {
                pos.today_volume -= closedQty;
            } else {
                uint32_t from_yesterday = closedQty - pos.today_volume;
                pos.today_volume = 0;
                pos.yesterday_volume -= std::min(pos.yesterday_volume, from_yesterday);
            }

            std::cout << "[SimulatorPlugin] 平多: " << closedQty << " @ " << price
                      << ", 多头均价 " << pos.avg_price << ", 盈亏 " << close_pnl
                      << ", 剩余 " << pos.volume << std::endl;

            qty -= closedQty;

            // 如果全部平仓，重置相关数据
            if (pos.volume == 0) {
                pos.avg_price = 0;
                pos.total_cost = 0;
                pos.total_volume_traded = 0;
                pos.direction = hft::plugin::OrderDirection::SELL;  // Reset direction
            }
        }

        // 2. 剩余数量开空仓
        if (qty > 0) {
            // 更新空头持仓和平均价
            double oldCost = pos.avg_price * pos.volume;
            pos.total_cost = oldCost + price * qty;
            pos.volume += qty;
            pos.today_volume += qty;
            pos.total_volume_traded += qty;
            pos.avg_price = pos.total_cost / pos.total_volume_traded;
            pos.direction = hft::plugin::OrderDirection::SELL;

            std::cout << "[SimulatorPlugin] 开空: " << qty << " @ " << price
                      << ", 空头均价 " << pos.avg_price << ", 总持仓 " << pos.volume << std::endl;
        }
    }

    // Recalculate margin
    if (pos.volume > 0) {
        pos.margin = CalculateMargin(trade.symbol, price, pos.volume);
    } else {
        // No position, remove from map
        m_positions.erase(pos_key);
        std::cout << "[SimulatorPlugin] ✅ 持仓归零，移除: " << pos_key << std::endl;
    }
}

void SimulatorPlugin::UpdateAccount() {
    std::lock_guard<std::mutex> lock(m_account_mutex);

    // Calculate total margin from all positions
    double total_margin = 0.0;
    {
        std::lock_guard<std::mutex> lock(m_position_mutex);
        for (const auto& pair : m_positions) {
            total_margin += pair.second.margin;
        }
    }

    // Update account fields
    m_margin = total_margin;
    m_available = m_balance - m_margin - m_commission;

    // Balance = initial_balance + close_profit - commission
    m_balance = m_config.initial_balance + m_close_profit - m_commission;
}

// ==================== 内部方法 - 工具函数 ====================

double SimulatorPlugin::CalculateMargin(const std::string& symbol, double price, uint32_t volume) {
    // Simple margin calculation: price * volume * margin_rate
    // In real implementation, this should be per-symbol configurable
    return price * volume * m_config.margin_rate;
}

double SimulatorPlugin::CalculateCommission(const std::string& symbol, double price, uint32_t volume) {
    // Simple commission calculation: price * volume * commission_rate
    // In real implementation, this could be per-trade or per-volume
    return price * volume * m_config.commission_rate;
}

bool SimulatorPlugin::CheckRisk(const hft::plugin::OrderRequest& request, std::string* error_msg) {
    // Check max position per symbol
    {
        std::lock_guard<std::mutex> lock(m_position_mutex);

        std::string pos_key = std::string(request.symbol) + "_" +
                             (request.direction == hft::plugin::OrderDirection::BUY ? "LONG" : "SHORT");

        auto it = m_positions.find(pos_key);
        uint32_t current_volume = (it != m_positions.end()) ? it->second.volume : 0;

        if (request.offset == hft::plugin::OffsetFlag::OPEN) {
            if (current_volume + request.volume > static_cast<uint32_t>(m_config.max_position_per_symbol)) {
                if (error_msg) {
                    *error_msg = "Exceeds max position per symbol: " +
                                std::to_string(m_config.max_position_per_symbol);
                }
                return false;
            }
        }
    }

    // Check available funds for open position
    if (request.offset == hft::plugin::OffsetFlag::OPEN) {
        double required_margin = CalculateMargin(request.symbol, request.price, request.volume);
        double required_commission = CalculateCommission(request.symbol, request.price, request.volume);

        std::lock_guard<std::mutex> lock(m_account_mutex);
        if (m_available < required_margin + required_commission) {
            if (error_msg) {
                *error_msg = "Insufficient funds. Required: " +
                            std::to_string(required_margin + required_commission) +
                            ", Available: " + std::to_string(m_available);
            }
            return false;
        }
    }

    // Check max daily loss
    {
        std::lock_guard<std::mutex> lock(m_account_mutex);
        if (m_daily_pnl < -m_config.max_daily_loss) {
            if (error_msg) {
                *error_msg = "Exceeds max daily loss: " + std::to_string(m_config.max_daily_loss);
            }
            return false;
        }
    }

    return true;
}

std::string SimulatorPlugin::GenerateOrderID() {
    uint64_t seq = m_order_ref.fetch_add(1);
    uint64_t timestamp = GetCurrentNanoTime();

    std::ostringstream oss;
    oss << "SIM_" << timestamp << "_" << seq;
    return oss.str();
}

std::string SimulatorPlugin::GenerateTradeID() {
    uint64_t seq = m_trade_count.load();
    uint64_t timestamp = GetCurrentNanoTime();

    std::ostringstream oss;
    oss << "TRD_" << timestamp << "_" << seq;
    return oss.str();
}

hft::plugin::OrderInfo SimulatorPlugin::ConvertToOrderInfo(const InternalOrder& order) {
    hft::plugin::OrderInfo order_info;

    std::strncpy(order_info.order_id, order.order_id.c_str(), sizeof(order_info.order_id) - 1);
    std::strncpy(order_info.client_order_id, order.client_order_id.c_str(), sizeof(order_info.client_order_id) - 1);
    std::strncpy(order_info.symbol, order.request.symbol, sizeof(order_info.symbol) - 1);
    std::strncpy(order_info.exchange, order.request.exchange, sizeof(order_info.exchange) - 1);

    order_info.direction = order.request.direction;
    order_info.offset = order.request.offset;
    order_info.price_type = order.request.price_type;
    order_info.price = order.request.price;
    order_info.volume = order.request.volume;
    order_info.traded_volume = order.traded_volume;
    order_info.status = order.status;
    order_info.insert_time = order.insert_time;
    order_info.update_time = order.update_time;

    // Set status message based on status
    const char* status_msg = "Unknown";
    switch (order.status) {
        case hft::plugin::OrderStatus::SUBMITTING:
            status_msg = "Submitting";
            break;
        case hft::plugin::OrderStatus::SUBMITTED:
            status_msg = "Submitted";
            break;
        case hft::plugin::OrderStatus::ACCEPTED:
            status_msg = "Accepted";
            break;
        case hft::plugin::OrderStatus::PARTIAL_FILLED:
            status_msg = "Partial Filled";
            break;
        case hft::plugin::OrderStatus::FILLED:
            status_msg = "Filled";
            break;
        case hft::plugin::OrderStatus::CANCELING:
            status_msg = "Canceling";
            break;
        case hft::plugin::OrderStatus::CANCELED:
            status_msg = "Canceled";
            break;
        case hft::plugin::OrderStatus::REJECTED:
            status_msg = "Rejected";
            break;
        case hft::plugin::OrderStatus::ERROR:
            status_msg = "Error";
            break;
        default:
            status_msg = "Unknown";
    }
    std::strncpy(order_info.status_msg, status_msg, sizeof(order_info.status_msg) - 1);

    return order_info;
}

uint64_t SimulatorPlugin::GetCurrentNanoTime() {
    auto now = std::chrono::system_clock::now();
    auto duration = now.time_since_epoch();
    return std::chrono::duration_cast<std::chrono::nanoseconds>(duration).count();
}

} // namespace simulator
} // namespace plugin
} // namespace hft

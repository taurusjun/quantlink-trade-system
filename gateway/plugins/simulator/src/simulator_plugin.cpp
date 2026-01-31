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

    // 复制 request（可能需要修改 offset）
    hft::plugin::OrderRequest modified_request = request;

    // 自动根据持仓设置 offset（与 CTP Plugin 行为一致）
    // 参考 ctp_td_plugin.cpp 的 SetOpenClose 逻辑
    hft::plugin::OffsetFlag original_offset = modified_request.offset;
    SetOpenClose(modified_request);

    if (original_offset != modified_request.offset) {
        std::cout << "[SimulatorPlugin] Auto-set offset: "
                  << modified_request.symbol << " "
                  << (modified_request.direction == hft::plugin::OrderDirection::BUY ? "BUY" : "SELL")
                  << " → "
                  << (modified_request.offset == hft::plugin::OffsetFlag::OPEN ? "OPEN" : "CLOSE")
                  << " (was "
                  << (original_offset == hft::plugin::OffsetFlag::OPEN ? "OPEN" : "CLOSE")
                  << ")" << std::endl;
    }

    // Generate order ID first (even if order will be rejected)
    // 即使订单会被拒绝，也先生成订单ID（与CTP行为一致）
    std::string order_id = GenerateOrderID();

    // Check risk (使用修改后的 request)
    std::string error_msg;
    if (!CheckRisk(modified_request, &error_msg)) {
        std::cerr << "[SimulatorPlugin] ❌ Risk check failed: " << error_msg << std::endl;

        // Create REJECTED order (与CTP的OnRspOrderInsert行为一致)
        InternalOrder internal_order;
        internal_order.order_id = order_id;
        internal_order.client_order_id = modified_request.client_order_id;
        internal_order.request = modified_request;
        internal_order.status = hft::plugin::OrderStatus::REJECTED;  // 拒绝状态
        internal_order.traded_volume = 0;
        internal_order.insert_time = GetCurrentNanoTime();
        internal_order.update_time = internal_order.insert_time;
        std::strncpy(internal_order.status_msg, error_msg.c_str(),
                    sizeof(internal_order.status_msg) - 1);

        // Store rejected order
        {
            std::lock_guard<std::mutex> lock(m_order_mutex);
            m_orders[order_id] = internal_order;
        }

        // Increment order count
        m_order_count.fetch_add(1);

        // Notify order callback with REJECTED status (与CTP一致)
        if (m_order_callback) {
            m_order_callback(ConvertToOrderInfo(internal_order));
        }

        // Also notify error callback (与CTP一致)
        if (m_error_callback) {
            m_error_callback(-2, error_msg);
        }

        return order_id;  // 返回订单ID（即使被拒绝，与CTP一致）
    }

    // Create internal order (使用修改后的 request)
    InternalOrder internal_order;
    internal_order.order_id = order_id;
    internal_order.client_order_id = modified_request.client_order_id;
    internal_order.request = modified_request;  // 保存修改后的 request
    internal_order.status = hft::plugin::OrderStatus::SUBMITTING;
    internal_order.traded_volume = 0;
    internal_order.insert_time = GetCurrentNanoTime();
    internal_order.update_time = internal_order.insert_time;
    internal_order.status_msg[0] = '\0';  // Initialize empty

    // Store order
    {
        std::lock_guard<std::mutex> lock(m_order_mutex);
        m_orders[order_id] = internal_order;
    }

    // Increment order count
    m_order_count.fetch_add(1);

    std::cout << "[SimulatorPlugin] Order submitted: " << order_id
              << " | " << modified_request.symbol
              << " | " << (modified_request.direction == hft::plugin::OrderDirection::BUY ? "BUY" : "SELL")
              << " | " << modified_request.volume << "@" << modified_request.price << std::endl;

    // Notify order callback (SUBMITTING)
    if (m_order_callback) {
        m_order_callback(ConvertToOrderInfo(internal_order));
    }

    // Process order based on mode
    if (m_config.mode == "immediate") {
        // Launch async thread to process order (传递修改后的 request)
        std::thread([this, order_id, modified_request]() {
            ProcessOrderImmediate(order_id, modified_request);
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

// UpdatePosition - 支持锁仓模式（与真实 CTP 一致）
// CTP 真实交易所规则：
// 1. 允许同一合约同时持有多空仓位（锁仓）
// 2. 根据 offset 字段判断开仓还是平仓
// 3. offset=OPEN 开仓，offset=CLOSE 平仓
void SimulatorPlugin::UpdatePosition(const hft::plugin::TradeInfo& trade) {
    std::lock_guard<std::mutex> lock(m_position_mutex);

    uint32_t qty = trade.volume;
    double price = trade.price;

    // 根据 offset 判断是开仓还是平仓
    bool is_open = (trade.offset == hft::plugin::OffsetFlag::OPEN);

    if (is_open) {
        // ========== 开仓逻辑 ==========
        // 使用 symbol_direction 作为 key，支持锁仓
        std::string pos_key = std::string(trade.symbol) + "_" +
                             (trade.direction == hft::plugin::OrderDirection::BUY ? "LONG" : "SHORT");

        InternalPosition& pos = m_positions[pos_key];

        // 初始化持仓
        if (pos.volume == 0 && pos.total_volume_traded == 0) {
            pos.symbol = trade.symbol;
            pos.exchange = trade.exchange;
            pos.direction = trade.direction;
            pos.yesterday_volume = 0;
        }

        // 更新持仓和平均价
        double oldCost = pos.avg_price * pos.volume;
        pos.total_cost = oldCost + price * qty;
        pos.volume += qty;
        pos.today_volume += qty;
        pos.total_volume_traded += qty;
        pos.avg_price = pos.total_cost / pos.total_volume_traded;
        pos.margin = CalculateMargin(trade.symbol, price, pos.volume);

        std::string direction_str = (trade.direction == hft::plugin::OrderDirection::BUY) ? "多" : "空";
        std::cout << "[SimulatorPlugin] 开" << direction_str << ": " << qty << " @ " << price
                  << ", " << direction_str << "头均价 " << pos.avg_price
                  << ", 总持仓 " << pos.volume << std::endl;

    } else {
        // ========== 平仓逻辑 ==========
        // 平仓方向与开仓相反：买入平空，卖出平多
        hft::plugin::OrderDirection close_direction =
            (trade.direction == hft::plugin::OrderDirection::BUY)
                ? hft::plugin::OrderDirection::SELL   // 买入 → 平空头
                : hft::plugin::OrderDirection::BUY;   // 卖出 → 平多头

        std::string pos_key = std::string(trade.symbol) + "_" +
                             (close_direction == hft::plugin::OrderDirection::BUY ? "LONG" : "SHORT");

        auto it = m_positions.find(pos_key);
        if (it == m_positions.end() || it->second.volume == 0) {
            std::cerr << "[SimulatorPlugin] ⚠️ 平仓失败：无持仓 " << pos_key << std::endl;
            return;
        }

        InternalPosition& pos = it->second;

        // 根据 offset 类型确定平仓数量和来源
        // CLOSE_TODAY: 只能平今仓
        // CLOSE_YESTERDAY: 只能平昨仓
        // CLOSE: 优先平今，不足部分平昨
        uint32_t closedQty = 0;
        uint32_t close_today = 0;
        uint32_t close_yesterday = 0;
        std::string offset_str = "平仓";

        if (trade.offset == hft::plugin::OffsetFlag::CLOSE_TODAY) {
            // 平今仓
            closedQty = std::min(qty, pos.today_volume);
            close_today = closedQty;
            offset_str = "平今";
            if (closedQty < qty) {
                std::cerr << "[SimulatorPlugin] ⚠️ 平今仓不足：需要 " << qty
                          << "，今仓只有 " << pos.today_volume << std::endl;
            }
        } else if (trade.offset == hft::plugin::OffsetFlag::CLOSE_YESTERDAY) {
            // 平昨仓
            closedQty = std::min(qty, pos.yesterday_volume);
            close_yesterday = closedQty;
            offset_str = "平昨";
            if (closedQty < qty) {
                std::cerr << "[SimulatorPlugin] ⚠️ 平昨仓不足：需要 " << qty
                          << "，昨仓只有 " << pos.yesterday_volume << std::endl;
            }
        } else {
            // CLOSE: 优先平今，不足部分平昨
            closedQty = std::min(qty, pos.volume);
            if (pos.today_volume >= closedQty) {
                close_today = closedQty;
            } else {
                close_today = pos.today_volume;
                close_yesterday = closedQty - close_today;
            }
        }

        if (closedQty == 0) {
            std::cerr << "[SimulatorPlugin] ⚠️ 平仓失败：可平数量为0" << std::endl;
            return;
        }

        // 计算平仓盈亏
        double close_pnl = 0;
        if (close_direction == hft::plugin::OrderDirection::BUY) {
            // 平多：(卖出价 - 买入均价) × 数量
            close_pnl = (price - pos.avg_price) * closedQty;
        } else {
            // 平空：(卖出均价 - 买入价) × 数量
            close_pnl = (pos.avg_price - price) * closedQty;
        }

        {
            std::lock_guard<std::mutex> acc_lock(m_account_mutex);
            m_close_profit += close_pnl;
            m_daily_pnl += close_pnl;
        }

        // 减少持仓
        pos.volume -= closedQty;
        pos.today_volume -= close_today;
        pos.yesterday_volume -= close_yesterday;

        std::string direction_str = (close_direction == hft::plugin::OrderDirection::BUY) ? "多" : "空";
        std::cout << "[SimulatorPlugin] " << offset_str << direction_str << ": " << closedQty << " @ " << price
                  << " (今:" << close_today << ", 昨:" << close_yesterday << ")"
                  << ", " << direction_str << "头均价 " << pos.avg_price << ", 盈亏 " << close_pnl
                  << ", 剩余 " << pos.volume << "(今:" << pos.today_volume << ", 昨:" << pos.yesterday_volume << ")"
                  << std::endl;

        // 持仓归零，移除
        if (pos.volume == 0) {
            m_positions.erase(pos_key);
            std::cout << "[SimulatorPlugin] ✅ 持仓归零，移除: " << pos_key << std::endl;
        } else {
            pos.margin = CalculateMargin(trade.symbol, price, pos.volume);
        }
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
    // CTP 真实交易所允许锁仓（同一合约同时持有多空仓位）
    // 不再检查"不能开反向仓"的限制

    // Check max position per symbol (支持锁仓模式，使用 symbol_direction 作为 key)
    {
        std::lock_guard<std::mutex> lock(m_position_mutex);

        if (request.offset == hft::plugin::OffsetFlag::OPEN) {
            // 开仓时检查同方向持仓
            std::string pos_key = std::string(request.symbol) + "_" +
                                 (request.direction == hft::plugin::OrderDirection::BUY ? "LONG" : "SHORT");
            auto it = m_positions.find(pos_key);
            uint32_t current_volume = (it != m_positions.end()) ? it->second.volume : 0;

            if (current_volume + request.volume > static_cast<uint32_t>(m_config.max_position_per_symbol)) {
                if (error_msg) {
                    *error_msg = "Exceeds max position per symbol: " +
                                std::to_string(m_config.max_position_per_symbol);
                }
                return false;
            }
        } else {
            // 平仓时检查是否有对应持仓（支持今昨仓区分）
            hft::plugin::OrderDirection close_direction =
                (request.direction == hft::plugin::OrderDirection::BUY)
                    ? hft::plugin::OrderDirection::SELL   // 买入 → 平空头
                    : hft::plugin::OrderDirection::BUY;   // 卖出 → 平多头

            std::string pos_key = std::string(request.symbol) + "_" +
                                 (close_direction == hft::plugin::OrderDirection::BUY ? "LONG" : "SHORT");
            auto it = m_positions.find(pos_key);

            if (it == m_positions.end()) {
                if (error_msg) {
                    *error_msg = "No position to close for " + std::string(request.symbol);
                }
                return false;
            }

            const auto& pos = it->second;
            uint32_t available = 0;
            std::string close_type = "总持仓";

            // 根据 offset 类型检查对应的持仓
            if (request.offset == hft::plugin::OffsetFlag::CLOSE_TODAY) {
                available = pos.today_volume;
                close_type = "今仓";
            } else if (request.offset == hft::plugin::OffsetFlag::CLOSE_YESTERDAY) {
                available = pos.yesterday_volume;
                close_type = "昨仓";
            } else {
                // CLOSE: 检查总持仓
                available = pos.volume;
            }

            if (available < request.volume) {
                if (error_msg) {
                    *error_msg = "Insufficient " + close_type + " to close. Required: " +
                                std::to_string(request.volume) +
                                ", Available: " + std::to_string(available);
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

// 根据当前持仓自动设置订单的 offset 字段
// 与 CTP Plugin 的 SetOpenClose 逻辑一致
// 使用 symbol_LONG / symbol_SHORT 作为持仓 key（支持锁仓模式）
// 支持上期所今昨仓区分：CLOSE_TODAY / CLOSE_YESTERDAY
void SimulatorPlugin::SetOpenClose(hft::plugin::OrderRequest& request) {
    std::lock_guard<std::mutex> lock(m_position_mutex);

    // 查找多头和空头持仓
    std::string long_key = std::string(request.symbol) + "_LONG";
    std::string short_key = std::string(request.symbol) + "_SHORT";

    auto long_it = m_positions.find(long_key);
    auto short_it = m_positions.find(short_key);

    // 判断是否是上期所（需要区分今昨仓）
    bool is_shfe = (std::string(request.exchange) == "SHFE");

    if (request.direction == hft::plugin::OrderDirection::BUY) {
        // 买入：如果有空仓 → 平空，否则 → 开多
        if (short_it != m_positions.end() && short_it->second.volume > 0) {
            const auto& pos = short_it->second;
            if (is_shfe) {
                // 上期所：优先平今，不足则平昨
                if (pos.today_volume > 0) {
                    request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
                } else if (pos.yesterday_volume > 0) {
                    request.offset = hft::plugin::OffsetFlag::CLOSE_YESTERDAY;
                } else {
                    request.offset = hft::plugin::OffsetFlag::CLOSE;
                }
            } else {
                request.offset = hft::plugin::OffsetFlag::CLOSE;
            }
        } else {
            request.offset = hft::plugin::OffsetFlag::OPEN;
        }
    } else {
        // 卖出：如果有多仓 → 平多，否则 → 开空
        if (long_it != m_positions.end() && long_it->second.volume > 0) {
            const auto& pos = long_it->second;
            if (is_shfe) {
                // 上期所：优先平今，不足则平昨
                if (pos.today_volume > 0) {
                    request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
                } else if (pos.yesterday_volume > 0) {
                    request.offset = hft::plugin::OffsetFlag::CLOSE_YESTERDAY;
                } else {
                    request.offset = hft::plugin::OffsetFlag::CLOSE;
                }
            } else {
                request.offset = hft::plugin::OffsetFlag::CLOSE;
            }
        } else {
            request.offset = hft::plugin::OffsetFlag::OPEN;
        }
    }
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

    // Set status message
    // For REJECTED orders, use the custom error message from InternalOrder
    // Otherwise, use default status description
    if (order.status == hft::plugin::OrderStatus::REJECTED && order.status_msg[0] != '\0') {
        // Use custom error message from CheckRisk (与CTP的ErrorMsg一致)
        std::strncpy(order_info.status_msg, order.status_msg, sizeof(order_info.status_msg) - 1);
    } else {
        // Use default status description
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
    }

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

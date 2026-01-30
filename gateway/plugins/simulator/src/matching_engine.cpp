#include "../include/matching_engine.h"
#include <chrono>
#include <iostream>

namespace hft {
namespace plugin {
namespace simulator {

MatchingEngine::MatchingEngine() {
}

MatchingEngine::~MatchingEngine() {
}

void MatchingEngine::SetMatchCallback(MatchCallback callback) {
    m_match_callback = callback;
}

void MatchingEngine::AddOrder(const std::string& order_id,
                               const std::string& symbol,
                               double price,
                               uint32_t volume,
                               bool is_buy,
                               hft::plugin::PriceType price_type) {
    std::lock_guard<std::mutex> lock(m_mutex);

    // Create order book if not exists
    if (m_order_books.find(symbol) == m_order_books.end()) {
        m_order_books[symbol] = std::make_unique<OrderBook>(symbol);
    }

    // Store pending order
    PendingOrder pending;
    pending.order_id = order_id;
    pending.symbol = symbol;
    pending.price = price;
    pending.volume = volume;
    pending.is_buy = is_buy;
    pending.price_type = price_type;
    pending.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()
    ).count();
    m_pending_orders[order_id] = pending;

    // Try immediate match if we have market data
    auto md_it = m_market_data.find(symbol);
    if (md_it != m_market_data.end()) {
        MatchResult result = TryMatchImmediate(order_id, symbol, price, volume, is_buy, price_type);

        if (result.matched && m_match_callback) {
            m_match_callback(order_id, result);
            m_pending_orders.erase(order_id);
            return;
        }
    }

    // If not matched immediately, add to order book (for limit orders)
    if (price_type == hft::plugin::PriceType::LIMIT) {
        m_order_books[symbol]->AddOrder(order_id, price, volume, is_buy);
    }
}

bool MatchingEngine::CancelOrder(const std::string& order_id) {
    std::lock_guard<std::mutex> lock(m_mutex);

    auto it = m_pending_orders.find(order_id);
    if (it == m_pending_orders.end()) {
        return false;
    }

    const PendingOrder& order = it->second;
    const std::string& symbol = order.symbol;

    // Remove from order book
    auto book_it = m_order_books.find(symbol);
    if (book_it != m_order_books.end()) {
        book_it->second->RemoveOrder(order_id);
    }

    // Remove from pending orders
    m_pending_orders.erase(it);
    return true;
}

void MatchingEngine::OnMarketData(const std::string& symbol,
                                  double bid_price,
                                  double ask_price,
                                  double last_price) {
    std::lock_guard<std::mutex> lock(m_mutex);

    // Update market data
    MarketData& md = m_market_data[symbol];
    md.bid_price = bid_price;
    md.ask_price = ask_price;
    md.last_price = last_price;
    md.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()
    ).count();

    // Try to match pending orders
    TryMatchLimitOrders(symbol, bid_price, ask_price);
}

MatchResult MatchingEngine::TryMatchImmediate(const std::string& order_id,
                                              const std::string& symbol,
                                              double price,
                                              uint32_t volume,
                                              bool is_buy,
                                              hft::plugin::PriceType price_type) {
    MatchResult result;
    result.matched = false;
    result.fill_volume = 0;
    result.fill_price = 0.0;

    auto md_it = m_market_data.find(symbol);
    if (md_it == m_market_data.end()) {
        return result;  // No market data yet
    }

    const MarketData& md = md_it->second;

    // Market order - match immediately at opposite side best price
    if (price_type == hft::plugin::PriceType::MARKET) {
        result.matched = true;
        result.fill_volume = volume;
        result.fill_price = is_buy ? md.ask_price : md.bid_price;
        result.match_type = "market";
        return result;
    }

    // Limit order - check if price crosses spread
    if (price_type == hft::plugin::PriceType::LIMIT) {
        if (is_buy && price >= md.ask_price) {
            // Buy limit order at or above ask - execute at ask
            result.matched = true;
            result.fill_volume = volume;
            result.fill_price = md.ask_price;
            result.match_type = "limit_aggressive";
            return result;
        } else if (!is_buy && price <= md.bid_price) {
            // Sell limit order at or below bid - execute at bid
            result.matched = true;
            result.fill_volume = volume;
            result.fill_price = md.bid_price;
            result.match_type = "limit_aggressive";
            return result;
        }
    }

    return result;  // No match
}

void MatchingEngine::TryMatchLimitOrders(const std::string& symbol,
                                         double bid_price,
                                         double ask_price) {
    auto book_it = m_order_books.find(symbol);
    if (book_it == m_order_books.end()) {
        return;  // No order book for this symbol
    }

    OrderBook* book = book_it->second.get();

    // Check all pending orders for this symbol
    std::vector<std::string> matched_orders;

    for (auto& [oid, order] : m_pending_orders) {
        if (order.symbol != symbol) {
            continue;
        }

        if (order.price_type != hft::plugin::PriceType::LIMIT) {
            continue;
        }

        bool should_match = false;
        double fill_price = 0.0;

        if (order.is_buy && order.price >= ask_price) {
            // Buy limit order can be filled at ask
            should_match = true;
            fill_price = ask_price;
        } else if (!order.is_buy && order.price <= bid_price) {
            // Sell limit order can be filled at bid
            should_match = true;
            fill_price = bid_price;
        }

        if (should_match && m_match_callback) {
            MatchResult result;
            result.matched = true;
            result.fill_volume = order.volume;
            result.fill_price = fill_price;
            result.match_type = "limit_triggered";

            m_match_callback(oid, result);
            matched_orders.push_back(oid);

            // Remove from order book
            book->RemoveOrder(oid);
        }
    }

    // Remove matched orders from pending
    for (const auto& oid : matched_orders) {
        m_pending_orders.erase(oid);
    }
}

MatchResult MatchingEngine::MatchAgainstMarket(const std::string& order_id,
                                               double market_bid,
                                               double market_ask) {
    MatchResult result;
    result.matched = false;

    auto it = m_pending_orders.find(order_id);
    if (it == m_pending_orders.end()) {
        return result;
    }

    const PendingOrder& order = it->second;

    if (order.is_buy && order.price >= market_ask) {
        result.matched = true;
        result.fill_volume = order.volume;
        result.fill_price = market_ask;
        result.match_type = "limit_match";
    } else if (!order.is_buy && order.price <= market_bid) {
        result.matched = true;
        result.fill_volume = order.volume;
        result.fill_price = market_bid;
        result.match_type = "limit_match";
    }

    return result;
}

OrderBook* MatchingEngine::GetOrderBook(const std::string& symbol) {
    std::lock_guard<std::mutex> lock(m_mutex);

    auto it = m_order_books.find(symbol);
    if (it == m_order_books.end()) {
        m_order_books[symbol] = std::make_unique<OrderBook>(symbol);
        return m_order_books[symbol].get();
    }

    return it->second.get();
}

std::vector<std::string> MatchingEngine::GetSymbols() const {
    std::lock_guard<std::mutex> lock(m_mutex);

    std::vector<std::string> symbols;
    for (const auto& [symbol, book] : m_order_books) {
        symbols.push_back(symbol);
    }
    return symbols;
}

void MatchingEngine::Clear() {
    std::lock_guard<std::mutex> lock(m_mutex);
    m_order_books.clear();
    m_pending_orders.clear();
    m_market_data.clear();
}

} // namespace simulator
} // namespace plugin
} // namespace hft

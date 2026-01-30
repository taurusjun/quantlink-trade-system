#include "../include/order_book.h"
#include <algorithm>
#include <chrono>
#include <iostream>

namespace hft {
namespace plugin {
namespace simulator {

OrderBook::OrderBook(const std::string& symbol)
    : m_symbol(symbol) {
}

OrderBook::~OrderBook() {
}

void OrderBook::AddOrder(const std::string& order_id, double price, uint32_t volume, bool is_buy) {
    std::lock_guard<std::mutex> lock(m_mutex);

    // Store order info
    OrderInfo info;
    info.price = price;
    info.volume = volume;
    info.is_buy = is_buy;
    m_order_map[order_id] = info;

    // Add to appropriate side
    if (is_buy) {
        auto& level = m_bids[price];
        level.price = price;
        level.volume += volume;
        level.order_ids.push_back(order_id);
    } else {
        auto& level = m_asks[price];
        level.price = price;
        level.volume += volume;
        level.order_ids.push_back(order_id);
    }
}

bool OrderBook::RemoveOrder(const std::string& order_id) {
    std::lock_guard<std::mutex> lock(m_mutex);

    auto it = m_order_map.find(order_id);
    if (it == m_order_map.end()) {
        return false;
    }

    const OrderInfo& info = it->second;
    double price = info.price;
    uint32_t volume = info.volume;
    bool is_buy = info.is_buy;

    // Remove from price level
    if (is_buy) {
        auto level_it = m_bids.find(price);
        if (level_it != m_bids.end()) {
            auto& level = level_it->second;
            level.volume -= volume;

            // Remove order_id from list
            auto& order_ids = level.order_ids;
            order_ids.erase(std::remove(order_ids.begin(), order_ids.end(), order_id), order_ids.end());

            // Remove price level if empty
            if (level.volume == 0) {
                m_bids.erase(level_it);
            }
        }
    } else {
        auto level_it = m_asks.find(price);
        if (level_it != m_asks.end()) {
            auto& level = level_it->second;
            level.volume -= volume;

            // Remove order_id from list
            auto& order_ids = level.order_ids;
            order_ids.erase(std::remove(order_ids.begin(), order_ids.end(), order_id), order_ids.end());

            // Remove price level if empty
            if (level.volume == 0) {
                m_asks.erase(level_it);
            }
        }
    }

    // Remove from order map
    m_order_map.erase(it);
    return true;
}

double OrderBook::GetBestBid() const {
    std::lock_guard<std::mutex> lock(m_mutex);
    if (m_bids.empty()) {
        return 0.0;
    }
    return m_bids.begin()->first;
}

double OrderBook::GetBestAsk() const {
    std::lock_guard<std::mutex> lock(m_mutex);
    if (m_asks.empty()) {
        return 0.0;
    }
    return m_asks.begin()->first;
}

double OrderBook::GetMidPrice() const {
    std::lock_guard<std::mutex> lock(m_mutex);
    if (m_bids.empty() || m_asks.empty()) {
        return 0.0;
    }
    double best_bid = m_bids.begin()->first;
    double best_ask = m_asks.begin()->first;
    return (best_bid + best_ask) / 2.0;
}

uint32_t OrderBook::GetVolumeAtPrice(double price, bool is_buy) const {
    std::lock_guard<std::mutex> lock(m_mutex);

    if (is_buy) {
        auto it = m_bids.find(price);
        return (it != m_bids.end()) ? it->second.volume : 0;
    } else {
        auto it = m_asks.find(price);
        return (it != m_asks.end()) ? it->second.volume : 0;
    }
}

bool OrderBook::CanMatch(double price, bool is_buy) const {
    std::lock_guard<std::mutex> lock(m_mutex);

    if (is_buy) {
        // Buy order can match if there's an ask <= price
        if (m_asks.empty()) return false;
        return m_asks.begin()->first <= price;
    } else {
        // Sell order can match if there's a bid >= price
        if (m_bids.empty()) return false;
        return m_bids.begin()->first >= price;
    }
}

uint32_t OrderBook::GetMatchedVolume(double price, bool is_buy) const {
    std::lock_guard<std::mutex> lock(m_mutex);

    uint32_t total_volume = 0;

    if (is_buy) {
        // Buy order matches with asks at price <= order price
        for (const auto& [ask_price, level] : m_asks) {
            if (ask_price <= price) {
                total_volume += level.volume;
            } else {
                break;  // Asks are sorted ascending
            }
        }
    } else {
        // Sell order matches with bids at price >= order price
        for (const auto& [bid_price, level] : m_bids) {
            if (bid_price >= price) {
                total_volume += level.volume;
            } else {
                break;  // Bids are sorted descending
            }
        }
    }

    return total_volume;
}

OrderBookSnapshot OrderBook::GetSnapshot(int depth) const {
    std::lock_guard<std::mutex> lock(m_mutex);

    OrderBookSnapshot snapshot;
    snapshot.symbol = m_symbol;
    snapshot.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()
    ).count();

    // Get top N bids
    int count = 0;
    for (const auto& [price, level] : m_bids) {
        if (count >= depth) break;
        snapshot.bids.push_back(level);
        count++;
    }

    // Get top N asks
    count = 0;
    for (const auto& [price, level] : m_asks) {
        if (count >= depth) break;
        snapshot.asks.push_back(level);
        count++;
    }

    return snapshot;
}

void OrderBook::Clear() {
    std::lock_guard<std::mutex> lock(m_mutex);
    m_bids.clear();
    m_asks.clear();
    m_order_map.clear();
}

} // namespace simulator
} // namespace plugin
} // namespace hft

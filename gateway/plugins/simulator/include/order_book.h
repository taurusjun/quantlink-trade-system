#pragma once

#include <map>
#include <vector>
#include <mutex>
#include <string>

namespace hft {
namespace plugin {
namespace simulator {

// Price level in order book
struct PriceLevel {
    double price;
    uint32_t volume;
    std::vector<std::string> order_ids;  // Orders at this price level

    PriceLevel() : price(0.0), volume(0) {}
    PriceLevel(double p, uint32_t v) : price(p), volume(v) {}
};

// Order book snapshot for API queries
struct OrderBookSnapshot {
    std::string symbol;
    std::vector<PriceLevel> bids;  // Sorted by price descending
    std::vector<PriceLevel> asks;  // Sorted by price ascending
    uint64_t timestamp;
};

// Order book - maintains buy and sell orders for a symbol
class OrderBook {
public:
    OrderBook(const std::string& symbol);
    ~OrderBook();

    // Add order to book
    void AddOrder(const std::string& order_id, double price, uint32_t volume, bool is_buy);

    // Remove order from book
    bool RemoveOrder(const std::string& order_id);

    // Get best bid price (highest buy price)
    double GetBestBid() const;

    // Get best ask price (lowest sell price)
    double GetBestAsk() const;

    // Get mid price
    double GetMidPrice() const;

    // Get total volume at price level
    uint32_t GetVolumeAtPrice(double price, bool is_buy) const;

    // Check if order can be matched
    bool CanMatch(double price, bool is_buy) const;

    // Get matched volume at price
    uint32_t GetMatchedVolume(double price, bool is_buy) const;

    // Get order book snapshot (for API)
    OrderBookSnapshot GetSnapshot(int depth = 5) const;

    // Clear all orders
    void Clear();

    std::string GetSymbol() const { return m_symbol; }

private:
    std::string m_symbol;

    // Buy orders: price -> {volume, order_ids}
    // Using map with greater<double> for descending order (highest bid first)
    std::map<double, PriceLevel, std::greater<double>> m_bids;

    // Sell orders: price -> {volume, order_ids}
    // Using map with less<double> for ascending order (lowest ask first)
    std::map<double, PriceLevel> m_asks;

    // Order ID -> {price, volume, is_buy}
    struct OrderInfo {
        double price;
        uint32_t volume;
        bool is_buy;
    };
    std::map<std::string, OrderInfo> m_order_map;

    mutable std::mutex m_mutex;
};

} // namespace simulator
} // namespace plugin
} // namespace hft

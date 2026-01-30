#pragma once

#include "order_book.h"
#include "../../include/plugin/td_plugin_interface.h"
#include <map>
#include <memory>
#include <functional>
#include <mutex>

namespace hft {
namespace plugin {
namespace simulator {

// Match result
struct MatchResult {
    bool matched;
    double fill_price;
    uint32_t fill_volume;
    std::string match_type;  // "immediate", "limit", "market"
};

// Matching engine - handles order matching logic
class MatchingEngine {
public:
    using MatchCallback = std::function<void(const std::string& order_id, const MatchResult& result)>;

    MatchingEngine();
    ~MatchingEngine();

    // Initialize with callback
    void SetMatchCallback(MatchCallback callback);

    // Add order to engine
    void AddOrder(const std::string& order_id,
                   const std::string& symbol,
                   double price,
                   uint32_t volume,
                   bool is_buy,
                   hft::plugin::PriceType price_type);

    // Cancel order
    bool CancelOrder(const std::string& order_id);

    // Process market data tick - triggers matching
    void OnMarketData(const std::string& symbol,
                      double bid_price,
                      double ask_price,
                      double last_price);

    // Get order book for symbol
    OrderBook* GetOrderBook(const std::string& symbol);

    // Get all symbols
    std::vector<std::string> GetSymbols() const;

    // Clear all order books
    void Clear();

private:
    // Try to match order immediately
    MatchResult TryMatchImmediate(const std::string& order_id,
                                  const std::string& symbol,
                                  double price,
                                  uint32_t volume,
                                  bool is_buy,
                                  hft::plugin::PriceType price_type);

    // Try to match limit orders in order book
    void TryMatchLimitOrders(const std::string& symbol,
                             double bid_price,
                             double ask_price);

    // Match a specific order against market
    MatchResult MatchAgainstMarket(const std::string& order_id,
                                   double market_bid,
                                   double market_ask);

    struct PendingOrder {
        std::string order_id;
        std::string symbol;
        double price;
        uint32_t volume;
        bool is_buy;
        hft::plugin::PriceType price_type;
        uint64_t timestamp;
    };

    // Order books per symbol
    std::map<std::string, std::unique_ptr<OrderBook>> m_order_books;

    // Pending orders waiting for match
    std::map<std::string, PendingOrder> m_pending_orders;

    // Latest market data per symbol
    struct MarketData {
        double bid_price;
        double ask_price;
        double last_price;
        uint64_t timestamp;
    };
    std::map<std::string, MarketData> m_market_data;

    // Callback for match results
    MatchCallback m_match_callback;

    mutable std::mutex m_mutex;
};

} // namespace simulator
} // namespace plugin
} // namespace hft

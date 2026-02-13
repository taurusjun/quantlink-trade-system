// Counter Bridge - hftbase MWMR 兼容订单路由网关
// 功能：从 SysV MWMR 共享内存读取 RequestMsg，路由到各券商插件（CTP/Simulator 等），
//       接收回报写回 SysV MWMR ResponseMsg
//
// 架构（改造后）：
//   Go trader (tbsrc-golang) → [SysV MWMR SHM] → Counter Bridge → [ITDPlugin] → CTP/Simulator
//                            ← [SysV MWMR SHM] ←                   ↓
//                                                              CTP / Simulator
//
// 改造要点：
//   1. POSIX SPSC + OrderRequestRaw/OrderResponseRaw → SysV MWMR + RequestMsg/ResponseMsg
//   2. 新增 SetCombOffsetFlag（开平自动推断，与原 ORS 一致）
//   3. 新增 mapContractPos（持仓跟踪，与原 ORS 一致）
//   4. 删除 HTTP /positions 端点（原 C++ 系统不存在，Go 通过 TRADE_CONFIRM 累计跟踪持仓）
//
// C++ 参考:
//   - ors/China/src/ORSServer.cpp:488-605   (SetCombOffsetFlag)
//   - ors/China/src/ORSServer.cpp:1186-1281 (updatePosition)
//   - ors/Shengli/include/ORSServer.h:422-431 (mapContractPos)

#include <iostream>
#include <memory>
#include <thread>
#include <atomic>
#include <signal.h>
#include <cstring>
#include <map>
#include <mutex>
#include <vector>
#include <sstream>
#include <fstream>
#include <chrono>

#include "plugin/td_plugin_interface.h"
#include "hftbase_shm.h"
#include "hftbase_types.h"
#include "../third_party/httplib.h"

// CTP plugin (if enabled)
#if defined(ENABLE_CTP_PLUGIN)
#include "../plugins/ctp/include/ctp_td_plugin.h"
#endif

// Simulator plugin (if enabled)
#if defined(ENABLE_SIMULATOR_PLUGIN)
#include "../plugins/simulator/include/simulator_plugin.h"
#endif

using namespace hft::plugin;
using namespace hftbase_compat;
using namespace illuminati::infra;

using ReqQueue  = MWMRQueue<RequestMsg>;
using RespQueue = MWMRQueue<ResponseMsg>;

// ============================================================
// Position tracking structures
// C++ source: ors/Shengli/include/ORSServer.h:102-108
// ============================================================
struct contractPos {
    int ONLongPos      = 0;   // overnight long position
    int todayLongPos   = 0;   // today long position
    int ONShortPos     = 0;   // overnight short position
    int todayShortPos  = 0;   // today short position
};

static std::map<std::string, contractPos> g_mapContractPos;
static std::mutex g_posLock;

// Open/close flag constants
// C++ source: ors/China/src/ORSServer.cpp:28-30
static const int OPEN_ORDER       = 3;
static const int CLOSE_TODAY_FLAG = 1;
static const int CLOSE_YESTD_FLAG = 2;

// ============================================================
// Order cache (augmented with hftbase fields)
// ============================================================
struct CachedOrderInfo {
    uint32_t order_id;         // hftbase uint32 OrderID
    int strategy_id;           // int StrategyID
    std::string symbol;
    std::string exchange;
    unsigned char side;        // 'B'/'S'
    std::string client_order_id; // ITDPlugin string order ID
    int openCloseFlag;         // OPEN_ORDER / CLOSE_TODAY_FLAG / CLOSE_YESTD_FLAG
};

// ============================================================
// SHM configuration
// ============================================================
struct SHMConfig {
    int request_key      = 0x0F20; // 3872
    int request_size     = 4096;
    int response_key     = 0x1308; // 4872
    int response_size    = 4096;
    int client_store_key = 0x16F0; // 5872
};

// ============================================================
// Global variables
// ============================================================
static std::atomic<bool> g_running{true};
static RespQueue* g_response_queue = nullptr;
static std::map<std::string, CachedOrderInfo> g_order_map;  // broker_order_id -> order info
static std::mutex g_orders_mutex;

// Broker plugin registry
static std::map<std::string, std::unique_ptr<ITDPlugin>> g_brokers;
static std::map<std::string, std::string> g_symbol_to_broker;

// Statistics
struct Statistics {
    std::atomic<uint64_t> total_orders{0};
    std::atomic<uint64_t> success_orders{0};
    std::atomic<uint64_t> failed_orders{0};
    std::atomic<uint64_t> filled_orders{0};
    std::atomic<uint64_t> rejected_orders{0};

    void Print() const {
        std::cout << "[Statistics] Total=" << total_orders
                  << " Success=" << success_orders
                  << " Failed=" << failed_orders
                  << " Filled=" << filled_orders
                  << " Rejected=" << rejected_orders << std::endl;
    }
};
static Statistics g_stats;

// ============================================================
// Signal handler
// ============================================================
void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << ", shutting down..." << std::endl;
    g_running = false;
}

void PrintBanner() {
    std::cout << R"(
╔═══════════════════════════════════════════════════════════╗
║     Counter Bridge - MWMR Multi-Broker Gateway            ║
║   SysV MWMR SHM → Broker Plugins → Exchange              ║
╚═══════════════════════════════════════════════════════════╝
)" << std::endl;
}

// ============================================================
// SetCombOffsetFlag — auto-determine open/close direction
// C++ source: ors/China/src/ORSServer.cpp:488-605
// C++ source: ors/Shengli/src/ORSServer.cpp:672-779
//
// Logic: for each order, check if we can close existing positions first.
// SHFE/INE: distinguish close_today vs close_yesterday.
// Other exchanges: all closings use CLOSE_YESTD_FLAG.
// If no positions to close, open new position.
// ============================================================
void SetCombOffsetFlag(
    const RequestMsg* request,
    int& openCloseFlag,
    unsigned char exchangeType)
{
    std::string symbol(request->Contract_Description.Symbol);
    bool isSHFE = (exchangeType == CHINA_SHFE);
    // INE also distinguishes close_today/close_yesterday (same rules as SHFE)
    // For now, only SHFE implemented. INE can be added if needed.

    std::lock_guard<std::mutex> lock(g_posLock);
    auto& pos = g_mapContractPos[symbol];

    if (request->Transaction_Type == SIDE_BUY) {
        // Buy → try to close short positions first

        // 1. Close today short positions first (SHFE uses CLOSE_TODAY)
        if (request->Quantity <= pos.todayShortPos) {
            openCloseFlag = isSHFE ? CLOSE_TODAY_FLAG : CLOSE_YESTD_FLAG;
            pos.todayShortPos -= request->Quantity;
            return;
        }

        // 2. Close overnight short positions
        if (request->Quantity <= pos.ONShortPos) {
            openCloseFlag = CLOSE_YESTD_FLAG;
            pos.ONShortPos -= request->Quantity;
            return;
        }

        // 3. Open new long position
        openCloseFlag = OPEN_ORDER;

    } else {
        // Sell → try to close long positions first

        // 1. Close today long positions first
        if (request->Quantity <= pos.todayLongPos) {
            openCloseFlag = isSHFE ? CLOSE_TODAY_FLAG : CLOSE_YESTD_FLAG;
            pos.todayLongPos -= request->Quantity;
            return;
        }

        // 2. Close overnight long positions
        if (request->Quantity <= pos.ONLongPos) {
            openCloseFlag = CLOSE_YESTD_FLAG;
            pos.ONLongPos -= request->Quantity;
            return;
        }

        // 3. Open new short position
        openCloseFlag = OPEN_ORDER;
    }
}

// ============================================================
// updatePosition — update position tracking on trade/reject/cancel
// C++ source: ors/China/src/ORSServer.cpp:1186-1281
// C++ source: ors/Shengli/src/ORSServer.cpp:1637-1736
// ============================================================
void updatePosition(
    const ResponseMsg* resp,
    const CachedOrderInfo& info)
{
    std::lock_guard<std::mutex> lock(g_posLock);
    auto& pos = g_mapContractPos[info.symbol];

    if (resp->Response_Type == TRADE_CONFIRM) {
        // Trade: add positions for open orders
        if (info.openCloseFlag == OPEN_ORDER) {
            if (resp->Side == SIDE_BUY) {
                pos.todayLongPos += resp->Quantity;
            } else {
                pos.todayShortPos += resp->Quantity;
            }
        }
        // Close: positions already deducted in SetCombOffsetFlag

    } else if (resp->Response_Type == ORDER_ERROR ||
               resp->Response_Type == ORS_REJECT ||
               resp->Response_Type == RMS_REJECT ||
               resp->Response_Type == CANCEL_ORDER_CONFIRM) {
        // Reject/Cancel: unfreeze positions (reverse the deduction)
        int qty = resp->Quantity;  // unfilled quantity

        if (info.openCloseFlag == CLOSE_TODAY_FLAG) {
            if (info.side == SIDE_BUY) {
                pos.todayShortPos += qty;
            } else {
                pos.todayLongPos += qty;
            }
        } else if (info.openCloseFlag == CLOSE_YESTD_FLAG) {
            if (info.side == SIDE_BUY) {
                pos.ONShortPos += qty;
            } else {
                pos.ONLongPos += qty;
            }
        }
        // OPEN_ORDER reject/cancel: no position to unfreeze
    }
}

// ============================================================
// Load position file (optional)
// Format: symbol,ONLong,todayLong,ONShort,todayShort
// Example: ag2506,0,3,0,5
// ============================================================
void loadPositionFile(const std::string& filename) {
    if (filename.empty()) return;
    std::ifstream file(filename);
    if (!file.is_open()) {
        std::cerr << "[Position] Cannot open: " << filename << std::endl;
        return;
    }
    std::string line;
    while (std::getline(file, line)) {
        if (line.empty() || line[0] == '#') continue;
        std::istringstream ss(line);
        std::string symbol;
        int onLong = 0, todayLong = 0, onShort = 0, todayShort = 0;
        if (std::getline(ss, symbol, ',')) {
            ss >> onLong;
            ss.ignore(1);
            ss >> todayLong;
            ss.ignore(1);
            ss >> onShort;
            ss.ignore(1);
            ss >> todayShort;

            contractPos& pos = g_mapContractPos[symbol];
            pos.ONLongPos = onLong;
            pos.todayLongPos = todayLong;
            pos.ONShortPos = onShort;
            pos.todayShortPos = todayShort;
        }
    }
    std::cout << "[Position] Loaded " << g_mapContractPos.size()
              << " positions from " << filename << std::endl;
}

// ============================================================
// Get broker plugin for symbol
// ============================================================
ITDPlugin* GetBrokerForSymbol(const std::string& symbol) {
    // 1. Exact match
    auto it = g_symbol_to_broker.find(symbol);
    if (it != g_symbol_to_broker.end()) {
        auto broker_it = g_brokers.find(it->second);
        if (broker_it != g_brokers.end()) {
            return broker_it->second.get();
        }
    }

    // 2. Default: first logged-in broker
    for (auto& [name, broker] : g_brokers) {
        if (broker && broker->IsLoggedIn()) {
            return broker.get();
        }
    }

    return nullptr;
}

// ============================================================
// HTTP server (kept for health check and simulator stats only)
// /positions endpoint REMOVED — Go tracks positions via TRADE_CONFIRM
// ============================================================
std::unique_ptr<httplib::Server> g_http_server;
std::thread g_http_thread;

std::string JsonEscape(const std::string& str) {
    std::string escaped;
    for (char c : str) {
        switch (c) {
            case '"':  escaped += "\\\""; break;
            case '\\': escaped += "\\\\"; break;
            case '\b': escaped += "\\b";  break;
            case '\f': escaped += "\\f";  break;
            case '\n': escaped += "\\n";  break;
            case '\r': escaped += "\\r";  break;
            case '\t': escaped += "\\t";  break;
            default:   escaped += c;      break;
        }
    }
    return escaped;
}

ITDPlugin* GetSimulatorPlugin() {
    auto it = g_brokers.find("simulator");
    if (it != g_brokers.end()) {
        return it->second.get();
    }
    return nullptr;
}

void HandleSimulatorStats(const httplib::Request& /*req*/, httplib::Response& res) {
    auto* sim = GetSimulatorPlugin();
    if (!sim) {
        res.set_content("{\"success\":false,\"error\":\"Simulator not found\"}", "application/json");
        return;
    }

    std::ostringstream json;
    json << "{\n";
    json << "  \"success\": true,\n";
    json << "  \"plugin_name\": \"" << sim->GetPluginName() << "\",\n";
    json << "  \"plugin_version\": \"" << sim->GetPluginVersion() << "\",\n";
    json << "  \"order_count\": " << sim->GetOrderCount() << ",\n";
    json << "  \"trade_count\": " << sim->GetTradeCount() << ",\n";
    json << "  \"is_connected\": " << (sim->IsConnected() ? "true" : "false") << ",\n";
    json << "  \"is_logged_in\": " << (sim->IsLoggedIn() ? "true" : "false") << "\n";
    json << "}\n";

    res.set_content(json.str(), "application/json");
}

void HandleSimulatorAccount(const httplib::Request& /*req*/, httplib::Response& res) {
    auto* sim = GetSimulatorPlugin();
    if (!sim) {
        res.set_content("{\"success\":false,\"error\":\"Simulator not found\"}", "application/json");
        return;
    }

    hft::plugin::AccountInfo account;
    if (!sim->QueryAccount(account)) {
        res.set_content("{\"success\":false,\"error\":\"Failed to query account\"}", "application/json");
        return;
    }

    std::ostringstream json;
    json << "{\n";
    json << "  \"success\": true,\n";
    json << "  \"account_id\": \"" << JsonEscape(account.account_id) << "\",\n";
    json << "  \"balance\": " << account.balance << ",\n";
    json << "  \"available\": " << account.available << ",\n";
    json << "  \"margin\": " << account.margin << ",\n";
    json << "  \"frozen_margin\": " << account.frozen_margin << ",\n";
    json << "  \"commission\": " << account.commission << ",\n";
    json << "  \"close_profit\": " << account.close_profit << ",\n";
    json << "  \"position_profit\": " << account.position_profit << "\n";
    json << "}\n";

    res.set_content(json.str(), "application/json");
}

void StartHTTPServer(int port = 8080) {
    g_http_server = std::make_unique<httplib::Server>();

    // Simulator endpoints (kept)
    g_http_server->Get("/simulator/stats", HandleSimulatorStats);
    g_http_server->Get("/simulator/account", HandleSimulatorAccount);

    // Health check endpoint (kept)
    g_http_server->Get("/health", [](const httplib::Request&, httplib::Response& res) {
        res.set_content("{\"status\":\"ok\",\"mode\":\"mwmr\"}", "application/json");
    });

    // /positions endpoint: REMOVED
    // C++ original system does not have this.
    // Go tracks positions via TRADE_CONFIRM in MWMR response queue.

    std::cout << "[HTTP] Starting HTTP server on port " << port << "..." << std::endl;

    g_http_thread = std::thread([port]() {
        if (!g_http_server->listen("0.0.0.0", port)) {
            std::cerr << "[HTTP] Failed to start HTTP server on port " << port << std::endl;
        }
    });

    std::cout << "[HTTP] HTTP server started on port " << port << std::endl;
}

void StopHTTPServer() {
    if (g_http_server) {
        std::cout << "[HTTP] Stopping HTTP server..." << std::endl;
        g_http_server->stop();
    }
    if (g_http_thread.joinable()) {
        g_http_thread.join();
    }
    std::cout << "[HTTP] HTTP server stopped" << std::endl;
}

// ============================================================
// Broker order callback — convert ITDPlugin OrderInfo → hftbase ResponseMsg
// C++ reference: ExecutionStrategy.cpp ORS callback state machine
// ============================================================
void OnBrokerOrderCallback(const hft::plugin::OrderInfo& order_info) {
    if (!g_response_queue) return;

    ResponseMsg resp;
    std::memset(&resp, 0, sizeof(resp));

    // Look up cached order info
    CachedOrderInfo cached_info;
    {
        std::lock_guard<std::mutex> lock(g_orders_mutex);
        auto it = g_order_map.find(order_info.order_id);
        if (it != g_order_map.end()) {
            cached_info = it->second;
        } else {
            std::cerr << "[Bridge] Order not in cache: " << order_info.order_id << std::endl;
            return;
        }
    }

    // Fill ResponseMsg fields
    resp.OrderID = cached_info.order_id;         // uint32 hftbase OrderID
    resp.StrategyID = cached_info.strategy_id;   // int StrategyID
    resp.Side = cached_info.side;                // 'B' or 'S'
    std::strncpy(resp.Symbol, cached_info.symbol.c_str(), sizeof(resp.Symbol) - 1);

    // Status mapping: plugin::OrderStatus → hftbase ResponseType
    switch (order_info.status) {
        case hft::plugin::OrderStatus::ACCEPTED:
        case hft::plugin::OrderStatus::SUBMITTED:
            resp.Response_Type = NEW_ORDER_CONFIRM;
            break;

        case hft::plugin::OrderStatus::PARTIAL_FILLED:
        case hft::plugin::OrderStatus::FILLED:
            resp.Response_Type = TRADE_CONFIRM;
            resp.Quantity = order_info.traded_volume;
            resp.Price = order_info.price;
            if (order_info.status == hft::plugin::OrderStatus::FILLED) {
                g_stats.filled_orders++;
            }
            break;

        case hft::plugin::OrderStatus::CANCELED:
            resp.Response_Type = CANCEL_ORDER_CONFIRM;
            resp.Quantity = order_info.volume - order_info.traded_volume; // unfilled qty
            break;

        case hft::plugin::OrderStatus::REJECTED:
        case hft::plugin::OrderStatus::ERROR:
            resp.Response_Type = ORDER_ERROR;
            resp.ErrorCode = 1;
            resp.Quantity = order_info.volume;
            g_stats.rejected_orders++;
            break;

        default:
            resp.Response_Type = ORDER_ERROR;
            break;
    }

    resp.TimeStamp = order_info.update_time;

    // Update position tracking
    updatePosition(&resp, cached_info);

    // Write to MWMR response queue
    g_response_queue->enqueue(resp);

    std::cout << "[Bridge] Response: OID=" << resp.OrderID
              << " type=" << resp.Response_Type
              << " qty=" << resp.Quantity
              << " price=" << resp.Price << std::endl;
}

// Broker trade callback
void OnBrokerTradeCallback(const hft::plugin::TradeInfo& trade_info) {
    std::cout << "[Bridge] Trade: " << trade_info.order_id
              << " price=" << trade_info.price
              << " volume=" << trade_info.volume << std::endl;
}

// Broker error callback
void OnBrokerErrorCallback(int error_id, const std::string& error_msg) {
    std::cerr << "[Bridge] Broker Error: [" << error_id << "] " << error_msg << std::endl;
}

// ============================================================
// Order request processor — reads RequestMsg from MWMR queue
// ============================================================
void OrderRequestProcessor(ReqQueue* req_queue) {
    std::cout << "[Processor] Order request processor started (MWMR mode)" << std::endl;

    RequestMsg req;

    while (g_running.load()) {
        if (!req_queue->isEmpty()) {
            req_queue->dequeuePtr(&req);
            g_stats.total_orders++;

            // Extract symbol
            std::string symbol(req.Contract_Description.Symbol);

            // Get broker for symbol
            ITDPlugin* broker = GetBrokerForSymbol(symbol);
            if (!broker) {
                std::cerr << "[Processor] No broker for: " << symbol << std::endl;
                g_stats.failed_orders++;

                // Send ORS_REJECT
                ResponseMsg resp;
                std::memset(&resp, 0, sizeof(resp));
                resp.Response_Type = ORS_REJECT;
                resp.OrderID = req.OrderID;
                resp.ErrorCode = 1;
                resp.StrategyID = req.StrategyID;
                std::strncpy(resp.Symbol, symbol.c_str(), sizeof(resp.Symbol) - 1);
                g_response_queue->enqueue(resp);
                continue;
            }

            // Auto-determine open/close flag
            // C++ source: ors/China/src/ORSServer.cpp:488-605
            int openCloseFlag = OPEN_ORDER;
            SetCombOffsetFlag(&req, openCloseFlag, req.Exchange_Type);

            // Convert to ITDPlugin unified format
            hft::plugin::OrderRequest unified_req;
            std::memset(&unified_req, 0, sizeof(unified_req));

            std::strncpy(unified_req.symbol, symbol.c_str(), sizeof(unified_req.symbol) - 1);

            // Exchange_Type (byte) → string
            switch (req.Exchange_Type) {
                case CHINA_SHFE:  std::strcpy(unified_req.exchange, "SHFE"); break;
                case CHINA_CFFEX: std::strcpy(unified_req.exchange, "CFFEX"); break;
                case CHINA_ZCE:   std::strcpy(unified_req.exchange, "CZCE"); break;
                case CHINA_DCE:   std::strcpy(unified_req.exchange, "DCE"); break;
                case CHINA_GFEX:  std::strcpy(unified_req.exchange, "GFEX"); break;
                default:          std::strcpy(unified_req.exchange, "SHFE"); break;
            }

            // Direction: 'B'/'S' → BUY/SELL
            unified_req.direction = (req.Transaction_Type == SIDE_BUY)
                ? hft::plugin::OrderDirection::BUY
                : hft::plugin::OrderDirection::SELL;

            // Open/close: SetCombOffsetFlag result → OffsetFlag
            switch (openCloseFlag) {
                case OPEN_ORDER:       unified_req.offset = hft::plugin::OffsetFlag::OPEN; break;
                case CLOSE_TODAY_FLAG: unified_req.offset = hft::plugin::OffsetFlag::CLOSE_TODAY; break;
                case CLOSE_YESTD_FLAG: unified_req.offset = hft::plugin::OffsetFlag::CLOSE_YESTERDAY; break;
                default:               unified_req.offset = hft::plugin::OffsetFlag::OPEN; break;
            }

            // Price
            unified_req.price_type = (req.OrdType == OT_MARKET)
                ? hft::plugin::PriceType::MARKET
                : hft::plugin::PriceType::LIMIT;
            unified_req.price = req.Price;
            unified_req.volume = static_cast<uint32_t>(req.Quantity);

            // OrderID → string client_order_id (ITDPlugin uses strings)
            snprintf(unified_req.client_order_id,
                     sizeof(unified_req.client_order_id),
                     "%u", req.OrderID);

            std::cout << "[Processor] " << broker->GetPluginName() << ": "
                      << symbol << " "
                      << (req.Transaction_Type == SIDE_BUY ? "BUY" : "SELL")
                      << " " << req.Quantity << "@" << req.Price
                      << " (OID=" << req.OrderID << " flag=" << openCloseFlag << ")"
                      << std::endl;

            // Send to broker
            try {
                std::string broker_order_id = broker->SendOrder(unified_req);

                if (!broker_order_id.empty()) {
                    g_stats.success_orders++;

                    // Cache order info
                    std::lock_guard<std::mutex> lock(g_orders_mutex);
                    CachedOrderInfo info;
                    info.order_id = req.OrderID;
                    info.strategy_id = req.StrategyID;
                    info.symbol = symbol;
                    info.exchange = unified_req.exchange;
                    info.side = req.Transaction_Type;
                    info.client_order_id = unified_req.client_order_id;
                    info.openCloseFlag = openCloseFlag;
                    g_order_map[broker_order_id] = info;
                } else {
                    g_stats.failed_orders++;

                    // Send reject + unfreeze position
                    ResponseMsg resp;
                    std::memset(&resp, 0, sizeof(resp));
                    resp.Response_Type = ORDER_ERROR;
                    resp.OrderID = req.OrderID;
                    resp.ErrorCode = 1;
                    resp.Quantity = req.Quantity;
                    resp.Side = req.Transaction_Type;
                    resp.StrategyID = req.StrategyID;
                    std::strncpy(resp.Symbol, symbol.c_str(), sizeof(resp.Symbol) - 1);

                    CachedOrderInfo tmpInfo;
                    tmpInfo.symbol = symbol;
                    tmpInfo.side = req.Transaction_Type;
                    tmpInfo.openCloseFlag = openCloseFlag;
                    updatePosition(&resp, tmpInfo);

                    g_response_queue->enqueue(resp);
                }
            } catch (const std::exception& e) {
                g_stats.failed_orders++;
                std::cerr << "[Processor] Exception: " << e.what() << std::endl;
            }

            // Print stats periodically
            if (g_stats.total_orders % 10 == 0) {
                g_stats.Print();
            }

        } else {
            // Queue empty, short sleep
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }

    std::cout << "[Processor] Order request processor stopped." << std::endl;
    g_stats.Print();
}

// ============================================================
// main
// ============================================================
int main(int argc, char** argv) {
    PrintBanner();

    if (argc < 2) {
        std::cerr << "Usage: " << argv[0] << " <broker_config> [--position-file <file>]" << std::endl;
        std::cerr << "\nExamples:" << std::endl;
        std::cerr << "  " << argv[0] << " ctp:/path/to/ctp_td.yaml" << std::endl;
        std::cerr << "  " << argv[0] << " simulator:/path/to/sim.yaml --position-file positions.csv" << std::endl;
        std::cerr << "\nSupported brokers: ctp, simulator" << std::endl;
        return 1;
    }

    // Register signal handlers
    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // Parse optional --position-file argument
    std::string position_file;
    std::vector<std::string> broker_args;
    for (int i = 1; i < argc; i++) {
        std::string arg(argv[i]);
        if (arg == "--position-file" && i + 1 < argc) {
            position_file = argv[++i];
        } else {
            broker_args.push_back(arg);
        }
    }

    // Load initial positions (if specified)
    if (!position_file.empty()) {
        loadPositionFile(position_file);
    }

    // 1. Create SysV MWMR shared memory queues
    std::cout << "[Main] Creating SysV MWMR shared memory queues..." << std::endl;
    SHMConfig shm_cfg;

    ReqQueue* req_queue = nullptr;
    try {
        req_queue = ReqQueue::Create(shm_cfg.request_key, shm_cfg.request_size);
        std::cout << "[Main] Request MWMR queue ready (SysV key=0x"
                  << std::hex << shm_cfg.request_key << std::dec << ")" << std::endl;
    } catch (const std::exception& e) {
        std::cerr << "[Main] Failed to create request queue: " << e.what() << std::endl;
        return 1;
    }

    RespQueue* resp_queue = nullptr;
    try {
        resp_queue = RespQueue::Create(shm_cfg.response_key, shm_cfg.response_size);
        g_response_queue = resp_queue;
        std::cout << "[Main] Response MWMR queue ready (SysV key=0x"
                  << std::hex << shm_cfg.response_key << std::dec << ")" << std::endl;
    } catch (const std::exception& e) {
        std::cerr << "[Main] Failed to create response queue: " << e.what() << std::endl;
        return 1;
    }

    ClientStore* client_store = nullptr;
    try {
        client_store = ClientStore::Create(shm_cfg.client_store_key);
        std::cout << "[Main] Client store ready (SysV key=0x"
                  << std::hex << shm_cfg.client_store_key << std::dec << ")" << std::endl;
    } catch (const std::exception& e) {
        std::cerr << "[Main] Failed to create client store: " << e.what() << std::endl;
        return 1;
    }

    // 2. Initialize broker plugins
    std::cout << "\n[Main] Initializing broker plugins..." << std::endl;

    for (const auto& arg : broker_args) {
        size_t separator = arg.find(':');

        if (separator == std::string::npos) {
            std::cerr << "[Main] Invalid broker config format: " << arg << std::endl;
            std::cerr << "[Main]   Expected format: <broker>:<config_file>" << std::endl;
            continue;
        }

        std::string broker_name = arg.substr(0, separator);
        std::string config_file = arg.substr(separator + 1);

        std::cout << "[Main] Loading broker: " << broker_name << std::endl;
        std::cout << "[Main]   Config: " << config_file << std::endl;

        ITDPlugin* plugin = nullptr;

#if defined(ENABLE_CTP_PLUGIN)
        if (broker_name == "ctp") {
            auto ctp_plugin = std::make_unique<hft::plugin::ctp::CTPTDPlugin>();
            if (!ctp_plugin->Initialize(config_file)) {
                std::cerr << "[Main] Failed to initialize CTP plugin" << std::endl;
                continue;
            }

            ctp_plugin->RegisterOrderCallback(OnBrokerOrderCallback);
            ctp_plugin->RegisterTradeCallback(OnBrokerTradeCallback);
            ctp_plugin->RegisterErrorCallback(OnBrokerErrorCallback);

            if (!ctp_plugin->Login()) {
                std::cerr << "[Main] CTP login failed" << std::endl;
                continue;
            }

            std::cout << "[Main] CTP plugin initialized and logged in" << std::endl;
            plugin = ctp_plugin.get();
            g_brokers["ctp"] = std::move(ctp_plugin);
        }
#endif

#if defined(ENABLE_SIMULATOR_PLUGIN)
        if (broker_name == "simulator") {
            auto sim_plugin = std::make_unique<hft::plugin::simulator::SimulatorPlugin>();
            if (!sim_plugin->Initialize(config_file)) {
                std::cerr << "[Main] Failed to initialize Simulator plugin" << std::endl;
                continue;
            }

            sim_plugin->RegisterOrderCallback(OnBrokerOrderCallback);
            sim_plugin->RegisterTradeCallback(OnBrokerTradeCallback);
            sim_plugin->RegisterErrorCallback(OnBrokerErrorCallback);

            if (!sim_plugin->Login()) {
                std::cerr << "[Main] Simulator login failed" << std::endl;
                continue;
            }

            std::cout << "[Main] Simulator plugin initialized (immediate matching mode)" << std::endl;
            plugin = sim_plugin.get();
            g_brokers["simulator"] = std::move(sim_plugin);
        }
#endif

        if (!plugin) {
            std::cerr << "[Main] Unsupported broker: " << broker_name << std::endl;
            std::cerr << "[Main]   Supported: ";
#if defined(ENABLE_CTP_PLUGIN)
            std::cerr << "ctp ";
#endif
#if defined(ENABLE_SIMULATOR_PLUGIN)
            std::cerr << "simulator ";
#endif
            std::cerr << std::endl;
        }
    }

    if (g_brokers.empty()) {
        std::cerr << "[Main] No brokers initialized, exiting" << std::endl;
        return 1;
    }

    // 3. Wait for broker systems ready
    std::cout << "\n[Main] Waiting for broker systems ready (3 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // 4. Start HTTP server (health check + simulator stats only)
    std::cout << "\n[Main] Starting HTTP server..." << std::endl;
    StartHTTPServer(8080);

    // 5. Start order processor thread
    std::cout << "\n[Main] Starting order processor thread..." << std::endl;
    std::thread processor_thread(OrderRequestProcessor, req_queue);

    // 6. Print status
    std::cout << "\n╔════════════════════════════════════════════════════════════╗" << std::endl;
    std::cout << "║ Counter Bridge started successfully (MWMR mode)           ║" << std::endl;
    std::cout << "╠════════════════════════════════════════════════════════════╣" << std::endl;
    std::cout << "║ Request Queue:  SysV MWMR key=0x" << std::hex << shm_cfg.request_key << std::dec;
    std::cout << std::string(26, ' ') << "║" << std::endl;
    std::cout << "║ Response Queue: SysV MWMR key=0x" << std::hex << shm_cfg.response_key << std::dec;
    std::cout << std::string(26, ' ') << "║" << std::endl;
    std::cout << "║ Client Store:   SysV key=0x" << std::hex << shm_cfg.client_store_key << std::dec;
    std::cout << std::string(31, ' ') << "║" << std::endl;
    std::cout << "║ Active Brokers: " << g_brokers.size() << " broker(s)";
    std::cout << std::string(41, ' ') << "║" << std::endl;
    for (const auto& [name, broker] : g_brokers) {
        std::cout << "║   - " << name << " (" << broker->GetPluginName() << ")";
        int pad = 49 - static_cast<int>(name.length()) - static_cast<int>(broker->GetPluginName().length());
        if (pad > 0) std::cout << std::string(pad, ' ');
        std::cout << "║" << std::endl;
    }
    std::cout << "╚════════════════════════════════════════════════════════════╝" << std::endl;
    std::cout << "\nHTTP Endpoints:" << std::endl;
    std::cout << "  - Health Check: http://localhost:8080/health" << std::endl;
    std::cout << "\nWaiting for orders from Go trader via MWMR SHM..." << std::endl;
    std::cout << "Press Ctrl+C to stop...\n" << std::endl;

    // 7. Wait for exit signal
    processor_thread.join();

    // 8. Cleanup
    std::cout << "\n[Main] Cleaning up..." << std::endl;

    StopHTTPServer();

    // Logout all brokers
    for (auto& [name, broker] : g_brokers) {
        std::cout << "[Main] Logging out " << name << "..." << std::endl;
        broker->Logout();
    }
    g_brokers.clear();

    // Cleanup SHM
    if (req_queue) req_queue->close();
    if (resp_queue) resp_queue->close();
    if (client_store) client_store->close();

    std::cout << "[Main] Counter Bridge stopped" << std::endl;
    g_stats.Print();
    return 0;
}

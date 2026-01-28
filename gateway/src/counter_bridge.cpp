// Counter Bridge - 通用订单路由网关
// 功能：从ORS共享内存读取订单，路由到各券商插件（CTP/盛立/XTP等），接收回报写回共享内存
//
// 架构：
//   ORS Gateway → ors_request → Counter Bridge → [ITDPlugin接口] → CTP/盛立/XTP/...
//                                      ↓
//                 ors_response ← 订单回报

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

#include "plugin/td_plugin_interface.h"
#include "shm_queue.h"
#include "ors_gateway.h"
#include "../third_party/httplib.h"

// CTP插件（如果启用）
#if defined(ENABLE_CTP_PLUGIN)
#include "../plugins/ctp/include/ctp_td_plugin.h"
#endif

using OrderReqQueue = hft::shm::SPSCQueue<hft::ors::OrderRequestRaw, 4096>;
using OrderRespQueue = hft::shm::SPSCQueue<hft::ors::OrderResponseRaw, 4096>;
using namespace hft::plugin;

// 全局变量
static std::atomic<bool> g_running{true};
static OrderRespQueue* g_response_queue = nullptr;
static std::map<std::string, std::string> g_order_map;  // 券商订单ID -> 客户端订单ID映射
static std::mutex g_orders_mutex;

// 券商插件注册表
static std::map<std::string, std::unique_ptr<ITDPlugin>> g_brokers;
static std::map<std::string, std::string> g_symbol_to_broker;  // 品种 -> 券商映射

// 统计信息
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

void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << ", shutting down..." << std::endl;
    g_running = false;
}

void PrintBanner() {
    std::cout << R"(
╔═══════════════════════════════════════════════════════════╗
║          Counter Bridge - Multi-Broker Gateway            ║
║        ORS Shared Memory → Broker Plugins → Exchange      ║
╚═══════════════════════════════════════════════════════════╝
)" << std::endl;
}

// 券商订单回调 - 转发到ORS响应队列
void OnBrokerOrderCallback(const OrderInfo& order_info) {
    if (!g_response_queue) return;

    hft::ors::OrderResponseRaw resp;
    std::memset(&resp, 0, sizeof(resp));

    // 订单ID映射：券商订单ID -> 客户端订单ID
    std::string client_order_id;
    {
        std::lock_guard<std::mutex> lock(g_orders_mutex);
        auto it = g_order_map.find(order_info.order_id);
        if (it != g_order_map.end()) {
            client_order_id = it->second;
        } else {
            client_order_id = order_info.client_order_id;
        }
    }

    std::strncpy(resp.order_id, order_info.order_id, sizeof(resp.order_id) - 1);
    std::strncpy(resp.client_order_id, client_order_id.c_str(), sizeof(resp.client_order_id) - 1);

    // 状态映射：券商状态 -> ORS状态
    switch (order_info.status) {
        case hft::plugin::OrderStatus::ACCEPTED:
        case hft::plugin::OrderStatus::SUBMITTED:
            resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::ACCEPTED);
            std::cout << "[Bridge] ✅ Order ACCEPTED: " << order_info.order_id << std::endl;
            break;
        case hft::plugin::OrderStatus::PARTIAL_FILLED:
            resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::PARTIALLY_FILLED);
            std::cout << "[Bridge] 🟡 Order PARTIAL_FILLED: " << order_info.order_id
                     << " (" << order_info.traded_volume << "/" << order_info.volume << ")" << std::endl;
            break;
        case hft::plugin::OrderStatus::FILLED:
            resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::FILLED);
            g_stats.filled_orders++;
            std::cout << "[Bridge] ✅ Order FILLED: " << order_info.order_id
                     << " (" << order_info.traded_volume << "/" << order_info.volume << ")" << std::endl;
            break;
        case hft::plugin::OrderStatus::CANCELED:
            resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::CANCELED);
            std::cout << "[Bridge] ⚠️  Order CANCELED: " << order_info.order_id << std::endl;
            break;
        case hft::plugin::OrderStatus::REJECTED:
        case hft::plugin::OrderStatus::ERROR:
            resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::REJECTED);
            resp.error_code = 1;
            g_stats.rejected_orders++;
            std::strncpy(resp.error_msg, order_info.status_msg, sizeof(resp.error_msg) - 1);
            std::cout << "[Bridge] ❌ Order REJECTED: " << order_info.order_id
                     << " - " << order_info.status_msg << std::endl;
            break;
        default:
            resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::STATUS_UNKNOWN);
            break;
    }

    resp.filled_qty = order_info.traded_volume;
    resp.quantity = order_info.volume;
    resp.price = order_info.price;
    resp.last_fill_price = order_info.price;
    resp.last_fill_qty = order_info.traded_volume;
    resp.avg_price = order_info.price;
    resp.timestamp = order_info.update_time;

    // 写入响应队列
    if (!g_response_queue->Push(resp)) {
        std::cerr << "[Bridge] ❌ Failed to push response to queue" << std::endl;
    }
}

// 券商成交回调
void OnBrokerTradeCallback(const TradeInfo& trade_info) {
    std::cout << "[Bridge] 💰 Trade: " << trade_info.order_id
              << " price=" << trade_info.price
              << " volume=" << trade_info.volume << std::endl;
}

// 券商错误回调
void OnBrokerErrorCallback(int error_id, const std::string& error_msg) {
    std::cerr << "[Bridge] ❌ Broker Error: [" << error_id << "] " << error_msg << std::endl;
}

// 根据品种获取对应的券商插件
ITDPlugin* GetBrokerForSymbol(const std::string& symbol) {
    // 1. 先查找精确映射
    auto it = g_symbol_to_broker.find(symbol);
    if (it != g_symbol_to_broker.end()) {
        auto broker_it = g_brokers.find(it->second);
        if (broker_it != g_brokers.end()) {
            return broker_it->second.get();
        }
    }

    // 2. 使用默认券商（第一个已登录的）
    for (auto& [name, broker] : g_brokers) {
        if (broker && broker->IsLoggedIn()) {
            return broker.get();
        }
    }

    return nullptr;
}

// HTTP服务器 - 用于持仓查询
std::unique_ptr<httplib::Server> g_http_server;
std::thread g_http_thread;

// JSON转义辅助函数
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

// 处理持仓查询请求
void HandlePositionQuery(const httplib::Request& req, httplib::Response& res) {
    std::cout << "[HTTP] Position query received" << std::endl;

    // 从查询参数获取过滤条件（可选）
    std::string symbol = req.has_param("symbol") ? req.get_param_value("symbol") : "";
    std::string exchange = req.has_param("exchange") ? req.get_param_value("exchange") : "";

    // 按交易所分组的持仓数据
    std::map<std::string, std::vector<hft::plugin::PositionInfo>> positions_by_exchange;

    // 遍历所有券商插件，查询持仓
    for (auto& [broker_name, broker] : g_brokers) {
        if (!broker || !broker->IsLoggedIn()) {
            std::cout << "[HTTP] Skipping " << broker_name << " (not logged in)" << std::endl;
            continue;
        }

        std::vector<hft::plugin::PositionInfo> positions;
        if (broker->QueryPositions(positions)) {
            std::cout << "[HTTP] " << broker_name << " returned " << positions.size() << " positions" << std::endl;

            // 按交易所分组
            for (const auto& pos : positions) {
                std::string pos_exchange(pos.exchange);
                std::string pos_symbol(pos.symbol);

                // 应用过滤条件
                if (!exchange.empty() && pos_exchange != exchange) continue;
                if (!symbol.empty() && pos_symbol != symbol) continue;

                positions_by_exchange[pos_exchange].push_back(pos);
            }
        } else {
            std::cerr << "[HTTP] Failed to query positions from " << broker_name << std::endl;
        }
    }

    // 构建JSON响应
    std::ostringstream json;
    json << "{\n";
    json << "  \"success\": true,\n";
    json << "  \"data\": {\n";

    bool first_exchange = true;
    for (const auto& [exch, positions] : positions_by_exchange) {
        if (!first_exchange) json << ",\n";
        first_exchange = false;

        json << "    \"" << JsonEscape(exch) << "\": [\n";

        bool first_pos = true;
        for (const auto& pos : positions) {
            if (!first_pos) json << ",\n";
            first_pos = false;

            std::string direction = (pos.direction == hft::plugin::OrderDirection::BUY) ? "long" : "short";

            json << "      {\n";
            json << "        \"symbol\": \"" << JsonEscape(pos.symbol) << "\",\n";
            json << "        \"exchange\": \"" << JsonEscape(pos.exchange) << "\",\n";
            json << "        \"direction\": \"" << direction << "\",\n";
            json << "        \"volume\": " << pos.volume << ",\n";
            json << "        \"today_volume\": " << pos.today_volume << ",\n";
            json << "        \"yesterday_volume\": " << pos.yesterday_volume << ",\n";
            json << "        \"avg_price\": " << pos.avg_price << ",\n";
            json << "        \"position_profit\": " << pos.position_profit << ",\n";
            json << "        \"margin\": " << pos.margin << "\n";
            json << "      }";
        }

        json << "\n    ]";
    }

    json << "\n  }\n";
    json << "}\n";

    res.set_content(json.str(), "application/json");
    std::cout << "[HTTP] Position query response sent" << std::endl;
}

// 启动HTTP服务器
void StartHTTPServer(int port = 8080) {
    g_http_server = std::make_unique<httplib::Server>();

    // 注册endpoint
    g_http_server->Get("/positions", HandlePositionQuery);

    // 健康检查endpoint
    g_http_server->Get("/health", [](const httplib::Request&, httplib::Response& res) {
        res.set_content("{\"status\":\"ok\"}", "application/json");
    });

    std::cout << "[HTTP] Starting HTTP server on port " << port << "..." << std::endl;

    // 在单独的线程中运行服务器
    g_http_thread = std::thread([port]() {
        if (!g_http_server->listen("0.0.0.0", port)) {
            std::cerr << "[HTTP] Failed to start HTTP server on port " << port << std::endl;
        }
    });

    std::cout << "[HTTP] ✅ HTTP server started on port " << port << std::endl;
    std::cout << "[HTTP] Position query endpoint: http://localhost:" << port << "/positions" << std::endl;
}

// 停止HTTP服务器
void StopHTTPServer() {
    if (g_http_server) {
        std::cout << "[HTTP] Stopping HTTP server..." << std::endl;
        g_http_server->stop();
    }

    if (g_http_thread.joinable()) {
        g_http_thread.join();
    }

    std::cout << "[HTTP] ✅ HTTP server stopped" << std::endl;
}

// 订单请求处理线程
void OrderRequestProcessor(OrderReqQueue* req_queue) {
    std::cout << "[Processor] Order request processor started" << std::endl;

    hft::ors::OrderRequestRaw raw_req;

    while (g_running.load()) {
        if (req_queue->Pop(raw_req)) {
            g_stats.total_orders++;

            std::string symbol(raw_req.symbol);

            // 获取对应的券商插件
            ITDPlugin* broker = GetBrokerForSymbol(symbol);
            if (!broker) {
                std::cerr << "[Processor] ❌ No broker available for symbol: " << symbol << std::endl;
                g_stats.failed_orders++;

                // 发送拒绝回报
                hft::ors::OrderResponseRaw resp;
                std::memset(&resp, 0, sizeof(resp));
                std::strncpy(resp.order_id, raw_req.client_order_id, sizeof(resp.order_id) - 1);
                std::strncpy(resp.client_order_id, raw_req.client_order_id, sizeof(resp.client_order_id) - 1);
                resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::REJECTED);
                resp.error_code = 1;
                std::strncpy(resp.error_msg, "No broker available", sizeof(resp.error_msg) - 1);
                resp.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
                    std::chrono::system_clock::now().time_since_epoch()).count();
                g_response_queue->Push(resp);
                continue;
            }

            // 转换ORS订单到统一OrderRequest格式
            hft::plugin::OrderRequest unified_req;
            std::memset(&unified_req, 0, sizeof(unified_req));

            std::strncpy(unified_req.symbol, raw_req.symbol, sizeof(unified_req.symbol) - 1);
            std::strncpy(unified_req.exchange, raw_req.exchange, sizeof(unified_req.exchange) - 1);

            // 方向转换
            unified_req.direction = (raw_req.side == static_cast<uint8_t>(hft::ors::OrderSide::BUY))
                                   ? hft::plugin::OrderDirection::BUY : hft::plugin::OrderDirection::SELL;

            // 开平标志（从raw_req获取）
            unified_req.offset = static_cast<hft::plugin::OffsetFlag>(raw_req.open_close);

            // 价格类型（从OrderType推断）
            if (raw_req.order_type == static_cast<uint8_t>(hft::ors::OrderType::LIMIT)) {
                unified_req.price_type = hft::plugin::PriceType::LIMIT;
                unified_req.price = raw_req.price;
            } else if (raw_req.order_type == static_cast<uint8_t>(hft::ors::OrderType::MARKET)) {
                unified_req.price_type = hft::plugin::PriceType::MARKET;
                unified_req.price = 0.0;
            } else {
                // 默认限价单
                unified_req.price_type = hft::plugin::PriceType::LIMIT;
                unified_req.price = raw_req.price;
            }

            unified_req.volume = static_cast<uint32_t>(raw_req.quantity);
            std::strncpy(unified_req.client_order_id, raw_req.client_order_id, sizeof(unified_req.client_order_id) - 1);

            std::cout << "[Processor] 📤 Sending to " << broker->GetPluginName() << ": "
                      << unified_req.symbol
                      << " " << (unified_req.direction == OrderDirection::BUY ? "BUY" : "SELL")
                      << " " << unified_req.volume << "@" << unified_req.price
                      << " (ClientID: " << unified_req.client_order_id << ")" << std::endl;

            // 发送到券商
            try {
                std::string broker_order_id = broker->SendOrder(unified_req);

                if (!broker_order_id.empty()) {
                    g_stats.success_orders++;

                    // 保存订单ID映射
                    {
                        std::lock_guard<std::mutex> lock(g_orders_mutex);
                        g_order_map[broker_order_id] = raw_req.client_order_id;
                    }

                    std::cout << "[Processor] ✅ Order sent, BrokerOrderID: " << broker_order_id << std::endl;
                } else {
                    g_stats.failed_orders++;
                    std::cerr << "[Processor] ❌ Failed to send order to broker" << std::endl;

                    // 发送拒绝回报
                    hft::ors::OrderResponseRaw resp;
                    std::memset(&resp, 0, sizeof(resp));
                    std::strncpy(resp.order_id, raw_req.client_order_id, sizeof(resp.order_id) - 1);
                    std::strncpy(resp.client_order_id, raw_req.client_order_id, sizeof(resp.client_order_id) - 1);
                    resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::REJECTED);
                    resp.error_code = 1;
                    std::strncpy(resp.error_msg, "Broker SendOrder failed", sizeof(resp.error_msg) - 1);
                    resp.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
                        std::chrono::system_clock::now().time_since_epoch()).count();
                    g_response_queue->Push(resp);
                }
            } catch (const std::exception& e) {
                g_stats.failed_orders++;
                std::cerr << "[Processor] ❌ Exception sending order: " << e.what() << std::endl;
            }

            // 定期打印统计
            if (g_stats.total_orders % 10 == 0) {
                g_stats.Print();
            }

        } else {
            // 队列空，短暂休眠
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }

    std::cout << "[Processor] Order request processor stopped." << std::endl;
    g_stats.Print();
}

int main(int argc, char** argv) {
    PrintBanner();

    if (argc < 2) {
        std::cerr << "Usage: " << argv[0] << " <broker_config>" << std::endl;
        std::cerr << "\nExamples:" << std::endl;
        std::cerr << "  " << argv[0] << " ctp:/path/to/ctp_td.yaml" << std::endl;
        std::cerr << "  " << argv[0] << " ctp:/path/to/ctp.yaml suntime:/path/to/suntime.yaml" << std::endl;
        std::cerr << "\nSupported brokers: ctp, suntime, xtp, femas" << std::endl;
        return 1;
    }

    // 注册信号处理
    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // 1. 打开共享内存队列
    std::cout << "[Main] Opening shared memory queues..." << std::endl;
    auto* req_queue_raw = hft::shm::ShmManager::CreateOrOpen("ors_request");
    if (!req_queue_raw) {
        std::cerr << "[Main] ❌ Failed to open request queue" << std::endl;
        return 1;
    }
    auto* req_queue = reinterpret_cast<OrderReqQueue*>(req_queue_raw);
    std::cout << "[Main] ✅ Request queue ready" << std::endl;

    auto* resp_queue_raw = hft::shm::ShmManager::CreateOrOpen("ors_response");
    if (!resp_queue_raw) {
        std::cerr << "[Main] ❌ Failed to create response queue" << std::endl;
        return 1;
    }
    auto* resp_queue = reinterpret_cast<OrderRespQueue*>(resp_queue_raw);
    g_response_queue = resp_queue;
    std::cout << "[Main] ✅ Response queue ready" << std::endl;

    // 2. 初始化券商插件
    std::cout << "\n[Main] Initializing broker plugins..." << std::endl;

    for (int i = 1; i < argc; i++) {
        std::string arg(argv[i]);
        size_t separator = arg.find(':');

        if (separator == std::string::npos) {
            std::cerr << "[Main] ⚠️  Invalid broker config format: " << arg << std::endl;
            std::cerr << "[Main]    Expected format: <broker>:<config_file>" << std::endl;
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
                std::cerr << "[Main] ❌ Failed to initialize CTP plugin" << std::endl;
                continue;
            }

            // 注册回调
            ctp_plugin->RegisterOrderCallback(OnBrokerOrderCallback);
            ctp_plugin->RegisterTradeCallback(OnBrokerTradeCallback);
            ctp_plugin->RegisterErrorCallback(OnBrokerErrorCallback);

            // 登录
            if (!ctp_plugin->Login()) {
                std::cerr << "[Main] ❌ CTP login failed" << std::endl;
                continue;
            }

            std::cout << "[Main] ✅ CTP plugin initialized and logged in" << std::endl;
            plugin = ctp_plugin.get();
            g_brokers["ctp"] = std::move(ctp_plugin);
        }
#endif

        // TODO: 添加其他券商插件
        // if (broker_name == "suntime") { ... }
        // if (broker_name == "xtp") { ... }

        if (!plugin) {
            std::cerr << "[Main] ⚠️  Unsupported broker: " << broker_name << std::endl;
            std::cerr << "[Main]    Supported: ";
#if defined(ENABLE_CTP_PLUGIN)
            std::cerr << "ctp ";
#endif
            std::cerr << std::endl;
        }
    }

    if (g_brokers.empty()) {
        std::cerr << "[Main] ❌ No brokers initialized, exiting" << std::endl;
        return 1;
    }

    // 3. 等待券商系统就绪
    std::cout << "\n[Main] Waiting for broker systems ready (3 seconds)..." << std::endl;
    std::this_thread::sleep_for(std::chrono::seconds(3));

    // 4. 启动HTTP服务器（用于持仓查询）
    std::cout << "\n[Main] Starting HTTP server..." << std::endl;
    StartHTTPServer(8080);

    // 5. 启动订单处理线程
    std::cout << "\n[Main] Starting order processor thread..." << std::endl;
    std::thread processor_thread(OrderRequestProcessor, req_queue);

    // 6. 打印状态
    std::cout << "\n╔════════════════════════════════════════════════════════════╗" << std::endl;
    std::cout << "║ Counter Bridge started successfully                        ║" << std::endl;
    std::cout << "╠════════════════════════════════════════════════════════════╣" << std::endl;
    std::cout << "║ Request Queue:  ors_request                                ║" << std::endl;
    std::cout << "║ Response Queue: ors_response                               ║" << std::endl;
    std::cout << "║ HTTP Server:    http://localhost:8080                      ║" << std::endl;
    std::cout << "║ Active Brokers: " << g_brokers.size() << " broker(s)                                 ║" << std::endl;
    for (const auto& [name, broker] : g_brokers) {
        std::cout << "║   - " << name << " (" << broker->GetPluginName() << ")";
        std::cout << std::string(49 - name.length() - broker->GetPluginName().length(), ' ') << "║" << std::endl;
    }
    std::cout << "╚════════════════════════════════════════════════════════════╝" << std::endl;
    std::cout << "\nEndpoints:" << std::endl;
    std::cout << "  - Position Query: http://localhost:8080/positions" << std::endl;
    std::cout << "  - Health Check:   http://localhost:8080/health" << std::endl;
    std::cout << "\nWaiting for orders from ORS Gateway..." << std::endl;
    std::cout << "Press Ctrl+C to stop...\n" << std::endl;

    // 7. 等待退出信号
    processor_thread.join();

    // 8. 清理
    std::cout << "\n[Main] Cleaning up..." << std::endl;

    // 停止HTTP服务器
    StopHTTPServer();

    // 登出所有券商
    for (auto& [name, broker] : g_brokers) {
        std::cout << "[Main] Logging out " << name << "..." << std::endl;
        broker->Logout();
    }
    g_brokers.clear();

    std::cout << "[Main] ✅ Counter Bridge stopped" << std::endl;
    g_stats.Print();
    return 0;
}

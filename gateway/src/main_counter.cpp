#include <iostream>
#include <memory>
#include <thread>
#include <atomic>
#include <signal.h>
#include <cstring>
#include <map>
#include <mutex>

#include "counter_api.h"
#include "simulated_counter.h"
#include "shm_queue.h"
#include "ors_gateway.h"

using OrderReqQueue = hft::shm::SPSCQueue<hft::ors::OrderRequestRaw, 4096>;
using OrderRespQueue = hft::shm::SPSCQueue<hft::ors::OrderResponseRaw, 4096>;

// 订单信息缓存
struct CachedOrderInfo {
    std::string symbol;
    std::string exchange;
    uint8_t side;
};

// 全局变量
static std::atomic<bool> g_running{true};
static std::unique_ptr<hft::counter::ICounterAPI> g_counter;
static OrderRespQueue* g_response_queue = nullptr;
static std::map<std::string, CachedOrderInfo> g_order_cache;  // order_id -> 订单信息
static std::mutex g_cache_mutex;

void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << ", shutting down..." << std::endl;
    g_running = false;
}

void PrintBanner() {
    std::cout << R"(
╔═══════════════════════════════════════════════════════════╗
║             HFT Counter Gateway (Simulated)              ║
║                  Connecting to Exchange                   ║
╚═══════════════════════════════════════════════════════════╝
)" << std::endl;
}

// Counter回调实现
class CounterCallback : public hft::counter::ICounterCallback {
public:
    explicit CounterCallback(OrderRespQueue* resp_queue)
        : m_response_queue(resp_queue)
        , m_accept_count(0)
        , m_reject_count(0)
        , m_fill_count(0)
    {}

    void OnOrderAccept(const std::string& strategy_id,
                       const std::string& order_id,
                       const std::string& exchange_order_id) override {
        hft::ors::OrderResponseRaw resp;
        std::memset(&resp, 0, sizeof(resp));

        // 填充响应
        std::strncpy(resp.strategy_id, strategy_id.c_str(), sizeof(resp.strategy_id) - 1);
        std::strncpy(resp.order_id, order_id.c_str(), sizeof(resp.order_id) - 1);
        std::strncpy(resp.client_order_id, order_id.c_str(), sizeof(resp.client_order_id) - 1);

        // 从缓存中获取订单信息
        {
            std::lock_guard<std::mutex> lock(g_cache_mutex);
            auto it = g_order_cache.find(order_id);
            if (it != g_order_cache.end()) {
                std::strncpy(resp.symbol, it->second.symbol.c_str(), sizeof(resp.symbol) - 1);
                std::strncpy(resp.exchange, it->second.exchange.c_str(), sizeof(resp.exchange) - 1);
                resp.side = it->second.side;
            }
        }

        resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::ACCEPTED);
        resp.error_code = 0;
        resp.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch()).count();

        // 写入响应队列
        if (m_response_queue && m_response_queue->Push(resp)) {
            m_accept_count++;
            std::cout << "[Callback] Order accepted: " << order_id
                      << " (total: " << m_accept_count << ")"
                      << std::endl;
        }
    }

    void OnOrderReject(const std::string& strategy_id,
                      const std::string& order_id,
                      uint8_t error_code,
                      const std::string& error_msg) override {
        hft::ors::OrderResponseRaw resp;
        std::memset(&resp, 0, sizeof(resp));

        // 填充响应
        std::strncpy(resp.strategy_id, strategy_id.c_str(), sizeof(resp.strategy_id) - 1);
        std::strncpy(resp.order_id, order_id.c_str(), sizeof(resp.order_id) - 1);
        std::strncpy(resp.client_order_id, order_id.c_str(), sizeof(resp.client_order_id) - 1);
        resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::REJECTED);
        resp.error_code = error_code;
        std::strncpy(resp.error_msg, error_msg.c_str(), sizeof(resp.error_msg) - 1);
        resp.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch()).count();

        // 写入响应队列
        if (m_response_queue && m_response_queue->Push(resp)) {
            m_reject_count++;
            std::cout << "[Callback] Order rejected: " << order_id
                      << " error=" << error_msg
                      << " (total: " << m_reject_count << ")"
                      << std::endl;
        }
    }

    void OnOrderFilled(const std::string& strategy_id,
                      const std::string& order_id,
                      const std::string& exec_id,
                      double price,
                      int64_t quantity,
                      int64_t filled_qty) override {
        hft::ors::OrderResponseRaw resp;
        std::memset(&resp, 0, sizeof(resp));

        // 填充响应
        std::strncpy(resp.strategy_id, strategy_id.c_str(), sizeof(resp.strategy_id) - 1);
        std::strncpy(resp.order_id, order_id.c_str(), sizeof(resp.order_id) - 1);
        std::strncpy(resp.client_order_id, order_id.c_str(), sizeof(resp.client_order_id) - 1);
        std::strncpy(resp.exec_id, exec_id.c_str(), sizeof(resp.exec_id) - 1);

        // 从缓存中获取订单信息
        {
            std::lock_guard<std::mutex> lock(g_cache_mutex);
            auto it = g_order_cache.find(order_id);
            if (it != g_order_cache.end()) {
                std::strncpy(resp.symbol, it->second.symbol.c_str(), sizeof(resp.symbol) - 1);
                std::strncpy(resp.exchange, it->second.exchange.c_str(), sizeof(resp.exchange) - 1);
                resp.side = it->second.side;
            }
        }

        resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::FILLED);
        resp.error_code = 0;
        resp.price = price;
        resp.quantity = quantity;
        resp.filled_qty = filled_qty;
        resp.avg_price = price;  // 简化：平均价等于成交价
        resp.last_fill_price = price;
        resp.last_fill_qty = filled_qty;
        resp.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch()).count();

        // 写入响应队列
        if (m_response_queue && m_response_queue->Push(resp)) {
            m_fill_count++;
            std::cout << "[Callback] Order filled: " << order_id
                      << " exec_id=" << exec_id
                      << " price=" << price
                      << " qty=" << filled_qty
                      << " (total: " << m_fill_count << ")"
                      << std::endl;
        }
    }

    void OnOrderCanceled(const std::string& strategy_id,
                        const std::string& order_id) override {
        hft::ors::OrderResponseRaw resp;
        std::memset(&resp, 0, sizeof(resp));

        // 填充响应
        std::strncpy(resp.strategy_id, strategy_id.c_str(), sizeof(resp.strategy_id) - 1);
        std::strncpy(resp.order_id, order_id.c_str(), sizeof(resp.order_id) - 1);
        std::strncpy(resp.client_order_id, order_id.c_str(), sizeof(resp.client_order_id) - 1);
        resp.status = static_cast<uint8_t>(hft::ors::OrderStatus::CANCELED);
        resp.error_code = 0;
        resp.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch()).count();

        // 写入响应队列
        if (m_response_queue && m_response_queue->Push(resp)) {
            std::cout << "[Callback] Order canceled: " << order_id << std::endl;
        }
    }

    uint64_t GetAcceptCount() const { return m_accept_count; }
    uint64_t GetRejectCount() const { return m_reject_count; }
    uint64_t GetFillCount() const { return m_fill_count; }

private:
    OrderRespQueue* m_response_queue;
    uint64_t m_accept_count;
    uint64_t m_reject_count;
    uint64_t m_fill_count;
};

// 请求队列读取线程
void RequestQueueReaderThread(OrderReqQueue* req_queue,
                              hft::counter::ICounterAPI* counter) {
    std::cout << "[ReqReader] Request queue reader thread started" << std::endl;

    hft::ors::OrderRequestRaw raw_req;
    uint64_t read_count = 0;
    uint64_t last_print_count = 0;
    auto last_print_time = std::chrono::steady_clock::now();

    while (g_running.load()) {
        if (req_queue->Pop(raw_req)) {
            // 保存订单信息到缓存
            {
                std::lock_guard<std::mutex> lock(g_cache_mutex);
                CachedOrderInfo info;
                info.symbol = raw_req.symbol;
                info.exchange = raw_req.exchange;
                info.side = raw_req.side;
                // 先用 client_order_id 作为 key，稍后收到 exchange order_id 时更新
                g_order_cache[raw_req.client_order_id] = info;
            }

            // 发送到柜台
            std::string order_id;
            int ret = counter->SendOrder(raw_req, order_id);

            if (ret == 0) {
                read_count++;

                // 用 exchange order_id 更新缓存
                {
                    std::lock_guard<std::mutex> lock(g_cache_mutex);
                    auto it = g_order_cache.find(raw_req.client_order_id);
                    if (it != g_order_cache.end()) {
                        g_order_cache[order_id] = it->second;
                        // 保留 client_order_id 的映射以便后续查找
                    }
                }

                // 定期打印统计
                auto now = std::chrono::steady_clock::now();
                auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(now - last_print_time).count();
                if (elapsed >= 10) {
                    uint64_t delta = read_count - last_print_count;
                    std::cout << "[ReqReader] Read: " << read_count
                              << ", Rate: " << (delta / elapsed) << " req/s"
                              << std::endl;
                    last_print_count = read_count;
                    last_print_time = now;
                }
            } else {
                std::cerr << "[ReqReader] Failed to send order to counter" << std::endl;
            }
        } else {
            // 队列空，短暂休眠
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }

    std::cout << "[ReqReader] Request queue reader thread stopped. Total read: "
              << read_count << std::endl;
}

int main(int argc, char** argv) {
    PrintBanner();

    // 解析命令行参数
    std::string req_queue_name = "ors_request";
    std::string resp_queue_name = "ors_response";
    std::string counter_type = "simulated";

    for (int i = 1; i < argc; i++) {
        std::string arg = argv[i];
        if (arg == "-r" || arg == "--req-queue") {
            if (i + 1 < argc) {
                req_queue_name = argv[++i];
            }
        } else if (arg == "-s" || arg == "--resp-queue") {
            if (i + 1 < argc) {
                resp_queue_name = argv[++i];
            }
        } else if (arg == "-t" || arg == "--type") {
            if (i + 1 < argc) {
                counter_type = argv[++i];
            }
        }
    }

    // 注册信号处理
    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // 1. 打开共享内存队列
    std::cout << "[Main] Opening request queue: " << req_queue_name << std::endl;
    auto* req_queue = hft::shm::ShmManager::CreateOrOpenGeneric<hft::ors::OrderRequestRaw, 4096>(req_queue_name);
    if (!req_queue) {
        std::cerr << "[Main] Failed to open request queue" << std::endl;
        return 1;
    }
    std::cout << "[Main] Request queue ready" << std::endl;

    std::cout << "[Main] Creating response queue: " << resp_queue_name << std::endl;
    auto* resp_queue = hft::shm::ShmManager::CreateOrOpenGeneric<hft::ors::OrderResponseRaw, 4096>(resp_queue_name);
    if (!resp_queue) {
        std::cerr << "[Main] Failed to create response queue" << std::endl;
        munmap(req_queue, sizeof(OrderReqQueue));
        return 1;
    }
    g_response_queue = resp_queue;
    std::cout << "[Main] Response queue ready" << std::endl;

    // 2. 创建 Counter API
    std::cout << "[Main] Creating counter: " << counter_type << std::endl;
    if (counter_type == "simulated") {
        auto sim_counter = std::make_unique<hft::counter::SimulatedCounter>();

        // 配置模拟参数
        hft::counter::SimulatedCounter::Config config;
        config.accept_delay_ms = 10;
        config.fill_delay_ms = 50;
        config.fill_probability = 0.95;
        config.reject_probability = 0.02;
        config.immediate_fill = true;
        sim_counter->SetConfig(config);

        g_counter = std::move(sim_counter);
    } else {
        std::cerr << "[Main] Unsupported counter type: " << counter_type << std::endl;
        munmap(req_queue, sizeof(OrderReqQueue));
        munmap(resp_queue, sizeof(OrderRespQueue));
        return 1;
    }

    // 3. 设置回调
    auto callback = std::make_unique<CounterCallback>(resp_queue);
    g_counter->SetCallback(callback.get());

    // 4. 连接Counter
    if (!g_counter->Connect()) {
        std::cerr << "[Main] Failed to connect to counter" << std::endl;
        munmap(req_queue, sizeof(OrderReqQueue));
        munmap(resp_queue, sizeof(OrderRespQueue));
        return 1;
    }

    // 5. 启动请求队列读取线程
    std::thread req_reader_thread([req_queue]() {
        RequestQueueReaderThread(req_queue, g_counter.get());
    });

    std::cout << "\n╔════════════════════════════════════════════════════════════╗" << std::endl;
    std::cout << "║ Counter Gateway started successfully                       ║" << std::endl;
    std::cout << "╠════════════════════════════════════════════════════════════╣" << std::endl;
    std::cout << "║ Counter Type:   " << std::left << std::setw(43) << counter_type << "║" << std::endl;
    std::cout << "║ Request Queue:  " << std::left << std::setw(43) << req_queue_name << "║" << std::endl;
    std::cout << "║ Response Queue: " << std::left << std::setw(43) << resp_queue_name << "║" << std::endl;
    std::cout << "╚════════════════════════════════════════════════════════════╝\n" << std::endl;
    std::cout << "[Main] Press Ctrl+C to stop...\n" << std::endl;

    // 6. 主线程等待退出信号
    while (g_running.load()) {
        std::this_thread::sleep_for(std::chrono::seconds(1));
    }

    // 7. 清理
    std::cout << "[Main] Shutting down Counter Gateway..." << std::endl;
    g_running = false;

    // 等待线程结束
    if (req_reader_thread.joinable()) {
        req_reader_thread.join();
    }

    // 断开Counter连接
    g_counter->Disconnect();

    // 关闭共享内存
    std::cout << "[Main] Closing shared memory queues..." << std::endl;
    munmap(req_queue, sizeof(OrderReqQueue));
    munmap(resp_queue, sizeof(OrderRespQueue));

    // 打印统计
    std::cout << "\n╔════════════════════════════════════════════════════════════╗" << std::endl;
    std::cout << "║                    Session Statistics                      ║" << std::endl;
    std::cout << "╠════════════════════════════════════════════════════════════╣" << std::endl;
    std::cout << "║ Orders Accepted:  " << std::left << std::setw(42) << callback->GetAcceptCount() << "║" << std::endl;
    std::cout << "║ Orders Rejected:  " << std::left << std::setw(42) << callback->GetRejectCount() << "║" << std::endl;
    std::cout << "║ Orders Filled:    " << std::left << std::setw(42) << callback->GetFillCount() << "║" << std::endl;
    std::cout << "╚════════════════════════════════════════════════════════════╝\n" << std::endl;

    std::cout << "[Main] Counter Gateway stopped. Goodbye!" << std::endl;
    return 0;
}

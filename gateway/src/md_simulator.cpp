#include "shm_queue.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <signal.h>
#include <random>
#include <iomanip>

using namespace hft::shm;

// 全局标志
std::atomic<bool> g_running{true};

// 信号处理
void SignalHandler(int signal) {
    std::cout << "\n[Simulator] Received signal " << signal << ", stopping..." << std::endl;
    g_running.store(false);
}

// 模拟行情生成器
class MarketDataSimulator {
public:
    MarketDataSimulator(ShmManager::Queue* queue)
        : m_queue(queue), m_seq_num(0) {
        // 初始化随机数生成器
        std::random_device rd;
        m_rng.seed(rd());
    }

    void Start(int frequency_hz = 100) {
        std::cout << "[Simulator] Starting market data generation..." << std::endl;
        std::cout << "[Simulator] Frequency: " << frequency_hz << " Hz" << std::endl;

        auto interval = std::chrono::microseconds(1000000 / frequency_hz);
        auto next_time = std::chrono::high_resolution_clock::now();

        uint64_t total_pushed = 0;
        uint64_t total_dropped = 0;
        auto start_time = std::chrono::steady_clock::now();

        while (g_running.load()) {
            auto now = std::chrono::high_resolution_clock::now();

            if (now >= next_time) {
                // 生成行情数据
                MarketDataRaw md = GenerateMarketData();

                // 推送到共享内存队列
                if (m_queue->Push(md)) {
                    total_pushed++;

                    // 每1000条打印一次统计
                    if (total_pushed % 1000 == 0) {
                        auto elapsed = std::chrono::steady_clock::now() - start_time;
                        auto elapsed_sec = std::chrono::duration<double>(elapsed).count();
                        double rate = total_pushed / elapsed_sec;

                        std::cout << "[Simulator] Pushed: " << total_pushed
                                  << ", Dropped: " << total_dropped
                                  << ", Queue Size: " << m_queue->GetSize()
                                  << ", Rate: " << std::fixed << std::setprecision(0) << rate << " msg/s"
                                  << std::endl;
                    }
                } else {
                    total_dropped++;
                    if (total_dropped % 100 == 0) {
                        std::cerr << "[Simulator] WARNING: Queue full, dropped " << total_dropped << " messages" << std::endl;
                    }
                }

                next_time += interval;
            } else {
                // 精确睡眠
                std::this_thread::sleep_for(std::chrono::microseconds(10));
            }
        }

        std::cout << "\n[Simulator] Stopped" << std::endl;
        std::cout << "[Simulator] Total pushed: " << total_pushed << std::endl;
        std::cout << "[Simulator] Total dropped: " << total_dropped << std::endl;
    }

private:
    MarketDataRaw GenerateMarketData() {
        MarketDataRaw md{};

        // 合约信息
        std::strncpy(md.symbol, "ag2412", sizeof(md.symbol));
        std::strncpy(md.exchange, "SHFE", sizeof(md.exchange));
        md.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        md.seq_num = ++m_seq_num;

        // 基准价格（加入随机波动）
        std::uniform_real_distribution<double> price_dist(-0.5, 0.5);
        double base_bid = 7950.0 + price_dist(m_rng);
        double base_ask = base_bid + 1.0;

        // 生成10档买卖盘
        for (int i = 0; i < 10; ++i) {
            md.bid_price[i] = base_bid - i;
            md.bid_qty[i] = 10 + i * 5;
            md.ask_price[i] = base_ask + i;
            md.ask_qty[i] = 12 + i * 5;
        }

        // 成交信息
        md.last_price = (base_bid + base_ask) / 2.0;
        md.last_qty = 5;
        md.total_volume = 123456 + m_seq_num;

        return md;
    }

    ShmManager::Queue* m_queue;
    uint64_t m_seq_num;
    std::mt19937 m_rng;
};

int main(int argc, char* argv[]) {
    std::cout << R"(
╔═══════════════════════════════════════════════════════╗
║      Market Data Simulator (Shared Memory)           ║
╚═══════════════════════════════════════════════════════╝
)" << std::endl;

    // 注册信号处理
    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // 解析参数
    int frequency = 1000;  // 默认1000 Hz
    std::string shm_name = "queue";

    if (argc > 1) {
        frequency = std::atoi(argv[1]);
    }
    if (argc > 2) {
        shm_name = argv[2];
    }

    try {
        // 清理旧的共享内存
        ShmManager::Remove(shm_name);

        // 创建共享内存队列
        std::cout << "[Simulator] Creating shared memory: " << shm_name << std::endl;
        auto* queue = ShmManager::Create(shm_name);

        std::cout << "[Simulator] Shared memory created successfully" << std::endl;
        std::cout << "[Simulator] Queue size: " << ShmManager::QUEUE_SIZE << " slots" << std::endl;
        std::cout << "[Simulator] Data size: " << sizeof(MarketDataRaw) << " bytes/slot" << std::endl;
        std::cout << "[Simulator] Total memory: "
                  << (sizeof(ShmManager::Queue) / 1024.0) << " KB" << std::endl;

        // 创建并启动模拟器
        MarketDataSimulator simulator(queue);
        simulator.Start(frequency);

        // 清理
        ShmManager::Close(queue);
        ShmManager::Remove(shm_name);

        std::cout << "[Simulator] Cleanup complete" << std::endl;

    } catch (const std::exception& e) {
        std::cerr << "[Simulator] Error: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}

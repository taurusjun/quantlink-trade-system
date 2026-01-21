#include "shm_queue.h"
#include "performance_monitor.h"
#include <iostream>
#include <thread>
#include <chrono>
#include <signal.h>
#include <iomanip>

using namespace hft::shm;
using namespace hft::perf;

std::atomic<bool> g_running{true};

void SignalHandler(int signal) {
    std::cout << "\n[Benchmark] Received signal " << signal << std::endl;
    g_running.store(false);
}

// 生产者线程
void ProducerThread(ShmManager::Queue* queue, int frequency_hz, PerformanceMonitor* monitor) {
    std::cout << "[Producer] Started at " << frequency_hz << " Hz" << std::endl;

    uint64_t seq_num = 0;
    auto interval = std::chrono::microseconds(1000000 / frequency_hz);
    auto next_send = std::chrono::steady_clock::now();

    while (g_running.load()) {
        auto now = std::chrono::steady_clock::now();

        if (now >= next_send) {
            MarketDataRaw md{};
            std::snprintf(md.symbol, sizeof(md.symbol), "TEST%04llu", seq_num % 1000);
            std::snprintf(md.exchange, sizeof(md.exchange), "TEST");
            md.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
                now.time_since_epoch()).count();
            md.seq_num = seq_num++;

            // 模拟10档行情
            for (int i = 0; i < 10; ++i) {
                md.bid_price[i] = 100.0 - i * 0.1;
                md.bid_qty[i] = 100 + i * 10;
                md.ask_price[i] = 100.0 + i * 0.1;
                md.ask_qty[i] = 100 + i * 10;
            }

            md.last_price = 100.0;
            md.last_qty = 100;
            md.total_volume = seq_num * 100;

            if (!queue->Push(md)) {
                // 队列满，记录丢包
                monitor->RecordMessage();  // 即使丢包也计数
            } else {
                monitor->RecordMessage();
            }

            next_send += interval;

            // 如果落后太多，重置时间
            if (next_send < now - std::chrono::milliseconds(100)) {
                next_send = now + interval;
            }
        } else {
            std::this_thread::sleep_for(std::chrono::microseconds(1));
        }
    }

    std::cout << "[Producer] Stopped. Total sent: " << seq_num << std::endl;
}

// 消费者线程
void ConsumerThread(ShmManager::Queue* queue, PerformanceMonitor* monitor) {
    std::cout << "[Consumer] Started" << std::endl;

    uint64_t count = 0;
    uint64_t last_seq = 0;
    uint64_t missing = 0;

    while (g_running.load()) {
        MarketDataRaw md;
        if (queue->Pop(md)) {
            auto now = std::chrono::steady_clock::now();
            auto receive_time = std::chrono::duration_cast<std::chrono::nanoseconds>(
                now.time_since_epoch()).count();

            // 计算延迟（从生成到接收）
            uint64_t latency_ns = receive_time - md.timestamp;
            monitor->RecordLatency(latency_ns);
            monitor->RecordMessage();

            // 检测丢包
            if (last_seq > 0 && md.seq_num != last_seq + 1) {
                missing += (md.seq_num - last_seq - 1);
            }
            last_seq = md.seq_num;

            count++;
        } else {
            std::this_thread::sleep_for(std::chrono::microseconds(1));
        }
    }

    std::cout << "[Consumer] Stopped. Total received: " << count
              << ", Missing: " << missing << std::endl;
}

int main(int argc, char* argv[]) {
    std::cout << R"(
╔═══════════════════════════════════════════════════════╗
║         MD Gateway Performance Benchmark              ║
╚═══════════════════════════════════════════════════════╝
)" << std::endl;

    // 注册信号处理
    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // 解析参数
    int frequency_hz = 10000;  // 默认10k Hz
    int duration_sec = 30;     // 默认30秒
    std::string shm_name = "benchmark";

    if (argc > 1) {
        frequency_hz = std::atoi(argv[1]);
    }
    if (argc > 2) {
        duration_sec = std::atoi(argv[2]);
    }
    if (argc > 3) {
        shm_name = argv[3];
    }

    std::cout << "[Config] Frequency: " << frequency_hz << " Hz" << std::endl;
    std::cout << "[Config] Duration: " << duration_sec << " seconds" << std::endl;
    std::cout << "[Config] Shared Memory: " << shm_name << std::endl;
    std::cout << std::endl;

    try {
        // 创建共享内存
        std::cout << "[Main] Creating shared memory..." << std::endl;
        auto* queue = ShmManager::Create(shm_name);
        std::cout << "[Main] Queue size: " << ShmManager::QUEUE_SIZE << " slots" << std::endl;
        std::cout << "[Main] Data size: " << sizeof(MarketDataRaw) << " bytes/slot" << std::endl;
        std::cout << "[Main] Total memory: " << std::fixed << std::setprecision(1)
                  << (sizeof(*queue) / 1024.0) << " KB" << std::endl;
        std::cout << std::endl;

        // 性能监控器
        PerformanceMonitor producer_monitor("Producer");
        PerformanceMonitor consumer_monitor("Consumer");

        // 启动消费者线程
        std::thread consumer([queue, &consumer_monitor]() {
            ConsumerThread(queue, &consumer_monitor);
        });

        // 等待一下让消费者准备好
        std::this_thread::sleep_for(std::chrono::milliseconds(100));

        // 启动生产者线程
        std::thread producer([queue, frequency_hz, &producer_monitor]() {
            ProducerThread(queue, frequency_hz, &producer_monitor);
        });

        // 性能监控线程
        std::thread monitor_thread([&producer_monitor, &consumer_monitor]() {
            auto last_report = std::chrono::steady_clock::now();

            while (g_running.load()) {
                std::this_thread::sleep_for(std::chrono::seconds(1));

                // 更新统计
                producer_monitor.Update();
                consumer_monitor.Update();

                // 每5秒打印一次
                auto now = std::chrono::steady_clock::now();
                if (std::chrono::duration_cast<std::chrono::seconds>(
                        now - last_report).count() >= 5) {

                    auto producer_stats = producer_monitor.GetThroughputStats();
                    auto consumer_stats = consumer_monitor.GetThroughputStats();
                    auto latency_stats = consumer_monitor.GetLatencyStats();

                    std::cout << "[Stats] Producer: " << std::fixed << std::setprecision(0)
                              << producer_stats.instant_rate << " msg/s, "
                              << "Consumer: " << consumer_stats.instant_rate << " msg/s, "
                              << "Latency: " << std::fixed << std::setprecision(2)
                              << latency_stats.GetAvg() / 1000.0 << " μs"
                              << std::endl;

                    last_report = now;
                }
            }
        });

        // 等待指定时间
        for (int i = duration_sec; i > 0 && g_running.load(); --i) {
            std::cout << "\r[Countdown] " << i << " seconds remaining...  " << std::flush;
            std::this_thread::sleep_for(std::chrono::seconds(1));
        }
        std::cout << std::endl;

        // 停止测试
        g_running.store(false);

        // 等待线程结束
        if (producer.joinable()) producer.join();
        if (consumer.joinable()) consumer.join();
        if (monitor_thread.joinable()) monitor_thread.join();

        // 最终更新
        producer_monitor.Update();
        consumer_monitor.Update();

        // 打印最终报告
        std::cout << "\n╔═══════════════════════════════════════════════════════╗" << std::endl;
        std::cout << "║                  Final Results                        ║" << std::endl;
        std::cout << "╚═══════════════════════════════════════════════════════╝\n" << std::endl;

        producer_monitor.PrintReport();
        consumer_monitor.PrintReport();

        // 清理
        ShmManager::Close(queue);
        ShmManager::Remove(shm_name);

    } catch (const std::exception& e) {
        std::cerr << "[Main] Error: " << e.what() << std::endl;
        return 1;
    }

    return 0;
}

#include "md_gateway.h"
#include "shm_queue.h"
#include <iostream>
#include <signal.h>
#include <thread>
#include <chrono>

using namespace hft::gateway;
using namespace hft::shm;

// 全局指针用于信号处理
MDGateway* g_gateway = nullptr;
std::atomic<bool> g_running{true};

// 信号处理函数
void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << std::endl;
    g_running.store(false);
    if (g_gateway) {
        g_gateway->Shutdown();
    }
}

// 从共享内存读取并转换为Protobuf格式
void ConvertToProtobuf(const MarketDataRaw& raw, hft::md::MarketDataUpdate* pb) {
    pb->set_symbol(raw.symbol);
    pb->set_exchange(raw.exchange);
    pb->set_timestamp(raw.timestamp);

    // 买盘
    for (int i = 0; i < 10; ++i) {
        if (raw.bid_price[i] > 0) {
            pb->add_bid_price(raw.bid_price[i]);
            pb->add_bid_qty(raw.bid_qty[i]);
            pb->add_bid_order_count(3 + i);
        }
    }

    // 卖盘
    for (int i = 0; i < 10; ++i) {
        if (raw.ask_price[i] > 0) {
            pb->add_ask_price(raw.ask_price[i]);
            pb->add_ask_qty(raw.ask_qty[i]);
            pb->add_ask_order_count(4 + i);
        }
    }

    // 成交信息
    pb->set_last_price(raw.last_price);
    pb->set_last_qty(raw.last_qty);
    pb->set_total_volume(raw.total_volume);
}

// 共享内存读取线程
void SharedMemoryReaderThread(MDGateway* gateway, ShmManager::Queue* queue) {
    std::cout << "[Reader] Shared memory reader thread started" << std::endl;

    uint64_t total_read = 0;
    uint64_t last_seq = 0;
    uint64_t missing_seq = 0;
    auto start_time = std::chrono::steady_clock::now();

    while (g_running.load()) {
        MarketDataRaw raw_md;

        // 从共享内存队列读取
        if (queue->Pop(raw_md)) {
            // 检测序列号跳跃（消息丢失）
            if (last_seq > 0 && raw_md.seq_num != last_seq + 1) {
                uint64_t gap = raw_md.seq_num - last_seq - 1;
                missing_seq += gap;
                std::cerr << "[Reader] WARNING: Missing " << gap
                          << " messages (seq: " << last_seq << " -> " << raw_md.seq_num << ")"
                          << std::endl;
            }
            last_seq = raw_md.seq_num;

            // 转换为Protobuf格式
            hft::md::MarketDataUpdate pb_md;
            ConvertToProtobuf(raw_md, &pb_md);

            // 推送到Gateway
            gateway->PushMarketData(pb_md);

            total_read++;

            // 每10000条打印一次统计
            if (total_read % 10000 == 0) {
                auto elapsed = std::chrono::steady_clock::now() - start_time;
                auto elapsed_sec = std::chrono::duration<double>(elapsed).count();
                double rate = total_read / elapsed_sec;

                std::cout << "[Reader] Read: " << total_read
                          << ", Missing: " << missing_seq
                          << ", Queue Size: " << queue->GetSize()
                          << ", Rate: " << std::fixed << std::setprecision(0) << rate << " msg/s"
                          << std::endl;
            }
        } else {
            // 队列为空，短暂睡眠避免空转
            std::this_thread::sleep_for(std::chrono::microseconds(1));
        }
    }

    std::cout << "[Reader] Stopped" << std::endl;
    std::cout << "[Reader] Total read: " << total_read << std::endl;
    std::cout << "[Reader] Missing messages: " << missing_seq << std::endl;
}

int main(int argc, char* argv[]) {
    std::cout << R"(
╔═══════════════════════════════════════════════════════╗
║    HFT Market Data Gateway - Shared Memory Mode      ║
╚═══════════════════════════════════════════════════════╝
)" << std::endl;

    // 注册信号处理
    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // 解析共享内存名称
    std::string shm_name = "queue";
    if (argc > 1) {
        shm_name = argv[1];
    }

    try {
        // 打开共享内存队列
        std::cout << "[Main] Opening shared memory: " << shm_name << std::endl;
        auto* queue = ShmManager::Open(shm_name);
        std::cout << "[Main] Shared memory opened successfully" << std::endl;

        // 配置Gateway
        MDGatewayConfig config;
        config.grpc_listen_addr = "0.0.0.0:50051";
        config.nats_url = "nats://localhost:4222";
        config.max_depth = 10;
        config.enable_nats = true;
        config.enable_grpc = true;

        // 创建Gateway
        auto gateway = std::make_unique<MDGateway>(config);
        g_gateway = gateway.get();

        // 启动共享内存读取线程
        std::thread reader_thread([&gateway, queue]() {
            SharedMemoryReaderThread(gateway.get(), queue);
        });

        // 运行Gateway（阻塞）
        gateway->Run();

        // 等待读取线程结束
        if (reader_thread.joinable()) {
            reader_thread.join();
        }

        // 清理
        ShmManager::Close(queue);

        std::cout << "[Main] Goodbye!" << std::endl;

    } catch (const std::exception& e) {
        std::cerr << "[Main] Error: " << e.what() << std::endl;
        std::cerr << "[Main] Make sure md_simulator is running first!" << std::endl;
        return 1;
    }

    return 0;
}

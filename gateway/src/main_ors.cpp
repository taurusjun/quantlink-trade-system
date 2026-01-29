#include <iostream>
#include <memory>
#include <iomanip>
#include <thread>
#include <atomic>
#include <signal.h>
#include <grpcpp/grpcpp.h>

#include "ors_gateway.h"
#include "shm_queue.h"

using OrderReqQueue = hft::shm::SPSCQueue<hft::ors::OrderRequestRaw, 4096>;
using OrderRespQueue = hft::shm::SPSCQueue<hft::ors::OrderResponseRaw, 4096>;

// 全局变量
static std::unique_ptr<grpc::Server> g_server;
static std::atomic<bool> g_running{true};
static std::atomic<bool> g_shutdown_requested{false};

void SignalHandler(int signal) {
    std::cout << "\n[Main] Received signal " << signal << ", shutting down..." << std::endl;
    g_running = false;
    g_shutdown_requested = true;
    if (g_server) {
        g_server->Shutdown();
    }
}

void PrintBanner() {
    std::cout << R"(
╔═══════════════════════════════════════════════════════════╗
║         HFT Order Routing Service (ORS) Gateway          ║
║                   Shared Memory Mode                      ║
╚═══════════════════════════════════════════════════════════╝
)" << std::endl;
}

void PrintUsage(const char* prog_name) {
    std::cout << "Usage: " << prog_name << " [OPTIONS]\n"
              << "\nOptions:\n"
              << "  -a, --address <addr>    gRPC server address (default: 0.0.0.0:50052)\n"
              << "  -n, --nats <url>        NATS server URL (default: nats://localhost:4222)\n"
              << "  -r, --req-queue <name>  Request queue name (default: ors_request)\n"
              << "  -s, --resp-queue <name> Response queue name (default: ors_response)\n"
              << "  -c, --config <file>     Config file path\n"
              << "  -h, --help              Show this help message\n"
              << std::endl;
}

// 转换函数：OrderResponseRaw -> OrderUpdate
void ConvertToProtobuf(const hft::ors::OrderResponseRaw& raw_resp, hft::ors::OrderUpdate* proto_update) {
    proto_update->set_strategy_id(raw_resp.strategy_id);
    proto_update->set_order_id(raw_resp.order_id);
    proto_update->set_client_order_id(raw_resp.client_order_id);

    // 设置 symbol, exchange, side（新增）
    proto_update->set_symbol(raw_resp.symbol);
    // 将交易所字符串转换为枚举
    std::string exchange_str(raw_resp.exchange);
    if (exchange_str == "SHFE") {
        proto_update->set_exchange(hft::common::Exchange::SHFE);
    } else if (exchange_str == "DCE") {
        proto_update->set_exchange(hft::common::Exchange::DCE);
    } else if (exchange_str == "CZCE") {
        proto_update->set_exchange(hft::common::Exchange::CZCE);
    } else if (exchange_str == "CFFEX") {
        proto_update->set_exchange(hft::common::Exchange::CFFEX);
    } else if (exchange_str == "INE") {
        proto_update->set_exchange(hft::common::Exchange::INE);
    } else {
        proto_update->set_exchange(hft::common::Exchange::UNKNOWN_EXCHANGE);
    }
    proto_update->set_side(static_cast<hft::ors::OrderSide>(raw_resp.side));
    proto_update->set_status(static_cast<hft::ors::OrderStatus>(raw_resp.status));

    proto_update->set_price(raw_resp.price);
    proto_update->set_quantity(raw_resp.quantity);
    proto_update->set_filled_qty(raw_resp.filled_qty);
    proto_update->set_remaining_qty(raw_resp.quantity - raw_resp.filled_qty);

    proto_update->set_avg_price(raw_resp.avg_price);
    proto_update->set_last_fill_price(raw_resp.last_fill_price);
    proto_update->set_last_fill_qty(raw_resp.last_fill_qty);

    proto_update->set_exec_id(raw_resp.exec_id);
    proto_update->set_exchange_timestamp(raw_resp.exchange_timestamp);
    proto_update->set_timestamp(raw_resp.timestamp);

    proto_update->set_error_code(static_cast<hft::ors::ErrorCode>(raw_resp.error_code));
    proto_update->set_error_msg(raw_resp.error_msg);
}

// 请求队列写入线程（从Gateway获取请求，写入共享内存）
void RequestQueueWriterThread(hft::ors::ORSGatewayImpl* gateway, OrderReqQueue* req_queue) {
    std::cout << "[ReqWriter] Request queue writer thread started" << std::endl;

    hft::ors::OrderRequestRaw raw_req;
    uint64_t written_count = 0;
    uint64_t last_print_count = 0;
    auto last_print_time = std::chrono::steady_clock::now();

    while (g_running.load()) {
        // 从Gateway获取待发送的订单请求
        if (gateway->GetOrderRequest(&raw_req)) {
            // 写入共享内存
            if (req_queue->Push(raw_req)) {
                written_count++;

                // 定期打印统计
                auto now = std::chrono::steady_clock::now();
                auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(now - last_print_time).count();
                if (elapsed >= 10) {
                    uint64_t delta = written_count - last_print_count;
                    std::cout << "[ReqWriter] Written: " << written_count
                              << ", Rate: " << (delta / elapsed) << " req/s"
                              << std::endl;
                    last_print_count = written_count;
                    last_print_time = now;
                }
            } else {
                std::cerr << "[ReqWriter] WARNING: Request queue is full, order dropped!" << std::endl;
            }
        } else {
            // 队列空，短暂休眠
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }

    std::cout << "[ReqWriter] Request queue writer thread stopped. Total written: "
              << written_count << std::endl;
}

// 响应队列读取线程（从共享内存读取响应，调用Gateway）
void ResponseQueueReaderThread(hft::ors::ORSGatewayImpl* gateway, OrderRespQueue* resp_queue) {
    std::cout << "[RespReader] Response queue reader thread started" << std::endl;

    hft::ors::OrderResponseRaw raw_resp;
    hft::ors::OrderUpdate proto_update;
    uint64_t read_count = 0;
    uint64_t last_print_count = 0;
    auto last_print_time = std::chrono::steady_clock::now();

    while (g_running.load()) {
        if (resp_queue->Pop(raw_resp)) {
            // 转换为Protobuf格式
            ConvertToProtobuf(raw_resp, &proto_update);

            // 调用Gateway处理
            gateway->OnOrderResponse(proto_update);

            read_count++;

            // 定期打印统计
            auto now = std::chrono::steady_clock::now();
            auto elapsed = std::chrono::duration_cast<std::chrono::seconds>(now - last_print_time).count();
            if (elapsed >= 10) {
                uint64_t delta = read_count - last_print_count;
                std::cout << "[RespReader] Read: " << read_count
                          << ", Rate: " << (delta / elapsed) << " resp/s"
                          << std::endl;
                last_print_count = read_count;
                last_print_time = now;
            }
        } else {
            // 队列空，短暂休眠
            std::this_thread::sleep_for(std::chrono::microseconds(100));
        }
    }

    std::cout << "[RespReader] Response queue reader thread stopped. Total read: "
              << read_count << std::endl;
}

int main(int argc, char** argv) {
    PrintBanner();

    // 解析命令行参数
    std::string grpc_address = "0.0.0.0:50052";
    std::string config_file;
    std::string req_queue_name = "ors_request";
    std::string resp_queue_name = "ors_response";

    for (int i = 1; i < argc; i++) {
        std::string arg = argv[i];
        if (arg == "-h" || arg == "--help") {
            PrintUsage(argv[0]);
            return 0;
        } else if (arg == "-a" || arg == "--address") {
            if (i + 1 < argc) {
                grpc_address = argv[++i];
            }
        } else if (arg == "-c" || arg == "--config") {
            if (i + 1 < argc) {
                config_file = argv[++i];
            }
        } else if (arg == "-r" || arg == "--req-queue") {
            if (i + 1 < argc) {
                req_queue_name = argv[++i];
            }
        } else if (arg == "-s" || arg == "--resp-queue") {
            if (i + 1 < argc) {
                resp_queue_name = argv[++i];
            }
        }
    }

    // 注册信号处理
    signal(SIGINT, SignalHandler);
    signal(SIGTERM, SignalHandler);

    // 1. 创建/打开共享内存队列
    std::cout << "[Main] Creating request queue: " << req_queue_name << std::endl;
    auto* req_queue = hft::shm::ShmManager::CreateOrOpenGeneric<hft::ors::OrderRequestRaw, 4096>(req_queue_name);
    std::cout << "[Main] Request queue ready" << std::endl;

    std::cout << "[Main] Creating response queue: " << resp_queue_name << std::endl;
    auto* resp_queue = hft::shm::ShmManager::CreateOrOpenGeneric<hft::ors::OrderResponseRaw, 4096>(resp_queue_name);
    std::cout << "[Main] Response queue ready" << std::endl;

    // 2. 创建ORS Gateway实例
    auto gateway = std::make_unique<hft::ors::ORSGatewayImpl>();

    // 3. 初始化Gateway
    if (!gateway->Initialize(config_file)) {
        std::cerr << "[Main] Failed to initialize ORS Gateway" << std::endl;
        munmap(req_queue, sizeof(OrderReqQueue));
        munmap(resp_queue, sizeof(OrderRespQueue));
        return 1;
    }

    // 4. 启动Gateway
    gateway->Start();

    // 5. 启动请求队列写入线程
    std::thread req_writer_thread([&gateway, req_queue]() {
        RequestQueueWriterThread(gateway.get(), req_queue);
    });

    // 6. 启动响应队列读取线程
    std::thread resp_reader_thread([&gateway, resp_queue]() {
        ResponseQueueReaderThread(gateway.get(), resp_queue);
    });

    // 7. 构建gRPC服务器
    grpc::ServerBuilder builder;
    builder.AddListeningPort(grpc_address, grpc::InsecureServerCredentials());
    builder.RegisterService(gateway.get());

    // 8. 启动gRPC服务器
    g_server = builder.BuildAndStart();
    if (!g_server) {
        std::cerr << "[Main] Failed to start gRPC server" << std::endl;
        g_running = false;

        // 等待线程结束
        if (req_writer_thread.joinable()) {
            req_writer_thread.join();
        }
        if (resp_reader_thread.joinable()) {
            resp_reader_thread.join();
        }

        gateway->Stop();
        munmap(req_queue, sizeof(OrderReqQueue));
        munmap(resp_queue, sizeof(OrderRespQueue));
        return 1;
    }

    std::cout << "\n╔════════════════════════════════════════════════════════════╗" << std::endl;
    std::cout << "║ ORS Gateway started successfully                           ║" << std::endl;
    std::cout << "╠════════════════════════════════════════════════════════════╣" << std::endl;
    std::cout << "║ gRPC Server:    " << std::left << std::setw(43) << grpc_address << "║" << std::endl;
    std::cout << "║ Request Queue:  " << std::left << std::setw(43) << req_queue_name << "║" << std::endl;
    std::cout << "║ Response Queue: " << std::left << std::setw(43) << resp_queue_name << "║" << std::endl;
#ifdef ENABLE_NATS
    std::cout << "║ NATS Status:    Enabled                                    ║" << std::endl;
#else
    std::cout << "║ NATS Status:    Disabled                                   ║" << std::endl;
#endif
    std::cout << "╚════════════════════════════════════════════════════════════╝\n" << std::endl;

    // 9. 等待关闭信号
    g_server->Wait();

    // 10. 清理
    std::cout << "[Main] Shutting down ORS Gateway..." << std::endl;
    g_running = false;

    // 等待队列线程结束
    std::cout << "[Main] Waiting for queue threads to finish..." << std::endl;
    if (req_writer_thread.joinable()) {
        req_writer_thread.join();
    }
    if (resp_reader_thread.joinable()) {
        resp_reader_thread.join();
    }

    // 停止Gateway
    gateway->Stop();

    // 关闭共享内存
    std::cout << "[Main] Closing shared memory queues..." << std::endl;
    munmap(req_queue, sizeof(OrderReqQueue));
    munmap(resp_queue, sizeof(OrderRespQueue));

    // 打印统计信息
    const auto& stats = gateway->GetStatistics();
    std::cout << "\n╔════════════════════════════════════════════════════════════╗" << std::endl;
    std::cout << "║                    Session Statistics                      ║" << std::endl;
    std::cout << "╠════════════════════════════════════════════════════════════╣" << std::endl;
    std::cout << "║ Total Orders:     " << std::left << std::setw(42) << stats.total_orders.load() << "║" << std::endl;
    std::cout << "║ Accepted Orders:  " << std::left << std::setw(42) << stats.accepted_orders.load() << "║" << std::endl;
    std::cout << "║ Rejected Orders:  " << std::left << std::setw(42) << stats.rejected_orders.load() << "║" << std::endl;
    std::cout << "║ Filled Orders:    " << std::left << std::setw(42) << stats.filled_orders.load() << "║" << std::endl;
    std::cout << "║ Canceled Orders:  " << std::left << std::setw(42) << stats.canceled_orders.load() << "║" << std::endl;
    std::cout << "║ Last Latency:     " << std::left << std::setw(42)
              << (std::to_string(stats.last_latency_ns.load()) + " ns") << "║" << std::endl;
    std::cout << "╚════════════════════════════════════════════════════════════╝\n" << std::endl;

    std::cout << "[Main] ORS Gateway stopped. Goodbye!" << std::endl;
    return 0;
}

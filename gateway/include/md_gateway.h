#pragma once

#include <memory>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <atomic>
#include <thread>
#include <vector>
#include <shared_mutex>
#include <grpcpp/grpcpp.h>

// NATS是可选依赖
#ifdef ENABLE_NATS
#include <nats/nats.h>
#endif

#include "market_data.grpc.pb.h"

namespace hft {
namespace gateway {

// 订单簿快照
struct OrderBook {
    struct Level {
        double price;
        uint32_t qty;
        uint32_t order_count;
    };

    std::string symbol;
    std::string exchange;
    std::vector<Level> bids;
    std::vector<Level> asks;
    uint64_t last_update_time;

    void Update(const hft::md::MarketDataUpdate& md);
    void GetSnapshot(hft::md::MarketDataUpdate* snapshot) const;
};

// MD Gateway配置
struct MDGatewayConfig {
    std::string grpc_listen_addr = "0.0.0.0:50051";
    std::string nats_url = "nats://localhost:4222";
    int max_depth = 10;
    bool enable_nats = true;
    bool enable_grpc = true;

    // 性能选项
    int grpc_max_concurrent_streams = 1000;
    int nats_batch_size = 100;
    int nats_batch_timeout_ms = 10;
};

// MD Gateway服务实现
class MDGatewayImpl final : public hft::md::MDGateway::Service {
public:
    explicit MDGatewayImpl(const MDGatewayConfig& config);
    ~MDGatewayImpl() override;

    // gRPC接口实现
    grpc::Status SubscribeMarketData(
        grpc::ServerContext* context,
        const hft::md::SubscribeRequest* request,
        grpc::ServerWriter<hft::md::MarketDataUpdate>* writer) override;

    grpc::Status UnsubscribeMarketData(
        grpc::ServerContext* context,
        const hft::md::UnsubscribeRequest* request,
        hft::md::SubscribeResponse* response) override;

    grpc::Status GetSnapshot(
        grpc::ServerContext* context,
        const hft::md::SnapshotRequest* request,
        hft::md::MarketDataUpdate* response) override;

    // 启动和停止
    void Start();
    void Stop();

    // 推送行情更新（从共享内存或其他来源）
    void PushMarketData(const hft::md::MarketDataUpdate& md);

private:
    // NATS相关
    void InitNATS();
    void PublishToNATS(const hft::md::MarketDataUpdate& md);
    void NATSPublishThread();

    // 订单簿管理
    void UpdateOrderBook(const hft::md::MarketDataUpdate& md);
    OrderBook* GetOrderBook(const std::string& symbol);

    // 订阅管理
    void AddSubscription(const std::string& symbol,
                        grpc::ServerWriter<hft::md::MarketDataUpdate>* writer);
    void RemoveSubscription(const std::string& symbol,
                           grpc::ServerWriter<hft::md::MarketDataUpdate>* writer);

    // 配置
    MDGatewayConfig m_config;

    // 订单簿缓存
    std::unordered_map<std::string, std::unique_ptr<OrderBook>> m_orderbooks;
    mutable std::shared_mutex m_orderbook_mutex;

    // gRPC订阅管理
    struct Subscription {
        grpc::ServerWriter<hft::md::MarketDataUpdate>* writer;
        uint64_t subscribe_time;
    };
    std::unordered_map<std::string, std::vector<Subscription>> m_subscriptions;
    mutable std::shared_mutex m_subscription_mutex;

    // NATS（可选）
#ifdef ENABLE_NATS
    natsConnection* m_nats_conn = nullptr;
    std::thread m_nats_thread;
#endif
    std::atomic<bool> m_running{false};

    // 性能统计
    std::atomic<uint64_t> m_md_count{0};
    std::atomic<uint64_t> m_last_latency_ns{0};
};

// MD Gateway主类
class MDGateway {
public:
    explicit MDGateway(const MDGatewayConfig& config);
    ~MDGateway();

    void Run();
    void Shutdown();

    // 推送行情数据（供外部调用）
    void PushMarketData(const hft::md::MarketDataUpdate& md);

private:
    MDGatewayConfig m_config;
    std::unique_ptr<MDGatewayImpl> m_service;
    std::unique_ptr<grpc::Server> m_grpc_server;
    std::atomic<bool> m_running{false};
};

} // namespace gateway
} // namespace hft

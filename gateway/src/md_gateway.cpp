#include "md_gateway.h"
#include <iostream>
#include <chrono>
#include <sstream>

namespace hft {
namespace gateway {

// OrderBook实现
void OrderBook::Update(const hft::md::MarketDataUpdate& md) {
    symbol = md.symbol();
    exchange = md.exchange();
    last_update_time = md.timestamp();

    // 更新买盘
    bids.clear();
    for (int i = 0; i < md.bid_price_size() && i < 10; ++i) {
        bids.push_back({
            md.bid_price(i),
            md.bid_qty(i),
            i < md.bid_order_count_size() ? md.bid_order_count(i) : 0u
        });
    }

    // 更新卖盘
    asks.clear();
    for (int i = 0; i < md.ask_price_size() && i < 10; ++i) {
        asks.push_back({
            md.ask_price(i),
            md.ask_qty(i),
            i < md.ask_order_count_size() ? md.ask_order_count(i) : 0u
        });
    }
}

void OrderBook::GetSnapshot(hft::md::MarketDataUpdate* snapshot) const {
    snapshot->set_symbol(symbol);
    snapshot->set_exchange(exchange);
    snapshot->set_timestamp(last_update_time);

    for (const auto& bid : bids) {
        snapshot->add_bid_price(bid.price);
        snapshot->add_bid_qty(bid.qty);
        snapshot->add_bid_order_count(bid.order_count);
    }

    for (const auto& ask : asks) {
        snapshot->add_ask_price(ask.price);
        snapshot->add_ask_qty(ask.qty);
        snapshot->add_ask_order_count(ask.order_count);
    }
}

// MDGatewayImpl实现
MDGatewayImpl::MDGatewayImpl(const MDGatewayConfig& config)
    : m_config(config) {
#ifdef ENABLE_NATS
    if (m_config.enable_nats) {
        InitNATS();
    }
#else
    if (m_config.enable_nats) {
        std::cerr << "[MDGateway] WARNING: NATS requested but not compiled in" << std::endl;
        m_config.enable_nats = false;
    }
#endif
}

MDGatewayImpl::~MDGatewayImpl() {
    Stop();
}

void MDGatewayImpl::Start() {
    m_running.store(true);

#ifdef ENABLE_NATS
    if (m_config.enable_nats) {
        m_nats_thread = std::thread([this]() { NATSPublishThread(); });
    }
#endif

    std::cout << "[MDGateway] Started successfully" << std::endl;
    std::cout << "[MDGateway] NATS: " << (m_config.enable_nats ? "Enabled" : "Disabled") << std::endl;
}

void MDGatewayImpl::Stop() {
    m_running.store(false);

#ifdef ENABLE_NATS
    if (m_nats_thread.joinable()) {
        m_nats_thread.join();
    }

    if (m_nats_conn) {
        natsConnection_Close(m_nats_conn);
        natsConnection_Destroy(m_nats_conn);
        m_nats_conn = nullptr;
    }
#endif

    std::cout << "[MDGateway] Stopped" << std::endl;
}

void MDGatewayImpl::InitNATS() {
#ifdef ENABLE_NATS
    natsOptions* opts = nullptr;
    natsStatus s;

    s = natsOptions_Create(&opts);
    if (s != NATS_OK) {
        std::cerr << "[MDGateway] Failed to create NATS options: "
                  << natsStatus_GetText(s) << std::endl;
        return;
    }

    natsOptions_SetURL(opts, m_config.nats_url.c_str());
    natsOptions_SetMaxReconnect(opts, 10);
    natsOptions_SetReconnectWait(opts, 1000);

    s = natsConnection_Connect(&m_nats_conn, opts);
    natsOptions_Destroy(opts);

    if (s != NATS_OK) {
        std::cerr << "[MDGateway] Failed to connect to NATS: "
                  << natsStatus_GetText(s) << std::endl;
        m_nats_conn = nullptr;
        return;
    }

    std::cout << "[MDGateway] Connected to NATS: " << m_config.nats_url << std::endl;
#endif
}

void MDGatewayImpl::PublishToNATS(const hft::md::MarketDataUpdate& md) {
#ifdef ENABLE_NATS
    if (!m_nats_conn) {
        static bool warned = false;
        if (!warned) {
            std::cerr << "[MDGateway] NATS connection is null, cannot publish" << std::endl;
            warned = true;
        }
        return;
    }

    // 构造主题: md.{exchange}.{symbol}
    std::string subject = "md." + md.exchange() + "." + md.symbol();

    // 序列化为字符串
    std::string data;
    if (!md.SerializeToString(&data)) {
        std::cerr << "[MDGateway] Failed to serialize MD" << std::endl;
        return;
    }

    // 发布
    natsStatus s = natsConnection_Publish(m_nats_conn, subject.c_str(),
                                          data.c_str(), data.size());
    if (s != NATS_OK) {
        std::cerr << "[MDGateway] Failed to publish to NATS: "
                  << natsStatus_GetText(s) << std::endl;
    } else {
        // 每1000条打印一次成功信息
        static std::atomic<uint64_t> publish_count{0};
        if (++publish_count % 1000 == 0) {
            std::cout << "[MDGateway] Published " << publish_count
                      << " messages to NATS (latest: " << subject << ")" << std::endl;
        }
    }
#else
    (void)md;  // 避免未使用参数警告
#endif
}

void MDGatewayImpl::NATSPublishThread() {
#ifdef ENABLE_NATS
    std::cout << "[MDGateway] NATS publish thread started" << std::endl;

    while (m_running.load()) {
        // 这里可以实现批量发布优化
        std::this_thread::sleep_for(std::chrono::milliseconds(1));
    }

    std::cout << "[MDGateway] NATS publish thread stopped" << std::endl;
#endif
}

void MDGatewayImpl::PushMarketData(const hft::md::MarketDataUpdate& md) {
    auto start = std::chrono::high_resolution_clock::now();

    // 1. 更新订单簿
    UpdateOrderBook(md);

    // 2. 发布到NATS
    if (m_config.enable_nats) {
        PublishToNATS(md);
    }

    // 3. 推送给gRPC订阅者
    {
        std::shared_lock lock(m_subscription_mutex);
        auto it = m_subscriptions.find(md.symbol());
        if (it != m_subscriptions.end()) {
            for (auto& sub : it->second) {
                // 异步写入，避免阻塞
                sub.writer->Write(md);
            }
        }
    }

    // 4. 统计
    auto end = std::chrono::high_resolution_clock::now();
    auto latency = std::chrono::duration_cast<std::chrono::nanoseconds>(end - start).count();
    m_last_latency_ns.store(latency);
    m_md_count.fetch_add(1);

    if (m_md_count.load() % 10000 == 0) {
        std::cout << "[MDGateway] Processed " << m_md_count.load()
                  << " updates, last latency: " << latency << " ns" << std::endl;
    }
}

void MDGatewayImpl::UpdateOrderBook(const hft::md::MarketDataUpdate& md) {
    std::unique_lock lock(m_orderbook_mutex);

    auto it = m_orderbooks.find(md.symbol());
    if (it == m_orderbooks.end()) {
        auto ob = std::make_unique<OrderBook>();
        ob->Update(md);
        m_orderbooks[md.symbol()] = std::move(ob);
    } else {
        it->second->Update(md);
    }
}

OrderBook* MDGatewayImpl::GetOrderBook(const std::string& symbol) {
    std::shared_lock lock(m_orderbook_mutex);
    auto it = m_orderbooks.find(symbol);
    return (it != m_orderbooks.end()) ? it->second.get() : nullptr;
}

grpc::Status MDGatewayImpl::SubscribeMarketData(
    grpc::ServerContext* context,
    const hft::md::SubscribeRequest* request,
    grpc::ServerWriter<hft::md::MarketDataUpdate>* writer) {

    std::cout << "[MDGateway] New subscription from "
              << context->peer() << std::endl;

    // 记录订阅
    for (const auto& symbol : request->symbols()) {
        std::unique_lock lock(m_subscription_mutex);
        m_subscriptions[symbol].push_back({
            writer,
            static_cast<uint64_t>(std::chrono::system_clock::now().time_since_epoch().count())
        });
        std::cout << "[MDGateway] Subscribed to " << symbol << std::endl;

        // 如果有订单簿快照，立即发送
        if (request->full_depth()) {
            auto* ob = GetOrderBook(symbol);
            if (ob) {
                hft::md::MarketDataUpdate snapshot;
                ob->GetSnapshot(&snapshot);
                writer->Write(snapshot);
            }
        }
    }

    // 保持连接，直到客户端断开
    while (!context->IsCancelled()) {
        std::this_thread::sleep_for(std::chrono::milliseconds(100));
    }

    // 清理订阅
    for (const auto& symbol : request->symbols()) {
        std::unique_lock lock(m_subscription_mutex);
        auto& subs = m_subscriptions[symbol];
        subs.erase(std::remove_if(subs.begin(), subs.end(),
            [writer](const Subscription& sub) { return sub.writer == writer; }),
            subs.end());
    }

    std::cout << "[MDGateway] Client disconnected" << std::endl;
    return grpc::Status::OK;
}

grpc::Status MDGatewayImpl::UnsubscribeMarketData(
    grpc::ServerContext* context,
    const hft::md::UnsubscribeRequest* request,
    hft::md::SubscribeResponse* response) {

    std::unique_lock lock(m_subscription_mutex);
    for (const auto& symbol : request->symbols()) {
        m_subscriptions.erase(symbol);
    }

    response->set_status(hft::md::SubscribeResponse::SUCCESS);
    response->set_message("Unsubscribed successfully");
    return grpc::Status::OK;
}

grpc::Status MDGatewayImpl::GetSnapshot(
    grpc::ServerContext* context,
    const hft::md::SnapshotRequest* request,
    hft::md::MarketDataUpdate* response) {

    auto* ob = GetOrderBook(request->symbol());
    if (!ob) {
        return grpc::Status(grpc::StatusCode::NOT_FOUND,
                           "Symbol not found");
    }

    ob->GetSnapshot(response);
    return grpc::Status::OK;
}

// MDGateway实现
MDGateway::MDGateway(const MDGatewayConfig& config)
    : m_config(config) {
    m_service = std::make_unique<MDGatewayImpl>(config);
}

MDGateway::~MDGateway() {
    Shutdown();
}

void MDGateway::Run() {
    m_running.store(true);
    m_service->Start();

    // 构建gRPC服务器
    grpc::ServerBuilder builder;
    builder.AddListeningPort(m_config.grpc_listen_addr,
                            grpc::InsecureServerCredentials());
    builder.RegisterService(m_service.get());
    builder.SetMaxReceiveMessageSize(10 * 1024 * 1024);
    builder.SetMaxSendMessageSize(10 * 1024 * 1024);

    m_grpc_server = builder.BuildAndStart();
    if (!m_grpc_server) {
        std::cerr << "[MDGateway] Failed to start gRPC server" << std::endl;
        return;
    }

    std::cout << "[MDGateway] gRPC server listening on "
              << m_config.grpc_listen_addr << std::endl;

    // 等待服务器关闭
    m_grpc_server->Wait();
}

void MDGateway::Shutdown() {
    if (!m_running.load()) {
        return;
    }

    m_running.store(false);
    m_service->Stop();

    if (m_grpc_server) {
        m_grpc_server->Shutdown();
    }

    std::cout << "[MDGateway] Shutdown complete" << std::endl;
}

void MDGateway::PushMarketData(const hft::md::MarketDataUpdate& md) {
    if (m_service) {
        m_service->PushMarketData(md);
    }
}

} // namespace gateway
} // namespace hft

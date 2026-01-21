#pragma once

#include <atomic>
#include <memory>
#include <string>
#include <unordered_map>
#include <mutex>
#include <queue>

#include <grpcpp/grpcpp.h>
#include "order.grpc.pb.h"

#ifdef ENABLE_NATS
#include <nats/nats.h>
#endif

namespace hft {
namespace ors {

// ============================================================================
// 订单数据结构（共享内存中使用）
// ============================================================================

// 订单请求（写入共享内存）
struct OrderRequestRaw {
    char strategy_id[32];       // 策略ID
    char symbol[16];            // 合约代码
    char exchange[8];           // 交易所

    uint8_t side;               // 买卖方向 (1=买, 2=卖)
    uint8_t order_type;         // 订单类型
    uint8_t time_in_force;      // 时效类型
    uint8_t open_close;         // 开平标志

    double price;               // 价格
    int64_t quantity;           // 数量

    char client_order_id[32];   // 客户端订单ID
    char account[16];           // 账户

    uint64_t client_token;      // 客户端Token
    uint64_t timestamp;         // 时间戳（纳秒）
    uint64_t seq_num;           // 序列号
};

// 订单响应（从共享内存读取）
struct OrderResponseRaw {
    char order_id[32];          // 系统订单ID
    char client_order_id[32];   // 客户端订单ID
    uint64_t client_token;      // 客户端Token

    uint8_t status;             // 订单状态
    uint8_t error_code;         // 错误码

    double price;               // 委托价格
    int64_t quantity;           // 委托数量
    int64_t filled_qty;         // 成交数量

    double avg_price;           // 平均成交价
    double last_fill_price;     // 最新成交价
    int64_t last_fill_qty;      // 最新成交量

    char exec_id[32];           // 成交ID
    char error_msg[128];        // 错误信息

    uint64_t exchange_timestamp;  // 交易所时间戳
    uint64_t timestamp;         // 系统时间戳
    uint64_t seq_num;           // 序列号
};

// ============================================================================
// ORS Gateway服务实现
// ============================================================================

class ORSGatewayImpl final : public hft::ors::ORSGateway::Service {
public:
    ORSGatewayImpl();
    ~ORSGatewayImpl();

    // 初始化（不再管理共享内存）
    bool Initialize(const std::string& config_file = "");

    // 启动和停止
    void Start();
    void Stop();

    // gRPC服务接口实现
    grpc::Status SendOrder(
        grpc::ServerContext* context,
        const OrderRequest* request,
        OrderResponse* response) override;

    grpc::Status CancelOrder(
        grpc::ServerContext* context,
        const CancelRequest* request,
        CancelResponse* response) override;

    grpc::Status QueryOrders(
        grpc::ServerContext* context,
        const OrderQuery* request,
        grpc::ServerWriter<OrderData>* writer) override;

    grpc::Status QueryPosition(
        grpc::ServerContext* context,
        const PositionQuery* request,
        grpc::ServerWriter<PositionData>* writer) override;

    // ========================================================================
    // 新增：外部数据源接口（由main_ors.cpp调用）
    // ========================================================================

    // 获取待发送的订单请求（从内部队列取出）
    bool GetOrderRequest(OrderRequestRaw* raw_req);

    // 推送订单回报（由外部响应队列线程调用）
    void OnOrderResponse(const OrderUpdate& update);

    // 获取统计信息
    struct Statistics {
        std::atomic<uint64_t> total_orders{0};
        std::atomic<uint64_t> accepted_orders{0};
        std::atomic<uint64_t> rejected_orders{0};
        std::atomic<uint64_t> filled_orders{0};
        std::atomic<uint64_t> canceled_orders{0};
        std::atomic<uint64_t> last_latency_ns{0};
    };

    const Statistics& GetStatistics() const { return m_stats; }

private:
    // 订单处理
    bool ValidateOrder(const OrderRequest* request, std::string& error_msg);
    bool CheckRisk(const OrderRequest* request, std::string& error_msg);
    std::string GenerateOrderID();
    uint64_t GenerateClientToken();

    // 转换函数
    void ConvertToRaw(const OrderRequest& proto_req, OrderRequestRaw* raw_req);

    // NATS发布
#ifdef ENABLE_NATS
    void PublishOrderUpdate(const OrderUpdate& update);
    void PublishToNATS(const std::string& subject, const std::string& data);
#endif

    // 订单管理
    struct OrderInfo {
        std::string order_id;
        std::string client_order_id;
        std::string strategy_id;
        std::string symbol;
        OrderStatus status;
        uint64_t timestamp;
    };

    void UpdateOrderBook(const OrderUpdate& update);
    OrderInfo* GetOrder(const std::string& order_id);

    // 内部订单请求队列（缓冲待发送的订单）
    std::queue<OrderRequestRaw> m_pending_requests;
    std::mutex m_pending_requests_mutex;

    // NATS连接
#ifdef ENABLE_NATS
    natsConnection* m_nats_conn;
    natsOptions* m_nats_opts;
    std::string m_nats_url;
#endif

    // 订单簿
    std::unordered_map<std::string, OrderInfo> m_orders;  // order_id -> OrderInfo
    std::unordered_map<std::string, std::string> m_client_order_map;  // client_order_id -> order_id
    std::unordered_map<uint64_t, std::string> m_token_map;  // client_token -> order_id
    mutable std::mutex m_orders_mutex;

    // 状态
    std::atomic<bool> m_running;
    std::atomic<uint64_t> m_order_counter;
    std::atomic<uint64_t> m_token_counter;

    // 统计
    Statistics m_stats;
};

} // namespace ors
} // namespace hft

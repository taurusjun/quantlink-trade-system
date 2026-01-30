#include "ors_gateway.h"

#include <chrono>
#include <cstring>
#include <iostream>
#include <sstream>
#include <iomanip>

namespace hft {
namespace ors {

namespace {
// 辅助转换函数（供main_ors.cpp使用）
void ConvertToProtobuf(const OrderResponseRaw& raw_resp, OrderUpdate* proto_update) {
    proto_update->set_order_id(raw_resp.order_id);
    proto_update->set_client_order_id(raw_resp.client_order_id);
    proto_update->set_status(static_cast<OrderStatus>(raw_resp.status));

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

    proto_update->set_error_code(static_cast<ErrorCode>(raw_resp.error_code));
    proto_update->set_error_msg(raw_resp.error_msg);
}
} // namespace

// ============================================================================
// 构造和析构
// ============================================================================

ORSGatewayImpl::ORSGatewayImpl()
#ifdef ENABLE_NATS
    : m_nats_conn(nullptr)
    , m_nats_opts(nullptr)
    , m_nats_url("nats://localhost:4222")
    , m_running(false)
#else
    : m_running(false)
#endif
    , m_order_counter(0)
    , m_token_counter(0)
{
}

ORSGatewayImpl::~ORSGatewayImpl() {
    Stop();
}

// ============================================================================
// 初始化
// ============================================================================

bool ORSGatewayImpl::Initialize(const std::string& config_file) {
    std::cout << "[ORSGateway] Initializing..." << std::endl;

    // TODO: 从配置文件读取配置
    (void)config_file;

#ifdef ENABLE_NATS
    // 初始化NATS连接
    natsOptions_Create(&m_nats_opts);
    natsOptions_SetURL(m_nats_opts, m_nats_url.c_str());

    natsStatus s = natsConnection_Connect(&m_nats_conn, m_nats_opts);
    if (s != NATS_OK) {
        std::cerr << "[ORSGateway] Failed to connect to NATS: "
                  << natsStatus_GetText(s) << std::endl;
        return false;
    }
    std::cout << "[ORSGateway] Connected to NATS: " << m_nats_url << std::endl;
#else
    std::cout << "[ORSGateway] NATS support disabled" << std::endl;
#endif

    std::cout << "[ORSGateway] Initialization complete" << std::endl;
    return true;
}

// ============================================================================
// 启动和停止
// ============================================================================

void ORSGatewayImpl::Start() {
    m_running = true;
    std::cout << "[ORSGateway] Started successfully" << std::endl;
}

void ORSGatewayImpl::Stop() {
    if (m_running.exchange(false)) {
        std::cout << "[ORSGateway] Stopping..." << std::endl;

#ifdef ENABLE_NATS
        // 关闭NATS连接
        if (m_nats_conn) {
            natsConnection_Close(m_nats_conn);
            natsConnection_Destroy(m_nats_conn);
            m_nats_conn = nullptr;
        }

        if (m_nats_opts) {
            natsOptions_Destroy(m_nats_opts);
            m_nats_opts = nullptr;
        }
#endif

        std::cout << "[ORSGateway] Stopped" << std::endl;
    }
}

// ============================================================================
// gRPC服务接口实现
// ============================================================================

grpc::Status ORSGatewayImpl::SendOrder(
    grpc::ServerContext* context,
    const OrderRequest* request,
    OrderResponse* response) {

    auto start_time = std::chrono::high_resolution_clock::now();

    // 1. 参数校验
    std::string error_msg;
    if (!ValidateOrder(request, error_msg)) {
        response->set_error_code(ErrorCode::INVALID_PARAMETER);
        response->set_error_msg(error_msg);
        m_stats.rejected_orders++;
        return grpc::Status::OK;
    }

    // 2. 风控检查
    if (!CheckRisk(request, error_msg)) {
        response->set_error_code(ErrorCode::RISK_CHECK_FAILED);
        response->set_error_msg(error_msg);
        m_stats.rejected_orders++;
        return grpc::Status::OK;
    }

    // 3. 生成订单ID和Token
    std::string order_id = GenerateOrderID();
    uint64_t client_token = GenerateClientToken();

    // 4. 转换为原始格式
    OrderRequestRaw raw_req;
    std::memset(&raw_req, 0, sizeof(raw_req));
    ConvertToRaw(*request, &raw_req);
    raw_req.client_token = client_token;
    raw_req.timestamp = std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count();

    // 5. 推送到内部队列（由main_ors.cpp线程写入共享内存）
    {
        std::lock_guard<std::mutex> lock(m_pending_requests_mutex);
        m_pending_requests.push(raw_req);
    }

    // 6. 记录订单映射
    {
        std::lock_guard<std::mutex> lock(m_orders_mutex);
        m_token_map[client_token] = order_id;
        if (!request->client_order_id().empty()) {
            m_client_order_map[request->client_order_id()] = order_id;
        }

        OrderInfo info;
        info.order_id = order_id;
        info.client_order_id = request->client_order_id();
        info.strategy_id = request->strategy_id();
        info.symbol = request->symbol();
        info.status = OrderStatus::SUBMITTED;
        info.timestamp = raw_req.timestamp;
        m_orders[order_id] = info;
    }

    // 7. 返回响应
    response->set_order_id(order_id);
    response->set_client_order_id(request->client_order_id());
    response->set_client_token(client_token);
    response->set_error_code(ErrorCode::SUCCESS);
    response->set_timestamp(raw_req.timestamp);

    auto end_time = std::chrono::high_resolution_clock::now();
    auto latency_ns = std::chrono::duration_cast<std::chrono::nanoseconds>(
        end_time - start_time).count();
    m_stats.last_latency_ns = latency_ns;
    m_stats.total_orders++;

    std::cout << "[ORSGateway] SendOrder: " << order_id
              << " symbol=" << request->symbol()
              << " side=" << (request->side() == OrderSide::BUY ? "BUY" : "SELL")
              << " price=" << request->price()
              << " qty=" << request->quantity()
              << " latency=" << latency_ns << "ns"
              << std::endl;

    return grpc::Status::OK;
}

grpc::Status ORSGatewayImpl::CancelOrder(
    grpc::ServerContext* context,
    const CancelRequest* request,
    CancelResponse* response) {

    std::string order_id;

    // 根据订单ID或客户端订单ID查找
    {
        std::lock_guard<std::mutex> lock(m_orders_mutex);
        if (!request->order_id().empty()) {
            order_id = request->order_id();
        } else if (!request->client_order_id().empty()) {
            auto it = m_client_order_map.find(request->client_order_id());
            if (it != m_client_order_map.end()) {
                order_id = it->second;
            }
        }
    }

    if (order_id.empty()) {
        response->set_error_code(ErrorCode::ORDER_NOT_FOUND);
        response->set_error_msg("Order not found");
        return grpc::Status::OK;
    }

    // TODO: 实现撤单逻辑（写入撤单请求到共享内存）

    response->set_order_id(order_id);
    response->set_client_order_id(request->client_order_id());
    response->set_error_code(ErrorCode::SUCCESS);
    response->set_timestamp(std::chrono::duration_cast<std::chrono::nanoseconds>(
        std::chrono::system_clock::now().time_since_epoch()).count());

    std::cout << "[ORSGateway] CancelOrder: " << order_id << std::endl;

    return grpc::Status::OK;
}

grpc::Status ORSGatewayImpl::QueryOrders(
    grpc::ServerContext* context,
    const OrderQuery* request,
    grpc::ServerWriter<OrderData>* writer) {

    std::lock_guard<std::mutex> lock(m_orders_mutex);

    for (const auto& pair : m_orders) {
        const OrderInfo& info = pair.second;

        // 过滤条件
        if (!request->strategy_id().empty() && info.strategy_id != request->strategy_id()) {
            continue;
        }
        if (!request->symbol().empty() && info.symbol != request->symbol()) {
            continue;
        }

        // 构造返回数据
        OrderData data;
        OrderUpdate* update = data.mutable_order();
        update->set_order_id(info.order_id);
        update->set_client_order_id(info.client_order_id);
        update->set_strategy_id(info.strategy_id);
        update->set_symbol(info.symbol);
        update->set_status(info.status);
        update->set_timestamp(info.timestamp);

        writer->Write(data);
    }

    return grpc::Status::OK;
}

grpc::Status ORSGatewayImpl::QueryPosition(
    grpc::ServerContext* context,
    const PositionQuery* request,
    grpc::ServerWriter<PositionData>* writer) {

    std::cout << "[ORS Gateway] Position query received" << std::endl;

    // 注意：当前简化实现，直接返回空结果
    // 完整实现需要通过共享内存与Counter Bridge通信
    // 或者由Counter Bridge通过HTTP/gRPC暴露查询接口
    //
    // Phase 2简化方案：
    // 1. ORS Gateway调用本地函数查询（如果Counter Bridge是同一进程）
    // 2. 或者返回错误，提示使用其他查询方式

    std::cout << "[ORS Gateway] Position query not yet implemented via gRPC" << std::endl;
    std::cout << "[ORS Gateway] Use Trader's internal position tracking instead" << std::endl;

    // 返回空结果（表示查询成功但无持仓）
    return grpc::Status::OK;
}

// ============================================================================
// 外部数据源接口（由main_ors.cpp调用）
// ============================================================================

bool ORSGatewayImpl::GetOrderRequest(OrderRequestRaw* raw_req) {
    std::lock_guard<std::mutex> lock(m_pending_requests_mutex);
    if (m_pending_requests.empty()) {
        return false;
    }
    *raw_req = m_pending_requests.front();
    m_pending_requests.pop();
    return true;
}

void ORSGatewayImpl::OnOrderResponse(const OrderUpdate& update) {
    // 更新订单簿
    UpdateOrderBook(update);

    // 发布到NATS
#ifdef ENABLE_NATS
    PublishOrderUpdate(update);
#endif
}

// ============================================================================
// 订单处理辅助函数
// ============================================================================

bool ORSGatewayImpl::ValidateOrder(const OrderRequest* request, std::string& error_msg) {
    if (request->symbol().empty()) {
        error_msg = "Symbol is empty";
        return false;
    }

    if (request->quantity() <= 0) {
        error_msg = "Quantity must be positive";
        return false;
    }

    if (request->order_type() == OrderType::LIMIT && request->price() <= 0) {
        error_msg = "Limit order must have positive price";
        return false;
    }

    return true;
}

bool ORSGatewayImpl::CheckRisk(const OrderRequest* request, std::string& error_msg) {
    // TODO: 实现风控检查
    // - 订单量限制
    // - 流控限制
    // - 自成交检查
    // - 仓位限制
    (void)request;
    (void)error_msg;
    return true;
}

std::string ORSGatewayImpl::GenerateOrderID() {
    auto now = std::chrono::system_clock::now();
    auto timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
        now.time_since_epoch()).count();

    uint64_t counter = m_order_counter.fetch_add(1);

    std::ostringstream oss;
    oss << "ORD_" << timestamp << "_" << std::setfill('0') << std::setw(6) << counter;
    return oss.str();
}

uint64_t ORSGatewayImpl::GenerateClientToken() {
    return m_token_counter.fetch_add(1);
}

// ============================================================================
// 转换函数
// ============================================================================

void ORSGatewayImpl::ConvertToRaw(const OrderRequest& proto_req, OrderRequestRaw* raw_req) {
    std::strncpy(raw_req->strategy_id, proto_req.strategy_id().c_str(), sizeof(raw_req->strategy_id) - 1);
    std::strncpy(raw_req->symbol, proto_req.symbol().c_str(), sizeof(raw_req->symbol) - 1);
    std::strncpy(raw_req->exchange,
                 proto_req.exchange() == hft::common::Exchange::SHFE ? "SHFE" :
                 proto_req.exchange() == hft::common::Exchange::DCE ? "DCE" :
                 proto_req.exchange() == hft::common::Exchange::CZCE ? "CZCE" : "UNKNOWN",
                 sizeof(raw_req->exchange) - 1);

    raw_req->side = static_cast<uint8_t>(proto_req.side());
    raw_req->order_type = static_cast<uint8_t>(proto_req.order_type());
    raw_req->time_in_force = static_cast<uint8_t>(proto_req.time_in_force());
    raw_req->open_close = static_cast<uint8_t>(proto_req.open_close());

    raw_req->price = proto_req.price();
    raw_req->quantity = proto_req.quantity();

    std::strncpy(raw_req->client_order_id, proto_req.client_order_id().c_str(),
                 sizeof(raw_req->client_order_id) - 1);
    std::strncpy(raw_req->account, proto_req.account().c_str(),
                 sizeof(raw_req->account) - 1);
}


// ============================================================================
// NATS发布
// ============================================================================

#ifdef ENABLE_NATS
void ORSGatewayImpl::PublishOrderUpdate(const OrderUpdate& update) {
    std::string subject = "order." + update.strategy_id() + "." + update.order_id();

    std::string data;
    if (!update.SerializeToString(&data)) {
        std::cerr << "[ORSGateway] Failed to serialize order update" << std::endl;
        return;
    }

    std::cout << "[ORSGateway] Publishing order update: " << subject
              << " status=" << static_cast<int>(update.status()) << std::endl;

    PublishToNATS(subject, data);
}

void ORSGatewayImpl::PublishToNATS(const std::string& subject, const std::string& data) {
    if (!m_nats_conn) {
        return;
    }

    natsStatus s = natsConnection_Publish(m_nats_conn, subject.c_str(),
                                          data.c_str(), data.size());
    if (s != NATS_OK) {
        std::cerr << "[ORSGateway] Failed to publish to NATS: "
                  << natsStatus_GetText(s) << std::endl;
    }
}
#endif

// ============================================================================
// 订单管理
// ============================================================================

void ORSGatewayImpl::UpdateOrderBook(const OrderUpdate& update) {
    std::lock_guard<std::mutex> lock(m_orders_mutex);

    auto it = m_orders.find(update.order_id());
    if (it != m_orders.end()) {
        it->second.status = update.status();
        it->second.timestamp = update.timestamp();

        // 更新统计
        if (update.status() == OrderStatus::ACCEPTED) {
            m_stats.accepted_orders++;
        } else if (update.status() == OrderStatus::FILLED) {
            m_stats.filled_orders++;
        } else if (update.status() == OrderStatus::CANCELED) {
            m_stats.canceled_orders++;
        } else if (update.status() == OrderStatus::REJECTED) {
            m_stats.rejected_orders++;
        }
    }
}

ORSGatewayImpl::OrderInfo* ORSGatewayImpl::GetOrder(const std::string& order_id) {
    std::lock_guard<std::mutex> lock(m_orders_mutex);
    auto it = m_orders.find(order_id);
    return (it != m_orders.end()) ? &it->second : nullptr;
}

} // namespace ors
} // namespace hft

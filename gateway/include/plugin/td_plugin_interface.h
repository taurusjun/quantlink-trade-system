#pragma once

#include <string>
#include <vector>
#include <cstdint>
#include <functional>

namespace hft {
namespace plugin {

// ==================== 数据结构定义 ====================

/**
 * 订单方向
 */
enum class OrderDirection {
    BUY = 0,    // 买入
    SELL = 1    // 卖出
};

/**
 * 开平标志
 */
enum class OffsetFlag {
    OPEN = 0,           // 开仓
    CLOSE = 1,          // 平仓
    CLOSE_TODAY = 2,    // 平今
    CLOSE_YESTERDAY = 3 // 平昨
};

/**
 * 订单状态
 */
enum class OrderStatus {
    UNKNOWN = 0,        // 未知
    SUBMITTING = 1,     // 提交中
    SUBMITTED = 2,      // 已提交
    ACCEPTED = 3,       // 已接受
    PARTIAL_FILLED = 4, // 部分成交
    FILLED = 5,         // 全部成交
    CANCELING = 6,      // 撤单中
    CANCELED = 7,       // 已撤单
    REJECTED = 8,       // 已拒绝
    ERROR = 9           // 错误
};

/**
 * 价格类型
 */
enum class PriceType {
    LIMIT = 0,          // 限价单
    MARKET = 1,         // 市价单
    BEST = 2            // 最优价
};

/**
 * 订单请求
 */
struct OrderRequest {
    char symbol[32];            // 合约代码（如"ag2603"）
    char exchange[16];          // 交易所代码（如"SHFE"）
    OrderDirection direction;   // 买卖方向
    OffsetFlag offset;          // 开平标志
    PriceType price_type;       // 价格类型
    double price;               // 价格（限价单必填）
    uint32_t volume;            // 数量
    char client_order_id[64];   // 客户端订单ID（可选，用于跟踪）

    OrderRequest() {
        symbol[0] = '\0';
        exchange[0] = '\0';
        direction = OrderDirection::BUY;
        offset = OffsetFlag::OPEN;
        price_type = PriceType::LIMIT;
        price = 0.0;
        volume = 0;
        client_order_id[0] = '\0';
    }
};

/**
 * 订单信息
 */
struct OrderInfo {
    char order_id[64];          // 系统订单ID
    char client_order_id[64];   // 客户端订单ID
    char symbol[32];            // 合约代码
    char exchange[16];          // 交易所代码
    OrderDirection direction;   // 买卖方向
    OffsetFlag offset;          // 开平标志
    PriceType price_type;       // 价格类型
    double price;               // 价格
    uint32_t volume;            // 总数量
    uint32_t traded_volume;     // 已成交数量
    OrderStatus status;         // 订单状态
    int64_t insert_time;        // 报单时间（纳秒时间戳）
    int64_t update_time;        // 更新时间（纳秒时间戳）
    char status_msg[256];       // 状态信息

    OrderInfo() {
        order_id[0] = '\0';
        client_order_id[0] = '\0';
        symbol[0] = '\0';
        exchange[0] = '\0';
        direction = OrderDirection::BUY;
        offset = OffsetFlag::OPEN;
        price_type = PriceType::LIMIT;
        price = 0.0;
        volume = 0;
        traded_volume = 0;
        status = OrderStatus::UNKNOWN;
        insert_time = 0;
        update_time = 0;
        status_msg[0] = '\0';
    }
};

/**
 * 成交信息
 */
struct TradeInfo {
    char trade_id[64];          // 成交ID
    char order_id[64];          // 订单ID
    char symbol[32];            // 合约代码
    char exchange[16];          // 交易所代码
    OrderDirection direction;   // 买卖方向
    OffsetFlag offset;          // 开平标志
    double price;               // 成交价格
    uint32_t volume;            // 成交数量
    int64_t trade_time;         // 成交时间（纳秒时间戳）

    TradeInfo() {
        trade_id[0] = '\0';
        order_id[0] = '\0';
        symbol[0] = '\0';
        exchange[0] = '\0';
        direction = OrderDirection::BUY;
        offset = OffsetFlag::OPEN;
        price = 0.0;
        volume = 0;
        trade_time = 0;
    }
};

/**
 * 持仓信息
 */
struct PositionInfo {
    char symbol[32];            // 合约代码
    char exchange[16];          // 交易所代码
    OrderDirection direction;   // 持仓方向（多头/空头）
    uint32_t volume;            // 持仓数量
    uint32_t today_volume;      // 今日持仓
    uint32_t yesterday_volume;  // 昨日持仓
    double avg_price;           // 持仓均价
    double position_profit;     // 持仓盈亏
    double margin;              // 占用保证金

    PositionInfo() {
        symbol[0] = '\0';
        exchange[0] = '\0';
        direction = OrderDirection::BUY;
        volume = 0;
        today_volume = 0;
        yesterday_volume = 0;
        avg_price = 0.0;
        position_profit = 0.0;
        margin = 0.0;
    }
};

/**
 * 资金账户信息
 */
struct AccountInfo {
    char account_id[32];        // 账户ID
    double balance;             // 账户余额
    double available;           // 可用资金
    double margin;              // 占用保证金
    double frozen_margin;       // 冻结保证金
    double commission;          // 手续费
    double close_profit;        // 平仓盈亏
    double position_profit;     // 持仓盈亏

    AccountInfo() {
        account_id[0] = '\0';
        balance = 0.0;
        available = 0.0;
        margin = 0.0;
        frozen_margin = 0.0;
        commission = 0.0;
        close_profit = 0.0;
        position_profit = 0.0;
    }
};

// ==================== 回调函数类型定义 ====================

/**
 * 订单回报回调
 * @param order 订单信息
 */
using OrderCallback = std::function<void(const OrderInfo& order)>;

/**
 * 成交回报回调
 * @param trade 成交信息
 */
using TradeCallback = std::function<void(const TradeInfo& trade)>;

/**
 * 错误通知回调
 * @param error_id 错误代码
 * @param error_msg 错误信息
 */
using ErrorCallback = std::function<void(int error_id, const std::string& error_msg)>;

// ==================== 交易插件统一接口 ====================

/**
 * 交易插件统一接口
 * 所有交易插件（CTP、XTP、飞马等）必须实现此接口
 */
class ITDPlugin {
public:
    virtual ~ITDPlugin() = default;

    // ==================== 生命周期管理 ====================

    /**
     * 初始化插件
     * @param config_file 配置文件路径（YAML格式）
     * @return 成功返回true，失败返回false
     */
    virtual bool Initialize(const std::string& config_file) = 0;

    /**
     * 连接并登录
     * @return 成功返回true，失败返回false
     */
    virtual bool Login() = 0;

    /**
     * 登出并断开连接
     */
    virtual void Logout() = 0;

    /**
     * 查询是否已登录
     * @return 已登录返回true，否则返回false
     */
    virtual bool IsLoggedIn() const = 0;

    /**
     * 查询是否已连接
     * @return 已连接返回true，否则返回false
     */
    virtual bool IsConnected() const = 0;

    // ==================== 交易功能 ====================

    /**
     * 发送订单
     * @param request 订单请求
     * @return 成功返回订单ID，失败返回空字符串
     */
    virtual std::string SendOrder(const OrderRequest& request) = 0;

    /**
     * 撤销订单
     * @param order_id 订单ID（系统订单ID或客户端订单ID）
     * @return 成功返回true，失败返回false
     */
    virtual bool CancelOrder(const std::string& order_id) = 0;

    // ==================== 查询功能 ====================

    /**
     * 查询资金账户
     * @param account_info 输出参数，账户信息
     * @return 成功返回true，失败返回false
     */
    virtual bool QueryAccount(AccountInfo& account_info) = 0;

    /**
     * 查询持仓
     * @param positions 输出参数，持仓列表
     * @return 成功返回true，失败返回false
     */
    virtual bool QueryPositions(std::vector<PositionInfo>& positions) = 0;

    /**
     * 查询订单
     * @param orders 输出参数，订单列表
     * @return 成功返回true，失败返回false
     */
    virtual bool QueryOrders(std::vector<OrderInfo>& orders) = 0;

    /**
     * 查询成交
     * @param trades 输出参数，成交列表
     * @return 成功返回true，失败返回false
     */
    virtual bool QueryTrades(std::vector<TradeInfo>& trades) = 0;

    /**
     * 根据订单ID查询订单
     * @param order_id 订单ID
     * @param order_info 输出参数，订单信息
     * @return 成功返回true，失败返回false
     */
    virtual bool GetOrder(const std::string& order_id, OrderInfo& order_info) = 0;

    // ==================== 回调注册 ====================

    /**
     * 注册订单回报回调
     * @param callback 回调函数
     */
    virtual void RegisterOrderCallback(OrderCallback callback) = 0;

    /**
     * 注册成交回报回调
     * @param callback 回调函数
     */
    virtual void RegisterTradeCallback(TradeCallback callback) = 0;

    /**
     * 注册错误通知回调
     * @param callback 回调函数
     */
    virtual void RegisterErrorCallback(ErrorCallback callback) = 0;

    // ==================== 状态查询 ====================

    /**
     * 获取插件名称
     * @return 插件名称（如 "CTP", "XTP", "FEMAS"）
     */
    virtual std::string GetPluginName() const = 0;

    /**
     * 获取插件版本
     * @return 版本号（如 "1.0.0"）
     */
    virtual std::string GetPluginVersion() const = 0;

    /**
     * 获取可用资金
     * @return 可用资金金额
     */
    virtual double GetAvailableFund() const = 0;

    // ==================== 统计信息 ====================

    /**
     * 获取今日发送订单数量
     * @return 订单数量
     */
    virtual uint64_t GetOrderCount() const = 0;

    /**
     * 获取今日成交数量
     * @return 成交数量
     */
    virtual uint64_t GetTradeCount() const = 0;
};

} // namespace plugin
} // namespace hft

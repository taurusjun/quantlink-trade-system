#pragma once

#include <string>
#include <functional>
#include <cstdint>

#include "ors_gateway.h"

namespace hft {
namespace counter {

// ============================================================================
// Counter 抽象接口（支持多柜台：EES/CTP/模拟等）
// ============================================================================

// Counter 回调接口
class ICounterCallback {
public:
    virtual ~ICounterCallback() = default;

    // 订单接受
    virtual void OnOrderAccept(const std::string& strategy_id,
                               const std::string& order_id,
                               const std::string& exchange_order_id) = 0;

    // 订单拒绝
    virtual void OnOrderReject(const std::string& strategy_id,
                              const std::string& order_id,
                              uint8_t error_code,
                              const std::string& error_msg) = 0;

    // 订单成交
    virtual void OnOrderFilled(const std::string& strategy_id,
                              const std::string& order_id,
                              const std::string& exec_id,
                              double price,
                              int64_t quantity,
                              int64_t filled_qty) = 0;

    // 订单撤销确认
    virtual void OnOrderCanceled(const std::string& strategy_id,
                                const std::string& order_id) = 0;
};

// Counter API 抽象接口
class ICounterAPI {
public:
    virtual ~ICounterAPI() = default;

    // 连接管理
    virtual bool Connect() = 0;
    virtual void Disconnect() = 0;
    virtual bool IsConnected() const = 0;

    // 订单操作
    virtual int SendOrder(const hft::ors::OrderRequestRaw& req,
                         std::string& order_id) = 0;

    virtual int CancelOrder(const std::string& order_id) = 0;

    // 查询接口
    virtual int QueryPosition(const std::string& symbol,
                             hft::ors::OrderResponseRaw* position) = 0;

    // 回调设置
    virtual void SetCallback(ICounterCallback* callback) = 0;

    // 获取柜台类型
    virtual std::string GetCounterType() const = 0;
};

// ============================================================================
// Counter 工厂
// ============================================================================

class CounterFactory {
public:
    // 创建 Counter API
    static ICounterAPI* CreateCounter(const std::string& type);

    // 支持的柜台类型
    enum class CounterType {
        SIMULATED,  // 模拟柜台（用于测试）
        EES,        // 盛立EES
        CTP,        // 上期CTP
    };
};

} // namespace counter
} // namespace hft

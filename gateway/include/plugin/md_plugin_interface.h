#pragma once

#include <string>
#include <vector>
#include <cstdint>

namespace hft {
namespace plugin {

/**
 * 行情插件统一接口
 * 所有行情插件（CTP、XTP、飞马等）必须实现此接口
 */
class IMDPlugin {
public:
    virtual ~IMDPlugin() = default;

    // ==================== 生命周期管理 ====================

    /**
     * 初始化插件
     * @param config_file 配置文件路径（YAML格式）
     * @return 成功返回true，失败返回false
     */
    virtual bool Initialize(const std::string& config_file) = 0;

    /**
     * 启动插件（连接到交易所/柜台）
     * @return 成功返回true，失败返回false
     */
    virtual bool Start() = 0;

    /**
     * 停止插件（断开连接，释放资源）
     */
    virtual void Stop() = 0;

    /**
     * 查询插件是否正在运行
     * @return 运行中返回true，否则返回false
     */
    virtual bool IsRunning() const = 0;

    // ==================== 订阅管理 ====================

    /**
     * 订阅行情
     * @param symbols 合约列表（如 ["ag2505", "rb2505"]）
     * @return 成功返回true，失败返回false
     */
    virtual bool Subscribe(const std::vector<std::string>& symbols) = 0;

    /**
     * 取消订阅
     * @param symbols 合约列表
     * @return 成功返回true，失败返回false
     */
    virtual bool Unsubscribe(const std::vector<std::string>& symbols) = 0;

    // ==================== 状态查询 ====================

    /**
     * 查询是否已连接到交易所
     * @return 已连接返回true，否则返回false
     */
    virtual bool IsConnected() const = 0;

    /**
     * 查询是否已登录
     * @return 已登录返回true，否则返回false
     */
    virtual bool IsLoggedIn() const = 0;

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

    // ==================== 统计信息 ====================

    /**
     * 获取已接收的行情消息数量
     * @return 消息数量
     */
    virtual uint64_t GetMessageCount() const = 0;

    /**
     * 获取因队列满而丢弃的消息数量
     * @return 丢弃数量
     */
    virtual uint64_t GetDroppedCount() const = 0;
};

} // namespace plugin
} // namespace hft

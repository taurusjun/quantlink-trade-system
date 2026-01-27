#pragma once

#include <string>
#include <vector>

namespace hft {
namespace gateway {

/**
 * CTP交易配置
 */
struct CTPTDConfig {
    // 连接信息
    std::string front_addr;         // 交易前置地址（如 "tcp://180.168.146.187:10201"）
    std::string broker_id;          // 经纪商代码（如 "9999"）

    // 用户信息
    std::string user_id;            // 用户ID
    std::string password;           // 密码
    std::string investor_id;        // 投资者ID（通常与user_id相同）

    // 终端认证信息（看穿式前置必需）
    std::string app_id;             // 应用ID
    std::string auth_code;          // 授权码

    // 产品信息
    std::string product_info;       // 产品信息

    // 重连配置
    int reconnect_interval_sec = 5;     // 重连间隔（秒）
    int max_reconnect_attempts = -1;    // 最大重连次数（-1表示无限重连）

    // 日志配置
    std::string log_level = "info";     // 日志级别：debug/info/warn/error
    std::string log_file;               // 日志文件路径
    bool log_to_console = true;         // 是否输出到控制台

    // 查询配置
    int query_interval_ms = 1000;       // 查询间隔（毫秒）

    // 默认构造函数
    CTPTDConfig() = default;

    /**
     * 从YAML文件加载配置
     * @param config_file 主配置文件路径
     * @param secret_file 密码配置文件路径（可选）
     * @return 成功返回true，失败返回false
     */
    bool LoadFromYaml(const std::string& config_file,
                      const std::string& secret_file = "");

    /**
     * 验证配置有效性
     * @param error 输出参数，错误信息
     * @return 有效返回true，无效返回false
     */
    bool Validate(std::string* error = nullptr) const;

    /**
     * 打印配置信息（隐藏敏感信息）
     */
    void Print() const;
};

} // namespace gateway
} // namespace hft

#pragma once

#include <string>
#include <vector>

namespace hft {
namespace gateway {

// CTP行情网关配置
struct CTPMDConfig {
    // CTP连接配置
    std::string front_addr = "tcp://180.168.146.187:10211";  // 行情前置地址
    std::string broker_id = "9999";                          // 经纪商代码
    std::string user_id;                                     // 用户名
    std::string password;                                    // 密码

    // 终端认证（看穿式前置必需）
    std::string app_id = "simnow_client_test";               // 应用ID
    std::string auth_code = "0000000000000000";              // 认证码

    // 订阅配置
    std::vector<std::string> instruments;                    // 订阅的合约列表

    // 共享内存配置
    std::string shm_queue_name = "md_queue";                 // 共享内存队列名称
    int shm_queue_size = 10000;                              // 队列容量

    // 重连配置
    int reconnect_interval_sec = 5;                          // 重连间隔（秒）
    int max_reconnect_attempts = -1;                         // 最大重连次数（-1=无限）

    // 日志配置
    std::string log_level = "info";                          // 日志级别
    std::string log_file = "log/ctp_md_gateway.log";         // 日志文件路径
    bool log_to_console = true;                              // 是否输出到控制台

    // 性能配置
    bool enable_latency_monitor = true;                      // 启用延迟监控
    int latency_log_interval = 10000;                        // 延迟统计间隔（条数）

    // 从YAML文件加载配置
    bool LoadFromYaml(const std::string& config_file, const std::string& secret_file = "");

    // 验证配置
    bool Validate(std::string* error_msg = nullptr) const;

    // 打印配置（隐藏密码）
    void Print() const;

private:
    // 从secret文件加载账号密码
    bool LoadCredentials(const std::string& secret_file);
};

} // namespace gateway
} // namespace hft
